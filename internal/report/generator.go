package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/carlosm/ipcrawler/internal/services"
	"gopkg.in/yaml.v3"
)

type ReportGenerator struct {
	target       string
	outputDir    string
	reportDir    string
	reportConfig *ReportConfig
	serviceDescs *ServiceDescriptions
	dnsConfig    *DNSConfig
}

// Database structures
type ReportConfig struct {
	FilePatterns      map[string]FilePattern       `yaml:"file_patterns"`
	RiskEmojis        map[string]string            `yaml:"risk_emojis"`
	ServiceCategories map[string]ServiceCategory   `yaml:"service_categories"`
	RiskThresholds    RiskThresholds              `yaml:"risk_thresholds"`
	ReportFormat      ReportFormat                `yaml:"report_format"`
}

type FilePattern struct {
	Patterns      []string `yaml:"patterns"`
	SectionTitle  string   `yaml:"section_title"`
	SectionEmoji  string   `yaml:"section_emoji"`
	Description   string   `yaml:"description"`
}

type ServiceCategory struct {
	Patterns    []string `yaml:"patterns"`
	Description string   `yaml:"description"`
	Warning     bool     `yaml:"warning"`
}

type RiskThresholds struct {
	HighRiskPortCount int   `yaml:"high_risk_port_count"`
	LegacyProtocols   []int `yaml:"legacy_protocols"`
}

type ReportFormat struct {
	TitleEmoji     string            `yaml:"title_emoji"`
	SectionEmojis  map[string]string `yaml:"section_emojis"`
	SubsectionEmojis map[string]string `yaml:"subsection_emojis"`
}

type ServiceDescriptions struct {
	ServiceDescriptions   map[string]string                    `yaml:"service_descriptions"`
	ServiceSecurityContext map[string]ServiceSecurityContext   `yaml:"service_security_context"`
}

type ServiceSecurityContext struct {
	RiskLevel      string `yaml:"risk_level"`
	SecurityNote   string `yaml:"security_note"`
	Recommendation string `yaml:"recommendation"`
}

type DNSConfig struct {
	DNSRecordTypes   map[string]string         `yaml:"dns_record_types"`
	DNSDisplayConfig map[string]DNSDisplayInfo `yaml:"dns_display_config"`
	ProcessingRules  ProcessingRules           `yaml:"processing_rules"`
}

type DNSDisplayInfo struct {
	Title             string `yaml:"title"`
	Format            string `yaml:"format"`
	SecurityRelevance string `yaml:"security_relevance"`
	Note              string `yaml:"note"`
}

type ProcessingRules struct {
	IgnoreComments  bool   `yaml:"ignore_comments"`
	CommentMarker   string `yaml:"comment_marker"`
	TrimWhitespace  bool   `yaml:"trim_whitespace"`
	SkipEmptyLines  bool   `yaml:"skip_empty_lines"`
}

func NewReportGenerator(target, outputDir, reportDir string) *ReportGenerator {
	rg := &ReportGenerator{
		target:    target,
		outputDir: outputDir,
		reportDir: reportDir,
	}
	
	// Load database configurations
	rg.loadDatabaseConfigs()
	
	return rg
}

func (r *ReportGenerator) GenerateReports() error {
	reportPath := filepath.Join(r.reportDir, r.target)
	if err := os.MkdirAll(reportPath, 0755); err != nil {
		return fmt.Errorf("creating report directory: %w", err)
	}
	
	if err := r.generateSummaryReport(); err != nil {
		return fmt.Errorf("generating summary report: %w", err)
	}
	
	return nil
}

// loadDatabaseConfigs loads all database configuration files
func (r *ReportGenerator) loadDatabaseConfigs() {
	// Load report configuration
	if data, err := os.ReadFile("config/reports.yaml"); err == nil {
		r.reportConfig = &ReportConfig{}
		yaml.Unmarshal(data, r.reportConfig)
	}
	
	// Load service descriptions
	if data, err := os.ReadFile("data/descriptions.yaml"); err == nil {
		r.serviceDescs = &ServiceDescriptions{}
		yaml.Unmarshal(data, r.serviceDescs)
	}
	
	// Load DNS configuration
	if data, err := os.ReadFile("data/dns.yaml"); err == nil {
		r.dnsConfig = &DNSConfig{}
		yaml.Unmarshal(data, r.dnsConfig)
	}
	
	// Set defaults if loading failed
	if r.reportConfig == nil {
		r.reportConfig = r.getDefaultReportConfig()
	}
	if r.serviceDescs == nil {
		r.serviceDescs = r.getDefaultServiceDescriptions()
	}
	if r.dnsConfig == nil {
		r.dnsConfig = r.getDefaultDNSConfig()
	}
}

