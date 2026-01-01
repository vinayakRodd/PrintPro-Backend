# End-to-End Printing Flow Documentation

This document describes the complete flow of printing from the moment a user uploads a file to when the printer actually prints the PDF.

## Overview

The printing system follows a multi-stage workflow with clear separation of responsibilities:
- **Customer**: Uploads PDF files
- **Partner**: Reviews and queues files for printing
- **Partner Agent (Python Script)**: Fetches and prints files
- **Backend Server**: Manages file lifecycle and coordination

## Directory Structure

```
test-print/
├── ready/          # Files ready to be queued for printing
├── processing/     # Files queued and being processed by agent
└── archived/      # Files already sent to agent (completed)
```

## Complete Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                    END-TO-END PRINTING FLOW                     │
└─────────────────────────────────────────────────────────────────┘

1. CUSTOMER UPLOADS FILE
   └─> POST /api/test-print/upload
       └─> File stored in: test-print/ready/
       └─> Status: "ready"
       └─> Frontend shows: "Print" button

2. PARTNER REVIEWS FILES
   └─> GET /api/test-print/list
       └─> Returns files from ready/ and processing/ folders
       └─> Shows status: "ready" or "processing"

3. PARTNER REQUESTS PRINT
   └─> POST /api/test-print/print
       └─> File: ready/ → processing/
       └─> Status: "processing"
       └─> Response: "Print queued"

4. PARTNER AGENT FETCHES FILE
   └─> GET /api/partner-agent/fetch-job
       └─> Fetches from: processing/
       └─> Sends PDF to agent
       └─> File: processing/ → archived/
       └─> File no longer shown in frontend

5. PARTNER AGENT PRINTS PDF
   └─> Python script receives PDF
       └─> Prints using system printer
       └─> Confirms completion

6. PARTNER AGENT CONFIRMS
   └─> POST /api/partner-agent/confirm
       └─> File already in archived/ (moved during fetch)
       └─> Cleanup and logging
