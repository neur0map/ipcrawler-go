package agents

import (
	"encoding/json"
	"fmt"
	"io/ioutil" 
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	
	"ipcrawler/internal/utils"
)

// ReporterAgent generates final reports from reviewed data
type ReporterAgent struct {
	*BaseAgent
	config *ReporterConfig
}

// ReporterConfig holds configuration for the reporter agent
type ReporterConfig struct {
	Templates       []string          `yaml:"templates"`
	IncludeRawData  bool              `yaml:"include_raw_data"`
	OutputFormats   []string          `yaml:"output_formats"`
	CompressReports bool              `yaml:"compress_reports"`
	MaxReportSize   int64             `yaml:"max_report_size"`
	CustomFields    map[string]string `yaml:"custom_fields"`
}

// DefaultReporterConfig returns default configuration
func DefaultReporterConfig() *ReporterConfig {
	return &ReporterConfig{
		Templates:       []string{"executive", "technical"},
		IncludeRawData:  true,
		OutputFormats:   []string{"txt", "md", "json"},
		CompressReports: false,
		MaxReportSize:   10 * 1024 * 1024, // 10MB
		CustomFields:    make(map[string]string),
	}
}

// NewReporterAgent creates a new reporter agent
func NewReporterAgent(config *ReporterConfig) *ReporterAgent {
	if config == nil {
		config = DefaultReporterConfig()
	}
	
	return &ReporterAgent{
		BaseAgent: NewBaseAgent("reporter", nil),
		config:    config,
	}
}

// Validate checks if the reporter agent is properly configured
func (r *ReporterAgent) Validate() error {
	if r.config == nil {
		return fmt.Errorf("reporter config is required")
	}
	if len(r.config.Templates) == 0 {
		return fmt.Errorf("at least one report template must be specified")
	}
	return nil
}

