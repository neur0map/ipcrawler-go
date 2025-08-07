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

// DefaultReportGenerator implements report generation
func (r *DefaultReportGenerator) GenerateSummary(results []scanners.ScanResult, outputFile string) error {
	var summary strings.Builder
	
	summary.WriteString("IPCrawler Scan Summary\n")
	summary.WriteString("======================\n\n")
	
	// Calculate totals
	var totalPorts, totalServices, successfulScans int
	var target string
	var totalDuration time.Duration
	dnsResolved := false
	
	for _, result := range results {
		if target == "" {
			target = result.Target
		}
		totalPorts += len(result.Ports)
		totalServices += len(result.Services)
		totalDuration += result.Duration
		
		if result.Success {
			successfulScans++
		}
		
		if result.DNSInfo != nil && len(result.DNSInfo.IPv4) > 0 {
			dnsResolved = true
		}
	}
	
	summary.WriteString(fmt.Sprintf("Target: %s\n", target))
	summary.WriteString(fmt.Sprintf("Scan Duration: %v\n", totalDuration.Round(time.Millisecond)))
	summary.WriteString(fmt.Sprintf("Successful Scans: %d/%d\n", successfulScans, len(results)))
	summary.WriteString(fmt.Sprintf("DNS Resolved: %t\n", dnsResolved))
	summary.WriteString(fmt.Sprintf("Total Open Ports: %d\n", totalPorts))
	summary.WriteString(fmt.Sprintf("Services Detected: %d\n\n", totalServices))
	
	// Scanner breakdown
	summary.WriteString("Scanner Breakdown:\n")
	for _, result := range results {
		status := "FAILED"
		if result.Success {
			status = "SUCCESS"
		}
		
		summary.WriteString(fmt.Sprintf("  %-10s: %s (%v) - %d results\n", 
			result.Scanner, 
			status, 
			result.Duration.Round(time.Millisecond),
			len(result.Ports)+len(result.Services)))
	}
	
	// Top ports if any found
	if totalPorts > 0 {
		summary.WriteString("\nTop Discovered Ports:\n")
		portCounts := make(map[int]int)
		for _, result := range results {
			for _, port := range result.Ports {
				if port.State == "open" {
					portCounts[port.Number]++
				}
			}
		}
		
		count := 0
		for port := range portCounts {
			if count >= 10 { // Top 10 ports
				break
			}
			summary.WriteString(fmt.Sprintf("  %d/tcp\n", port))
			count++
		}
	}
	
	return os.WriteFile(outputFile, []byte(summary.String()), 0644)
}

// GenerateDetailedReport creates a detailed report with all findings
func (r *DefaultReportGenerator) GenerateDetailedReport(results []scanners.ScanResult, outputFile string) error {
	return GlobalFiles.WriteReport(outputFile, results)
}

// GenerateJSONReport creates a machine-readable JSON report
func (r *DefaultReportGenerator) GenerateJSONReport(results []scanners.ScanResult, outputFile string) error {
	// Create structured JSON report
	report := struct {
		Timestamp   time.Time             `json:"timestamp"`
		Target      string               `json:"target"`
		Duration    time.Duration        `json:"duration"`
		Summary     ReportSummary        `json:"summary"`
		ScanResults []scanners.ScanResult `json:"scan_results"`
	}{
		Timestamp:   time.Now(),
		ScanResults: results,
	}
	
	// Calculate summary
	var totalPorts, totalServices int
	var totalDuration time.Duration
	var target string
	dnsResolved := false
	
	for _, result := range results {
		if target == "" {
			target = result.Target
		}
		totalPorts += len(result.Ports)
		totalServices += len(result.Services)
		totalDuration += result.Duration
		
		if result.DNSInfo != nil && len(result.DNSInfo.IPv4) > 0 {
			dnsResolved = true
		}
	}
	
	report.Target = target
	report.Duration = totalDuration
	report.Summary = ReportSummary{
		TotalPorts:   totalPorts,
		TotalServices: totalServices,
		DNSResolved:  dnsResolved,
		ScanCount:    len(results),
	}
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	
	return os.WriteFile(outputFile, jsonData, 0644)
}

// ReportSummary provides high-level statistics
type ReportSummary struct {
	TotalPorts    int  `json:"total_ports"`
	TotalServices int  `json:"total_services"`
	DNSResolved   bool `json:"dns_resolved"`
	ScanCount     int  `json:"scan_count"`
}