func (r *ReportGenerator) generateSummaryReport() error {
	outputPath := filepath.Join(r.outputDir, r.target)
	reportPath := filepath.Join(r.reportDir, r.target, "summary.md")
	
	var content strings.Builder
	titleEmoji := r.reportConfig.ReportFormat.TitleEmoji
	content.WriteString(fmt.Sprintf("# %s IPCrawler Security Scan Report: %s\n\n", titleEmoji, r.target))
	content.WriteString(fmt.Sprintf("**Generated:** %s\n", time.Now().Format("2006-01-02 15:04:05 MST")))
	content.WriteString(fmt.Sprintf("**Scan Time:** %s\n", time.Now().Format("2006-01-02 15:04:05 MST")))
	content.WriteString("---\n\n")
	
	// Load database for service lookups
	db, _ := services.LoadDatabase()
	
	// Dynamically discover all available data files
	availableFiles := r.discoverOutputFiles(outputPath)
	
	// Section 1: Executive Summary with all key discoveries
	execEmoji := r.reportConfig.ReportFormat.SectionEmojis["executive_summary"]
	content.WriteString(fmt.Sprintf("## %s Executive Summary\n\n", execEmoji))
	
	// Collect all findings dynamically
	portResults := r.parsePortScanResults(outputPath, availableFiles)
	dnsResults := r.parseDNSResults(outputPath, availableFiles)
	nmapResults := r.parseServiceFingerprints(outputPath, availableFiles)
	
	// Quick stats box
	statsEmoji := r.reportConfig.ReportFormat.SubsectionEmojis["stats"]
	content.WriteString(fmt.Sprintf("### %s Quick Stats\n", statsEmoji))
	content.WriteString("```\n")
	content.WriteString(fmt.Sprintf("Target:         %s\n", r.target))
	if len(dnsResults["A"]) > 0 {
		content.WriteString(fmt.Sprintf("IP Addresses:   %d discovered\n", len(dnsResults["A"])))
	}
	if len(portResults) > 0 {
		// Count unique ports consistently with Total Discoveries
		uniquePortsQuick := make(map[string]bool)
		for _, port := range portResults {
			key := fmt.Sprintf("%s:%d", port.IP, port.Port)
			uniquePortsQuick[key] = true
		}
		content.WriteString(fmt.Sprintf("Open Ports:     %d found\n", len(uniquePortsQuick)))
	}
	content.WriteString(fmt.Sprintf("Scan Date:      %s\n", time.Now().Format("2006-01-02")))
	content.WriteString("```\n\n")
	
	// Section 2: DNS Discovery Results - Dynamic based on available files
	if len(dnsResults) > 0 {
		pattern := r.reportConfig.FilePatterns["dns_resolution"]
		content.WriteString(fmt.Sprintf("## %s %s\n", pattern.SectionEmoji, pattern.SectionTitle))
		content.WriteString(fmt.Sprintf("**Description:** %s\n", pattern.Description))
		
		// List actual tools used based on discovered files
		dnsFiles := availableFiles["dns_resolution"]
		if len(dnsFiles) > 0 {
			var toolNames []string
			for _, filePath := range dnsFiles {
				filename := filepath.Base(filePath)
				if strings.HasPrefix(filename, "dig_") {
					toolNames = append(toolNames, "dig")
				} else if strings.HasPrefix(filename, "nslookup_") {
					toolNames = append(toolNames, "nslookup")
				} else if strings.HasPrefix(filename, "dns_") {
					toolNames = append(toolNames, "dns lookup tool")
				}
			}
			if len(toolNames) > 0 {
				content.WriteString(fmt.Sprintf("**Tools Used:** %s\n", strings.Join(r.uniqueStrings(toolNames), ", ")))
			}
		}
		content.WriteString("\n")
		
		// A Records with IPs
		if len(dnsResults["A"]) > 0 {
			content.WriteString("### IPv4 Addresses (A Records)\n")
			for _, ip := range dnsResults["A"] {
				content.WriteString(fmt.Sprintf("- **%s**\n", ip))
			}
			content.WriteString("\n")
		}
		
		// AAAA Records
		if len(dnsResults["AAAA"]) > 0 {
			content.WriteString("### IPv6 Addresses (AAAA Records)\n")
			for _, ip := range dnsResults["AAAA"] {
				content.WriteString(fmt.Sprintf("- **%s**\n", ip))
			}
			content.WriteString("\n")
		}
		
		// MX Records
		if len(dnsResults["MX"]) > 0 {
			content.WriteString("### Mail Servers (MX Records)\n")
			for _, mx := range dnsResults["MX"] {
				content.WriteString(fmt.Sprintf("- %s\n", mx))
			}
			content.WriteString("\n")
		}
		
		// NS Records
		if len(dnsResults["NS"]) > 0 {
			content.WriteString("### Name Servers (NS Records)\n")
			for _, ns := range dnsResults["NS"] {
				content.WriteString(fmt.Sprintf("- %s\n", ns))
			}
			content.WriteString("\n")
		}
		
		// TXT Records
		if len(dnsResults["TXT"]) > 0 {
			content.WriteString("### TXT Records\n")
			for _, txt := range dnsResults["TXT"] {
				content.WriteString(fmt.Sprintf("- `%s`\n", txt))
			}
			content.WriteString("\n")
		}
	}
	
	// Section 3: Port Scan Results - Dynamic based on available files
	if len(portResults) > 0 {
		pattern := r.reportConfig.FilePatterns["port_scan"]
		content.WriteString(fmt.Sprintf("## %s %s\n", pattern.SectionEmoji, pattern.SectionTitle))
		content.WriteString(fmt.Sprintf("**Description:** %s\n", pattern.Description))
		
		// List actual tools and files used
		portFiles := availableFiles["port_scan"]
		if len(portFiles) > 0 {
			var toolNames []string
			var sourceFiles []string
			for _, filePath := range portFiles {
				filename := filepath.Base(filePath)
				sourceFiles = append(sourceFiles, filename)
				if strings.Contains(filename, "naabu") {
					toolNames = append(toolNames, "naabu")
				} else if strings.Contains(filename, "nmap") {
					toolNames = append(toolNames, "nmap")
				} else if strings.Contains(filename, "ports") {
					toolNames = append(toolNames, "port scanner")
				}
			}
			if len(toolNames) > 0 {
				content.WriteString(fmt.Sprintf("**Tools Used:** %s\n", strings.Join(r.uniqueStrings(toolNames), ", ")))
			}
			if len(sourceFiles) > 0 {
				content.WriteString(fmt.Sprintf("**Source Files:** %s\n", strings.Join(sourceFiles, ", ")))
			}
		}
		content.WriteString("\n")
		
		// Show total unique ports discovered
		uniquePorts := make(map[string]bool)
		for _, port := range portResults {
			key := fmt.Sprintf("%s:%d", port.IP, port.Port)
			uniquePorts[key] = true
		}
		content.WriteString(fmt.Sprintf("**Total Discoveries:** %d unique port/IP combinations\n\n", len(uniquePorts)))
		
		// Group ports by IP address
		portsByIP := make(map[string][]PortResult)
		for _, port := range portResults {
			portsByIP[port.IP] = append(portsByIP[port.IP], port)
		}
		
		for ip, ports := range portsByIP {
			content.WriteString(fmt.Sprintf("### Host: %s\n\n", ip))
			content.WriteString("| Port | Service | Protocol | Risk Level | Description |\n")
			content.WriteString("|------|---------|----------|------------|-------------|\n")
			
			// Remove duplicates and sort by port number
			seenPorts := make(map[int]bool)
			for _, port := range ports {
				if seenPorts[port.Port] {
					continue
				}
				seenPorts[port.Port] = true
				
				serviceName := db.GetServiceName(port.Port)
				riskLevel := db.GetRiskLevel(port.Port)
				
				// Add risk emoji from database
				riskEmoji := r.getRiskEmoji(riskLevel)
				
				// Get service description from database
				description := r.getServiceDescriptionFromDB(serviceName)
				
				content.WriteString(fmt.Sprintf("| **%d** | %s | %s | %s %s | %s |\n", 
					port.Port, serviceName, port.Protocol, riskEmoji, riskLevel, description))
			}
			content.WriteString("\n")
		}
		
		// Port Analysis
		analysisEmoji := r.reportConfig.ReportFormat.SubsectionEmojis["analysis"]
		content.WriteString(fmt.Sprintf("### %s Port Analysis\n\n", analysisEmoji))
		
		// Service categories from database - use unique ports for accurate counting
		uniquePortsList := make([]PortResult, 0, len(uniquePorts))
		seen := make(map[string]bool)
		for _, port := range portResults {
			key := fmt.Sprintf("%s:%d", port.IP, port.Port)
			if !seen[key] {
				uniquePortsList = append(uniquePortsList, port)
				seen[key] = true
			}
		}
		categoryCounts := r.categorizeServices(uniquePortsList, db)
		
		// Display category counts with warnings from database config
		for categoryName, count := range categoryCounts {
			if count > 0 {
				category := r.reportConfig.ServiceCategories[categoryName]
				warningIcon := ""
				if category.Warning {
					warningIcon = " ⚠️"
				}
				content.WriteString(fmt.Sprintf("- **%s:** %d ports%s\n", category.Description, count, warningIcon))
			}
		}
		content.WriteString("\n")
	}
	
	// Section 4: Service Fingerprinting Results - Dynamic based on available files
	if len(nmapResults) > 0 {
		pattern := r.reportConfig.FilePatterns["service_fingerprint"]
		content.WriteString(fmt.Sprintf("## %s %s\n", pattern.SectionEmoji, pattern.SectionTitle))
		content.WriteString(fmt.Sprintf("**Description:** %s\n", pattern.Description))
		
		// List actual tools and files used
		fingerprintFiles := availableFiles["service_fingerprint"]
		if len(fingerprintFiles) > 0 {
			var toolNames []string
			var sourceFiles []string
			for _, filePath := range fingerprintFiles {
				filename := filepath.Base(filePath)
				sourceFiles = append(sourceFiles, filename)
				if strings.Contains(filename, "nmap") {
					toolNames = append(toolNames, "nmap")
				} else if strings.Contains(filename, "fingerprint") {
					toolNames = append(toolNames, "fingerprint tool")
				}
			}
			if len(toolNames) > 0 {
				content.WriteString(fmt.Sprintf("**Tools Used:** %s\n", strings.Join(r.uniqueStrings(toolNames), ", ")))
			}
			if len(sourceFiles) > 0 {
				content.WriteString(fmt.Sprintf("**Source Files:** %s\n", strings.Join(sourceFiles, ", ")))
			}
		}
		content.WriteString("\n")
		
		// Display raw nmap output in code blocks for better intelligence
		for _, result := range nmapResults {
			if result.Service == "Raw nmap scan results" && result.Version != "" {
				content.WriteString(fmt.Sprintf("### Raw Nmap Results for %s\n\n", result.Host))
				content.WriteString("```\n")
				content.WriteString(result.Version)
				content.WriteString("\n```\n\n")
			} else {
				// Fallback for other result types
				if result.Port > 0 {
					content.WriteString(fmt.Sprintf("### %s:%d\n", result.Host, result.Port))
				} else {
					content.WriteString(fmt.Sprintf("### %s\n", result.Host))
				}
				if result.Service != "" {
					content.WriteString(fmt.Sprintf("- **Service:** %s\n", result.Service))
				}
				if result.Version != "" {
					content.WriteString(fmt.Sprintf("- **Version:** %s\n", result.Version))
				}
				if result.OS != "" {
					content.WriteString(fmt.Sprintf("- **OS:** %s\n", result.OS))
				}
				content.WriteString("\n")
			}
		}
	}
	
	// Section 5: Security Recommendations
	securityEmoji := r.reportConfig.ReportFormat.SectionEmojis["security_recommendations"]
	content.WriteString(fmt.Sprintf("## %s Security Recommendations\n\n", securityEmoji))
	
	hasHighRisk := false
	hasCriticalRisk := false
	for _, port := range portResults {
		riskLevel := db.GetRiskLevel(port.Port)
		if riskLevel == "high" {
			hasHighRisk = true
		}
		if riskLevel == "critical" {
			hasCriticalRisk = true
		}
	}
	
	if hasCriticalRisk {
		warningsEmoji := r.reportConfig.ReportFormat.SubsectionEmojis["warnings"]
		content.WriteString(fmt.Sprintf("### %s Critical Risk Findings\n", warningsEmoji))
		for _, port := range portResults {
			if db.GetRiskLevel(port.Port) == "critical" {
				serviceName := db.GetServiceName(port.Port)
				content.WriteString(fmt.Sprintf("- **Port %d (%s):** Immediate security review required - verify access controls and encryption\n", 
					port.Port, serviceName))
			}
		}
		content.WriteString("\n")
	}
	
	if hasHighRisk {
		warningsEmoji := r.reportConfig.ReportFormat.SubsectionEmojis["warnings"]
		content.WriteString(fmt.Sprintf("### %s High Risk Findings\n", warningsEmoji))
		for _, port := range portResults {
			if db.GetRiskLevel(port.Port) == "high" {
				serviceName := db.GetServiceName(port.Port)
				content.WriteString(fmt.Sprintf("- **Port %d (%s):** Consider implementing access controls or closing if not required\n", 
					port.Port, serviceName))
			}
		}
		content.WriteString("\n")
	}
	
	// General recommendations based on findings
	generalEmoji := r.reportConfig.ReportFormat.SubsectionEmojis["general"]
	content.WriteString(fmt.Sprintf("### %s General Recommendations\n", generalEmoji))
	// Use thresholds from database
	if len(portResults) > r.reportConfig.RiskThresholds.HighRiskPortCount {
		content.WriteString("- Large attack surface detected. Consider closing unnecessary ports\n")
	}
	
	// Check for legacy protocols from database
	legacyFound := false
	for _, port := range portResults {
		for _, legacyPort := range r.reportConfig.RiskThresholds.LegacyProtocols {
			if port.Port == legacyPort {
				legacyFound = true
				break
			}
		}
		if legacyFound {
			break
		}
	}
	if legacyFound {
		content.WriteString("- Legacy protocols detected (Telnet/FTP). Migrate to secure alternatives\n")
	}
	content.WriteString("\n")
	
	// Section 6: Raw Output Files - Dynamic listing of all discovered files
	rawFilesEmoji := r.reportConfig.ReportFormat.SectionEmojis["raw_files"]
	content.WriteString(fmt.Sprintf("## %s Raw Output Files\n\n", rawFilesEmoji))
	content.WriteString("### Available Files\n")
	
	// Dynamically list all discovered files by category
	for categoryName, pattern := range r.reportConfig.FilePatterns {
		files := availableFiles[categoryName]
		if len(files) > 0 {
			content.WriteString(fmt.Sprintf("- **%s:** ", pattern.SectionTitle))
			var relativeFiles []string
			for _, filePath := range files {
				relativeFiles = append(relativeFiles, fmt.Sprintf("`%s/%s/%s`", r.outputDir, r.target, filepath.Base(filePath)))
			}
			content.WriteString(strings.Join(relativeFiles, ", "))
			content.WriteString("\n")
		}
	}
	
	content.WriteString("\n---\n")
	content.WriteString(fmt.Sprintf("*Report generated by IPCrawler at %s*\n", time.Now().Format("15:04:05 MST")))
	
	return os.WriteFile(reportPath, []byte(content.String()), 0644)
}

