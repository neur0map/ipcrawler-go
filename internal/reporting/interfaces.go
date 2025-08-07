package reporting

import (
	"ipcrawler/internal/scanners"
)

// WorkspaceManager handles workspace directory operations
type WorkspaceManager interface {
	// CreateWorkspace creates and initializes a workspace directory
	CreateWorkspace(target string) (string, error)
	
	// GetReportDir returns the reports subdirectory path
	GetReportDir(workspace string) string
	
	// GetJSONDir returns the JSON outputs subdirectory path  
	GetJSONDir(workspace string) string
	
	// GetLogsDir returns the logs subdirectory path
	GetLogsDir(workspace string) string
	
	// CleanupWorkspace removes temporary files (optional)
	CleanupWorkspace(workspace string) error
}

// PortManager handles port file operations
type PortManager interface {
	// CombinePorts merges multiple port files into one
	CombinePorts(outputFile string, inputFiles ...string) error
	
	// ExtractPorts reads ports from a JSON file and saves as text
	ExtractPorts(jsonFile, outputFile string) error
	
	// ReadPorts reads ports from a text file
	ReadPorts(portFile string) ([]scanners.Port, error)
	
	// WritePorts writes ports to a text file
	WritePorts(portFile string, ports []scanners.Port) error
}

// FileManager handles file operations for reports
type FileManager interface {
	// AppendFiles appends content from multiple files to a target file
	AppendFiles(targetFile string, sourceFiles ...string) error
	
	// WriteReport writes scan results to a formatted report
	WriteReport(reportFile string, results []scanners.ScanResult) error
	
	// ReadJSON reads and parses JSON output from tools
	ReadJSON(jsonFile string, target interface{}) error
	
	// WriteJSON writes data as JSON to a file
	WriteJSON(jsonFile string, data interface{}) error
}

// ReportGenerator creates comprehensive scan reports  
type ReportGenerator interface {
	// GenerateSummary creates a summary report from template results
	GenerateSummary(results []scanners.ScanResult, outputFile string) error
	
	// GenerateDetailedReport creates a detailed report with all findings
	GenerateDetailedReport(results []scanners.ScanResult, outputFile string) error
	
	// GenerateJSONReport creates a machine-readable JSON report
	GenerateJSONReport(results []scanners.ScanResult, outputFile string) error
}

// Default implementations

// DefaultWorkspaceManager provides standard workspace operations
type DefaultWorkspaceManager struct {
	BaseDir string
}

// DefaultPortManager provides standard port file operations
type DefaultPortManager struct{}

// DefaultFileManager provides standard file operations
type DefaultFileManager struct{}

// DefaultReportGenerator provides standard report generation
type DefaultReportGenerator struct{}

// Global instances for easy access
var (
	GlobalWorkspace = &DefaultWorkspaceManager{BaseDir: "workspace"}
	GlobalPorts     = &DefaultPortManager{}
	GlobalFiles     = &DefaultFileManager{}
	GlobalReports   = &DefaultReportGenerator{}
)