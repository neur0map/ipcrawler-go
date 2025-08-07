package scanners

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	
	"ipcrawler/internal/ui"
)

// NmapScanner performs comprehensive service detection using nmap
type NmapScanner struct {
	NaabuScanners []Scanner // Dependencies on naabu instances
}

// Name returns the scanner identifier
func (n *NmapScanner) Name() string {
	return "nmap"
}

// Dependencies returns the required naabu scanners
func (n *NmapScanner) Dependencies() []Scanner {
	return n.NaabuScanners
}

// Run executes nmap against discovered ports
func (n *NmapScanner) Run(ctx context.Context, target string, opts ScanOptions) (*ScanResult, error) {
	start := time.Now()
	
	// Update UI status
	ui.GlobalSimpleDashboard.AddTask("nmap", "nmap", ui.TaskRunning)
	defer func() {
		duration := time.Since(start)
		if duration > 0 {
			ui.GlobalSimpleDashboard.AddNotification(
				ui.NotificationSuccess,
				fmt.Sprintf("nmap completed in %v", duration.Round(time.Millisecond)),
			)
		}
	}()
	
	// Collect ports from both naabu instances
	allPorts, err := n.collectPortsFromNaabu(opts.Workspace)
	if err != nil {
		ui.GlobalSimpleDashboard.AddTask("nmap", "nmap", ui.TaskFailed)
		return &ScanResult{
			Scanner:  n.Name(),
			Target:   target,
			Duration: time.Since(start),
			Success:  false,
			Error:    fmt.Errorf("failed to collect ports from naabu: %w", err),
		}, err
	}
	
	// If no ports found, still run basic scan
	if len(allPorts) == 0 {
		ui.GlobalSimpleDashboard.AddNotification(
			ui.NotificationWarning,
			"No ports found from naabu scans, running basic nmap scan",
		)
	}
	
	// Prepare output files
	var xmlFile, outputFile string
	if opts.Workspace != "" {
		xmlFile = filepath.Join(opts.Workspace, "json", "nmap-results.xml")
		outputFile = filepath.Join(opts.Workspace, "reports", "nmap-services.txt")
		
		// Ensure directories exist
		os.MkdirAll(filepath.Dir(xmlFile), 0755)
		os.MkdirAll(filepath.Dir(outputFile), 0755)
	}
	
	// Build nmap command
	cmd := n.buildNmapCommand(ctx, target, allPorts, xmlFile)
	
	// Execute command
	_, err = cmd.Output()
	if err != nil {
		ui.GlobalSimpleDashboard.AddTask("nmap", "nmap", ui.TaskFailed)
		return &ScanResult{
			Scanner:  n.Name(),
			Target:   target,
			Duration: time.Since(start),
			Success:  false,
			Error:    fmt.Errorf("nmap failed: %w", err),
			ExitCode: cmd.ProcessState.ExitCode(),
		}, err
	}
	
	// Parse XML output to extract services
	services, parseErr := n.parseNmapXML(xmlFile)
	if parseErr != nil {
		ui.GlobalSimpleDashboard.AddNotification(
			ui.NotificationWarning,
			fmt.Sprintf("Service parsing warning: %v", parseErr),
		)
	}
	
	// Save service results to text file
	if outputFile != "" && len(services) > 0 {
		if writeErr := n.writeServicesFile(outputFile, services); writeErr != nil {
			ui.GlobalSimpleDashboard.AddNotification(
				ui.NotificationWarning,
				fmt.Sprintf("Failed to write services file: %v", writeErr),
			)
		}
	}
	
	// Update UI with completion
	ui.GlobalSimpleDashboard.AddTask("nmap", "nmap", ui.TaskCompleted)
	
	result := &ScanResult{
		Scanner:    n.Name(),
		Target:     target,
		Duration:   time.Since(start),
		Success:    true,
		Services:   services,
		JSONFile:   xmlFile,
		OutputFile: outputFile,
		ExitCode:   0,
	}
	
	return result, nil
}

// collectPortsFromNaabu reads discovered ports from both naabu instances
func (n *NmapScanner) collectPortsFromNaabu(workspace string) ([]Port, error) {
	var allPorts []Port
	
	if workspace == "" {
		return allPorts, nil
	}
	
	// Read from both naabu port files
	portFiles := []string{
		filepath.Join(workspace, "reports", "ports-1.txt"),
		filepath.Join(workspace, "reports", "ports-2.txt"),
	}
	
	for _, portFile := range portFiles {
		if _, err := os.Stat(portFile); os.IsNotExist(err) {
			continue // Skip missing files
		}
		
		data, err := os.ReadFile(portFile)
		if err != nil {
			continue // Skip unreadable files
		}
		
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			
			if portNum, err := strconv.Atoi(line); err == nil {
				allPorts = append(allPorts, Port{
					Number:   portNum,
					Protocol: "tcp", // Naabu primarily scans TCP
					State:    "open",
				})
			}
		}
	}
	
	return allPorts, nil
}

