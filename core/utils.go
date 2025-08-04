package core

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	
	"ipcrawler/internal/utils"
	"github.com/pterm/pterm"
)

// PortInfo represents discovered port information
type PortInfo struct {
	Number  int
	State   string
	Service string
	Version string
}

// ScanResults represents the results from a scan
type ScanResults struct {
	Ports           []PortInfo
	Vulnerabilities []VulnerabilityInfo
	Target          string
	ScanType        string
}

// VulnerabilityInfo represents discovered vulnerability information
type VulnerabilityInfo struct {
	TemplateID   string
	Name         string
	Severity     string
	Description  string
	URL          string
	CVE          []string
	CWE          []string
	Tags         []string
}

// CreateReportDirectory creates the report directory structure for a target
func CreateReportDirectory(baseDir, target string) (string, error) {
	// Sanitize target name for directory
	sanitizedTarget := strings.ReplaceAll(target, ".", "_")
	sanitizedTarget = strings.ReplaceAll(sanitizedTarget, ":", "_")
	sanitizedTarget = strings.ReplaceAll(sanitizedTarget, "/", "_")
	
	timestamp := time.Now().Format("20060102_150405")
	reportPath := filepath.Join(baseDir, sanitizedTarget, fmt.Sprintf("timestamp_%s", timestamp))
	
	// Create directory structure
	dirs := []string{
		reportPath,
		filepath.Join(reportPath, "raw"),
		filepath.Join(reportPath, "processed"),
		filepath.Join(reportPath, "summary"),
		filepath.Join(reportPath, "logs"),
	}
	
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	
	// Fix permissions if running with sudo
	if err := utils.FixSudoPermissions(reportPath); err != nil {
		// Log warning but don't fail - the directories still work for root
		pterm.Warning.Printf("Could not fix permissions for %s: %v\n", reportPath, err)
	}
	
	return reportPath, nil
}

// ExecuteCommand executes a security tool command with the given arguments
func ExecuteCommand(tool string, args []string, debugMode bool) error {
	return ExecuteCommandWithContext(context.Background(), tool, args, debugMode)
}

func ExecuteCommandWithContext(ctx context.Context, tool string, args []string, debugMode bool) error {
	// Create command with context
	cmd := exec.CommandContext(ctx, tool, args...)
	
	// Set up stdout and stderr based on debug mode
	if debugMode {
		// In debug mode, show all output
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		// In normal mode, suppress output (but still capture for potential error reporting)
		cmd.Stdout = nil
		cmd.Stderr = nil
	}
	
	// Run the command
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("command cancelled: %w", ctx.Err())
		}
		return fmt.Errorf("failed to execute %s: %w", tool, err)
	}
	
	return nil
}

// ExecuteCommandWithRealTimeResults executes a command and returns parsed results
func ExecuteCommandWithRealTimeResults(tool string, args []string, debugMode bool, useSudo bool) (*ScanResults, error) {
	return ExecuteCommandWithRealTimeResultsContext(context.Background(), tool, args, debugMode, useSudo)
}