// discoverOutputFiles dynamically discovers all output files in the target directory
func (r *ReportGenerator) discoverOutputFiles(outputPath string) map[string][]string {
	files := make(map[string][]string)
	
	// Read all files in the output directory
	entries, err := os.ReadDir(outputPath)
	if err != nil {
		return files
	}
	
	// Group files by pattern type from database config
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		filename := entry.Name()
		filePath := filepath.Join(outputPath, filename)
		
		// Check against all pattern types from config
		for patternType, pattern := range r.reportConfig.FilePatterns {
			for _, glob := range pattern.Patterns {
				if matched, _ := filepath.Match(glob, filename); matched {
					files[patternType] = append(files[patternType], filePath)
					break
				}
			}
		}
	}
	
	return files
}

func (r *ReportGenerator) summarizeJSONFile(filePath string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Sprintf("Error reading file: %v", err)
	}
	
	content := strings.TrimSpace(string(data))
	if content == "" {
		return "Empty file"
	}
	
	// Try to parse as standard JSON first
	var parsed interface{}
	if err := json.Unmarshal(data, &parsed); err == nil {
		switch v := parsed.(type) {
		case []interface{}:
			return fmt.Sprintf("Array with %d items", len(v))
		case map[string]interface{}:
			return fmt.Sprintf("Object with %d fields", len(v))
		default:
			return "Single value"
		}
	}
	
	// Try to parse as JSON Lines (NDJSON)
	lines := strings.Split(content, "\n")
	validJSONLines := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var obj interface{}
		if json.Unmarshal([]byte(line), &obj) == nil {
			validJSONLines++
		}
	}
	
	if validJSONLines > 0 {
		return fmt.Sprintf("JSON Lines format with %d records", validJSONLines)
	}
	
	return fmt.Sprintf("Text file with %d lines", len(lines))
}

