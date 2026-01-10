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

class PrintAgentGUI:
    def __init__(self, root):
        self.root = root
        self.root.title("Cloud Print Agent - Multi-Printer Support")
        self.root.geometry("700x550")
        
        # --- DYNAMIC SERVER IP VARIABLE ---
        self.server_ip = tk.StringVar(value="10.213.120.142") 
        self.is_connected = False
        
        self.log_queue = queue.Queue()
        self.flip_response = queue.Queue()
        self.flip_event = threading.Event()  # Use Event instead of Queue for blocking wait
        self.flip_result = False  # Store the result

        # --- UI LAYOUT ---
        config_frame = tk.Frame(root, bg="#2c3e50", pady=12)
        config_frame.pack(fill=tk.X)

        tk.Label(config_frame, text="SERVER IP:", bg="#2c3e50", fg="white", font=("Segoe UI", 9, "bold")).pack(side=tk.LEFT, padx=(20, 5))
        self.ip_entry = tk.Entry(config_frame, textvariable=self.server_ip, width=18, font=("Consolas", 10))
        self.ip_entry.pack(side=tk.LEFT, padx=5)

        self.connect_btn = tk.Button(config_frame, text="START AGENT", command=self.start_networking, bg="#27ae60", fg="white", relief=tk.FLAT, font=("Segoe UI", 9, "bold"), padx=15)
        self.connect_btn.pack(side=tk.LEFT, padx=10)

        self.ws_status = tk.Label(config_frame, text="● OFFLINE", bg="#2c3e50", fg="#e74c3c", font=("Segoe UI", 10, "bold"))
        self.ws_status.pack(side=tk.RIGHT, padx=20)

        self.log_area = scrolledtext.ScrolledText(root, state='disabled', bg="#1e1e1e", fg="#dcdcdc", font=("Consolas", 10))
        self.log_area.pack(fill=tk.BOTH, expand=True, padx=10, pady=10)

        self.root.after(100, self.process_logs)

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

    def start_networking(self):
        if self.is_connected: return
        self.is_connected = True
        self.ip_entry.config(state='disabled')
        self.connect_btn.config(state='disabled', bg="#7f8c8d", text="RUNNING...")
        self.write_log(f"🚀 Initializing connection using IP: {self.server_ip.get()}...")
        threading.Thread(target=self.ws_thread, daemon=True).start()
        threading.Thread(target=self.printer_sync_thread, daemon=True).start()

    def trigger_flip_popup(self):
        """Show flip confirmation popup - Uses messagebox for EXE compatibility."""
        # This function runs in GUI thread - safe to use GUI operations
        # Reset the event and result
        self.flip_event.clear()
        self.flip_result = False
        
        # CRITICAL: Use messagebox.askyesno - it's a native Windows dialog
        # This is MUCH more reliable in EXE mode than custom Toplevel windows
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
        
        # Set result and signal the waiting thread
        self.flip_result = result
        self.flip_event.set()
        
        if result:
            self.write_log("✅ User confirmed flip - continuing with back pages...")
        else:
            self.write_log("⚠️ User cancelled flip - skipping back pages")

    def wait_for_flip_confirmation(self, timeout=300):
        """Wait for user to confirm flip - called from print thread."""
        # Reset event before showing popup
        self.flip_event.clear()
        self.flip_result = False
        
        # CRITICAL: Schedule popup in GUI thread using after()
        # Use after(0) to ensure it runs in the next event loop cycle
        self.root.after(0, self.trigger_flip_popup)
        
        # CRITICAL: Don't call update_idletasks() from non-GUI thread in EXE mode!
        # Just wait for the event - the GUI thread will handle the popup
        
        # Wait for event (with timeout) - this blocks the print thread, NOT the GUI thread
        # The GUI thread continues processing events independently
        if self.flip_event.wait(timeout=timeout):
            return self.flip_result
        else:
            self.write_log("⚠️ Flip confirmation timeout - skipping back pages")
            return False

    def printer_sync_thread(self):
        """Sync printer list periodically."""
        while self.is_connected:
            try:
                ip = self.server_ip.get()
                printers = self.get_realtime_physical_printers()
                payload = {"printers": printers}
                requests.post(f"http://{ip}:8080/api/partner-agent/sync-printers", json=payload, timeout=5)
                if printers:
                    self.write_log(f"✅ Printers synced: {printers}")
                else:
                    self.write_log("🔍 No physical printers detected")
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

    # --- NETWORKING (Uses dynamic IP variable) ---
    def ws_thread(self):
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
                
                # CRITICAL: Run print job in separate thread to prevent blocking
                threading.Thread(target=self.handle_print_job, args=(ip,), daemon=True).start()
        except Exception as e:
            self.write_log(f"⚠️ Job Error: {e}")
            import traceback
            traceback.print_exc()
    
    def handle_print_job(self, ip):
        """Handle print job in separate thread to keep GUI responsive."""
        try:
            res = requests.get(f"http://{ip}:8080/api/partner-agent/fetch-job", timeout=20)
            if res.status_code == 200:
                # Parse all print parameters from headers
                job_filename = res.headers.get("X-File-Name")
                # MULTI-PRINTER SUPPORT: Get printer name from server
                printer_name = res.headers.get("X-Print-Printer-Name")  # Server-selected printer
                color_str = res.headers.get("X-Print-Color", "false").lower()
                copies_str = res.headers.get("X-Print-Copies", "1")
                start_page_str = res.headers.get("X-Print-Start-Page")
                end_page_str = res.headers.get("X-Print-End-Page")
                page_filter_type = res.headers.get("X-Print-Page-Filter", "all").lower()
                individual_color_pages_str = res.headers.get("X-Print-Individual-Color-Pages")
                skip_pages_str = res.headers.get("X-Print-Skip-Pages")
                back_to_back_str = res.headers.get("X-Print-Back-To-Back", "false").lower()
                
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
                
                # MULTI-PRINTER SUPPORT: Pass printer name to print_job
                if self.print_job(res.content, color, num_copies, start_page, end_page, page_filter_type, 
                                 individual_color_pages, skip_pages, back_to_back, printer_name=printer_name):
                    requests.post(f"http://{ip}:8080/api/partner-agent/confirm-print", 
                                 json={"filename": job_filename, "status": "completed"}, timeout=5)
                    self.write_log("✅ Job Confirmed.")
                else:
                    self.write_log("❌ Print Failed.")
            elif res.status_code == 204:
                self.write_log("ℹ️ No jobs available")
        except Exception as e:
            self.write_log(f"⚠️ Job Error: {e}")
            import traceback
            traceback.print_exc()

    # --- PRINTING LOGIC (Enhanced with all page options) ---
    # MULTI-PRINTER SUPPORT: Accept printer_name parameter from server
    def print_job(self, pdf_data, color, num_copies=1, start_page=None, end_page=None, 
                  page_filter_type="all", individual_color_pages=None, skip_pages=None, back_to_back=False, printer_name=None):
        """
        Print PDF with multi-printer support.
        
        Args:
            pdf_data: PDF file content (bytes)
            color: Global color setting (bool)
            num_copies: Number of copies to print
            start_page: Starting page number (1-indexed, None = from start)
            end_page: Ending page number (1-indexed, None = to end)
            page_filter_type: "all", "odd", or "even"
            individual_color_pages: List of page numbers (1-indexed) to print in color
            skip_pages: List of page numbers (1-indexed) to skip
            back_to_back: True for duplex printing (manual flip)
            printer_name: Server-selected printer name (None = use default)
        """
        try:
            doc = fitz.open(stream=pdf_data, filetype="pdf")
            # MULTI-PRINTER SUPPORT: Use server-selected printer, or fallback to find_printer()
            if not printer_name:
                printer_name = self.find_printer()
            else:
                # Verify the specified printer exists and is available
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
            
            # Determine page range (1-indexed to 0-indexed conversion)
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
                    # Manual duplex: Print odd pages first, then even pages
                    odd_pages = [p for p in pages_to_print if (p + 1) % 2 == 1]
                    even_pages = [p for p in pages_to_print if (p + 1) % 2 == 0]
                    
                    if odd_pages:
                        self.write_log("📄 Printing Front Pages (Odds)...")
                        hdc.StartDoc("Cloud_Front")
                        self.render_pages(hdc, doc, odd_pages, color, individual_color_pages)
                        hdc.EndDoc()
                        self.write_log("✅ Front pages printed successfully!")
                    
                    if even_pages:
                        # IMPORTANT: Close printer handle BEFORE showing popup to prevent GUI blocking
                        hdc.DeleteDC()
                        
                        # Wait for user to flip paper - popup will appear NOW
                        self.write_log("⏳ Front pages complete. Waiting for paper flip...")
                        self.write_log("📋 Please flip the pages and click CONTINUE in the popup window.")
                        
                        # Wait for user confirmation (popup appears here)
                        if self.wait_for_flip_confirmation():
                            # User confirmed - recreate printer handle and print back pages
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
        """
        Render pages with color handling.
        
        Args:
            hdc: Windows device context
            doc: PyMuPDF document
            indices: List of page indices (0-indexed) to render
            color: Global color setting (bool)
            individual_color_pages: List of page numbers (1-indexed) to print in color
        """
        for idx in indices:
            if idx >= len(doc):
                continue
            
            hdc.StartPage()
            page = doc.load_page(idx)
            pix = page.get_pixmap(matrix=fitz.Matrix(300/72, 300/72))
            img = Image.frombytes("RGB", [pix.width, pix.height], pix.samples)
            
            # Determine if this page should be printed in color
            page_number_1_indexed = idx + 1
            page_should_be_color = color  # Default to global color setting
            
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
