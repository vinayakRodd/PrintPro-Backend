package test_print

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"print-pro-backend/internal/middleware/auth_middleware"
	"print-pro-backend/internal/repositories"
	"strconv"
	"strings"
	"time"
)

// UploadHandler handles file uploads from customers
type UploadHandler struct {
	uploadDir              string
	agentHandler           interface {
		QueueJobForAgent(sourceFilePath string) error
	}
	partnerProfileRepository *repositories.PartnerProfileRepository
	printJobRepository       *repositories.PrintJobRepository
}

// NewUploadHandler creates a new upload handler
func NewUploadHandler(
	uploadDir string,
	agentHandler interface {
		QueueJobForAgent(sourceFilePath string) error
	},
	partnerProfileRepository *repositories.PartnerProfileRepository,
	printJobRepository *repositories.PrintJobRepository,
) *UploadHandler {
	return &UploadHandler{
		uploadDir:                uploadDir,
		agentHandler:             agentHandler,
		partnerProfileRepository: partnerProfileRepository,
		printJobRepository:       printJobRepository,
	}
}

// UploadFile handles file upload requests from customers
func (h *UploadHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", "Only POST method is allowed")
		return
	}

	// Get user from context (set by auth middleware)
	user, ok := auth_middleware.GetUserFromContext(r)
	if !ok {
		h.sendErrorResponse(w, http.StatusUnauthorized, "Unauthorized", "User not found in session")
		return
	}

	// Verify user is a customer
	if user.UserType != "customer" {
		h.sendErrorResponse(w, http.StatusForbidden, "Forbidden", "Only customers can upload files")
		return
	}

	// Check Content-Type header
	contentType := r.Header.Get("Content-Type")
	log.Printf("DEBUG: Upload request - Method: %s, Content-Type: %s, Content-Length: %s", 
		r.Method, contentType, r.Header.Get("Content-Length"))
	
	// Check if Content-Type is multipart/form-data (must start with "multipart/form-data")
	if contentType == "" {
		log.Printf("ERROR: Missing Content-Type header")
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid request", "Missing Content-Type header. Request must be multipart/form-data.")
		return
	}
	
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		log.Printf("ERROR: Invalid Content-Type - Expected multipart/form-data, got: %s", contentType)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid Content-Type", 
			fmt.Sprintf("Request must be multipart/form-data. Received: %s. Please use FormData in your frontend.", contentType))
		return
	}
	
	// Parse multipart form (max 10MB)
	err := r.ParseMultipartForm(10 << 20) // 10MB
	if err != nil {
		log.Printf("ERROR: Failed to parse multipart form - Content-Type: %s, Error: %v", contentType, err)
		h.sendErrorResponse(w, http.StatusBadRequest, "Invalid form data", 
			fmt.Sprintf("Failed to parse multipart form. Content-Type: %s. Error: %v", contentType, err))
		return
	}
	
	log.Printf("DEBUG: Multipart form parsed successfully")

	// Get shop_name from form (required)
	shopName := strings.TrimSpace(r.FormValue("shop_name"))
	if shopName == "" {
		log.Printf("ERROR: Missing shop_name in form")
		h.sendErrorResponse(w, http.StatusBadRequest, "Shop name required", "Please provide a shop_name in the form")
		return
	}

	// Get optional print job parameters from form
	pType := strings.TrimSpace(r.FormValue("p_type"))
	var pTypePtr *string
	if pType != "" {
		pTypePtr = &pType
	}

	// Parse color (default: false)
	colorStr := strings.TrimSpace(strings.ToLower(r.FormValue("color")))
	var colorPtr *bool
	if colorStr != "" {
		color := colorStr == "true" || colorStr == "1" || colorStr == "yes"
		colorPtr = &color
	}

	// Parse num_copies (default: 1)
	numCopiesStr := strings.TrimSpace(r.FormValue("num_copies"))
	var numCopiesPtr *int
	if numCopiesStr != "" {
		if numCopies, err := strconv.Atoi(numCopiesStr); err == nil && numCopies > 0 {
			numCopiesPtr = &numCopies
		} else {
			log.Printf("WARNING: Invalid num_copies value '%s', using default (1)", numCopiesStr)
		}
	}

	// Parse start_page (optional, 1-indexed)
	startPageStr := strings.TrimSpace(r.FormValue("start_page"))
	var startPagePtr *int
	if startPageStr != "" {
		if startPage, err := strconv.Atoi(startPageStr); err == nil && startPage > 0 {
			startPagePtr = &startPage
		} else {
			log.Printf("WARNING: Invalid start_page value '%s', ignoring", startPageStr)
		}
	}

	// Parse end_page (optional, 1-indexed)
	endPageStr := strings.TrimSpace(r.FormValue("end_page"))
	var endPagePtr *int
	if endPageStr != "" {
		if endPage, err := strconv.Atoi(endPageStr); err == nil && endPage > 0 {
			endPagePtr = &endPage
		} else {
			log.Printf("WARNING: Invalid end_page value '%s', ignoring", endPageStr)
		}
	}

	// Parse page_filter_type (optional): all | odd | even
	pageFilterTypeStr := strings.TrimSpace(strings.ToLower(r.FormValue("page_filter_type")))
	var pageFilterTypePtr *string
	if pageFilterTypeStr != "" {
		switch pageFilterTypeStr {
		case "all", "odd", "even":
			pageFilterTypePtr = &pageFilterTypeStr
		default:
			log.Printf("WARNING: Invalid page_filter_type value '%s', using default (all)", pageFilterTypeStr)
		}
	}

	// Parse individual_color_print_pages (optional): JSON array of page numbers
	// Example: "[1, 3, 5]" means pages 1, 3, and 5 should be printed in color
	individualColorPagesStr := strings.TrimSpace(r.FormValue("individual_color_print_pages"))
	var individualColorPages []int
	if individualColorPagesStr != "" {
		if err := json.Unmarshal([]byte(individualColorPagesStr), &individualColorPages); err != nil {
			log.Printf("WARNING: Invalid individual_color_print_pages JSON '%s' - %v, ignoring", individualColorPagesStr, err)
			individualColorPages = nil
		} else if len(individualColorPages) == 0 {
			// Empty array is treated as nil (no individual color pages)
			individualColorPages = nil
		}
	}

	// Parse skip_pages (optional): JSON array of page numbers (1-indexed) to skip
	// Example: "[2, 5, 8]" means pages 2, 5, and 8 will be skipped during printing
	skipPagesStr := strings.TrimSpace(r.FormValue("skip_pages"))
	var skipPages []int
	if skipPagesStr != "" {
		if err := json.Unmarshal([]byte(skipPagesStr), &skipPages); err != nil {
			log.Printf("WARNING: Invalid skip_pages JSON '%s' - %v, ignoring", skipPagesStr, err)
			skipPages = nil
		} else if len(skipPages) == 0 {
			// Empty array is treated as nil (no pages to skip)
			skipPages = nil
		} else {
			// Validate page numbers are positive
			validPages := true
			for _, p := range skipPages {
				if p < 1 {
					validPages = false
					break
				}
			}
			if !validPages {
				log.Printf("WARNING: Invalid skip_pages - page numbers must be >= 1, ignoring")
				skipPages = nil
			}
		}
	}

	// Parse back_to_back (optional): boolean for duplex printing
	backToBackStr := strings.TrimSpace(strings.ToLower(r.FormValue("back_to_back")))
	var backToBackPtr *bool
	if backToBackStr != "" {
		backToBack := backToBackStr == "true" || backToBackStr == "1" || backToBackStr == "yes"
		backToBackPtr = &backToBack
	}

	// Validate page range if both are provided
	if startPagePtr != nil && endPagePtr != nil {
		if *startPagePtr > *endPagePtr {
			log.Printf("WARNING: Invalid page range - start_page (%d) > end_page (%d), ignoring page range", *startPagePtr, *endPagePtr)
			startPagePtr = nil
			endPagePtr = nil
		}
	}

	log.Printf("DEBUG: Upload request - Shop: %s, Customer: %s, PType: %v, Color: %v, NumCopies: %v, StartPage: %v, EndPage: %v, PageFilterType: %v, IndividualColorPages: %v, SkipPages: %v, BackToBack: %v", 
		shopName, user.ID, pTypePtr, colorPtr, numCopiesPtr, startPagePtr, endPagePtr, pageFilterTypePtr, individualColorPages, skipPages, backToBackPtr)

	// Get file from form
	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("ERROR: Failed to get file from form - %v", err)
		h.sendErrorResponse(w, http.StatusBadRequest, "File required", "Please provide a file in the 'file' field")
		return
	}
	defer file.Close()

	// Validate file size (max 10MB)
	if header.Size > 10<<20 {
		h.sendErrorResponse(w, http.StatusBadRequest, "File too large", "File size exceeds 10MB limit")
		return
	}

	// Sanitize filename
	originalFilename := header.Filename
	ext := filepath.Ext(originalFilename)
	nameWithoutExt := strings.TrimSuffix(originalFilename, ext)
	
	// Remove dangerous characters
	nameWithoutExt = strings.ReplaceAll(nameWithoutExt, "..", "_")
	nameWithoutExt = strings.ReplaceAll(nameWithoutExt, "/", "_")
	nameWithoutExt = strings.ReplaceAll(nameWithoutExt, "\\", "_")
	
	// Create unique filename: timestamp_userid_originalname
	// Use nanosecond precision to ensure uniqueness even if multiple uploads happen simultaneously
	timestamp := time.Now().Format("20060102_150405")
	nanos := time.Now().Nanosecond()
	filename := fmt.Sprintf("%s_%d_%s_%s%s", timestamp, nanos, user.ID, nameWithoutExt, ext)

	// Save file directly to ready folder (as per requirements)
	readyDir := filepath.Join(h.uploadDir, "ready")
	finalFilePath := filepath.Join(readyDir, filename)

	// Create ready directory if it doesn't exist
	if err := os.MkdirAll(readyDir, 0755); err != nil {
		log.Printf("ERROR: Failed to create ready directory - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to create ready directory")
		return
	}

	// Check if file already exists (collision detection)
	// If it exists, append a counter to make it unique
	counter := 1
	originalFinalFilePath := finalFilePath
	for {
		if _, err := os.Stat(finalFilePath); os.IsNotExist(err) {
			// File doesn't exist, we can use this filename
			break
		}
		// File exists, try with counter suffix
		ext := filepath.Ext(filename)
		nameWithoutExt := strings.TrimSuffix(filename, ext)
		filename = fmt.Sprintf("%s_%d%s", nameWithoutExt, counter, ext)
		finalFilePath = filepath.Join(readyDir, filename)
		counter++
		// Safety limit to prevent infinite loop
		if counter > 1000 {
			log.Printf("ERROR: Too many filename collisions for %s", originalFinalFilePath)
			h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to generate unique filename")
			return
		}
	}

	// Create file directly in ready folder
	dst, err := os.Create(finalFilePath)
	if err != nil {
		log.Printf("ERROR: Failed to create file - %v", err)
		h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to save file")
		return
	}
	defer dst.Close()

	// Copy file content directly to ready folder
	_, err = io.Copy(dst, file)
	if err != nil {
		log.Printf("ERROR: Failed to write file - %v", err)
		os.Remove(finalFilePath) // Clean up on error
		h.sendErrorResponse(w, http.StatusInternalServerError, "Internal error", "Failed to save file")
		return
	}

	// NOTE: File is saved to ready folder but NOT queued in Redis
	// Files should only be queued when partner explicitly clicks "Print" button
	// This prevents automatic printing when files are uploaded

	// Get partner_id from shop_name
	ctx := r.Context()
	
	// SECURITY: Verify shop exists and get partner_id
	partnerProfile, err := h.partnerProfileRepository.GetByShopName(ctx, shopName)
	if err != nil {
		log.Printf("ERROR: Failed to get partner profile for shop '%s' - %v", shopName, err)
		// Don't fail the upload, but log the error
		log.Printf("WARNING: Print job not created in database due to partner lookup failure")
	} else {
		partnerID := partnerProfile.ID
		
		// SECURITY: Log detailed information to verify correct shop association
		log.Printf("INFO: Upload - Shop: '%s', PartnerID: %d, Customer: %s, File: %s", 
			shopName, partnerID, user.ID, filename)
		
		// Convert user.ID (string) to int64 for account_id
		accountID, err := strconv.ParseInt(user.ID, 10, 64)
		if err != nil {
			log.Printf("WARNING: Failed to parse user ID '%s' - %v", user.ID, err)
			accountID = 0
		}

		// Build file URL/path - use absolute file path
		absFilePath, _ := filepath.Abs(finalFilePath)
		fileURL := absFilePath
		if fileURL == "" {
			// Fallback to relative path if absolute path fails
			fileURL = fmt.Sprintf("/api/test-print/preview?filename=%s", filename)
		}

		// Create print job in database
		var accountIDPtr *int64
		if accountID > 0 {
			accountIDPtr = &accountID
		}
		
		printJob, err := h.printJobRepository.Create(ctx, accountIDPtr, partnerID, filename, fileURL, pTypePtr, colorPtr, numCopiesPtr, startPagePtr, endPagePtr, pageFilterTypePtr, individualColorPages, skipPages, backToBackPtr)
		if err != nil {
			log.Printf("ERROR: Failed to create print job in database - %v", err)
			// Don't fail the upload, but log the error
			log.Printf("WARNING: File uploaded but print job not created in database")
		} else {
			// SECURITY: Verify the created print job has the correct partner_id
			if printJob.PartnerID != partnerID {
				log.Printf("ERROR: SECURITY ISSUE - Print job created with wrong partner_id! Expected: %d, Got: %d, File: %s", 
					partnerID, printJob.PartnerID, filename)
			}
			log.Printf("SUCCESS: Print job created - ID: %d, Customer: %s (account_id: %v), Shop: %s (partner_id: %d), File: %s, PType: %v, Color: %v, NumCopies: %v, StartPage: %v, EndPage: %v", 
				printJob.ID, user.ID, printJob.AccountID, shopName, printJob.PartnerID, filename, printJob.PType, printJob.Color, printJob.NumCopies, printJob.StartPage, printJob.EndPage)
		}
	}

	log.Printf("SUCCESS: File uploaded - Customer: %s, Shop: %s, File: %s, Size: %d bytes", 
		user.ID, shopName, filename, header.Size)

	// Return success response
	h.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"message":  "File uploaded successfully",
		"filename": filename,
		"shop_name": shopName,
		"size":     header.Size,
	})
}

// sendJSONResponse sends a JSON response
func (h *UploadHandler) sendJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("ERROR: Failed to encode JSON response - %v", err)
	}
}

// sendErrorResponse sends an error JSON response
func (h *UploadHandler) sendErrorResponse(w http.ResponseWriter, statusCode int, message, error string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"message": message,
		"error":   error,
	})
}

