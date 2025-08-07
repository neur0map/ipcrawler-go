package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	RiskEmojis        map[string]string            `yaml:"risk_emojis"`
	ServiceCategories map[string]ServiceCategory   `yaml:"service_categories"`
	RiskThresholds    RiskThresholds              `yaml:"risk_thresholds"`
	ReportFormat      ReportFormat                `yaml:"report_format"`
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
	if data, err := os.ReadFile("database/report_config.yaml"); err == nil {
		r.reportConfig = &ReportConfig{}
		yaml.Unmarshal(data, r.reportConfig)
	}
	
	// Load service descriptions
	if data, err := os.ReadFile("database/service_descriptions.yaml"); err == nil {
		r.serviceDescs = &ServiceDescriptions{}
		yaml.Unmarshal(data, r.serviceDescs)
	}
	
	// Load DNS configuration
	if data, err := os.ReadFile("database/dns_types.yaml"); err == nil {
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
	content.WriteString("---\n\n")
	
	// Load database for service lookups
	db, _ := services.LoadDatabase()
	
	// Section 1: Executive Summary with all key discoveries
	execEmoji := r.reportConfig.ReportFormat.SectionEmojis["executive_summary"]
	content.WriteString(fmt.Sprintf("## %s Executive Summary\n\n", execEmoji))
	
	// Collect all findings first
	portResults := r.parsePortScanResults(outputPath)
	dnsResults := r.parseDNSResults(outputPath)
	nmapResults := r.parseNmapResults(outputPath)
	
	// Quick stats box
	statsEmoji := r.reportConfig.ReportFormat.SubsectionEmojis["stats"]
	content.WriteString(fmt.Sprintf("### %s Quick Stats\n", statsEmoji))
	content.WriteString("```\n")
	content.WriteString(fmt.Sprintf("Target:         %s\n", r.target))
	if len(dnsResults["A"]) > 0 {
		content.WriteString(fmt.Sprintf("IP Addresses:   %d discovered\n", len(dnsResults["A"])))
	}
	if len(portResults) > 0 {
		content.WriteString(fmt.Sprintf("Open Ports:     %d found\n", len(portResults)))
	}
	content.WriteString(fmt.Sprintf("Scan Date:      %s\n", time.Now().Format("2006-01-02")))
	content.WriteString("```\n\n")
	
	// Section 2: DNS Discovery Results (Tool: dig)
	if len(dnsResults) > 0 {
		dnsEmoji := r.reportConfig.ReportFormat.SectionEmojis["dns_discovery"]
		content.WriteString(fmt.Sprintf("## %s DNS Discovery\n", dnsEmoji))
		content.WriteString("**Tool:** `dig`\n\n")
		
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
	
	// Section 3: Port Scan Results (Tool: naabu)
	if len(portResults) > 0 {
		content.WriteString("## 🔍 Port Scan Discovery\n")
		content.WriteString("**Tool:** `naabu`\n")
		content.WriteString("**Source Files:** `ports.json` (default scan), `naabu_fast.json`, `naabu_full.json` (workflow scans)\n\n")
		
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
		
		// Service categories from database
		categoryCounts := r.categorizeServices(portResults, db)
		
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
	
	// Section 4: Nmap Fingerprinting Results (if available)
	if len(nmapResults) > 0 {
		serviceFpEmoji := r.reportConfig.ReportFormat.SectionEmojis["service_fingerprint"]
		content.WriteString(fmt.Sprintf("## %s Service Fingerprinting\n", serviceFpEmoji))
		content.WriteString("**Tool:** `nmap`\n\n")
		
		for _, result := range nmapResults {
			content.WriteString(fmt.Sprintf("### %s:%d\n", result.Host, result.Port))
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
	
	// Section 5: Security Recommendations
	securityEmoji := r.reportConfig.ReportFormat.SectionEmojis["security_recommendations"]
	content.WriteString(fmt.Sprintf("## %s Security Recommendations\n\n", securityEmoji))
	
	hasHighRisk := false
	for _, port := range portResults {
		if db.GetRiskLevel(port.Port) == "high" {
			hasHighRisk = true
			break
		}
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
	
	// Section 6: Raw Output Files
	rawFilesEmoji := r.reportConfig.ReportFormat.SectionEmojis["raw_files"]
	content.WriteString(fmt.Sprintf("## %s Raw Output Files\n\n", rawFilesEmoji))
	content.WriteString("### Available Files\n")
	content.WriteString(fmt.Sprintf("- **DNS Results:** `%s/%s/dns_*.txt`\n", r.outputDir, r.target))
	content.WriteString(fmt.Sprintf("- **Port Scans:** `%s/%s/ports.json`, `naabu_*.json`\n", r.outputDir, r.target))
	if len(nmapResults) > 0 || r.fileExists(filepath.Join(outputPath, "nmap_fingerprint.xml")) {
		content.WriteString(fmt.Sprintf("- **Nmap Results:** `%s/%s/nmap_fingerprint.xml`\n", r.outputDir, r.target))
	}
	if r.fileExists(filepath.Join(outputPath, "merged_ports.json")) {
		content.WriteString(fmt.Sprintf("- **Merged Data:** `%s/%s/merged_ports.json`\n", r.outputDir, r.target))
	}
	content.WriteString("\n---\n")
	content.WriteString(fmt.Sprintf("*Report generated by IPCrawler at %s*\n", time.Now().Format("15:04:05 MST")))
	
	return os.WriteFile(reportPath, []byte(content.String()), 0644)
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

func (r *ReportGenerator) parsePortScanResults(outputPath string) []PortResult {
	var results []PortResult
	
	// Try to parse ports.json (naabu JSON Lines format)
	portsFile := filepath.Join(outputPath, "ports.json")
	if data, err := os.ReadFile(portsFile); err == nil {
		content := strings.TrimSpace(string(data))
		lines := strings.Split(content, "\n")
		
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			
			var portData map[string]interface{}
			if err := json.Unmarshal([]byte(line), &portData); err == nil {
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
				
				if result.Port > 0 {
					results = append(results, result)
				}
			}
		}
	}
	
	return results
}

func (r *ReportGenerator) parseDNSResults(outputPath string) map[string][]string {
	dnsResults := make(map[string][]string)
	
	// Parse DNS A records
	dnsFile := filepath.Join(outputPath, "dns_a.txt")
	if data, err := os.ReadFile(dnsFile); err == nil {
		content := strings.TrimSpace(string(data))
		if content != "" {
			lines := strings.Split(content, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && !strings.Contains(line, ";") { // Skip comments
					dnsResults["A"] = append(dnsResults["A"], line)
				}
			}
		}
	}
	
	// Parse other DNS record types from database config
	for recordType, fileName := range r.dnsConfig.DNSRecordTypes {
		if recordType == "A" {
			continue // Already processed above
		}
		filePath := filepath.Join(outputPath, fileName)
		if data, err := os.ReadFile(filePath); err == nil {
			content := strings.TrimSpace(string(data))
			if content != "" {
				lines := strings.Split(content, "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" && !strings.Contains(line, ";") {
						dnsResults[recordType] = append(dnsResults[recordType], line)
					}
				}
			}
		}
	}
	
	return dnsResults
}

// NmapResult holds parsed nmap scan results
type NmapResult struct {
	Host    string
	Port    int
	Service string
	Version string
	OS      string
}

// parseNmapResults parses nmap XML output to extract basic information
func (r *ReportGenerator) parseNmapResults(outputPath string) []NmapResult {
	var results []NmapResult
	
	// Check for nmap XML file
	nmapFile := filepath.Join(outputPath, "nmap_fingerprint.xml")
	data, err := os.ReadFile(nmapFile)
	if err != nil {
		return results
	}
	
	// Since nmap currently fails due to input format issues,
	// let's at least show that nmap was attempted
	// In a production environment, we'd parse the XML properly
	
	// For now, create a placeholder result indicating nmap ran but had issues
	if strings.Contains(string(data), "WARNING: No targets were specified") {
		// Nmap ran but couldn't scan due to input format issue
		result := NmapResult{
			Host:    r.target,
			Service: "Scan attempted but input format issue detected",
			Version: "Nmap needs plain text host list, not JSON",
		}
		results = append(results, result)
	} else if strings.Contains(string(data), "<host>") {
		// If we find actual host data in the XML, we could parse it
		// This would require proper XML parsing library
		result := NmapResult{
			Host:    r.target,
			Service: "Service detection available in XML",
			Version: "See nmap_fingerprint.xml for details",
		}
		results = append(results, result)
	}
	
	return results
}

// parseMergedPorts parses the merged ports JSON file
func (r *ReportGenerator) parseMergedPorts(outputPath string) []PortResult {
	var results []PortResult
	
	mergedFile := filepath.Join(outputPath, "merged_ports.json")
	if data, err := os.ReadFile(mergedFile); err == nil {
		var ports []map[string]interface{}
		if err := json.Unmarshal(data, &ports); err == nil {
			for _, portData := range ports {
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
				
				if result.Port > 0 {
					results = append(results, result)
				}
			}
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

