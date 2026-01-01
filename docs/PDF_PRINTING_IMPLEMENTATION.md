# PDF Direct Printing Implementation

## Overview

This document describes the comprehensive PDF printing solution that uses Windows Print Spooler API directly, with multiple fallback methods to ensure reliable PDF printing without opening Edge or any other viewer application.

## Problem Statement

Unlike Word documents which have native Windows print handlers that can print silently, PDFs require a viewer application (typically Microsoft Edge) which opens a print dialog. This implementation solves this by using the Windows Print Spooler API directly, bypassing any viewer applications.

## Solution Architecture

The implementation uses a **three-tier fallback approach**:

1. **Method 1: Windows Print Spooler API (Primary)**
   - Uses .NET RawPrinterHelper via PowerShell
   - Sends PDF file bytes directly to the print spooler
   - Works if the printer driver supports PDF natively
   - No application windows are opened

2. **Method 2: Ghostscript Conversion (Fallback)**
   - Converts PDF to PostScript using Ghostscript
   - Prints the PostScript file using the same method as Word documents
   - Requires Ghostscript to be installed on the system
   - Works with all printers that support PostScript

3. **Method 3: Direct Port/Network Share (Last Resort)**
   - For network printers, attempts to copy PDF directly to printer share
   - Only works for network printers with file shares enabled
   - Least reliable method, used only as last resort

## Implementation Details

### Method 1: Windows Print Spooler API

**Location:** `printPDFViaSpoolerAPI()`

**How it works:**
- Uses .NET P/Invoke to call Windows Print Spooler API functions:
  - `OpenPrinter()` - Opens a connection to the printer
  - `StartDocPrinter()` - Starts a print job
  - `StartPagePrinter()` - Starts a page
  - `WritePrinter()` - Sends raw bytes to the printer
  - `EndPagePrinter()` - Ends the page
  - `EndDocPrinter()` - Ends the print job
  - `ClosePrinter()` - Closes the printer connection

**Advantages:**
- Direct communication with print spooler
- No application windows opened
- Fast and efficient
- Works if printer supports PDF natively

**Limitations:**
- Requires printer driver to support PDF format
- May not work with all printers

### Method 2: Ghostscript Conversion

**Location:** `printPDFViaGhostscript()`

**How it works:**
1. Checks if Ghostscript is installed
2. Converts PDF to PostScript using Ghostscript:
   ```bash
   gswin64c.exe -dNOPAUSE -dBATCH -sDEVICE=ps2write -sOutputFile=output.ps input.pdf
   ```
3. Prints the PostScript file using `printWindowsFile()` (same method as Word documents)
4. Cleans up temporary PostScript file

**Advantages:**
- Works with all printers that support PostScript
- Reliable conversion
- No application windows opened

**Limitations:**
- Requires Ghostscript to be installed
- Adds conversion overhead
- Creates temporary files

**Ghostscript Installation:**
- Download from: https://www.ghostscript.com/download/gsdnld.html
- Install to default location: `C:\Program Files\gs\`
- The implementation auto-detects Ghostscript in common installation paths

### Method 3: Direct Port/Network Share

**Location:** `printPDFDirectToPort()`

**How it works:**
- For network printers, attempts to copy PDF file directly to printer share
- Only works if printer supports direct PDF printing and has file share enabled

**Advantages:**
- Simple file copy operation
- Works for network printers with PDF support

**Limitations:**
- Only works for network printers
- Requires printer share to be accessible
- Not all printers support this method

## Code Flow

```
printWindowsPDF()
    │
    ├─> Method 1: printPDFViaSpoolerAPI()
    │   └─> Success? → Return
    │   └─> Failed? → Try Method 2
    │
    ├─> Method 2: printPDFViaGhostscript()
    │   └─> Success? → Return
    │   └─> Failed? → Try Method 3
    │
    └─> Method 3: printPDFDirectToPort()
        └─> Success? → Return
        └─> Failed? → Return Error (all methods failed)
```

## Usage

The implementation is automatically used when printing PDF files. No changes are needed in the API calls:

```json
POST /api/test-print/print
{
  "filename": "document.pdf",
  "printer": "EPSON Printer"
}
```

The system will automatically:
1. Try Method 1 (Spooler API)
2. If that fails, try Method 2 (Ghostscript)
3. If that fails, try Method 3 (Direct Port)
4. If all fail, return an error

## Error Handling

Each method logs its attempts and failures:
- Success logs: `SUCCESS: PDF printed via [Method Name]`
- Failure logs: `Method [N] failed: [error], trying Method [N+1]...`
- Final error: `all PDF printing methods failed. Last error: [error]`

## Requirements

### System Requirements
- Windows OS (Windows 10/11 recommended)
- PowerShell 5.1 or higher
- Administrator privileges (for some printer operations)

### Optional Requirements
- **Ghostscript** (for Method 2):
  - Download: https://www.ghostscript.com/download/gsdnld.html
  - Install to default location or ensure it's in PATH
  - Version 9.50 or higher recommended

### Printer Requirements
- **Method 1**: Printer driver must support PDF format natively
- **Method 2**: Printer must support PostScript (most modern printers do)
- **Method 3**: Network printer with file share enabled (rare)

## Troubleshooting

### Method 1 Fails
- **Issue**: Printer doesn't support PDF natively
- **Solution**: Method 2 (Ghostscript) will automatically be used

### Method 2 Fails
- **Issue**: Ghostscript not installed
- **Solution**: Install Ghostscript from the link above
- **Alternative**: Method 3 might work for network printers

### All Methods Fail
- **Check**: Printer is online and accessible
- **Check**: Printer name is correct
- **Check**: User has permissions to print
- **Check**: Printer driver is installed correctly

## Performance Considerations

- **Method 1**: Fastest (direct API call, no conversion)
- **Method 2**: Slower (PDF to PostScript conversion adds ~1-2 seconds)
- **Method 3**: Fastest for network printers (simple file copy)

## Security Considerations

- All methods run with the same privileges as the application
- Temporary PostScript files are automatically cleaned up
- No user interaction required (no security dialogs)

## Future Enhancements

Possible improvements:
1. Add PDF rendering library (e.g., go-pdfium) for Go-native PDF handling
2. Cache Ghostscript conversion results for repeated prints
3. Add printer capability detection to choose the best method automatically
4. Support for Linux/Mac printing (currently Windows-only)

## Testing

To test the implementation:

1. **Test Method 1** (Spooler API):
   - Use a printer that supports PDF natively
   - Check logs for "SUCCESS: PDF printed via Windows Print Spooler API"

2. **Test Method 2** (Ghostscript):
   - Install Ghostscript
   - Use any printer (PostScript support required)
   - Check logs for "SUCCESS: PDF printed via Ghostscript conversion"

3. **Test Method 3** (Direct Port):
   - Use a network printer with file share
   - Check logs for "SUCCESS: PDF printed via direct port"

## Related Files

- `internal/api/handlers/test_print/print_handler.go` - Main implementation
- `internal/api/handlers/test_print/print_handler.go:printWindowsPDF()` - Entry point
- `internal/api/handlers/test_print/print_handler.go:printPDFViaSpoolerAPI()` - Method 1
- `internal/api/handlers/test_print/print_handler.go:printPDFViaGhostscript()` - Method 2
- `internal/api/handlers/test_print/print_handler.go:printPDFDirectToPort()` - Method 3