type PortResult struct {
	IP       string
	Port     int
	Protocol string
	Host     string
}

func (r *ReportGenerator) parsePortScanResults(outputPath string, availableFiles map[string][]string) []PortResult {
	var results []PortResult
	
	// Use dynamically discovered port scan files
	portFiles := availableFiles["port_scan"]
	if len(portFiles) == 0 {
		return results
	}
	
	for _, filePath := range portFiles {
		filename := filepath.Base(filePath)
		
		// Handle different file formats based on extension and content
		if strings.HasSuffix(filename, ".json") {
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			
			content := strings.TrimSpace(string(data))
			if content == "" {
				continue
			}
			
			// Try as standard JSON array first
			var ports []map[string]interface{}
			if err := json.Unmarshal(data, &ports); err == nil {
				for _, portData := range ports {
					if result := r.parsePortData(portData); result.Port > 0 {
						results = append(results, result)
					}
				}
				continue
			}
			
			// Try as JSON Lines (NDJSON) format
			lines := strings.Split(content, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				
				var portData map[string]interface{}
				if err := json.Unmarshal([]byte(line), &portData); err == nil {
					if result := r.parsePortData(portData); result.Port > 0 {
						results = append(results, result)
					}
				}
			}
		}
	}
	
	return results
}

// parsePortData extracts PortResult from a map[string]interface{}
func (r *ReportGenerator) parsePortData(portData map[string]interface{}) PortResult {
	result := PortResult{}
	
	if ip, ok := portData["ip"].(string); ok {
		result.IP = ip
	}
	if host, ok := portData["host"].(string); ok {
		result.Host = host
	}
	if port, ok := portData["port"].(float64); ok {
		result.Port = int(port)
	}
	if protocol, ok := portData["protocol"].(string); ok {
		result.Protocol = protocol
	}
	
	return result
}

