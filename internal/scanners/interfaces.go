package scanners

import (
	"context"
	"time"
)

// Scanner represents a security scanning tool
type Scanner interface {
	// Name returns the scanner's identifier
	Name() string
	
	// Run executes the scanner with the given target and options
	Run(ctx context.Context, target string, opts ScanOptions) (*ScanResult, error)
	
	// Dependencies returns other scanners this one depends on
	Dependencies() []Scanner
}

// ScanOptions provides configuration for scanner execution
type ScanOptions struct {
	// Workspace directory for output files
	Workspace string
	
	// Ports to scan (for nmap after port discovery)
	Ports []Port
	
	// Additional scanner-specific options
	Rate     int    // For naabu: scan rate
	Timeout  int    // Command timeout in seconds
	Silent   bool   // Suppress verbose output
	Debug    bool   // Enable debug mode
}

// ScanResult contains the results from a scanner execution
type ScanResult struct {
	// Scanner identification
	Scanner  string        `json:"scanner"`
	Target   string        `json:"target"`
	Duration time.Duration `json:"duration"`
	Success  bool          `json:"success"`
	
	// Results data
	Ports     []Port         `json:"ports,omitempty"`
	Services  []Service      `json:"services,omitempty"`
	DNSInfo   *DNSInfo       `json:"dns_info,omitempty"`
	
	// Output files
	OutputFile string   `json:"output_file,omitempty"`
	JSONFile   string   `json:"json_file,omitempty"`
	
	// Error information
	Error     error    `json:"error,omitempty"`
	ExitCode  int      `json:"exit_code"`
}

// Port represents a discovered network port
type Port struct {
	Number   int    `json:"port"`
	Protocol string `json:"protocol"` // tcp, udp
	State    string `json:"state"`    // open, closed, filtered
	Host     string `json:"host"`
}

// Service represents a detected service on a port
type Service struct {
	Port        int               `json:"port"`
	Protocol    string           `json:"protocol"`
	Service     string           `json:"service"`
	Version     string           `json:"version,omitempty"`
	Product     string           `json:"product,omitempty"`
	ExtraInfo   string           `json:"extra_info,omitempty"`
	Confidence  int              `json:"confidence,omitempty"`
	CPE         []string         `json:"cpe,omitempty"`
}

// DNSInfo represents DNS lookup results
type DNSInfo struct {
	Hostname    string   `json:"hostname"`
	IPv4        []string `json:"ipv4,omitempty"`
	IPv6        []string `json:"ipv6,omitempty"`
	CNAME       []string `json:"cname,omitempty"`
	MX          []string `json:"mx,omitempty"`
	NS          []string `json:"ns,omitempty"`
	TXT         []string `json:"txt,omitempty"`
	ResolveTime time.Duration `json:"resolve_time"`
}

// ScannerRegistry manages available scanners
type ScannerRegistry struct {
	scanners map[string]Scanner
}

// NewScannerRegistry creates a new scanner registry
func NewScannerRegistry() *ScannerRegistry {
	return &ScannerRegistry{
		scanners: make(map[string]Scanner),
	}
}

// Register adds a scanner to the registry
func (r *ScannerRegistry) Register(scanner Scanner) {
	r.scanners[scanner.Name()] = scanner
}

// Get retrieves a scanner by name
func (r *ScannerRegistry) Get(name string) (Scanner, bool) {
	scanner, exists := r.scanners[name]
	return scanner, exists
}

// List returns all registered scanner names
func (r *ScannerRegistry) List() []string {
	names := make([]string, 0, len(r.scanners))
	for name := range r.scanners {
		names = append(names, name)
	}
	return names
}

// Global scanner registry
var GlobalRegistry = NewScannerRegistry()