```

## Detailed Step-by-Step Flow

### Step 1: Customer Uploads File

**Endpoint:** `POST /api/test-print/upload`

**Request:**
- Method: `POST`
- Content-Type: `multipart/form-data`
- Body: File in field named `file`
- Authentication: Required (Customer role)

**Process:**
1. Customer uploads PDF file via frontend
2. Backend validates file (PDF only, max 10MB)
3. File is saved to `test-print/ready/` directory
4. Filename format: `{timestamp}_{userID}_{originalname}.pdf`

**Response:**
```json
{
  "success": true,
  "message": "File uploaded successfully",
  "filename": "20260101_164536_12_testing print.pdf",
  "size": 12345
}
```

**File Location:** `test-print/ready/20260101_164536_12_testing print.pdf`

**Status:** `"ready"` - File is ready to be queued for printing

---

### Step 2: Partner Views Available Files

**Endpoint:** `GET /api/test-print/list`

**Request:**
- Method: `GET`
- Authentication: Required (Partner role)

**Process:**
1. Backend reads files from `ready/` and `processing/` folders
2. Returns list with status for each file
3. Archived files are NOT shown (already sent to agent)

**Response:**
```json
{
  "success": true,
  "files": [
    {
      "filename": "20260101_164536_12_testing print.pdf",
      "size": 12345,
      "modified_at": "2026-01-01T16:45:36Z",
      "status": "ready"
    },
    {
      "filename": "another_file.pdf",
      "size": 23456,
      "modified_at": "2026-01-01T17:00:00Z",
      "status": "processing"
    }
  ],
  "count": 2,
  "message": "PDFs from ready and processing folders"
}
```

**Status Values:**
- `"ready"` - File in ready folder (show "Print" button)
- `"processing"` - File in processing folder (show "Processing" status)

---

### Step 3: Partner Requests Print

**Endpoint:** `POST /api/test-print/print`

**Request:**
```json
{
  "filename": "20260101_164536_12_testing print.pdf",
  "printer": "EPSON L3210 Series"
}
```

**Process:**
1. Backend looks for file in `test-print/ready/` folder
2. Validates file exists and is a PDF
3. Moves file from `ready/` → `processing/` folder
4. File is now queued for the partner agent

**Response:**
```json
{
  "success": true,
  "message": "Print queued",
  "filename": "20260101_164536_12_testing print.pdf",
  "printer": "EPSON L3210 Series",
  "status": "processing"
}
```

**File Location:** `test-print/processing/20260101_164536_12_testing print.pdf`

**Status:** `"processing"` - File is queued and waiting for agent

**Frontend Display:** Shows "Processing" status/button

---

### Step 4: Partner Agent Fetches File

**Endpoint:** `GET /api/partner-agent/fetch-job`

**Request:**
- Method: `GET`
- Authentication: Not required (agent authenticates separately)
- Python script pings this endpoint periodically

**Process:**
1. Backend checks `test-print/processing/` folder
2. Finds oldest PDF file (FIFO queue)
3. Streams PDF file to agent
4. **Immediately moves file to `archived/` folder** after sending
5. File is no longer visible in frontend list

**Response Headers:**
```
X-Job-Filename: 20260101_164536_12_testing print.pdf
Content-Type: application/pdf
Content-Length: 12345
```

**Response Body:** PDF file binary data

**File Location After Fetch:**
- `test-print/archived/20260101_164536_12_testing print.pdf`
- File is removed from processing folder
- File is NOT shown in frontend anymore

**If No Files Available:**
- Status: `204 No Content`
- Agent should wait and retry

---

### Step 5: Partner Agent Prints PDF

**Process (Python Script):**
1. Python script receives PDF from `/api/partner-agent/fetch-job`
2. Saves PDF to local temporary location
3. Uses system printing command to print PDF
4. Prints to the specified printer (if provided)
5. Waits for print job to complete

**Example Python Code:**
```python
response = requests.get(f"{SERVER_URL}/api/partner-agent/fetch-job")
if response.status_code == 200:
    filename = response.headers.get('X-Job-Filename')
    pdf_data = response.content
    
    # Save PDF temporarily
    with open(f"/tmp/{filename}", "wb") as f:
        f.write(pdf_data)
    
    # Print using system command
    subprocess.run(["lp", "-d", printer_name, f"/tmp/{filename}"])
    
    # Confirm after printing
    requests.post(f"{SERVER_URL}/api/partner-agent/confirm", 
                  json={"filename": filename})
```

---

### Step 6: Partner Agent Confirms Completion

**Endpoint:** `POST /api/partner-agent/confirm`

**Request:**
```json
{
  "filename": "20260101_164536_12_testing print.pdf"
}
```

**Process:**
1. Backend checks if file exists in `archived/` folder
2. File should already be there (moved during fetch)
3. Logs confirmation
4. Cleanup and completion

**Response:**
```json
{
  "status": "success",
  "message": "File archived successfully",
  "filename": "20260101_164536_12_testing print.pdf"
}
```

**Note:** File is already in archived folder from Step 4, this is just confirmation.

---

## File Status Lifecycle

```
┌─────────┐
│  READY  │ ← Customer uploads file
└────┬────┘
     │ Partner requests print
     ↓
┌──────────────┐
│  PROCESSING  │ ← File queued for agent
└────┬─────────┘
     │ Agent fetches file
     ↓
┌───────────┐
│ ARCHIVED  │ ← File sent to agent (NOT shown in frontend)
└───────────┘
```

## Status Definitions

| Status | Location | Description | Frontend Display |
|--------|----------|-------------|------------------|
| `ready` | `ready/` | File uploaded, waiting to be queued | "Print" button |
| `processing` | `processing/` | File queued, waiting for agent | "Processing" status |
| `archived` | `archived/` | File sent to agent | NOT shown |

## API Endpoints Summary

### Customer Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/test-print/upload` | POST | Upload PDF file |