// Process generates final reports from reviewed data
func (r *ReporterAgent) Process(input *AgentInput) (*AgentOutput, error) {
	if err := r.ValidateInput(input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	
	r.LogInfo("Generating reports for target: %s", input.Target)
	
	output := r.CreateOutput(nil, input.Metadata, true)
	
	// Extract reviewed data
	reviewerResult, ok := input.Data.(*ReviewerResult)
	if !ok {
		return nil, fmt.Errorf("invalid input data format - expected ReviewerResult")
	}
	
	// Collect all data for reporting
	reportData := r.collectReportData(input, reviewerResult)
	
	// Generate reports based on configured templates
	reports := make(map[string]*GeneratedReport)
	
	for _, templateName := range r.config.Templates {
		r.LogInfo("Generating %s report", templateName)
		
		report, err := r.generateReport(templateName, reportData)
		if err != nil {
			r.LogError("Failed to generate %s report: %v", templateName, err)
			output.AddError(fmt.Errorf("failed to generate %s report: %w", templateName, err))
			continue
		}
		
		reports[templateName] = report
	}
	
	// Save reports
	summaryDir := filepath.Join(input.ReportDir, "summary")
	for templateName, report := range reports {
		if err := r.saveReport(summaryDir, templateName, report); err != nil {
			r.LogError("Failed to save %s report: %v", templateName, err)
			output.AddError(fmt.Errorf("failed to save %s report: %w", templateName, err))
		}
	}
	
	// Generate final index/summary
	indexReport := r.generateIndexReport(reportData, reports)
	if err := r.saveIndexReport(summaryDir, indexReport); err != nil {
		r.LogError("Failed to save index report: %v", err)
		output.AddError(fmt.Errorf("failed to save index report: %w", err))
	}
	
	result := &ReporterResult{
		Reports:     reports,
		IndexReport: indexReport,
		ReportData:  reportData,
		Generated:   time.Now(),
	}
	
	output.Data = result
	output.Metadata["reports_generated"] = fmt.Sprintf("%d", len(reports))
	output.Metadata["total_findings"] = fmt.Sprintf("%d", reportData.TotalFindings)
	output.Metadata["critical_findings"] = fmt.Sprintf("%d", reportData.CriticalFindings)
	
	r.LogInfo("Report generation completed. Generated %d reports", len(reports))
	
	return output, nil
}

// ReportData aggregates all data needed for report generation
type ReportData struct {
	Target              string                    `json:"target"`
	ScanTime            time.Time                 `json:"scan_time"`
	Templates           string                    `json:"template"`
	ValidationPassed    bool                      `json:"validation_passed"`
	ValidationScore     int                       `json:"validation_score"`
	ValidationSummary   string                    `json:"validation_summary"`
	
	// Aggregated findings
	TotalFindings       int                       `json:"total_findings"`
	CriticalFindings    int                       `json:"critical_findings"`
	HighFindings        int                       `json:"high_findings"`
	MediumFindings      int                       `json:"medium_findings"`
	LowFindings         int                       `json:"low_findings"`
	
	// Tool-specific data
	PortScanResults     *PortScanSummary         `json:"port_scan_results,omitempty"`
	VulnScanResults     *VulnScanSummary         `json:"vuln_scan_results,omitempty"`
	DirectoryScanResults *DirectoryScanSummary   `json:"directory_scan_results,omitempty"`
	
	// Raw data references
	RawDataFiles        []string                  `json:"raw_data_files"`
	ProcessedDataFiles  []string                  `json:"processed_data_files"`
}

// Summary structures for different tool types
type PortScanSummary struct {
	OpenPorts       int      `json:"open_ports"`
	Services        []string `json:"services"`
	OSDetected      string   `json:"os_detected,omitempty"`
	HighRiskPorts   []int    `json:"high_risk_ports"`
}

type VulnScanSummary struct {
	TotalVulns      int      `json:"total_vulns"`
	CriticalVulns   int      `json:"critical_vulns"`
	HighVulns       int      `json:"high_vulns"`
	TopCVEs         []string `json:"top_cves"`
	AffectedServices []string `json:"affected_services"`
}

type DirectoryScanSummary struct {
	PathsFound       int      `json:"paths_found"`
	InterestingFiles int      `json:"interesting_files"`
	AdminPaths       []string `json:"admin_paths"`
	ConfigFiles      []string `json:"config_files"`
}

// GeneratedReport represents a generated report
type GeneratedReport struct {
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Format      string    `json:"format"`
	Generated   time.Time `json:"generated"`
	Size        int64     `json:"size"`
}

// ReporterResult represents the complete reporter output
type ReporterResult struct {
	Reports     map[string]*GeneratedReport `json:"reports"`
	IndexReport *GeneratedReport           `json:"index_report"`
	ReportData  *ReportData                `json:"report_data"`
	Generated   time.Time                  `json:"generated"`
}

// collectReportData aggregates data from all sources
func (r *ReporterAgent) collectReportData(input *AgentInput, reviewerResult *ReviewerResult) *ReportData {
	// Get template name from metadata or set default
	templateName := input.Metadata["template"]
	if templateName == "" {
		templateName = "basic"
	}
	
	data := &ReportData{
		Target:             input.Target,
		ScanTime:           time.Now(),
		Templates:          templateName,
		ValidationPassed:   reviewerResult.Passed,
		ValidationScore:    reviewerResult.TotalScore,
		ValidationSummary:  reviewerResult.Summary,
		RawDataFiles:       make([]string, 0),
		ProcessedDataFiles: make([]string, 0),
	}
	
	// Extract data from cleaner outputs that came through the reviewer
	// Note: The actual cleaner data structure needs to be properly parsed
	// For now, we'll extract what we can from the input metadata and validation results
	
	// Count findings from validation results as a proxy for actual findings
	data.TotalFindings = len(reviewerResult.ValidationResults)
	data.CriticalFindings = r.countValidationsBySeverity(reviewerResult.ValidationResults, "critical")
	data.HighFindings = r.countValidationsBySeverity(reviewerResult.ValidationResults, "high")
	data.MediumFindings = r.countValidationsBySeverity(reviewerResult.ValidationResults, "medium")
	data.LowFindings = r.countValidationsBySeverity(reviewerResult.ValidationResults, "low")
	
	// Try to load actual nmap data from processed files
	nmapDataPath := filepath.Join(input.ReportDir, "processed", "nmap_cleaned.json")
	if nmapData := r.loadNmapData(nmapDataPath); nmapData != nil {
		services := make([]string, 0)
		highRiskPorts := make([]int, 0)
		
		for _, port := range nmapData.Ports {
			if port.State == "open" {
				if port.Service != "" {
					services = append(services, fmt.Sprintf("%s (%d/%s)", port.Service, port.Number, port.Protocol))
				}
				// Check if port is commonly high-risk
				highRiskPortNumbers := []int{21, 22, 23, 25, 135, 139, 445, 1433, 3306, 3389, 5432, 5900}
				for _, riskPort := range highRiskPortNumbers {
					if port.Number == riskPort {
						highRiskPorts = append(highRiskPorts, port.Number)
						r.LogInfo("High-risk port detected: %d", port.Number)
						break
					}
				}
			}
		}
		
		data.PortScanResults = &PortScanSummary{
			OpenPorts:     len(nmapData.Ports),
			Services:      services,
			HighRiskPorts: highRiskPorts,
		}
		
		// Extract OS info if available
		if nmapData.OSDetection != nil && nmapData.OSDetection.Name != "" {
			data.PortScanResults.OSDetected = nmapData.OSDetection.Name
		}
	} else if ports, exists := input.Metadata["ports_found"]; exists {
		// Fallback to metadata if file not available
		data.PortScanResults = &PortScanSummary{
			OpenPorts:     r.parseIntFromString(ports),
			Services:      []string{},
			HighRiskPorts: []int{},
		}
	}
	
	if vulns, exists := input.Metadata["vulnerabilities_found"]; exists {
		data.VulnScanResults = &VulnScanSummary{
			TotalVulns:    r.parseIntFromString(vulns),
			CriticalVulns: r.parseIntFromString(input.Metadata["critical_findings"]),
			HighVulns:     r.parseIntFromString(input.Metadata["high_findings"]),
			TopCVEs:       []string{}, // Would be populated from actual cleaner data
			AffectedServices: []string{}, // Would be populated from actual cleaner data
		}
	}
	
	if paths, exists := input.Metadata["paths_found"]; exists {
		data.DirectoryScanResults = &DirectoryScanSummary{
			PathsFound:       r.parseIntFromString(paths),
			InterestingFiles: r.parseIntFromString(input.Metadata["interesting_files"]),
			AdminPaths:       []string{}, // Would be populated from actual cleaner data
			ConfigFiles:      []string{}, // Would be populated from actual cleaner data
		}
	}
	
	return data
}

// generateReport generates a specific type of report
func (r *ReporterAgent) generateReport(templateName string, data *ReportData) (*GeneratedReport, error) {
	switch templateName {
	case "executive":
		return r.generateExecutiveReport(data)
	case "technical":
		return r.generateTechnicalReport(data)
	case "summary":
		return r.generateSummaryReport(data)
	default:
		return nil, fmt.Errorf("unknown report template: %s", templateName)
	}
}

// generateExecutiveReport creates an executive summary report
func (r *ReporterAgent) generateExecutiveReport(data *ReportData) (*GeneratedReport, error) {
	var content strings.Builder
	
	content.WriteString("EXECUTIVE SECURITY ASSESSMENT SUMMARY\n")
	content.WriteString("=====================================\n\n")
	
	content.WriteString(fmt.Sprintf("Target: %s\n", data.Target))
	content.WriteString(fmt.Sprintf("Assessment Date: %s\n", data.ScanTime.Format("January 2, 2006")))
	content.WriteString(fmt.Sprintf("Template Used: %s\n\n", data.Templates))
	
	// Risk Assessment
	riskLevel := r.calculateOverallRisk(data)
	content.WriteString("OVERALL RISK ASSESSMENT\n")
	content.WriteString("-----------------------\n")
	content.WriteString(fmt.Sprintf("Risk Level: %s\n\n", strings.ToUpper(riskLevel)))
	
	// Key Findings
	content.WriteString("KEY FINDINGS\n")
	content.WriteString("------------\n")
	content.WriteString(fmt.Sprintf("• Total Security Issues: %d\n", data.TotalFindings))
	content.WriteString(fmt.Sprintf("• Critical Issues: %d\n", data.CriticalFindings))
	content.WriteString(fmt.Sprintf("• High Priority Issues: %d\n", data.HighFindings))
	content.WriteString(fmt.Sprintf("• Medium Priority Issues: %d\n", data.MediumFindings))
	content.WriteString("\n")
	
	// Recommendations
	content.WriteString("IMMEDIATE ACTIONS REQUIRED\n")
	content.WriteString("--------------------------\n")
	recommendations := r.generateRecommendations(data)
	for i, rec := range recommendations {
		content.WriteString(fmt.Sprintf("%d. %s\n", i+1, rec))
	}
	content.WriteString("\n")
	
	// Validation Status
	content.WriteString("DATA QUALITY ASSESSMENT\n")
	content.WriteString("-----------------------\n")
	if data.ValidationPassed {
		content.WriteString("✅ All data quality checks passed\n")
	} else {
		content.WriteString(fmt.Sprintf("⚠️  Data quality issues detected (Score: %d)\n", data.ValidationScore))
		content.WriteString(fmt.Sprintf("Summary: %s\n", data.ValidationSummary))
	}
	
	report := &GeneratedReport{
		Type:      "executive",
		Title:     "Executive Security Assessment Summary",
		Content:   content.String(),
		Format:    "txt",
		Generated: time.Now(),
		Size:      int64(len(content.String())),
	}
	
	return report, nil
}

// generateTechnicalReport creates a detailed technical report
func (r *ReporterAgent) generateTechnicalReport(data *ReportData) (*GeneratedReport, error) {
	var content strings.Builder
	
	content.WriteString("# Technical Security Assessment Report\n\n")
	
	content.WriteString(fmt.Sprintf("**Target:** %s  \n", data.Target))
	content.WriteString(fmt.Sprintf("**Assessment Date:** %s  \n", data.ScanTime.Format("January 2, 2006 15:04 MST")))
	content.WriteString(fmt.Sprintf("**Template:** %s  \n", data.Templates))
	content.WriteString(fmt.Sprintf("**Validation Status:** %s  \n\n", r.getValidationStatus(data)))
	
	// Methodology
	content.WriteString("## Assessment Methodology\n\n")
	content.WriteString("This assessment was conducted using automated security scanning tools ")
	content.WriteString("with comprehensive validation and quality assurance processes.\n\n")
	
	// Detailed Findings
	content.WriteString("## Detailed Findings\n\n")
	
	if data.PortScanResults != nil {
		content.WriteString("### Network Services Analysis\n\n")
		content.WriteString(fmt.Sprintf("- **Open Ports:** %d\n", data.PortScanResults.OpenPorts))
		content.WriteString(fmt.Sprintf("- **Detected Services:** %s\n", strings.Join(data.PortScanResults.Services, ", ")))
		if data.PortScanResults.OSDetected != "" {
			content.WriteString(fmt.Sprintf("- **Operating System:** %s\n", data.PortScanResults.OSDetected))
		}
		content.WriteString("\n")
	}
	
	if data.VulnScanResults != nil {
		content.WriteString("### Vulnerability Analysis\n\n")
		content.WriteString(fmt.Sprintf("- **Total Vulnerabilities:** %d\n", data.VulnScanResults.TotalVulns))
		content.WriteString(fmt.Sprintf("- **Critical:** %d\n", data.VulnScanResults.CriticalVulns))
		content.WriteString(fmt.Sprintf("- **High:** %d\n", data.VulnScanResults.HighVulns))
		if len(data.VulnScanResults.TopCVEs) > 0 {
			content.WriteString(fmt.Sprintf("- **Notable CVEs:** %s\n", strings.Join(data.VulnScanResults.TopCVEs, ", ")))
		}
		content.WriteString("\n")
	}
	
	if data.DirectoryScanResults != nil {
		content.WriteString("### Web Application Analysis\n\n")
		content.WriteString(fmt.Sprintf("- **Discovered Paths:** %d\n", data.DirectoryScanResults.PathsFound))
		content.WriteString(fmt.Sprintf("- **Interesting Files:** %d\n", data.DirectoryScanResults.InterestingFiles))
		if len(data.DirectoryScanResults.AdminPaths) > 0 {
			content.WriteString(fmt.Sprintf("- **Admin Interfaces:** %s\n", strings.Join(data.DirectoryScanResults.AdminPaths, ", ")))
		}
		content.WriteString("\n")
	}
	
	// Data Sources
	content.WriteString("## Data Sources\n\n")
	if len(data.RawDataFiles) > 0 {
		content.WriteString("**Raw Data Files:**\n")
		for _, file := range data.RawDataFiles {
			content.WriteString(fmt.Sprintf("- %s\n", file))
		}
		content.WriteString("\n")
	}
	
	report := &GeneratedReport{
		Type:      "technical",
		Title:     "Technical Security Assessment Report",
		Content:   content.String(),
		Format:    "md",
		Generated: time.Now(),
		Size:      int64(len(content.String())),
	}
	
	return report, nil
}

// generateSummaryReport creates a brief summary report
func (r *ReporterAgent) generateSummaryReport(data *ReportData) (*GeneratedReport, error) {
	var content strings.Builder
	
	content.WriteString("SECURITY ASSESSMENT SUMMARY\n")
	content.WriteString("===========================\n\n")
	
	content.WriteString(fmt.Sprintf("Target: %s\n", data.Target))
	content.WriteString(fmt.Sprintf("Date: %s\n", data.ScanTime.Format("2006-01-02")))
	content.WriteString(fmt.Sprintf("Findings: %d total (%d critical, %d high)\n", 
		data.TotalFindings, data.CriticalFindings, data.HighFindings))
	
	report := &GeneratedReport{
		Type:      "summary",
		Title:     "Security Assessment Summary",
		Content:   content.String(),
		Format:    "txt",
		Generated: time.Now(),
		Size:      int64(len(content.String())),
	}
	
	return report, nil
}

// generateIndexReport creates an index of all reports
func (r *ReporterAgent) generateIndexReport(data *ReportData, reports map[string]*GeneratedReport) *GeneratedReport {
	var content strings.Builder
	
	content.WriteString("SECURITY ASSESSMENT REPORT INDEX\n")
	content.WriteString("=================================\n\n")
	
	content.WriteString(fmt.Sprintf("Target: %s\n", data.Target))
	content.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	
	content.WriteString("AVAILABLE REPORTS\n")
	content.WriteString("-----------------\n")
	
	for reportType, report := range reports {
		content.WriteString(fmt.Sprintf("• %s (%s) - %s\n", 
			report.Title, strings.ToUpper(report.Format), reportType))
		content.WriteString(fmt.Sprintf("  Size: %d bytes, Generated: %s\n", 
			report.Size, report.Generated.Format("15:04:05")))
	}
	
	return &GeneratedReport{
		Type:      "index",
		Title:     "Report Index",
		Content:   content.String(),
		Format:    "txt",
		Generated: time.Now(),
		Size:      int64(len(content.String())),
	}
}

// Helper functions

// Removed - using database-based calculateOverallRisk function below

func (r *ReporterAgent) generateRecommendations(data *ReportData) []string {
	recommendations := make([]string, 0)
	
	if data.CriticalFindings > 0 {
		recommendations = append(recommendations, "Address all critical security vulnerabilities immediately")
	}
	
	if data.HighFindings > 0 {
		recommendations = append(recommendations, "Review and remediate high-priority security issues")
	}
	
	if !data.ValidationPassed {
		recommendations = append(recommendations, "Review scan results for data quality issues")
	}
	
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Continue monitoring and maintain current security posture")
	}
	
	return recommendations
}