// buildNmapCommand constructs the nmap command with appropriate flags
func (n *NmapScanner) buildNmapCommand(ctx context.Context, target string, ports []Port, xmlFile string) *exec.Cmd {
	args := []string{
		"-sV",        // Service version detection
		"-sC",        // Default scripts
		"-O",         // OS detection
		"--version-intensity", "5",
		"--open",     // Only show open ports
		"-T4",        // Aggressive timing
		"--max-retries", "1",
		"--host-timeout", "300s",
	}
	
	// Add XML output if file specified
	if xmlFile != "" {
		args = append(args, "-oX", xmlFile)
	}
	
	// Add port specification if ports discovered
	if len(ports) > 0 {
		portList := n.buildPortList(ports)
		args = append(args, "-p", portList)
	} else {
		// Basic top ports scan if no ports from naabu
		args = append(args, "--top-ports", "1000")
	}
	
	args = append(args, target)
	
	return exec.CommandContext(ctx, "nmap", args...)
}

// buildPortList creates a comma-separated port list for nmap
func (n *NmapScanner) buildPortList(ports []Port) string {
	var portStrs []string
	for _, port := range ports {
		portStrs = append(portStrs, strconv.Itoa(port.Number))
	}
	return strings.Join(portStrs, ",")
}

// parseNmapXML parses nmap XML output to extract service information
func (n *NmapScanner) parseNmapXML(xmlFile string) ([]Service, error) {
	if xmlFile == "" || !n.fileExists(xmlFile) {
		return []Service{}, nil
	}
	
	data, err := os.ReadFile(xmlFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read XML file: %w", err)
	}
	
	var nmapRun NmapRun
	if err := xml.Unmarshal(data, &nmapRun); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}
	
	var services []Service
	for _, host := range nmapRun.Hosts {
		for _, port := range host.Ports.Ports {
			if port.State.State == "open" {
				service := Service{
					Port:       port.PortID,
					Protocol:   port.Protocol,
					Service:    port.Service.Name,
					Product:    port.Service.Product,
					Version:    port.Service.Version,
					ExtraInfo:  port.Service.ExtraInfo,
					Confidence: port.Service.Conf,
				}
				services = append(services, service)
			}
		}
	}
	
	return services, nil
}

// writeServicesFile saves detected services to a text file
func (n *NmapScanner) writeServicesFile(filePath string, services []Service) error {
	var lines []string
	for _, service := range services {
		line := fmt.Sprintf("%d/%s %s", service.Port, service.Protocol, service.Service)
		if service.Product != "" {
			line += fmt.Sprintf(" (%s", service.Product)
			if service.Version != "" {
				line += fmt.Sprintf(" %s", service.Version)
			}
			line += ")"
		}
		lines = append(lines, line)
	}
	
	content := strings.Join(lines, "\n")
	return os.WriteFile(filePath, []byte(content), 0644)
}

// fileExists checks if a file exists
func (n *NmapScanner) fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// Nmap XML structures for parsing
type NmapRun struct {
	XMLName xml.Name   `xml:"nmaprun"`
	Hosts   []NmapHost `xml:"host"`
}

type NmapHost struct {
	XMLName xml.Name  `xml:"host"`
	Ports   NmapPorts `xml:"ports"`
}

type NmapPorts struct {
	XMLName xml.Name   `xml:"ports"`
	Ports   []NmapPort `xml:"port"`
}

type NmapPort struct {
	XMLName  xml.Name     `xml:"port"`
	Protocol string       `xml:"protocol,attr"`
	PortID   int          `xml:"portid,attr"`
	State    NmapState    `xml:"state"`
	Service  NmapService  `xml:"service"`
}

type NmapState struct {
	XMLName xml.Name `xml:"state"`
	State   string   `xml:"state,attr"`
}

type NmapService struct {
	XMLName   xml.Name `xml:"service"`
	Name      string   `xml:"name,attr"`
	Product   string   `xml:"product,attr"`
	Version   string   `xml:"version,attr"`
	ExtraInfo string   `xml:"extrainfo,attr"`
	Conf      int      `xml:"conf,attr"`
}

// NewNmapScanner creates a new nmap scanner with naabu dependencies
func NewNmapScanner(naabuScanners []Scanner) *NmapScanner {
	return &NmapScanner{
		NaabuScanners: naabuScanners,
	}
}

// Register the nmap scanner with dependencies
func init() {
	// Get the registered naabu scanners as dependencies
	naabu1, exists1 := GlobalRegistry.Get("naabu-1")
	naabu2, exists2 := GlobalRegistry.Get("naabu-2")
	
	if exists1 && exists2 {
		nmapScanner := NewNmapScanner([]Scanner{naabu1, naabu2})
		GlobalRegistry.Register(nmapScanner)
	}
}