func (r *ReportGenerator) parseDNSResults(outputPath string, availableFiles map[string][]string) map[string][]string {
	dnsResults := make(map[string][]string)
	
	// Use dynamically discovered DNS files
	dnsFiles := availableFiles["dns_resolution"]
	if len(dnsFiles) == 0 {
		return dnsResults
	}
	
	for _, filePath := range dnsFiles {
		filename := filepath.Base(filePath)
		
		// Extract record type from filename (e.g., dns_a.txt -> A)
		recordType := r.extractDNSRecordType(filename)
		if recordType == "" {
			continue
		}
		
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			// Skip empty lines and comments
			if line != "" && !strings.Contains(line, ";") && !strings.HasPrefix(line, "#") {
				dnsResults[recordType] = append(dnsResults[recordType], line)
			}
		}
	}
	
	return dnsResults
}

// extractDNSRecordType extracts DNS record type from filename
func (r *ReportGenerator) extractDNSRecordType(filename string) string {
	// Extract from patterns like dns_a.txt, dns_mx.txt, etc.
	if strings.HasPrefix(filename, "dns_") && strings.HasSuffix(filename, ".txt") {
		recordType := strings.TrimPrefix(filename, "dns_")
		recordType = strings.TrimSuffix(recordType, ".txt")
		return strings.ToUpper(recordType)
	}
	return ""
}