func (r *ReporterAgent) getValidationStatus(data *ReportData) string {
	if data.ValidationPassed {
		return "✅ Passed"
	}
	return fmt.Sprintf("⚠️ Issues Detected (Score: %d)", data.ValidationScore)
}

// File I/O functions (placeholders)

func (r *ReporterAgent) saveReport(summaryDir, templateName string, report *GeneratedReport) error {
	// Create summary directory if it doesn't exist
	if err := os.MkdirAll(summaryDir, 0755); err != nil {
		return fmt.Errorf("failed to create summary directory: %w", err)
	}
	
	// Determine file extension based on format
	ext := ".txt"
	if report.Format == "md" {
		ext = ".md"
	} else if report.Format == "json" {
		ext = ".json"
	}
	
	// Write to file
	filePath := filepath.Join(summaryDir, fmt.Sprintf("%s_report%s", templateName, ext))
	if err := utils.WriteFileWithPermissions(filePath, []byte(report.Content), 0644); err != nil {
		return fmt.Errorf("failed to write %s report: %w", templateName, err)
	}
	
	r.LogInfo("Saved %s report to: %s", templateName, filePath)
	return nil
}

func (r *ReporterAgent) saveIndexReport(summaryDir string, report *GeneratedReport) error {
	// Create summary directory if it doesn't exist
	if err := os.MkdirAll(summaryDir, 0755); err != nil {
		return fmt.Errorf("failed to create summary directory: %w", err)
	}
	
	// Write to file
	filePath := filepath.Join(summaryDir, "index.txt")
	if err := utils.WriteFileWithPermissions(filePath, []byte(report.Content), 0644); err != nil {
		return fmt.Errorf("failed to write index report: %w", err)
	}
	
	r.LogInfo("Saved index report to: %s", filePath)
	return nil
}