func ExecuteCommandWithRealTimeResultsContext(ctx context.Context, tool string, args []string, debugMode bool, useSudo bool) (*ScanResults, error) {
	// Log the full command being executed for debugging
	fullCmd := fmt.Sprintf("%s %s", tool, strings.Join(args, " "))
	if debugMode {
		pterm.Info.Printf("Executing command: %s\n", fullCmd)
	}
	
	// Ensure output directories exist for any output files specified in args
	if err := ensureOutputDirectories(args, debugMode); err != nil {
		return nil, fmt.Errorf("failed to create output directories: %w", err)
	}
	
	// Validate that required placeholders have been replaced
	if err := validateArgsSubstitution(args); err != nil {
		return nil, fmt.Errorf("command validation failed: %w", err)
	}
	
	// Create command with context and sudo if needed
	var cmd *exec.Cmd
	if needsSudo(tool, args, useSudo) {
		// Prepend sudo to the command
		sudoArgs := append([]string{tool}, args...)
		cmd = exec.CommandContext(ctx, "sudo", sudoArgs...)
		if debugMode {
			pterm.Info.Printf("Running with sudo: sudo %s\n", strings.Join(sudoArgs, " "))
		}
	} else {
		cmd = exec.CommandContext(ctx, tool, args...)
	}
	
	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	
	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start %s: %w", tool, err)
	}
	
	// Read output in real-time
	var outputLines []string
	var errorLines []string
	
	// Use channels to read both stdout and stderr concurrently
	outputChan := make(chan string)
	errorChan := make(chan string)
	
	// Goroutine to read stdout
	go func() {
		defer close(outputChan)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			outputChan <- line
		}
	}()
	
	// Goroutine to read stderr
	go func() {
		defer close(errorChan)
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			errorChan <- line
		}
	}()
	
	// Collect output from both channels
	for outputChan != nil || errorChan != nil {
		select {
		case line, ok := <-outputChan:
			if !ok {
				outputChan = nil
			} else {
				outputLines = append(outputLines, line)
				if debugMode {
					fmt.Println(line)
				}
			}
		case line, ok := <-errorChan:
			if !ok {
				errorChan = nil
			} else {
				errorLines = append(errorLines, line)
				if debugMode {
					fmt.Fprintf(os.Stderr, "STDERR: %s\n", line)
				}
			}
		}
	}
	
	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		// Enhanced error reporting with stderr output
		var errorMessage strings.Builder
		errorMessage.WriteString(fmt.Sprintf("failed to execute %s", tool))
		
		if len(errorLines) > 0 {
			errorMessage.WriteString(fmt.Sprintf("\nStderr output:\n%s", strings.Join(errorLines, "\n")))
		}
		
		// Show command that failed
		errorMessage.WriteString(fmt.Sprintf("\nCommand: %s", fullCmd))
		
		return nil, fmt.Errorf("%s: %w", errorMessage.String(), err)
	}
	
	// If no stdout output but we have stderr, this might indicate the tool is writing to stderr instead
	if len(outputLines) == 0 && len(errorLines) > 0 {
		if debugMode {
			pterm.Warning.Printf("No stdout output detected, checking if tool wrote to stderr instead\n")
		}
		// For tools that might write JSON to stderr, try parsing that
		outputLines = errorLines
	}
	
	// Parse the output based on tool
	if tool == "nmap" {
		return parseNmapOutput(outputLines, args)
	} else if tool == "nuclei" {
		return parseNucleiOutput(outputLines, args)
	} else if tool == "naabu" {
		return parseNaabuOutput(outputLines, args)
	}
	
	return &ScanResults{}, nil
}

// ExecuteCommandWithOutput executes a command and returns its output
func ExecuteCommandWithOutput(tool string, args []string) ([]byte, error) {
	cmd := exec.Command(tool, args...)
	
	// Run the command and capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("failed to execute %s: %w", tool, err)
	}
	
	return output, nil
}

// needsSudo determines if a tool requires sudo privileges based on its arguments
func needsSudo(tool string, args []string, useSudo bool) bool {
	if !useSudo {
		return false // User didn't choose sudo mode
	}
	
	switch tool {
	case "nmap":
		// Check for nmap flags that require root privileges
		for _, arg := range args {
			switch arg {
			case "-sS", "-sF", "-sN", "-sX", "-sA", "-sW", "-sM", "-O":
				return true // These scans require root
			}
		}
		return false
	case "masscan":
		return true // masscan generally requires root
	default:
		// Most other tools (naabu, nuclei, etc.) don't need sudo
		return false
	}
}

// ExecuteCommandFast executes a command optimized for speed without real-time processing overhead
func ExecuteCommandFast(tool string, args []string, debugMode bool, useSudo bool) (*ScanResults, error) {
	return ExecuteCommandFastContext(context.Background(), tool, args, debugMode, useSudo)
}