// uniqueStrings returns unique strings from a slice
func (r *ReportGenerator) uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	var result []string
	
	for _, str := range slice {
		if !seen[str] {
			seen[str] = true
			result = append(result, str)
		}
	}
	
	return result
}

// NmapResult holds parsed nmap scan results
type NmapResult struct {
	Host    string
	Port    int
	Service string
	Version string
	OS      string
}

// parseServiceFingerprints parses service fingerprinting results from various tools
func (r *ReportGenerator) parseServiceFingerprints(outputPath string, availableFiles map[string][]string) []NmapResult {
	var results []NmapResult
	
	// Use dynamically discovered service fingerprint files
	fingerprintFiles := availableFiles["service_fingerprint"]
	if len(fingerprintFiles) == 0 {
		// No service fingerprint files found
		return results
	}
	
	for _, filePath := range fingerprintFiles {
		filename := filepath.Base(filePath)
		
		if strings.HasSuffix(filename, ".txt") && strings.Contains(filename, "nmap") {
			// Handle nmap text output files - show raw output for better intelligence
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			
			content := string(data)
			
			if strings.Contains(content, "PORT") && strings.Contains(content, "STATE") && strings.Contains(content, "SERVICE") {
				// Extract the raw nmap scan results (the important part)
				rawOutput := r.extractNmapScanResults(content)
				result := NmapResult{
					Host:    r.target,
					Port:    0, // Raw output covers multiple ports
					Service: "Raw nmap scan results",
					Version: rawOutput, // Store raw output in Version field
				}
				results = append(results, result)
			} else if strings.Contains(content, "WARNING: No targets were specified") {
				result := NmapResult{
					Host:    r.target,
					Port:    0,
					Service: "Scan attempted but input format issue detected",
					Version: fmt.Sprintf("Tool output available in %s", filename),
				}
				results = append(results, result)
			} else {
				// General nmap results - just show it ran
				result := NmapResult{
					Host:    r.target,
					Port:    0,
					Service: "Service fingerprinting completed",
					Version: fmt.Sprintf("See %s for detailed results", filename),
				}
				results = append(results, result)
			}
		} else if strings.HasSuffix(filename, ".json") {
			// Handle JSON service fingerprint files
			result := NmapResult{
				Host:    r.target,
				Service: "Service data available",
				Version: fmt.Sprintf("JSON results in %s", filename),
			}
			results = append(results, result)
		}
	}
	
	return results
}