// countValidationsBySeverity counts validation results by severity level
func (r *ReporterAgent) countValidationsBySeverity(results []ValidationResult, severity string) int {
	count := 0
	for _, result := range results {
		if !result.Passed && strings.ToLower(result.Severity) == strings.ToLower(severity) {
			count++
		}
	}
	return count
}

// parseIntFromString safely parses integer from string, returns 0 if parsing fails
func (r *ReporterAgent) parseIntFromString(s string) int {
	if s == "" {
		return 0
	}
	result, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return result
}

// NmapDataForReporting represents the structure we need from nmap_cleaned.json
type NmapDataForReporting struct {
	Target      string `json:"target"`
	ScanTime    string `json:"scan_time"`
	Ports       []struct {
		Number   int    `json:"number"`
		Protocol string `json:"protocol"`
		State    string `json:"state"`
		Service  string `json:"service"`
		Version  string `json:"version,omitempty"`
		Product  string `json:"product,omitempty"`
	} `json:"ports"`
	Services []struct {
		Name        string `json:"name"`
		Port        int    `json:"port"`
		Protocol    string `json:"protocol"`
		Version     string `json:"version,omitempty"`
		Product     string `json:"product,omitempty"`
		ExtraInfo   string `json:"extra_info,omitempty"`
	} `json:"services"`
	OSDetection *struct {
		Name       string  `json:"name,omitempty"`
		Accuracy   int     `json:"accuracy,omitempty"`
		Line       string  `json:"line,omitempty"`
	} `json:"os_detection,omitempty"`
}

// loadNmapData loads and parses nmap data from the cleaned JSON file
func (r *ReporterAgent) loadNmapData(filePath string) *NmapDataForReporting {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil
	}
	
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		r.LogError("Failed to read nmap data file %s: %v", filePath, err)
		return nil
	}
	
	var nmapData NmapDataForReporting
	if err := json.Unmarshal(data, &nmapData); err != nil {
		r.LogError("Failed to parse nmap data from %s: %v", filePath, err)
		return nil
	}
	
	return &nmapData
}

// calculateOverallRisk calculates the overall risk level for the target
func (r *ReporterAgent) calculateOverallRisk(data *ReportData) string {
	if data.PortScanResults == nil {
		return "unknown"
	}
	
	highRiskCount := len(data.PortScanResults.HighRiskPorts)
	totalPorts := data.PortScanResults.OpenPorts
	
	// Simple risk calculation based on high-risk ports
	if highRiskCount >= 3 {
		return "high"
	} else if highRiskCount > 0 {
		return "medium"
	} else if totalPorts > 10 {
		return "medium"
	} else {
		return "low"
	}
}