func ExecuteCommandFastContext(ctx context.Context, tool string, args []string, debugMode bool, useSudo bool) (*ScanResults, error) {
	// Log the full command being executed for debugging
	fullCmd := fmt.Sprintf("%s %s", tool, strings.Join(args, " "))
	if debugMode {
		pterm.Info.Printf("Executing command (fast mode): %s\n", fullCmd)
	}
	
	// Ensure output directories exist for any output files specified in args
	if err := ensureOutputDirectories(args, debugMode); err != nil {
		return nil, fmt.Errorf("failed to create output directories: %w", err)
	}
	
	// Validate that required placeholders have been replaced
	if err := validateArgsSubstitution(args); err != nil {
		return nil, fmt.Errorf("command validation failed: %w", err)
	}
	
	// Create command with context and sudo if needed
	var cmd *exec.Cmd
	if needsSudo(tool, args, useSudo) {
		// Prepend sudo to the command
		sudoArgs := append([]string{tool}, args...)
		cmd = exec.CommandContext(ctx, "sudo", sudoArgs...)
		if debugMode {
			pterm.Info.Printf("Running with sudo: sudo %s\n", strings.Join(sudoArgs, " "))
		}
	} else {
		cmd = exec.CommandContext(ctx, tool, args...)
	}
	
	// Use CombinedOutput for simplicity and speed - no need for real-time processing for fast scans
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Enhanced error reporting
		var errorMessage strings.Builder
		errorMessage.WriteString(fmt.Sprintf("failed to execute %s", tool))
		
		if len(output) > 0 {
			errorMessage.WriteString(fmt.Sprintf("\nOutput:\n%s", string(output)))
		}
		
		// Show command that failed
		errorMessage.WriteString(fmt.Sprintf("\nCommand: %s", fullCmd))
		
		return nil, fmt.Errorf("%s: %w", errorMessage.String(), err)
	}
	
	// Convert output to lines for parsing
	outputLines := strings.Split(string(output), "\n")
	
	// Parse the output based on tool
	if tool == "naabu" {
		return parseNaabuOutput(outputLines, args)
	}
	// Can extend for other fast tools if needed
	
	return &ScanResults{}, nil
}

// parseNmapOutput parses nmap output and extracts port information
func parseNmapOutput(outputLines []string, args []string) (*ScanResults, error) {
	results := &ScanResults{
		Ports: []PortInfo{},
	}
	
	// Determine scan type from args
	if contains(args, "-sV") || contains(args, "-sC") || contains(args, "-A") {
		results.ScanType = "deep-scan"
	} else {
		results.ScanType = "port-discovery"
	}
	
	// Extract target from args (usually the last argument)
	if len(args) > 0 {
		results.Target = args[len(args)-1]
	}
	
	// Regex patterns for parsing nmap output
	portRegex := regexp.MustCompile(`^(\d+)/(tcp|udp)\s+(\w+)\s+(.*)$`)
	
	for _, line := range outputLines {
		line = strings.TrimSpace(line)
		
		// Parse port lines (format: "22/tcp open ssh OpenSSH 7.4")
		if matches := portRegex.FindStringSubmatch(line); matches != nil {
			portNum, err := strconv.Atoi(matches[1])
			if err != nil {
				continue
			}
			
			port := PortInfo{
				Number: portNum,
				State:  matches[3],
			}
			
			// Parse service and version info
			serviceInfo := strings.TrimSpace(matches[4])
			if serviceInfo != "" {
				parts := strings.Fields(serviceInfo)
				if len(parts) > 0 {
					port.Service = parts[0]
					if len(parts) > 1 {
						port.Version = strings.Join(parts[1:], " ")
					}
				}
			}
			
			results.Ports = append(results.Ports, port)
		}
	}
	
	return results, nil
}

// parseNucleiOutput parses nuclei JSON output and extracts vulnerability information
func parseNucleiOutput(outputLines []string, args []string) (*ScanResults, error) {
	results := &ScanResults{
		Vulnerabilities: []VulnerabilityInfo{},
		ScanType:        "vulnerability-scan",
	}
	
	// Extract target from args (look for -u flag value)
	for i, arg := range args {
		if arg == "-u" && i+1 < len(args) {
			results.Target = args[i+1]
			break
		}
	}
	
	// Parse each line as JSON (nuclei outputs one JSON object per line)
	for _, line := range outputLines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		
		var nucleiResult struct {
			TemplateID   string `json:"template-id"`
			TemplateURL  string `json:"template-url"`
			Name         string `json:"template"`
			Info         struct {
				Name        string   `json:"name"`
				Severity    string   `json:"severity"`
				Description string   `json:"description"`
				Tags        []string `json:"tags"`
				Reference   []string `json:"reference"`
				CVE         []string `json:"classification.cve-id"`
				CWE         []string `json:"classification.cwe-id"`
			} `json:"info"`
			Type         string `json:"type"`
			Host         string `json:"host"`
			MatchedAt    string `json:"matched-at"`
			ExtractedResults []string `json:"extracted-results"`
			Request      string `json:"request"`
			Response     string `json:"response"`
			CurlCommand  string `json:"curl-command"`
			Timestamp    string `json:"timestamp"`
		}
		
		if err := json.Unmarshal([]byte(line), &nucleiResult); err != nil {
			continue // Skip malformed JSON lines
		}
		
		// Create vulnerability info from nuclei result
		vuln := VulnerabilityInfo{
			TemplateID:  nucleiResult.TemplateID,
			Name:        nucleiResult.Info.Name,
			Severity:    nucleiResult.Info.Severity,
			Description: nucleiResult.Info.Description,
			URL:         nucleiResult.MatchedAt,
			CVE:         nucleiResult.Info.CVE,
			CWE:         nucleiResult.Info.CWE,
			Tags:        nucleiResult.Info.Tags,
		}
		
		// Use template name if info name is empty
		if vuln.Name == "" {
			vuln.Name = nucleiResult.Name
		}
		
		results.Vulnerabilities = append(results.Vulnerabilities, vuln)
	}
	
	return results, nil
}