// fileExists checks if a file exists
func (r *ReportGenerator) fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// getServiceDescriptionFromDB returns a service description from the database
func (r *ReportGenerator) getServiceDescriptionFromDB(serviceName string) string {
	if r.serviceDescs != nil {
		if desc, exists := r.serviceDescs.ServiceDescriptions[serviceName]; exists {
			return desc
		}
	}
	return serviceName
}

// getRiskEmoji returns the risk emoji from database config
func (r *ReportGenerator) getRiskEmoji(riskLevel string) string {
	if r.reportConfig != nil {
		if emoji, exists := r.reportConfig.RiskEmojis[riskLevel]; exists {
			return emoji
		}
	}
	return "⚪" // Default
}

// categorizeServices categorizes services based on database patterns
func (r *ReportGenerator) categorizeServices(portResults []PortResult, db *services.Database) map[string]int {
	categoryCounts := make(map[string]int)
	
	for _, port := range portResults {
		serviceName := db.GetServiceName(port.Port)
		
		// Check each category pattern from database
		for categoryName, category := range r.reportConfig.ServiceCategories {
			for _, pattern := range category.Patterns {
				if strings.Contains(serviceName, pattern) {
					categoryCounts[categoryName]++
					break
				}
			}
		}
	}
	
	return categoryCounts
}