### Partner Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/test-print/list` | GET | List files (ready + processing) |
| `/api/test-print/print` | POST | Queue file for printing |
| `/api/test-print/printers` | GET | Get printer list from agent |

### Partner Agent Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/partner-agent/fetch-job` | GET | Fetch next print job |
| `/api/partner-agent/confirm` | POST | Confirm print completion |
| `/api/partner-agent/sync-printers` | POST | Sync printer list |

## Error Handling

### File Not Found
- **When:** Partner requests print but file not in ready folder
- **Response:** `404 Not Found`
- **Message:** "File 'filename.pdf' not found in ready folder"

### No Jobs Available
- **When:** Agent pings but no files in processing folder
- **Response:** `204 No Content`
- **Action:** Agent should wait and retry

### Invalid File Type
- **When:** Non-PDF file uploaded or requested
- **Response:** `400 Bad Request`
- **Message:** "Only PDF files can be printed"

## Security Considerations

1. **Authentication:** All partner/customer endpoints require authentication
2. **File Validation:** Only PDF files accepted
3. **Path Traversal Protection:** Filenames sanitized
4. **File Size Limit:** Maximum 10MB per file
5. **Agent Endpoints:** No authentication (agent authenticates separately)

## Printer List Synchronization

The partner agent (Python script) syncs available printers:

**Endpoint:** `POST /api/partner-agent/sync-printers`

**Request:**
```json
{
  "printers": ["EPSON L3210 Series", "HP LaserJet Pro", "Canon PIXMA"]
}
```

**Process:**
1. Backend stores printer list in memory
2. Stores in Redis with key `partner:printers`
3. Frontend retrieves via `/api/test-print/printers`

**Response:**
```json
{
  "status": "success",
  "message": "Printers synced successfully",
  "count": 3
}
```

## Frontend Integration

### Displaying Files

```javascript
// Fetch files list
const response = await fetch('/api/test-print/list');
const data = await response.json();

data.files.forEach(file => {
  if (file.status === 'ready') {
    // Show "Print" button
  } else if (file.status === 'processing') {
    // Show "Processing" status
  }
  // Archived files are not in the list
});
```

### Requesting Print

```javascript
// Request print
const response = await fetch('/api/test-print/print', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    filename: 'file.pdf',
    printer: 'EPSON L3210 Series'
  })
});

const result = await response.json();
// result.status = "processing"
// result.message = "Print queued"
```

## Python Agent Integration

### Fetching Jobs

```python
def fetch_print_job():
    response = requests.get(f"{SERVER_URL}/api/partner-agent/fetch-job")
    
    if response.status_code == 200:
        filename = response.headers.get('X-Job-Filename')
        pdf_data = response.content
        return filename, pdf_data
    elif response.status_code == 204:
        # No jobs available
        return None, None
    else:
        # Error
        return None, None
```

### Confirming Print

```python
def confirm_print(filename):
    response = requests.post(
        f"{SERVER_URL}/api/partner-agent/confirm",
        json={"filename": filename}
    )
    return response.status_code == 200
```

## Troubleshooting

### File Not Showing in Frontend
- Check if file is in `ready/` or `processing/` folder
- Archived files are NOT shown
- Verify file is a PDF

### Agent Not Receiving Files
- Check if files are in `processing/` folder
- Verify agent is pinging `/api/partner-agent/fetch-job`
- Check server logs for errors

### Printer List Not Showing
- Verify Python script has synced printers
- Check Redis for `partner:printers` key
- Check server logs for sync confirmation

## Logging

The system logs all important events:

- File uploads: `"SUCCESS: File uploaded - Customer: X, File: Y"`
- Print requests: `"INFO: Partner requested print - Moving file from ready to processing"`
- Agent fetches: `"INFO: Agent pinged - Found job in processing folder"`
- File archived: `"SUCCESS: File sent to agent and moved to archived folder"`

Check server logs for detailed debugging information.

