package scanners

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	
	"ipcrawler/internal/ui"
)

// NslookupScanner performs DNS resolution using nslookup
type NslookupScanner struct{}

// Name returns the scanner identifier
func (n *NslookupScanner) Name() string {
	return "nslookup"
}

// Dependencies returns no dependencies for DNS lookup
func (n *NslookupScanner) Dependencies() []Scanner {
	return []Scanner{} // DNS lookup has no dependencies
}

// Run executes nslookup against the target
func (n *NslookupScanner) Run(ctx context.Context, target string, opts ScanOptions) (*ScanResult, error) {
	start := time.Now()
	
	// Update UI status
	ui.GlobalSimpleDashboard.AddTask("nslookup", "nslookup", ui.TaskRunning)
	defer func() {
		duration := time.Since(start)
		if duration > 0 {
			ui.GlobalSimpleDashboard.AddNotification(
				ui.NotificationSuccess,
				fmt.Sprintf("nslookup completed in %v", duration.Round(time.Millisecond)),
			)
		}
	}()
	
	// Create command
	cmd := exec.CommandContext(ctx, "nslookup", target)
	
	// Execute command
	output, err := cmd.Output()
	if err != nil {
		ui.GlobalSimpleDashboard.AddTask("nslookup", "nslookup", ui.TaskFailed)
		return &ScanResult{
			Scanner:  n.Name(),
			Target:   target,
			Duration: time.Since(start),
			Success:  false,
			Error:    fmt.Errorf("nslookup failed: %w", err),
			ExitCode: cmd.ProcessState.ExitCode(),
		}, err
	}
	
	// Parse DNS information
	dnsInfo, parseErr := n.parseDNSOutput(string(output), target)
	if parseErr != nil {
		// Still return success if command ran, just with parsing error
		ui.GlobalSimpleDashboard.AddNotification(
			ui.NotificationWarning,
			fmt.Sprintf("DNS parsing warning: %v", parseErr),
		)
	}
	
	// Save output to file if workspace provided
	var outputFile string
	if opts.Workspace != "" {
		outputFile = filepath.Join(opts.Workspace, "reports", "nslookup-results.txt")
		if writeErr := n.writeOutputFile(outputFile, string(output)); writeErr != nil {
			ui.GlobalSimpleDashboard.AddNotification(
				ui.NotificationWarning,
				fmt.Sprintf("Failed to write output file: %v", writeErr),
			)
		}
	}
	
	// Update UI with completion
	ui.GlobalSimpleDashboard.AddTask("nslookup", "nslookup", ui.TaskCompleted)
	
	result := &ScanResult{
		Scanner:    n.Name(),
		Target:     target,
		Duration:   time.Since(start),
		Success:    true,
		DNSInfo:    dnsInfo,
		OutputFile: outputFile,
		ExitCode:   0,
	}
	
	return result, nil
}

// parseDNSOutput extracts DNS information from nslookup output
func (n *NslookupScanner) parseDNSOutput(output, target string) (*DNSInfo, error) {
	dnsInfo := &DNSInfo{
		Hostname: target,
	}
	
	// Regex patterns for different record types
	ipv4Pattern := regexp.MustCompile(`Address:\s*(\d+\.\d+\.\d+\.\d+)`)
	ipv6Pattern := regexp.MustCompile(`Address:\s*([0-9a-fA-F:]+:[0-9a-fA-F:]+)`)
	
	// Extract IPv4 addresses
	ipv4Matches := ipv4Pattern.FindAllStringSubmatch(output, -1)
	for _, match := range ipv4Matches {
		if len(match) > 1 {
			dnsInfo.IPv4 = append(dnsInfo.IPv4, match[1])
		}
	}
	
	// Extract IPv6 addresses
	ipv6Matches := ipv6Pattern.FindAllStringSubmatch(output, -1)
	for _, match := range ipv6Matches {
		if len(match) > 1 {
			dnsInfo.IPv6 = append(dnsInfo.IPv6, match[1])
		}
	}
	
	// Extract CNAME if present
	if strings.Contains(output, "canonical name") {
		cnamePattern := regexp.MustCompile(`canonical name = (.+)\.`)
		cnameMatches := cnamePattern.FindAllStringSubmatch(output, -1)
		for _, match := range cnameMatches {
			if len(match) > 1 {
				dnsInfo.CNAME = append(dnsInfo.CNAME, strings.TrimSpace(match[1]))
			}
		}
	}
	
	return dnsInfo, nil
}

// writeOutputFile saves the nslookup output to a file
func (n *NslookupScanner) writeOutputFile(filePath, content string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	// Write content to file
	return os.WriteFile(filePath, []byte(content), 0644)
}

// Register the nslookup scanner
func init() {
	GlobalRegistry.Register(&NslookupScanner{})
}