// Default configurations if database files are not available
func (r *ReportGenerator) getDefaultReportConfig() *ReportConfig {
	return &ReportConfig{
		RiskEmojis: map[string]string{
			"high":    "🔴",
			"medium":  "🟡",
			"low":     "🟢",
			"unknown": "⚪",
		},
		ServiceCategories: map[string]ServiceCategory{
			"web": {Patterns: []string{"HTTP", "HTTPS"}, Description: "Web Services", Warning: false},
			"database": {Patterns: []string{"SQL", "Redis", "MongoDB"}, Description: "Database Services", Warning: true},
		},
		RiskThresholds: RiskThresholds{HighRiskPortCount: 10, LegacyProtocols: []int{21, 23}},
		ReportFormat: ReportFormat{
			TitleEmoji: "🎯",
			SectionEmojis: map[string]string{
				"executive_summary": "📊",
				"dns_discovery": "🌐",
				"port_scan": "🔍",
				"service_fingerprint": "🔬",
				"security_recommendations": "🛡️",
				"raw_files": "📁",
			},
			SubsectionEmojis: map[string]string{
				"stats": "🎯",
				"analysis": "📈",
				"warnings": "⚠️",
				"general": "📝",
			},
		},
	}
}

func (r *ReportGenerator) getDefaultServiceDescriptions() *ServiceDescriptions {
	return &ServiceDescriptions{
		ServiceDescriptions: map[string]string{
			"HTTP": "Web Server",
			"HTTPS": "Secure Web Server",
			"SSH": "Secure Shell Access",
			"Unknown": "Unidentified Service",
		},
	}
}

func (r *ReportGenerator) getDefaultDNSConfig() *DNSConfig {
	return &DNSConfig{
		DNSRecordTypes: map[string]string{
			"A": "dns_a.txt",
			"AAAA": "dns_aaaa.txt",
			"MX": "dns_mx.txt",
			"TXT": "dns_txt.txt",
			"NS": "dns_ns.txt",
		},
	}
}

// parseNmapTextOutput parses nmap text output to extract service details
func (r *ReportGenerator) parseNmapTextOutput(content string) []NmapResult {
	var results []NmapResult
	
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Look for port lines like "80/tcp   open  http     nginx 1.25.1"
		if strings.Contains(line, "/tcp") && (strings.Contains(line, "open") || strings.Contains(line, "filtered")) {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				portProto := parts[0]
				state := parts[1]
				
				if state != "open" {
					continue // Skip non-open ports
				}
				
				// Extract port number
				portStr := strings.Split(portProto, "/")[0]
				port := 0
				if p, err := strconv.Atoi(portStr); err == nil {
					port = p
				}
				
				service := ""
				version := ""
				
				if len(parts) >= 3 {
					service = parts[2]
				}
				
				// Combine remaining parts as version info
				if len(parts) > 3 {
					version = strings.Join(parts[3:], " ")
				}
				
				result := NmapResult{
					Host:    r.target,
					Port:    port,
					Service: service,
					Version: version,
				}
				results = append(results, result)
			}
		}
	}
	
	return results
}

// extractNmapScanResults extracts the raw PORT/STATE/SERVICE/VERSION table and script results from nmap output
func (r *ReportGenerator) extractNmapScanResults(content string) string {
	lines := strings.Split(content, "\n")
	var result strings.Builder
	inScanResults := false
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Start capturing when we see the PORT header
		if strings.Contains(line, "PORT") && strings.Contains(line, "STATE") && strings.Contains(line, "SERVICE") {
			inScanResults = true
			result.WriteString(line + "\n")
			continue
		}
		
		if inScanResults {
			// Stop capturing at empty lines that indicate end of scan results
			if line == "" && result.Len() > 50 { // Only stop if we have substantial content
				break
			}
			
			// Include port lines, script output lines, and SSL certificate info
			if strings.Contains(line, "/tcp") || 
			   strings.HasPrefix(line, "|") || 
			   strings.HasPrefix(line, "| ssl-cert:") ||
			   strings.Contains(line, "Not valid") {
				result.WriteString(line + "\n")
			}
		}
	}
	
	return result.String()
}

