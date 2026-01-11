package cost_calculator

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"print-pro-backend/internal/models/jobcost"
	"print-pro-backend/internal/models/printjob"
)

// CostCalculator handles cost calculation for print jobs
type CostCalculator struct {
	defaultColorCostPerPage float64
	defaultBWCostPerPage    float64
}

// NewCostCalculator creates a new cost calculator
func NewCostCalculator(defaultColorCost, defaultBWCost float64) *CostCalculator {
	return &CostCalculator{
		defaultColorCostPerPage: defaultColorCost,
		defaultBWCostPerPage:    defaultBWCost,
	}
}

// CalculateCost calculates the cost for a print job based on file, page options, and pricing
// Formula: total_cost = (bw_cost*(total_pages-individual_color_pages-skip_pages) + color_page_cost*individual_color_pages)*num_copies
func (cc *CostCalculator) CalculateCost(
	filePath string,
	pageOptions printjob.PageOptions,
	color *bool,
	numCopies *int,
	individualColorPages []int,
) (*jobcost.JobCost, error) {
	// Get total pages - use from pageOptions if available, otherwise count from file
	var totalPages int
	if pageOptions.TotalPages != nil && *pageOptions.TotalPages > 0 {
		totalPages = *pageOptions.TotalPages
		log.Printf("Using total_pages from page_options: %d", totalPages)
	} else {
		// Count total pages in the file
		var err error
		totalPages, err = cc.countPagesInFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to count pages: %w", err)
		}
		log.Printf("Counted total pages from file: %d", totalPages)
	}

	// Get counts for calculation
	skipPagesCount := len(pageOptions.SkipPages)
	individualColorPagesCount := len(individualColorPages)
	
	log.Printf("Cost calculation inputs - Total pages: %d, Skip pages: %d, Individual color pages: %d", 
		totalPages, skipPagesCount, individualColorPagesCount)

	// Get number of copies
	copies := 1
	if numCopies != nil && *numCopies > 0 {
		copies = *numCopies
	}

	// Calculate B&W pages using the formula: total_pages - individual_color_pages - skip_pages
	bwPages := totalPages - individualColorPagesCount - skipPagesCount
	if bwPages < 0 {
		bwPages = 0
		log.Printf("WARNING: Calculated B&W pages is negative, setting to 0")
	}

	// Color pages = individual_color_pages_count
	colorPages := individualColorPagesCount

	// Create job cost
	jobCost := &jobcost.JobCost{
		TotalPages:      totalPages,
		PagesToPrint:    totalPages - skipPagesCount, // Pages that will actually be printed
		ColorPages:      colorPages,
		BlackWhitePages: bwPages,
		NumCopies:       copies,
	}

	// Calculate total cost using the formula:
	// total_cost = (bw_cost*(total_pages-individual_color_pages-skip_pages) + color_page_cost*individual_color_pages)*num_copies
	jobCost.CalculateTotalCost(cc.defaultColorCostPerPage, cc.defaultBWCostPerPage)

	log.Printf("Cost calculated - Total pages: %d, Pages to print: %d, Color: %d, B&W: %d, Copies: %d, Total cost: %.2f",
		jobCost.TotalPages, jobCost.PagesToPrint, jobCost.ColorPages, jobCost.BlackWhitePages, jobCost.NumCopies, jobCost.TotalCost)

	return jobCost, nil
}

// CountPagesInFile counts the total number of pages in a PDF or image file (public method)
func (cc *CostCalculator) CountPagesInFile(filePath string) (int, error) {
	return cc.countPagesInFile(filePath)
}

// countPagesInFile counts the total number of pages in a PDF or image file
func (cc *CostCalculator) countPagesInFile(filePath string) (int, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	// Images are always 1 page
	if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
		return 1, nil
	}

	// For PDFs, we need to count pages
	if ext == ".pdf" {
		return cc.countPDFPages(filePath)
	}

	return 0, fmt.Errorf("unsupported file type: %s", ext)
}

// countPDFPages counts pages in a PDF file using pdfcpu library
func (cc *CostCalculator) countPDFPages(filePath string) (int, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return 0, fmt.Errorf("file not found: %s", filePath)
	}

	// Use pdfcpu to get page count
	ctx, err := api.ReadContextFile(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to read PDF file with pdfcpu: %w", err)
	}

	pageCount := ctx.PageCount
	log.Printf("PDF page count (pdfcpu): %d pages in %s", pageCount, filePath)
	
	return pageCount, nil
}

// calculatePagesToPrint determines which pages will actually be printed
// after applying start_page, end_page, filter_type, and skip_pages
func (cc *CostCalculator) calculatePagesToPrint(totalPages int, pageOptions printjob.PageOptions) []int {
	// Start with all pages (1-indexed)
	startPage := 1
	endPage := totalPages

	// Apply page range
	if pageOptions.StartPage != nil && *pageOptions.StartPage > 0 {
		startPage = *pageOptions.StartPage
	}
	if pageOptions.EndPage != nil && *pageOptions.EndPage > 0 {
		endPage = *pageOptions.EndPage
		if endPage > totalPages {
			endPage = totalPages
		}
	}

	// Build initial page list
	pages := []int{}
	for i := startPage; i <= endPage; i++ {
		pages = append(pages, i)
	}

	// Apply filter_type (odd/even)
	if pageOptions.FilterType != nil {
		switch *pageOptions.FilterType {
		case "odd":
			filtered := []int{}
			for _, p := range pages {
				if p%2 == 1 {
					filtered = append(filtered, p)
				}
			}
			pages = filtered
		case "even":
			filtered := []int{}
			for _, p := range pages {
				if p%2 == 0 {
					filtered = append(filtered, p)
				}
			}
			pages = filtered
		}
	}

	// Apply skip_pages
	if len(pageOptions.SkipPages) > 0 {
		skipMap := make(map[int]bool)
		for _, skipPage := range pageOptions.SkipPages {
			skipMap[skipPage] = true
		}
		filtered := []int{}
		for _, p := range pages {
			if !skipMap[p] {
				filtered = append(filtered, p)
			}
		}
		pages = filtered
	}

	return pages
}

// countColorAndBWPages counts how many pages will be printed in color vs B&W
func (cc *CostCalculator) countColorAndBWPages(
	pagesToPrint []int,
	globalColor *bool,
	individualColorPages []int,
) (colorPages, bwPages int) {
	// Determine default color setting
	defaultColor := false
	if globalColor != nil {
		defaultColor = *globalColor
	}

	// Create map of color pages for quick lookup
	colorPageMap := make(map[int]bool)
	for _, pageNum := range individualColorPages {
		colorPageMap[pageNum] = true
	}

	// Count color and B&W pages
	for _, pageNum := range pagesToPrint {
		// Check if this page should be in color
		isColor := defaultColor
		if len(individualColorPages) > 0 {
			// If individual_color_pages is specified, use it to override
			isColor = colorPageMap[pageNum]
		}

		if isColor {
			colorPages++
		} else {
			bwPages++
		}
	}

	return colorPages, bwPages
}
