import requests
import time
import os
import win32print
import win32ui
import fitz  # PyMuPDF
from PIL import Image, ImageWin

# --- CONFIG ---
SERVER_IP = "10.16.10.142"
SERVER_URL = f"http://{SERVER_IP}:8080"
POLL_INTERVAL = 5
TEMP_PDF = "incoming_job.pdf"

def get_realtime_physical_printers():
    """
    Returns an EMPTY LIST [] if no physical printers are plugged in and turned on.
    """
    online_printers = []
    # Strict block list for software drivers
    BLOCK_LIST = ["MICROSOFT", "ONENOTE", "PDF", "FAX", "XPS", "SEND TO", "ROOT"]
    
    try:
        # Enumerate printers currently known to the system
        flags = win32print.PRINTER_ENUM_LOCAL | win32print.PRINTER_ENUM_CONNECTIONS
        printers = win32print.EnumPrinters(flags)
        
        for p in printers:
            printer_name = p[2]
            
            # 1. Name Filter
            if any(blocked in printer_name.upper() for blocked in BLOCK_LIST):
                continue
                
            # 2. Hardware Connectivity Check
            try:
                phandle = win32print.OpenPrinter(printer_name)
                info = win32print.GetPrinter(phandle, 2)
                win32print.ClosePrinter(phandle)

                attributes = info['Attributes']
                status = info['Status']

                # PRINTER_ATTRIBUTE_WORK_OFFLINE = 0x400
                is_offline = attributes & win32print.PRINTER_ATTRIBUTE_WORK_OFFLINE
                
                # We only add it if it is NOT offline
                if not is_offline:
                    online_printers.append(printer_name)
                    
            except Exception:
                continue
                
    except Exception as e:
        print(f"❌ Hardware Detection Error: {e}")
        
    return online_printers # Returns [] if none match

def sync_printer_list():
    """Sends the actual online list (or empty list) to the server."""
    try:
        current_online = get_realtime_physical_printers()
        
        # This will send [] if no printers are detected
        payload = {"printers": current_online}
        
        response = requests.post(
            f"{SERVER_URL}/api/partner-agent/sync-printers",
            json=payload,
            timeout=5
        )
        
        if not current_online:
            print("🔍 Status: No physical printers detected. Empty list sent to server.")
        else:
            print(f"✅ Status: Online printers synced: {current_online}")
            
    except Exception as e:
        print(f"❌ API Sync Failed: {e}")

def get_target_printer():
    """Finds the Epson L3210 ONLY if it is in our validated online list."""
    online_p = get_realtime_physical_printers()
    for name in online_p:
        if "EPSON L3210" in name.upper():
            return name
    return None

def print_pdf_internally(pdf_path, printer_name, color=True, num_copies=1):
    """
    Print PDF using Windows GDI.
    
    Args:
        pdf_path: Path to PDF file
        printer_name: Name of the printer
        color: True for color printing, False for black & white
        num_copies: Number of copies to print
    """
    try:
        doc = fitz.open(pdf_path)
        hdc = win32ui.CreateDC()
        hdc.CreatePrinterDC(printer_name)
        hdc.StartDoc("CloudPrintJob")
        
        # Print the specified number of copies
        for copy_num in range(num_copies):
            if num_copies > 1:
                print(f"  📄 Printing copy {copy_num + 1} of {num_copies}...")
            
            for page_num in range(len(doc)):
                hdc.StartPage()
                page = doc.load_page(page_num)
                
                # Render page to pixmap
                pix = page.get_pixmap(matrix=fitz.Matrix(300/72, 300/72))
                
                # Convert to PIL Image
                img = Image.frombytes("RGB", [pix.width, pix.height], pix.samples)
                
                # Convert to grayscale if black & white printing is requested
                if not color:
                    img = img.convert("L")  # Convert to grayscale
                    # Convert back to RGB for ImageWin compatibility
                    img = img.convert("RGB")
                
                # Get printer capabilities
                printable_w = hdc.GetDeviceCaps(110)  # HORZRES
                printable_h = hdc.GetDeviceCaps(111)  # VERTRES
                
                # Draw image to printer
                dib = ImageWin.Dib(img)
                dib.draw(hdc.GetHandleOutput(), (0, 0, printable_w, printable_h))
                
                hdc.EndPage()
        
        hdc.EndDoc()
        hdc.DeleteDC()
        doc.close()
        return True
    except Exception as e:
        print(f"❌ Print Error: {e}")
        return False

# --- MAIN LOOP ---
print(f"🚀 Agent running. Hardware Monitoring Active.")

while True:
    try:
        # Step 1: Tell the server exactly what is online (sends [] if none)
        sync_printer_list()

        # Step 2: Check for jobs only if our printer is actually online
        target = get_target_printer()
        
        if target:
            res = requests.get(f"{SERVER_URL}/api/partner-agent/fetch-job", timeout=5)
            
            if res.status_code == 200:
                # Read print parameters from headers
                job_filename = res.headers.get("X-File-Name")
                color_str = res.headers.get("X-Print-Color", "false").lower()
                copies_str = res.headers.get("X-Print-Copies", "1")
                
                # Parse color parameter (accepts "true", "1", "yes")
                color = color_str in ("true", "1", "yes")
                
                # Parse number of copies
                try:
                    num_copies = int(copies_str)
                    if num_copies < 1:
                        num_copies = 1
                except (ValueError, TypeError):
                    num_copies = 1
                
                if job_filename:
                    print(f"📦 Job Received: {job_filename}")
                    print(f"   Color: {'Yes' if color else 'No (B&W)'}, Copies: {num_copies}")
                    print(f"   Printing on: {target}")
                    
                    # Save PDF to temp file
                    with open(TEMP_PDF, "wb") as f:
                        f.write(res.content)
                    
                    # Print with parameters
                    if print_pdf_internally(TEMP_PDF, target, color=color, num_copies=num_copies):
                        ack_payload = {"filename": job_filename, "status": "completed"}
                        requests.post(f"{SERVER_URL}/api/partner-agent/confirm-print", json=ack_payload, timeout=5)
                        print(f"✅ Print confirmed.")
                    else:
                        print(f"❌ Print failed.")
                    
                    # Clean up temp file
                    if os.path.exists(TEMP_PDF):
                        os.remove(TEMP_PDF)
            elif res.status_code == 204:
                # No jobs available
                pass
        else:
            # If no physical printer is connected, we don't even poll for jobs
            pass
               
    except Exception as e:
        print(f"🔄 Loop error: {e}")
   
    time.sleep(POLL_INTERVAL)

