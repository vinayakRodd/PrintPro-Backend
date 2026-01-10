import ctypes
import os
import sys
import requests
import time
import json
import threading
import win32print
import win32ui
import fitz  # PyMuPDF
import io
from PIL import Image, ImageWin
import tkinter as tk
from tkinter import scrolledtext, ttk, messagebox
import queue
import base64
import hashlib
from Crypto.Cipher import AES
from Crypto.Util.Padding import pad, unpad
from Crypto.Random import get_random_bytes

# 1. DPI AWARENESS - Fixes blurry or scaled-down printing in EXE
try:
    ctypes.windll.shcore.SetProcessDpiAwareness(1)
except:
    pass

try:
    import websocket
except ImportError:
    print("❌ ERROR: websocket-client package not installed!")
    print("   Please install it using: pip install websocket-client")
    exit(1)

try:
    from Crypto.Cipher import AES
    from Crypto.Util.Padding import pad, unpad
except ImportError:
    print("❌ ERROR: pycryptodome package not installed!")
    print("   Please install it using: pip install pycryptodome")
    print("   (This is lighter than cryptography and easier to install)")
    exit(1)

# Token storage file
TOKEN_FILE = os.path.join(os.path.expanduser("~"), ".print_agent", "tokens.enc")

class TokenManager:
    """Manages JWT token storage and retrieval with encryption (using pycryptodome)."""
    
    @staticmethod
    def _get_key():
        """Derive encryption key from machine hostname."""
        hostname = os.environ.get('COMPUTERNAME', 'default')
        password = (hostname + "print-agent-secret-key").encode()
        # Use PBKDF2 to derive a 32-byte key
        key = hashlib.pbkdf2_hmac('sha256', password, b'print_agent_salt_2024', 100000)
        return key[:32]  # AES-256 requires 32 bytes
    
    @staticmethod
    def save_tokens(access_token, refresh_token):
        """Save tokens to encrypted file."""
        try:
            os.makedirs(os.path.dirname(TOKEN_FILE), exist_ok=True)
            data = {
                "access_token": access_token,
                "refresh_token": refresh_token,
                "saved_at": time.time()
            }
            key = TokenManager._get_key()
            cipher = AES.new(key, AES.MODE_CBC)
            plaintext = json.dumps(data).encode()
            padded = pad(plaintext, AES.block_size)
            ciphertext = cipher.encrypt(padded)
            # Save IV + ciphertext
            encrypted_data = cipher.iv + ciphertext
            with open(TOKEN_FILE, 'wb') as file:
                file.write(encrypted_data)
            return True
        except Exception as e:
            print(f"Error saving tokens: {e}")
            return False
    
    @staticmethod
    def load_tokens():
        """Load tokens from encrypted file."""
        try:
            if not os.path.exists(TOKEN_FILE):
                return None, None
            with open(TOKEN_FILE, 'rb') as file:
                encrypted_data = file.read()
            key = TokenManager._get_key()
            iv = encrypted_data[:16]  # First 16 bytes are IV
            ciphertext = encrypted_data[16:]
            cipher = AES.new(key, AES.MODE_CBC, iv=iv)
            decrypted = unpad(cipher.decrypt(ciphertext), AES.block_size)
            data = json.loads(decrypted.decode())
            # Check if tokens are too old (7 days)
            if time.time() - data.get("saved_at", 0) > 7 * 24 * 60 * 60:
                os.remove(TOKEN_FILE)
                return None, None
            return data.get("access_token"), data.get("refresh_token")
        except Exception:
            return None, None
    
    @staticmethod
    def delete_tokens():
        """Delete stored tokens."""
        try:
            if os.path.exists(TOKEN_FILE):
                os.remove(TOKEN_FILE)
        except Exception:
            pass

