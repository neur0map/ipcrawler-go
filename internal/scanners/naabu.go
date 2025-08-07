package scanners

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	
	"ipcrawler/internal/ui"
)

// NaabuScanner performs port scanning using naabu
type NaabuScanner struct {
	PortRange string // e.g., "1-500" or "501-1000"
	Rate      int    // Scan rate (default: 15000)
	ID        string // Unique identifier for this instance
}

// Name returns the scanner identifier  
func (n *NaabuScanner) Name() string {
	return fmt.Sprintf("naabu-%s", n.ID)
}

// Dependencies returns no dependencies for port scanning
func (n *NaabuScanner) Dependencies() []Scanner {
	return []Scanner{} // Naabu has no dependencies
}

// Run executes naabu against the target
func (n *NaabuScanner) Run(ctx context.Context, target string, opts ScanOptions) (*ScanResult, error) {
	start := time.Now()
	
	// Set default rate if not specified
	rate := n.Rate
	if rate == 0 {
		rate = 15000
	}
	
	// Update UI status
	taskID := fmt.Sprintf("naabu-%s", n.ID)
	ui.GlobalSimpleDashboard.AddTask(taskID, n.Name(), ui.TaskRunning)
	defer func() {
		duration := time.Since(start)
		if duration > 0 {
			ui.GlobalSimpleDashboard.AddNotification(
				ui.NotificationSuccess,
				fmt.Sprintf("%s completed in %v", n.Name(), duration.Round(time.Millisecond)),
			)
		}
	}()
	
	// Read port range from file or use provided range
	portSpec, err := n.getPortSpec(opts.Workspace)
	if err != nil {
		ui.GlobalSimpleDashboard.AddTask(taskID, n.Name(), ui.TaskFailed)
		return &ScanResult{
			Scanner:  n.Name(),
			Target:   target,
			Duration: time.Since(start),
			Success:  false,
			Error:    fmt.Errorf("failed to get port specification: %w", err),
		}, err
	}
	
	// Prepare output files
	var jsonFile, outputFile string
	if opts.Workspace != "" {
		jsonFile = filepath.Join(opts.Workspace, "json", fmt.Sprintf("naabu-%s.json", n.ID))
		outputFile = filepath.Join(opts.Workspace, "reports", fmt.Sprintf("ports-%s.txt", n.ID))
		
		// Ensure directories exist
		os.MkdirAll(filepath.Dir(jsonFile), 0755)
		os.MkdirAll(filepath.Dir(outputFile), 0755)
	}
	
	// Build naabu command
	cmd := exec.CommandContext(ctx, "naabu",
		"-host", target,
		"-p", portSpec,
		"-json",
		"-rate", strconv.Itoa(rate),
		"-c", "100",
		"-timeout", "300",
		"-retries", "1",
		"-silent",
	)
	
	// Execute command
	output, err := cmd.Output()
	if err != nil {
		ui.GlobalSimpleDashboard.AddTask(taskID, n.Name(), ui.TaskFailed)
		return &ScanResult{
			Scanner:  n.Name(),
			Target:   target,
			Duration: time.Since(start),
			Success:  false,
			Error:    fmt.Errorf("naabu failed: %w", err),
			ExitCode: cmd.ProcessState.ExitCode(),
		}, err
	}
	
	// Parse JSON output to extract ports
	ports, parseErr := n.parseNaabuOutput(string(output))
	if parseErr != nil {
		ui.GlobalSimpleDashboard.AddNotification(
			ui.NotificationWarning,
			fmt.Sprintf("Port parsing warning for %s: %v", n.Name(), parseErr),
		)
	}
	
	// Save JSON output
	if jsonFile != "" {
		if writeErr := os.WriteFile(jsonFile, output, 0644); writeErr != nil {
			ui.GlobalSimpleDashboard.AddNotification(
				ui.NotificationWarning,
				fmt.Sprintf("Failed to write JSON file: %v", writeErr),
			)
		}
	}
	
	// Extract and save port list to text file
	if outputFile != "" && len(ports) > 0 {
		if writeErr := n.writePortsFile(outputFile, ports); writeErr != nil {
			ui.GlobalSimpleDashboard.AddNotification(
				ui.NotificationWarning,
				fmt.Sprintf("Failed to write ports file: %v", writeErr),
			)
		}
	}
	
	// Update UI with completion
	ui.GlobalSimpleDashboard.AddTask(taskID, n.Name(), ui.TaskCompleted)
	
	result := &ScanResult{
		Scanner:    n.Name(),
		Target:     target,
		Duration:   time.Since(start),
		Success:    true,
		Ports:      ports,
		JSONFile:   jsonFile,
		OutputFile: outputFile,
		ExitCode:   0,
	}
	
	return result, nil
}

// getPortSpec returns the port specification for this scanner
func (n *NaabuScanner) getPortSpec(workspace string) (string, error) {
	// If PortRange is already specified, use it
	if n.PortRange != "" {
		return n.PortRange, nil
	}
	
	// Otherwise, read from the appropriate port file based on ID
	var portFile string
	if workspace != "" {
		if n.ID == "1" {
			portFile = "internal/database/top1k-ports/top-500-ports-comma.txt"
		} else if n.ID == "2" {
			portFile = "internal/database/top1k-ports/next-500-ports-comma.txt"
		}
	}
	
	if portFile == "" {
		return "", fmt.Errorf("no port specification available for naabu-%s", n.ID)
	}
	
	// Read port file
	data, err := os.ReadFile(portFile)
	if err != nil {
		return "", fmt.Errorf("failed to read port file %s: %w", portFile, err)
	}
	
	return strings.TrimSpace(string(data)), nil
}

// parseNaabuOutput parses JSON output from naabu to extract ports
func (n *NaabuScanner) parseNaabuOutput(output string) ([]Port, error) {
	var ports []Port
	
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		
		var naabuResult struct {
			Host      string `json:"host"`
			IP        string `json:"ip"`
			Port      int    `json:"port"`
			Protocol  string `json:"protocol"`
			TLS       bool   `json:"tls"`
			Timestamp string `json:"timestamp"`
		}
		
		if err := json.Unmarshal([]byte(line), &naabuResult); err != nil {
			continue // Skip malformed lines
		}
		
		port := Port{
			Number:   naabuResult.Port,
			Protocol: naabuResult.Protocol,
			State:    "open", // Naabu only reports open ports
			Host:     naabuResult.Host,
		}
		
		ports = append(ports, port)
	}
	
	return ports, nil
}

// writePortsFile saves discovered ports to a text file
func (n *NaabuScanner) writePortsFile(filePath string, ports []Port) error {
	var portNumbers []string
	for _, port := range ports {
		portNumbers = append(portNumbers, strconv.Itoa(port.Number))
	}
	
	content := strings.Join(portNumbers, "\n")
	return os.WriteFile(filePath, []byte(content), 0644)
}

// NewNaabuScanner creates a new naabu scanner with the specified configuration
func NewNaabuScanner(id, portRange string, rate int) *NaabuScanner {
	return &NaabuScanner{
		ID:        id,
		PortRange: portRange,
		Rate:      rate,
	}
}

// Register naabu scanners
func init() {
	// Register two naabu instances for the basic template
	GlobalRegistry.Register(NewNaabuScanner("1", "", 15000)) // Will use top-500 ports
	GlobalRegistry.Register(NewNaabuScanner("2", "", 15000)) // Will use next-500 ports
}