// parseNaabuOutput parses naabu JSON output and extracts port information
func parseNaabuOutput(outputLines []string, args []string) (*ScanResults, error) {
	results := &ScanResults{
		Ports:    []PortInfo{},
		ScanType: "port-discovery",
	}
	
	// Extract target from args (look for -host flag value)
	for i, arg := range args {
		if arg == "-host" && i+1 < len(args) {
			results.Target = args[i+1]
			break
		}
	}
	
	// Parse each line as JSON (naabu outputs one JSON object per line)
	for _, line := range outputLines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		
		var naabuResult struct {
			Host      string `json:"host"`
			IP        string `json:"ip"`
			Port      int    `json:"port"`
			Protocol  string `json:"protocol"`
			Timestamp string `json:"timestamp"`
			TLS       bool   `json:"tls"`
		}
		
		if err := json.Unmarshal([]byte(line), &naabuResult); err != nil {
			continue // Skip malformed JSON lines
		}
		
		// Create port info from naabu result
		port := PortInfo{
			Number:  naabuResult.Port,
			State:   "open", // naabu only reports open ports
			Service: "", // naabu doesn't provide service info
			Version: "", // naabu doesn't provide version info
		}
		
		results.Ports = append(results.Ports, port)
	}
	
	return results, nil
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ShowPortDiscoveryResults displays the results of port discovery scan
func ShowPortDiscoveryResults(results *ScanResults) {
	if results == nil || len(results.Ports) == 0 {
		return
	}
	
	var openPorts []string
	for _, port := range results.Ports {
		if port.State == "open" {
			openPorts = append(openPorts, strconv.Itoa(port.Number))
		}
	}
	
	if len(openPorts) > 0 {
		pterm.Info.Printf("🔍 Found %d open ports: %s\n", len(openPorts), strings.Join(openPorts, ", "))
	}
}

// ShowDeepScanResults displays the results of deep service scan
func ShowDeepScanResults(results *ScanResults) {
	if results == nil || len(results.Ports) == 0 {
		return
	}
	
	var services []string
	var tableData [][]string
	tableData = append(tableData, []string{"Port", "Service", "Version"})
	
	for _, port := range results.Ports {
		if port.State == "open" {
			service := port.Service
			if service == "" {
				service = "unknown"
			}
			
			version := port.Version
			if version == "" {
				version = "-"
			}
			
			tableData = append(tableData, []string{
				strconv.Itoa(port.Number),
				service,
				version,
			})
			
			if service != "unknown" && service != "" {
				services = append(services, service)
			}
		}
	}
	
	if len(services) > 0 {
		uniqueServices := removeDuplicateStrings(services)
		pterm.Info.Printf("🔎 Services detected: %s\n", strings.Join(uniqueServices, ", "))
		
		// Display detailed table
		if len(tableData) > 1 {
			pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
		}
	}
}

// removeDuplicateStrings removes duplicate strings from a slice
func removeDuplicateStrings(slice []string) []string {
	keys := make(map[string]bool)
	var result []string
	
	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}
	
	return result
}