class PrintAgentGUI:
    def __init__(self, root):
        self.root = root
        self.root.title("Cloud Print Agent - Multi-Printer Support (Authenticated)")
        self.root.geometry("700x600")
        
        # --- AUTHENTICATION STATE ---
        self.access_token = None
        self.refresh_token = None
        self.token_lock = threading.Lock()
        self.is_authenticated = False
        
        # --- DYNAMIC SERVER IP VARIABLE ---
        self.server_ip = tk.StringVar(value="10.213.120.142") 
        self.is_connected = False
        
        self.log_queue = queue.Queue()
        self.flip_response = queue.Queue()
        self.flip_event = threading.Event()
        self.flip_result = False

        # --- UI LAYOUT ---
        # Top frame: Server IP and Status
        config_frame = tk.Frame(root, bg="#2c3e50", pady=12)
        config_frame.pack(fill=tk.X)

        tk.Label(config_frame, text="SERVER IP:", bg="#2c3e50", fg="white", font=("Segoe UI", 9, "bold")).pack(side=tk.LEFT, padx=(20, 5))
        self.ip_entry = tk.Entry(config_frame, textvariable=self.server_ip, width=18, font=("Consolas", 10))
        self.ip_entry.pack(side=tk.LEFT, padx=5)

        self.connect_btn = tk.Button(config_frame, text="START AGENT", command=self.start_networking, bg="#27ae60", fg="white", relief=tk.FLAT, font=("Segoe UI", 9, "bold"), padx=15, state='disabled')
        self.connect_btn.pack(side=tk.LEFT, padx=10)

        self.reset_btn = tk.Button(config_frame, text="RESET", command=self.reset_agent, bg="#e74c3c", fg="white", relief=tk.FLAT, font=("Segoe UI", 9, "bold"), padx=15, state='disabled')
        self.reset_btn.pack(side=tk.LEFT, padx=5)

        self.ws_status = tk.Label(config_frame, text="● OFFLINE", bg="#2c3e50", fg="#e74c3c", font=("Segoe UI", 10, "bold"))
        self.ws_status.pack(side=tk.RIGHT, padx=20)

        # Authentication frame
        self.auth_frame = tk.Frame(root, bg="#34495e", pady=10)
        self.auth_frame.pack(fill=tk.X, padx=10, pady=5)

        auth_header = tk.Frame(self.auth_frame, bg="#34495e")
        auth_header.pack(fill=tk.X, padx=10, pady=(0, 5))

        tk.Label(auth_header, text="Authentication", bg="#34495e", fg="white", font=("Segoe UI", 10, "bold")).pack(side=tk.LEFT)

        self.clear_auth_btn = tk.Button(auth_header, text="Clear Authentication", command=self.clear_authentication, bg="#c0392b", fg="white", relief=tk.FLAT, font=("Segoe UI", 8, "bold"), padx=8, state='disabled')
        self.clear_auth_btn.pack(side=tk.RIGHT, padx=5)

        self.auth_status = tk.Label(auth_header, text="● NOT AUTHENTICATED", bg="#34495e", fg="#e74c3c", font=("Segoe UI", 9, "bold"))
        self.auth_status.pack(side=tk.RIGHT, padx=10)

        # Login row (shown when not authenticated)
        self.login_row = tk.Frame(self.auth_frame, bg="#34495e")
        self.login_row.pack(fill=tk.X, padx=10, pady=2)

        tk.Label(self.login_row, text="Email:", bg="#34495e", fg="white", font=("Segoe UI", 9)).pack(side=tk.LEFT, padx=(0, 5))
        self.email_entry = tk.Entry(self.login_row, width=20, font=("Consolas", 9))
        self.email_entry.pack(side=tk.LEFT, padx=5)

        tk.Label(self.login_row, text="Password:", bg="#34495e", fg="white", font=("Segoe UI", 9)).pack(side=tk.LEFT, padx=(10, 5))
        self.password_entry = tk.Entry(self.login_row, show="*", width=15, font=("Consolas", 9))
        self.password_entry.pack(side=tk.LEFT, padx=5)

        self.login_btn = tk.Button(self.login_row, text="Login", command=self.login, bg="#3498db", fg="white", relief=tk.FLAT, font=("Segoe UI", 9, "bold"), padx=10)
        self.login_btn.pack(side=tk.LEFT, padx=10)

        # OTP row (initially hidden)
        self.otp_row = tk.Frame(self.auth_frame, bg="#34495e")
        
        tk.Label(self.otp_row, text="OTP:", bg="#34495e", fg="white", font=("Segoe UI", 9)).pack(side=tk.LEFT, padx=(10, 5))
        self.otp_entry = tk.Entry(self.otp_row, width=15, font=("Consolas", 9))
        self.otp_entry.pack(side=tk.LEFT, padx=5)

        self.verify_otp_btn = tk.Button(self.otp_row, text="Verify OTP", command=self.verify_otp, bg="#e67e22", fg="white", relief=tk.FLAT, font=("Segoe UI", 9, "bold"), padx=10)
        self.verify_otp_btn.pack(side=tk.LEFT, padx=10)
        
        # Store email and password temporarily for OTP verification
        self.pending_email = None
        self.pending_password = None  # Stored temporarily in memory only

        # Log area
        self.log_area = scrolledtext.ScrolledText(root, state='disabled', bg="#1e1e1e", fg="#dcdcdc", font=("Consolas", 10))
        self.log_area.pack(fill=tk.BOTH, expand=True, padx=10, pady=10)

        self.root.after(100, self.process_logs)
        
        # Load saved tokens
        self.load_saved_tokens()

    def load_saved_tokens(self):
        """Load tokens from encrypted storage."""
        access_token, refresh_token = TokenManager.load_tokens()
        if access_token and refresh_token:
            with self.token_lock:
                self.access_token = access_token
                self.refresh_token = refresh_token
                self.is_authenticated = True
            self.root.after(0, lambda: self.on_authentication_success())
            self.write_log("✅ Loaded saved authentication tokens")
        else:
            self.write_log("ℹ️ No saved tokens found - please login")

    def write_log(self, message):
        self.log_queue.put(f"[{time.strftime('%H:%M:%S')}] {message}\n")

    def process_logs(self):
        while not self.log_queue.empty():
            msg = self.log_queue.get()
            self.log_area.config(state='normal')
            self.log_area.insert(tk.END, msg)
            self.log_area.see(tk.END)
            self.log_area.config(state='disabled')
        self.root.after(100, self.process_logs)

    def login(self):
        """Handle login with email and password - sends OTP if credentials are correct."""
        email = self.email_entry.get().strip()
        password = self.password_entry.get()
        
        if not email or not password:
            messagebox.showerror("Error", "Please enter email and password")
            return
        
        self.login_btn.config(state='disabled', text="Verifying...")
        self.write_log(f"🔐 Verifying credentials for {email}...")
        
        def login_thread():
            try:
                ip = self.server_ip.get()
                # Step 1: Verify email and password
                url = f"http://{ip}:8080/api/auth/login/partner"
                payload = {
                    "email": email,
                    "password": password
                }
                
                response = requests.post(url, json=payload, timeout=10)
                
                if response.status_code == 200:
                    # Credentials are correct - now send OTP
                    self.write_log("✅ Credentials verified. Sending OTP to email...")
                    
                    # Step 2: Send OTP to email
                    otp_url = f"http://{ip}:8080/api/auth/forgot-password"
                    otp_payload = {"email": email}
                    
                    otp_response = requests.post(otp_url, json=otp_payload, timeout=10)
                    
                    if otp_response.status_code == 200:
                        # OTP sent successfully - show OTP input
                        self.pending_email = email
                        self.pending_password = password  # Store temporarily for token retrieval
                        self.root.after(0, lambda: self.show_otp_input())
                        self.write_log("✅ OTP sent to your email. Please enter it below.")
                    else:
                        error_msg = otp_response.json().get("message", "Failed to send OTP")
                        self.write_log(f"❌ Failed to send OTP: {error_msg}")
                        self.root.after(0, lambda: self.login_btn.config(state='normal', text="Login"))
                else:
                    error_msg = response.json().get("message", "Login failed")
                    self.write_log(f"❌ Invalid credentials: {error_msg}")
                    self.root.after(0, lambda: self.login_btn.config(state='normal', text="Login"))
            except Exception as e:
                self.write_log(f"❌ Login error: {e}")
                self.root.after(0, lambda: self.login_btn.config(state='normal', text="Login"))
        
        threading.Thread(target=login_thread, daemon=True).start()

    def show_otp_input(self):
        """Show OTP input field and hide login button."""
        self.otp_row.pack(fill=tk.X, padx=10, pady=2)
        self.login_btn.config(state='disabled', text="OTP Sent")
        self.email_entry.config(state='disabled')
        self.password_entry.config(state='disabled')
        self.otp_entry.focus()

    def verify_otp(self):
        """Verify OTP and get tokens if correct."""
        if not self.pending_email:
            messagebox.showerror("Error", "Please login first")
            return
        
        otp = self.otp_entry.get().strip()
        if not otp:
            messagebox.showerror("Error", "Please enter OTP")
            return
        
        self.verify_otp_btn.config(state='disabled', text="Verifying...")
        self.write_log("🔐 Verifying OTP...")
        
        def verify_thread():
            try:
                ip = self.server_ip.get()
                email = self.pending_email
                
                # Step 1: Verify OTP
                verify_url = f"http://{ip}:8080/api/auth/otp/verify"
                verify_payload = {
                    "email": email,
                    "otp": otp
                }
                
                verify_response = requests.post(verify_url, json=verify_payload, timeout=10)
                
                if verify_response.status_code == 200:
                    # OTP verified - now get tokens
                    self.write_log("✅ OTP verified. Getting authentication tokens...")
                    
                    # Step 2: Get tokens by logging in again (password already verified)
                    # We need to call login again to get tokens
                    login_url = f"http://{ip}:8080/api/auth/login/partner"
                    login_payload = {
                        "email": email,
                        "password": self.pending_password  # Use stored password from memory
                    }
                    
                    token_response = requests.post(login_url, json=login_payload, timeout=10)
                    
                    if token_response.status_code == 200:
                        data = token_response.json()
                        access_token = data.get("access_token")
                        refresh_token = data.get("refresh_token")
                        
                        if access_token and refresh_token:
                            with self.token_lock:
                                self.access_token = access_token
                                self.refresh_token = refresh_token
                                self.is_authenticated = True
                            
                            TokenManager.save_tokens(access_token, refresh_token)
                            
                            self.root.after(0, lambda: self.on_otp_success())
                            self.write_log("✅ Authentication successful! Tokens saved.")
                            self.write_log("✅ You can now start the agent.")
                            self.write_log("ℹ️ Login inputs hidden. Use 'Clear Authentication' to login again.")
                        else:
                            self.write_log("❌ No tokens received from server")
                            self.root.after(0, lambda: self.verify_otp_btn.config(state='normal', text="Verify OTP"))
                    else:
                        error_msg = token_response.json().get("message", "Failed to get tokens")
                        self.write_log(f"❌ Failed to get tokens: {error_msg}")
                        self.root.after(0, lambda: self.verify_otp_btn.config(state='normal', text="Verify OTP"))
                else:
                    error_msg = verify_response.json().get("message", "Invalid OTP")
                    self.write_log(f"❌ OTP verification failed: {error_msg}")
                    self.root.after(0, lambda: self.verify_otp_btn.config(state='normal', text="Verify OTP"))
            except Exception as e:
                self.write_log(f"❌ OTP verification error: {e}")
                self.root.after(0, lambda: self.verify_otp_btn.config(state='normal', text="Verify OTP"))
        
        threading.Thread(target=verify_thread, daemon=True).start()

    def on_otp_success(self):
        """Called after successful OTP verification."""
        self.on_authentication_success()
        # Clear temporary credentials from memory
        self.pending_email = None
        self.pending_password = None  # Clear password from memory

    def on_authentication_success(self):
        """Called after successful authentication - hide login inputs and enable agent."""
        self.auth_status.config(text="● AUTHENTICATED", fg="#2ecc71")
        # Hide login and OTP inputs
        self.login_row.pack_forget()
        self.otp_row.pack_forget()
        # Enable clear auth button and start agent button
        self.clear_auth_btn.config(state='normal')
        self.connect_btn.config(state='normal')
        # Clear sensitive data
        self.password_entry.delete(0, tk.END)
        self.otp_entry.delete(0, tk.END)

    def clear_authentication(self, ask_confirmation=True):
        """Clear authentication tokens and show login inputs again."""
        if ask_confirmation:
            if not messagebox.askyesno("Clear Authentication", "Are you sure you want to clear authentication? You will need to login again."):
                return
        
        # Clear tokens
        with self.token_lock:
            self.access_token = None
            self.refresh_token = None
            self.is_authenticated = False
        
        TokenManager.delete_tokens()
        
        # Reset UI
        self.auth_status.config(text="● NOT AUTHENTICATED", fg="#e74c3c")
        self.clear_auth_btn.config(state='disabled')
        self.connect_btn.config(state='disabled')
        
        # Show login inputs again
        self.login_row.pack(fill=tk.X, padx=10, pady=2)
        self.otp_row.pack_forget()
        self.login_btn.config(state='normal', text="Login")
        self.email_entry.config(state='normal')
        self.password_entry.config(state='normal')
        self.email_entry.delete(0, tk.END)
        self.password_entry.delete(0, tk.END)
        self.otp_entry.delete(0, tk.END)
        
        # Stop agent if running
        if self.is_connected:
            self.is_connected = False
            self.ip_entry.config(state='normal')
            self.connect_btn.config(state='disabled', bg="#27ae60", text="START AGENT")
            self.reset_btn.config(state='disabled')
            self.ws_status.config(text="● OFFLINE", fg="#e74c3c")
        
        if ask_confirmation:
            self.write_log("✅ Authentication cleared. Please login again.")
        else:
            self.write_log("⚠️ Authentication expired. Please login again.")

    def refresh_access_token(self):
        """Refresh access token using refresh token."""
        with self.token_lock:
            refresh_token = self.refresh_token
        
        if not refresh_token:
            return False
        
        try:
            ip = self.server_ip.get()
            url = f"http://{ip}:8080/api/auth/refresh"
            headers = {
                "Authorization": f"Bearer {refresh_token}"
            }
            
            response = requests.post(url, headers=headers, timeout=10)
            
            if response.status_code == 200:
                data = response.json()
                new_access_token = data.get("access_token")
                
                if new_access_token:
                    with self.token_lock:
                        self.access_token = new_access_token
                    TokenManager.save_tokens(new_access_token, refresh_token)
                    self.write_log("✅ Access token refreshed")
                    return True
            else:
                self.write_log("⚠️ Token refresh failed - please login again")
                with self.token_lock:
                    self.access_token = None
                    self.refresh_token = None
                    self.is_authenticated = False
                TokenManager.delete_tokens()
                self.root.after(0, lambda: self.clear_authentication(ask_confirmation=False))
                return False
        except Exception as e:
            self.write_log(f"⚠️ Token refresh error: {e}")
            return False

    def get_auth_headers(self):
        """Get authentication headers for requests."""
        with self.token_lock:
            access_token = self.access_token
        
        if access_token:
            return {"Authorization": f"Bearer {access_token}"}
        return {}

    def make_authenticated_request(self, method, url, **kwargs):
        """Make an authenticated request with auto-refresh on 401."""
        headers = kwargs.get("headers", {})
        headers.update(self.get_auth_headers())
        kwargs["headers"] = headers
        
        response = requests.request(method, url, **kwargs)
        
        # Auto-refresh on 401
        if response.status_code == 401:
            self.write_log("⚠️ Access token expired, refreshing...")
            if self.refresh_access_token():
                # Retry request with new token
                headers.update(self.get_auth_headers())
                kwargs["headers"] = headers
                response = requests.request(method, url, **kwargs)
            else:
                self.write_log("❌ Token refresh failed - please login again")
        
        return response

    def start_networking(self):
        if self.is_connected:
            return
        
        # Check authentication - must be authenticated (OTP verified)
        if not self.is_authenticated:
            messagebox.showerror("Error", "Please login and verify OTP before starting the agent")
            return
        
        self.is_connected = True
        self.ip_entry.config(state='disabled')
        self.connect_btn.config(state='disabled', bg="#7f8c8d", text="RUNNING...")
        self.reset_btn.config(state='normal')
        self.write_log(f"🚀 Initializing connection using IP: {self.server_ip.get()}...")
        threading.Thread(target=self.ws_thread, daemon=True).start()
        threading.Thread(target=self.printer_sync_thread, daemon=True).start()

    def reset_agent(self):
        """Reset/stop the agent and allow restart."""
        if not self.is_connected:
            return
        
        if messagebox.askyesno("Reset Agent", "Are you sure you want to reset the agent? This will stop all connections."):
            self.is_connected = False
            self.ip_entry.config(state='normal')
            self.connect_btn.config(state='normal', bg="#27ae60", text="START AGENT")
            self.reset_btn.config(state='disabled')
            self.ws_status.config(text="● OFFLINE", fg="#e74c3c")
            self.write_log("🛑 Agent stopped. You can restart it now.")

    def trigger_flip_popup(self):
        """Show flip confirmation popup - Uses messagebox for EXE compatibility."""
        self.flip_event.clear()
        self.flip_result = False
        
        self.root.lift()
        self.root.focus_force()
        self.root.update()
        
        result = messagebox.askyesno(
            title="⚠️ FLIP PAPER - Manual Duplex Required",
            message="FRONT SIDE PRINTED ✅\n\n" +
                   "Please follow these steps:\n\n" +
                   "1. Remove printed pages from output tray\n" +
                   "2. Flip them over (turn upside down)\n" +
                   "3. Place them back in the input tray\n\n" +
                   "Click YES to continue printing back side\n" +
                   "Click NO to cancel",
            icon="question"
        )
        
        self.flip_result = result
        self.flip_event.set()
        
        if result:
            self.write_log("✅ User confirmed flip - continuing with back pages...")
        else:
            self.write_log("⚠️ User cancelled flip - skipping back pages")

    def wait_for_flip_confirmation(self, timeout=300):
        """Wait for user to confirm flip - called from print thread."""
        self.flip_event.clear()
        self.flip_result = False
        
        self.root.after(0, self.trigger_flip_popup)
        
        if self.flip_event.wait(timeout=timeout):
            return self.flip_result
        else:
            self.write_log("⚠️ Flip confirmation timeout - skipping back pages")
            return False

    def printer_sync_thread(self):
        """Sync printer list periodically with authentication."""
        while self.is_connected:
            try:
                ip = self.server_ip.get()
                printers = self.get_realtime_physical_printers()
                payload = {"printers": printers}
                url = f"http://{ip}:8080/api/partner-agent/sync-printers"
                
                response = self.make_authenticated_request("POST", url, json=payload, timeout=5)
                
                if response.status_code == 200:
                    if printers:
                        self.write_log(f"✅ Printers synced: {printers}")
                    else:
                        self.write_log("🔍 No physical printers detected")
                else:
                    self.write_log(f"⚠️ Sync error: {response.status_code}")
            except Exception as e:
                self.write_log(f"⚠️ Sync error: {e}")
            time.sleep(5)

    def get_realtime_physical_printers(self):
        """Returns list of online physical printers."""
        online_printers = []
        BLOCK_LIST = ["MICROSOFT", "ONENOTE", "PDF", "FAX", "XPS", "SEND TO", "ROOT"]
        try:
            flags = win32print.PRINTER_ENUM_LOCAL | win32print.PRINTER_ENUM_CONNECTIONS
            printers = win32print.EnumPrinters(flags)
            for p in printers:
                printer_name = p[2]
                if any(blocked in printer_name.upper() for blocked in BLOCK_LIST):
                    continue
                try:
                    phandle = win32print.OpenPrinter(printer_name)
                    info = win32print.GetPrinter(phandle, 2)
                    win32print.ClosePrinter(phandle)
                    attributes = info['Attributes']
                    is_offline = attributes & win32print.PRINTER_ATTRIBUTE_WORK_OFFLINE
                    if not is_offline:
                        online_printers.append(printer_name)
                except Exception:
                    continue
        except Exception as e:
            self.write_log(f"❌ Hardware Detection Error: {e}")
        return online_printers

    def find_printer(self):
        """Find Epson L3210 printer or return default."""
        printers = self.get_realtime_physical_printers()
        for name in printers:
            if "L3210" in name.upper():
                return name
        return win32print.GetDefaultPrinter()

    def ws_thread(self):
        """WebSocket connection thread."""
        current_ip = self.server_ip.get()
        ws_url = f"ws://{current_ip}:8080/ws/printer1"
        
        while self.is_connected:
            try:
                ws = websocket.WebSocketApp(
                    ws_url,
                    on_message=self.on_ws_message,
                    on_open=lambda w: self.root.after(0, lambda: self.ws_status.config(text="● ONLINE", fg="#2ecc71")),
                    on_close=lambda w,s,m: self.root.after(0, lambda: self.ws_status.config(text="● OFFLINE", fg="#e74c3c")),
                    on_error=lambda w,e: self.write_log(f"⚠️ WebSocket Error: {e}")
                )
                ws.run_forever()
            except Exception as e:
                self.write_log(f"⚠️ Connection Error: {e}")
            if self.is_connected:
                time.sleep(5)

    def on_ws_message(self, ws, message):
        try:
            data = json.loads(message)
            if data.get("action") == "print_job_available":
                ip = self.server_ip.get()
                self.write_log(f"📩 Job detected. Downloading from {ip}...")
                threading.Thread(target=self.handle_print_job, args=(ip,), daemon=True).start()
        except Exception as e:
            self.write_log(f"⚠️ Job Error: {e}")
            import traceback
            traceback.print_exc()
    
    def handle_print_job(self, ip):
        """Handle print job in separate thread to keep GUI responsive."""
        try:
            url = f"http://{ip}:8080/api/partner-agent/fetch-job"
            response = self.make_authenticated_request("GET", url, timeout=20)
            
            if response.status_code == 200:
                # Parse all print parameters from headers
                job_filename = response.headers.get("X-File-Name")
                printer_name = response.headers.get("X-Print-Printer-Name")
                color_str = response.headers.get("X-Print-Color", "false").lower()
                copies_str = response.headers.get("X-Print-Copies", "1")
                start_page_str = response.headers.get("X-Print-Start-Page")
                end_page_str = response.headers.get("X-Print-End-Page")
                page_filter_type = response.headers.get("X-Print-Page-Filter", "all").lower()
                individual_color_pages_str = response.headers.get("X-Print-Individual-Color-Pages")
                skip_pages_str = response.headers.get("X-Print-Skip-Pages")
                back_to_back_str = response.headers.get("X-Print-Back-To-Back", "false").lower()
                
                # Parse parameters
                color = color_str in ("true", "1", "yes")
                try:
                    num_copies = int(copies_str)
                    if num_copies < 1:
                        num_copies = 1
                except (ValueError, TypeError):
                    num_copies = 1
                
                # Parse page range
                start_page = None
                end_page = None
                if start_page_str:
                    try:
                        start_page = int(start_page_str)
                        if start_page < 1:
                            start_page = None
                    except (ValueError, TypeError):
                        start_page = None
                if end_page_str:
                    try:
                        end_page = int(end_page_str)
                        if end_page < 1:
                            end_page = None
                    except (ValueError, TypeError):
                        end_page = None
                
                # Normalize page_filter_type
                if page_filter_type not in ("all", "odd", "even"):
                    page_filter_type = "all"

                # Parse individual_color_pages (JSON array)
                individual_color_pages = None
                if individual_color_pages_str:
                    try:
                        individual_color_pages = json.loads(individual_color_pages_str)
                        if not isinstance(individual_color_pages, list) or len(individual_color_pages) == 0:
                            individual_color_pages = None
                    except (ValueError, TypeError, json.JSONDecodeError):
                        individual_color_pages = None

                # Parse skip_pages (JSON array)
                skip_pages = None
                if skip_pages_str:
                    try:
                        skip_pages = json.loads(skip_pages_str)
                        if not isinstance(skip_pages, list) or len(skip_pages) == 0:
                            skip_pages = None
                    except (ValueError, TypeError, json.JSONDecodeError):
                        skip_pages = None
                
                back_to_back = back_to_back_str in ("true", "1", "yes")
                
                self.write_log(f"📦 Job: {job_filename}")
                if printer_name:
                    self.write_log(f"   Printer: {printer_name} (server-selected)")
                self.write_log(f"   Color: {'Yes' if color else 'No (B&W)'}, Copies: {num_copies}")
                if start_page is not None or end_page is not None:
                    self.write_log(f"   Page Range: {start_page or 'start'} to {end_page or 'end'}")
                if page_filter_type != "all":
                    self.write_log(f"   Page Filter: {page_filter_type}")
                if individual_color_pages:
                    self.write_log(f"   Individual Color Pages: {individual_color_pages}")
                if skip_pages:
                    self.write_log(f"   Skip Pages: {skip_pages}")
                if back_to_back:
                    self.write_log(f"   Back-to-Back: Yes (Duplex)")
                
                # Print the job
                if self.print_job(response.content, color, num_copies, start_page, end_page, page_filter_type, 
                                 individual_color_pages, skip_pages, back_to_back, printer_name=printer_name):
                    # Confirm print with authentication
                    confirm_url = f"http://{ip}:8080/api/partner-agent/confirm-print"
                    confirm_payload = {"filename": job_filename, "status": "completed"}
                    self.make_authenticated_request("POST", confirm_url, json=confirm_payload, timeout=5)
                    self.write_log("✅ Job Confirmed.")
                else:
                    self.write_log("❌ Print Failed.")
            elif response.status_code == 204:
                self.write_log("ℹ️ No jobs available")
            elif response.status_code == 401:
                self.write_log("❌ Authentication failed - please login again")
        except Exception as e:
            self.write_log(f"⚠️ Job Error: {e}")
            import traceback
            traceback.print_exc()

    def print_job(self, pdf_data, color, num_copies=1, start_page=None, end_page=None, 
                  page_filter_type="all", individual_color_pages=None, skip_pages=None, back_to_back=False, printer_name=None):
        """
        Print PDF with multi-printer support.
        """
        try:
            doc = fitz.open(stream=pdf_data, filetype="pdf")
            if not printer_name:
                printer_name = self.find_printer()
            else:
                available_printers = self.get_realtime_physical_printers()
                if printer_name not in available_printers:
                    self.write_log(f"⚠️ Server-selected printer '{printer_name}' not available, using fallback")
                    printer_name = self.find_printer()
                else:
                    self.write_log(f"✅ Using server-selected printer: {printer_name}")
            
            if not printer_name:
                self.write_log("❌ No printer found!")
                return False
            
            hdc = win32ui.CreateDC()
            hdc.CreatePrinterDC(printer_name)
            
            total_pages = len(doc)
            
            # Determine page range
            page_start = 0
            page_end = total_pages
            
            if start_page is not None:
                page_start = max(0, min(start_page - 1, total_pages - 1))
            if end_page is not None:
                page_end = min(end_page, total_pages)
            
            if page_start >= page_end:
                self.write_log(f"⚠️ Invalid page range, printing all pages")
                page_start = 0
                page_end = total_pages
            
            # Build page list
            pages_to_print = list(range(page_start, page_end))
            
            # Apply odd/even filter
            if page_filter_type == "odd":
                pages_to_print = [p for p in pages_to_print if (p + 1) % 2 == 1]
            elif page_filter_type == "even":
                pages_to_print = [p for p in pages_to_print if (p + 1) % 2 == 0]
            
            # Apply skip_pages filter
            if skip_pages is not None and len(skip_pages) > 0:
                skip_pages_0_indexed = {p - 1 for p in skip_pages}
                pages_to_print = [p for p in pages_to_print if p not in skip_pages_0_indexed]
            
            if not pages_to_print:
                self.write_log("⚠️ No pages to print after filtering")
                doc.close()
                hdc.DeleteDC()
                return False
            
            # Print multiple copies
            for copy_num in range(num_copies):
                if num_copies > 1:
                    self.write_log(f"📄 Printing copy {copy_num + 1} of {num_copies}...")
                
                if back_to_back and len(pages_to_print) > 1:
                    # Manual duplex
                    odd_pages = [p for p in pages_to_print if (p + 1) % 2 == 1]
                    even_pages = [p for p in pages_to_print if (p + 1) % 2 == 0]
                    
                    if odd_pages:
                        self.write_log("📄 Printing Front Pages (Odds)...")
                        hdc.StartDoc("Cloud_Front")
                        self.render_pages(hdc, doc, odd_pages, color, individual_color_pages)
                        hdc.EndDoc()
                        self.write_log("✅ Front pages printed successfully!")
                    
                    if even_pages:
                        hdc.DeleteDC()
                        self.write_log("⏳ Front pages complete. Waiting for paper flip...")
                        self.write_log("📋 Please flip the pages and click CONTINUE in the popup window.")
                        
                        if self.wait_for_flip_confirmation():
                            self.write_log("📄 User confirmed flip - Printing Back Pages (Evens)...")
                            hdc = win32ui.CreateDC()
                            hdc.CreatePrinterDC(printer_name)
                            hdc.StartDoc("Cloud_Back")
                            self.render_pages(hdc, doc, even_pages, color, individual_color_pages)
                            hdc.EndDoc()
                            hdc.DeleteDC()
                            self.write_log("✅ Back pages printed successfully!")
                        else:
                            self.write_log("⚠️ Flip cancelled or timeout - skipping back pages")
                else:
                    # Simplex printing
                    hdc.StartDoc("Cloud_Single")
                    self.render_pages(hdc, doc, pages_to_print, color, individual_color_pages)
                    hdc.EndDoc()
                    hdc.DeleteDC()

            doc.close()
            return True
        except Exception as e:
            self.write_log(f"❌ Printer Error: {e}")
            import traceback
            traceback.print_exc()
            return False

    def render_pages(self, hdc, doc, indices, color, individual_color_pages=None):
        """Render pages with color handling."""
        for idx in indices:
            if idx >= len(doc):
                continue
            
            hdc.StartPage()
            page = doc.load_page(idx)
            pix = page.get_pixmap(matrix=fitz.Matrix(300/72, 300/72))
            img = Image.frombytes("RGB", [pix.width, pix.height], pix.samples)
            
            # Determine if this page should be printed in color
            page_number_1_indexed = idx + 1
            page_should_be_color = color
            
            if individual_color_pages is not None and len(individual_color_pages) > 0:
                if page_number_1_indexed in individual_color_pages:
                    page_should_be_color = True
                else:
                    page_should_be_color = False
            
            # Convert to grayscale if black & white printing is requested
            if not page_should_be_color:
                img = img.convert("L").convert("RGB")
            
            pw, ph = hdc.GetDeviceCaps(110), hdc.GetDeviceCaps(111)
            dib = ImageWin.Dib(img)
            dib.draw(hdc.GetHandleOutput(), (0, 0, pw, ph))
            hdc.EndPage()

if __name__ == "__main__":
    root = tk.Tk()
    app = PrintAgentGUI(root)
    root.mainloop()
