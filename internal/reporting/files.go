package reporting

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	
	"ipcrawler/internal/scanners"
)

// DefaultFileManager implements file operations for reports
func (f *DefaultFileManager) AppendFiles(targetFile string, sourceFiles ...string) error {
	// Ensure target directory exists
	if err := os.MkdirAll(filepath.Dir(targetFile), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	var content strings.Builder
	
	for _, sourceFile := range sourceFiles {
		if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
			continue // Skip missing files
		}
		
		data, err := os.ReadFile(sourceFile)
		if err != nil {
			continue // Skip unreadable files
		}
		
		if content.Len() > 0 {
			content.WriteString("\n\n")
		}
		
		// Add file header
		content.WriteString(fmt.Sprintf("=== %s ===\n", filepath.Base(sourceFile)))
		content.Write(data)
	}
	
	return os.WriteFile(targetFile, []byte(content.String()), 0644)
}

// WriteReport writes scan results to a formatted report
func (f *DefaultFileManager) WriteReport(reportFile string, results []scanners.ScanResult) error {
	var report strings.Builder
	
	report.WriteString("IPCrawler Scan Report\n")
	report.WriteString("=====================\n\n")
	
	// Summary section
	var totalPorts, totalServices int
	var target string
	var totalDuration time.Duration
	
	for _, result := range results {
		if target == "" {
			target = result.Target
		}
		totalPorts += len(result.Ports)
		totalServices += len(result.Services)
		totalDuration += result.Duration
	}
	
	report.WriteString(fmt.Sprintf("Target: %s\n", target))
	report.WriteString(fmt.Sprintf("Total Duration: %v\n", totalDuration.Round(time.Millisecond)))
	report.WriteString(fmt.Sprintf("Total Ports Found: %d\n", totalPorts))
	report.WriteString(fmt.Sprintf("Total Services Detected: %d\n\n", totalServices))
	
	// Individual scanner results
	for _, result := range results {
		report.WriteString(fmt.Sprintf("=== %s Results ===\n", result.Scanner))
		report.WriteString(fmt.Sprintf("Duration: %v\n", result.Duration.Round(time.Millisecond)))
		report.WriteString(fmt.Sprintf("Success: %t\n", result.Success))
		
		if result.Error != nil {
			report.WriteString(fmt.Sprintf("Error: %v\n", result.Error))
		}
		
		if len(result.Ports) > 0 {
			report.WriteString(fmt.Sprintf("Ports (%d):\n", len(result.Ports)))
			for _, port := range result.Ports {
				report.WriteString(fmt.Sprintf("  %d/%s (%s)\n", port.Number, port.Protocol, port.State))
			}
		}
		
		if len(result.Services) > 0 {
			report.WriteString(fmt.Sprintf("Services (%d):\n", len(result.Services)))
			for _, service := range result.Services {
				line := fmt.Sprintf("  %d/%s %s", service.Port, service.Protocol, service.Service)
				if service.Product != "" {
					line += fmt.Sprintf(" (%s", service.Product)
					if service.Version != "" {
						line += fmt.Sprintf(" %s", service.Version)
					}
					line += ")"
				}
				report.WriteString(line + "\n")
			}
		}
		
		if result.DNSInfo != nil {
			report.WriteString("DNS Information:\n")
			if len(result.DNSInfo.IPv4) > 0 {
				report.WriteString(fmt.Sprintf("  IPv4: %s\n", strings.Join(result.DNSInfo.IPv4, ", ")))
			}
			if len(result.DNSInfo.IPv6) > 0 {
				report.WriteString(fmt.Sprintf("  IPv6: %s\n", strings.Join(result.DNSInfo.IPv6, ", ")))
			}
			if len(result.DNSInfo.CNAME) > 0 {
				report.WriteString(fmt.Sprintf("  CNAME: %s\n", strings.Join(result.DNSInfo.CNAME, ", ")))
			}
		}
		
		report.WriteString("\n")
	}
	
	return os.WriteFile(reportFile, []byte(report.String()), 0644)
}

// ReadJSON reads and parses JSON output from tools
func (f *DefaultFileManager) ReadJSON(jsonFile string, target interface{}) error {
	data, err := os.ReadFile(jsonFile)
	if err != nil {
		return fmt.Errorf("failed to read JSON file: %w", err)
	}
	
	return json.Unmarshal(data, target)
}

// WriteJSON writes data as JSON to a file
func (f *DefaultFileManager) WriteJSON(jsonFile string, data interface{}) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(jsonFile), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	
	return os.WriteFile(jsonFile, jsonData, 0644)
}