// ShowNucleiResults displays the results of nuclei vulnerability scan
func ShowNucleiResults(results *ScanResults) {
	if results == nil || len(results.Vulnerabilities) == 0 {
		return
	}
	
	// Count vulnerabilities by severity
	severityCounts := make(map[string]int)
	var criticalVulns, highVulns, mediumVulns []string
	
	for _, vuln := range results.Vulnerabilities {
		severity := strings.ToLower(vuln.Severity)
		severityCounts[severity]++
		
		switch severity {
		case "critical":
			criticalVulns = append(criticalVulns, vuln.Name)
		case "high":
			highVulns = append(highVulns, vuln.Name)
		case "medium":
			mediumVulns = append(mediumVulns, vuln.Name)
		}
	}
	
	// Display summary
	totalVulns := len(results.Vulnerabilities)
	pterm.Info.Printf("🔒 Found %d vulnerabilities", totalVulns)
	
	// Show severity breakdown
	if severityCounts["critical"] > 0 {
		pterm.Error.Printf("   Critical: %d", severityCounts["critical"])
	}
	if severityCounts["high"] > 0 {
		pterm.Warning.Printf("   High: %d", severityCounts["high"])
	}
	if severityCounts["medium"] > 0 {
		pterm.Info.Printf("   Medium: %d", severityCounts["medium"])
	}
	if severityCounts["low"] > 0 {
		pterm.Info.Printf("   Low: %d", severityCounts["low"])
	}
	
	// Show detailed table for critical and high vulnerabilities
	if len(criticalVulns) > 0 || len(highVulns) > 0 {
		var tableData [][]string
		tableData = append(tableData, []string{"Severity", "Vulnerability", "Template ID"})
		
		// Add critical vulnerabilities
		for _, vuln := range results.Vulnerabilities {
			if strings.ToLower(vuln.Severity) == "critical" || strings.ToLower(vuln.Severity) == "high" {
				name := vuln.Name
				if len(name) > 50 {
					name = name[:47] + "..."
				}
				templateID := vuln.TemplateID
				if len(templateID) > 25 {
					templateID = templateID[:22] + "..."
				}
				
				tableData = append(tableData, []string{
					strings.ToUpper(vuln.Severity),
					name,
					templateID,
				})
			}
		}
		
		if len(tableData) > 1 {
			pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
		}
	}
}

// ConvertPortsToURLs converts discovered ports to nuclei-compatible target URLs
func ConvertPortsToURLs(target string, discoveredPorts string) string {
	if discoveredPorts == "" {
		return target
	}
	
	var urls []string
	ports := strings.Split(discoveredPorts, ",")
	
	for _, port := range ports {
		port = strings.TrimSpace(port)
		if port == "" {
			continue
		}
		
		// Convert port to appropriate URL format
		switch port {
		case "80":
			urls = append(urls, fmt.Sprintf("http://%s", target))
		case "443":
			urls = append(urls, fmt.Sprintf("https://%s", target))
		case "8080", "8000", "3000", "5000", "8888":
			urls = append(urls, fmt.Sprintf("http://%s:%s", target, port))
		case "8443", "9443":
			urls = append(urls, fmt.Sprintf("https://%s:%s", target, port))
		default:
			// For non-HTTP ports, use the target:port format for nuclei network templates
			urls = append(urls, fmt.Sprintf("%s:%s", target, port))
		}
	}
	
	if len(urls) == 0 {
		return target
	}
	
	return strings.Join(urls, ",")
}

// CheckSudoAvailability checks if sudo is available and configured properly
// Deprecated: Use utils.RequestPrivilegeEscalation() for better privilege management
func CheckSudoAvailability() error {
	// Check if already running as root
	if utils.IsRunningAsRoot() {
		return nil
	}
	
	// Check if sudo is available
	if !utils.IsSudoAvailable() {
		return fmt.Errorf("sudo is not installed or not in PATH")
	}
	
	// For backward compatibility, we'll just check if sudo exists
	// The new approach handles privilege escalation through process restart
	return nil
}

// ensureOutputDirectories creates any output directories referenced in command arguments
func ensureOutputDirectories(args []string, debugMode bool) error {
	for i, arg := range args {
		// Look for output flags (-o, -oX, etc.) followed by file paths
		if (arg == "-o" || arg == "-oX" || arg == "-oN" || arg == "-oG") && i+1 < len(args) {
			outputPath := args[i+1]
			
			// Extract directory from the output path
			dir := filepath.Dir(outputPath)
			if dir != "." && dir != "" {
				if debugMode {
					pterm.Info.Printf("Creating output directory: %s\n", dir)
				}
				
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", dir, err)
				}
			}
		}
	}
	return nil
}

// validateArgsSubstitution checks that all placeholder variables have been properly substituted
func validateArgsSubstitution(args []string) error {
	for i, arg := range args {
		// Check for unsubstituted placeholders ({{variable}})
		if strings.Contains(arg, "{{") && strings.Contains(arg, "}}") {
			// Extract the placeholder name
			start := strings.Index(arg, "{{")
			end := strings.Index(arg, "}}")
			if start != -1 && end != -1 && end > start {
				placeholder := arg[start:end+2]
				return fmt.Errorf("unsubstituted placeholder found in argument %d: %s", i+1, placeholder)
			}
		}
	}
	return nil
}

