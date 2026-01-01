package printer_list

// PrinterInfo represents a printer with its status information
type PrinterInfo struct {
	Name          string
	PrinterStatus *int
	WorkOffline   *bool
	Availability  *int
	DriverName    string
	PortName      string
	Default       *bool
	Local         *bool
	Network       *bool
	Status        string
	IsAvailable   bool
}

// BasicPrinterInfo represents basic printer information
type BasicPrinterInfo struct {
	Name          string
	PrinterStatus *int
	DriverName    string
	PortName      string
	Default       *bool
}

