package print_handler

// PrintHandler handles print requests from partners
type PrintHandler struct {
	uploadDir string
}

// NewPrintHandler creates a new print handler
func NewPrintHandler(uploadDir string) *PrintHandler {
	return &PrintHandler{
		uploadDir: uploadDir,
	}
}

// PrintRequest represents the request to print a file
type PrintRequest struct {
	Filename string `json:"filename"`
	Printer  string `json:"printer,omitempty"` // Optional: specific printer name (if not provided, uses default)
}

