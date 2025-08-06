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
	"syscall"
	"time"
	
	"ipcrawler/internal/utils"
	"ipcrawler/internal/ui"
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

// GenerateReportDirectoryPath generates the report directory path without creating directories
func GenerateReportDirectoryPath(baseDir, target string) string {
	// Just return current directory
	return "."
}

// CreateReportDirectory creates the report directory structure for a target
func CreateReportDirectory(baseDir, target string) (string, error) {
	// Just return current directory, no need to create subdirectories
	return ".", nil
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
		ui.Global.Messages.ExecutingCommandDebug(fullCmd)
	}
	
	// Extract target from args for progress display
	target := extractTargetFromArgs(args)
	
	// Create streaming output processor for real-time display
	// Always show enhanced UI, but in debug mode also show raw output
	var processor *ui.StreamingOutputProcessor
	processor = ui.Global.CreateStreamingProcessor(tool, target)
	if processor != nil {
		processor.Start()
		defer func() {
			if processor != nil {
				processor.Complete()
			}
		}()
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
			ui.Global.Messages.RunningSudo(sudoArgs)
		}
	} else {
		cmd = exec.CommandContext(ctx, tool, args...)
	}
	
	// Set process group ID to make it easier to kill child processes
	// This creates a new process group with the child as the leader
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,  // Create new process group
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
	
	// Collect output from both channels with context cancellation support
	done := make(chan bool)
	go func() {
		defer close(done)
		for outputChan != nil || errorChan != nil {
			select {
			case line, ok := <-outputChan:
				if !ok {
					outputChan = nil
				} else {
					outputLines = append(outputLines, line)
					if debugMode {
						ui.Global.Messages.Println(line)
					}
					if processor != nil {
						// Process line for real-time display (works in both debug and normal modes)
						processor.ProcessLine(line)
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
		
		// Wait for command to complete in this goroutine
		cmd.Wait()
	}()
	
	// Wait for either completion or context cancellation
	select {
	case <-ctx.Done():
		// Context was cancelled, kill the process immediately and aggressively
		if cmd.Process != nil {
			pid := cmd.Process.Pid
			
			// Kill the main process
			cmd.Process.Kill()
			
			// Also try to kill the entire process group (especially important for sudo)
			// Use negative PID to target the process group
			syscall.Kill(-pid, syscall.SIGTERM)
			
			// Give a very short grace period
			time.Sleep(50 * time.Millisecond)
			
			// Force kill the process group
			syscall.Kill(-pid, syscall.SIGKILL)
			
			// Force kill the main process if it's still alive
			if cmd.Process != nil {
				syscall.Kill(pid, syscall.SIGKILL)
			}
		}
		
		// Mark processor as failed if we have one
		if processor != nil {
			processor.Fail("command cancelled")
		}
		
		// Don't wait for the done channel - return immediately
		return nil, fmt.Errorf("command cancelled: %w", ctx.Err())
	case <-done:
		// Command completed normally, check if there was an error
	}
	
	// Check if the process had an error (after normal completion)
	if cmd.ProcessState != nil && !cmd.ProcessState.Success() {
		// Mark processor as failed if we have one
		if processor != nil {
			processor.Fail("command execution failed")
		}
		
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
			ui.Global.Messages.NoStdoutDetected()
		}
		// For tools that might write JSON to stderr, try parsing that
		outputLines = errorLines
	}
	
	// Parse the output based on tool
	if tool == "nmap" {
		return parseNmapOutput(outputLines, args)
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
		// Most other tools (naabu, etc.) don't need sudo
		return false
	}
}

// ExecuteCommandFast executes a command optimized for speed without real-time processing overhead
func ExecuteCommandFast(tool string, args []string, debugMode bool, useSudo bool) (*ScanResults, error) {
	return ExecuteCommandFastContext(context.Background(), tool, args, debugMode, useSudo)
}

func ExecuteCommandFastContext(ctx context.Context, tool string, args []string, debugMode bool, useSudo bool) (*ScanResults, error) {
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("command cancelled before execution: %w", ctx.Err())
	default:
	}
	
	// Log the full command being executed for debugging
	fullCmd := fmt.Sprintf("%s %s", tool, strings.Join(args, " "))
	if debugMode {
		ui.Global.Messages.ExecutingCommandFast(fullCmd)
	}
	
	// Extract target from args for progress display
	target := extractTargetFromArgs(args)
	
	// Create streaming output processor for real-time display
	// Always show enhanced UI, but in debug mode also show raw output
	var processor *ui.StreamingOutputProcessor
	processor = ui.Global.CreateStreamingProcessor(tool, target)
	if processor != nil {
		processor.Start()
		defer func() {
			if processor != nil {
				processor.Complete()
			}
		}()
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
			ui.Global.Messages.RunningSudo(sudoArgs)
		}
	} else {
		cmd = exec.CommandContext(ctx, tool, args...)
	}
	
	// Create pipes for real-time processing (similar to ExecuteCommandWithRealTimeResultsContext)
	var output []byte
	var outputLines []string
	
	if processor != nil {
		// Use real-time processing with pipes
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
		done := make(chan bool)
		go func() {
			defer close(done)
			for outputChan != nil || errorChan != nil {
				select {
				case line, ok := <-outputChan:
					if !ok {
						outputChan = nil
					} else {
						outputLines = append(outputLines, line)
						// Process line for real-time display
						processor.ProcessLine(line)
					}
				case line, ok := <-errorChan:
					if !ok {
						errorChan = nil
					} else {
						outputLines = append(outputLines, line)
					}
				}
			}
			cmd.Wait()
		}()
		
		// Wait for either completion or context cancellation
		select {
		case <-ctx.Done():
			// Mark processor as failed
			processor.Fail("command cancelled")
			
			// Kill the process
			if cmd.Process != nil {
				cmd.Process.Kill()
				time.Sleep(100 * time.Millisecond)
				syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
			return nil, fmt.Errorf("command cancelled: %w", ctx.Err())
		case <-done:
			// Command completed normally
			if cmd.ProcessState != nil && !cmd.ProcessState.Success() {
				processor.Fail("command execution failed")
				return nil, fmt.Errorf("failed to execute %s", tool)
			}
		}
	} else {
		// Use CombinedOutput for debug mode (original behavior)
		done := make(chan error, 1)
		
		go func() {
			var execErr error
			output, execErr = cmd.CombinedOutput()
			done <- execErr
		}()
		
		// Wait for either command completion or context cancellation
		select {
		case <-ctx.Done():
			if cmd.Process != nil {
				cmd.Process.Kill()
				time.Sleep(100 * time.Millisecond)
				syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
			return nil, fmt.Errorf("command cancelled: %w", ctx.Err())
		case err := <-done:
			if err != nil {
				var errorMessage strings.Builder
				errorMessage.WriteString(fmt.Sprintf("failed to execute %s", tool))
				
				if len(output) > 0 {
					errorMessage.WriteString(fmt.Sprintf("\nOutput:\n%s", string(output)))
				}
				
				errorMessage.WriteString(fmt.Sprintf("\nCommand: %s", fullCmd))
				return nil, fmt.Errorf("%s: %w", errorMessage.String(), err)
			}
		}
		
		// Convert output to lines for parsing
		outputLines = strings.Split(string(output), "\n")
	}
	
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

// extractTargetFromArgs extracts the target from command arguments
func extractTargetFromArgs(args []string) string {
	// For most tools, the target is usually the last argument
	// or after specific flags like -host
	for i, arg := range args {
		if arg == "-host" && i+1 < len(args) {
			return args[i+1]
		}
	}
	
	// If no -host flag found, check for the last non-flag argument
	for i := len(args) - 1; i >= 0; i-- {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") && 
		   !strings.Contains(arg, ".xml") && 
		   !strings.Contains(arg, ".json") &&
		   !strings.Contains(arg, "=") {
			return arg
		}
	}
	
	return "unknown"
}

// ShowPortDiscoveryResults displays the results of port discovery scan
func ShowPortDiscoveryResults(results *ScanResults) {
	if results == nil || len(results.Ports) == 0 {
		return
	}
	
	var openPorts []ui.PortInfo
	for _, port := range results.Ports {
		if port.State == "open" {
			openPorts = append(openPorts, ui.PortInfo{
				Number:  port.Number,
				State:   port.State,
				Service: port.Service,
				Version: port.Version,
			})
		}
	}
	
	if len(openPorts) > 0 {
		ui.Global.Messages.FoundOpenPorts(len(openPorts))
		ui.Global.Tables.RenderPortDiscoveryTable(openPorts)
		ui.Global.Messages.EmptyLine()
	}
}

// ShowDeepScanResults displays the results of deep service scan
func ShowDeepScanResults(results *ScanResults) {
	if results == nil || len(results.Ports) == 0 {
		return
	}
	
	var services []string
	var openPorts []ui.PortInfo
	
	for _, port := range results.Ports {
		if port.State == "open" {
			openPorts = append(openPorts, ui.PortInfo{
				Number:  port.Number,
				State:   port.State,
				Service: port.Service,
				Version: port.Version,
			})
			
			service := port.Service
			if service != "" && service != "unknown" {
				services = append(services, service)
			}
		}
	}
	
	if len(services) > 0 {
		uniqueServices := removeDuplicateStrings(services)
		ui.Global.Messages.ServicesDetected(uniqueServices)
		
		// Display detailed table
		if len(openPorts) > 0 {
			ui.Global.Tables.RenderServiceDetectionTable(openPorts)
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

// ShowVulnerabilityResults displays the results of vulnerability scans
func ShowVulnerabilityResults(results *ScanResults) {
	if results == nil || len(results.Vulnerabilities) == 0 {
		return
	}
	
	// Count vulnerabilities by severity
	severityCounts := make(map[string]int)
	var criticalAndHighVulns []ui.VulnerabilityInfo
	
	for _, vuln := range results.Vulnerabilities {
		severity := strings.ToLower(vuln.Severity)
		severityCounts[severity]++
		
		// Collect critical and high vulnerabilities for detailed display
		if severity == "critical" || severity == "high" {
			criticalAndHighVulns = append(criticalAndHighVulns, ui.VulnerabilityInfo{
				TemplateID: vuln.TemplateID,
				Name:       vuln.Name,
				Severity:   strings.ToUpper(vuln.Severity),
			})
		}
	}
	
	// Display summary
	totalVulns := len(results.Vulnerabilities)
	ui.Global.Messages.VulnerabilitiesFound(totalVulns)
	
	// Show severity breakdown
	if severityCounts["critical"] > 0 {
		ui.Global.Messages.CriticalVulnerabilities(severityCounts["critical"])
	}
	if severityCounts["high"] > 0 {
		ui.Global.Messages.HighVulnerabilities(severityCounts["high"])
	}
	if severityCounts["medium"] > 0 {
		ui.Global.Messages.MediumVulnerabilities(severityCounts["medium"])
	}
	if severityCounts["low"] > 0 {
		ui.Global.Messages.LowVulnerabilities(severityCounts["low"])
	}
	
	// Show detailed table for critical and high vulnerabilities
	if len(criticalAndHighVulns) > 0 {
		ui.Global.Tables.RenderVulnerabilityTable(criticalAndHighVulns)
	}
}

// ConvertPortsToURLs converts discovered ports to URL format
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
			// For non-HTTP ports, use the target:port format
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
					ui.Global.Messages.CreatingOutputDirectory(dir)
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

// WaitForToolCompletion waits for expected output files to be created and completed
func WaitForToolCompletion(reportDir string, workflows map[string]*Workflow, timeout time.Duration, debugMode bool) error {
	if debugMode {
		ui.Global.Messages.WaitingForToolOutputs()
	}
	
	// Collect expected output files from all workflows
	expectedFiles := collectExpectedOutputFiles(reportDir, workflows)
	if len(expectedFiles) == 0 {
		if debugMode {
			ui.Global.Messages.NoOutputFilesExpected()
		}
		return nil
	}
	
	if debugMode {
		ui.Global.Messages.WaitingForFiles(len(expectedFiles))
		for _, file := range expectedFiles {
			ui.Global.Messages.ExpectedFile(file)
		}
	}
	
	start := time.Now()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-time.After(timeout):
			missing := []string{}
			for _, file := range expectedFiles {
				if !isFileCompletelyWritten(file) {
					missing = append(missing, file)
				}
			}
			if len(missing) > 0 {
				return fmt.Errorf("timeout waiting for tools to complete. Missing files: %v", missing)
			}
			return nil
			
		case <-ticker.C:
			allComplete := true
			for _, file := range expectedFiles {
				if !isFileCompletelyWritten(file) {
					allComplete = false
					break
				}
			}
			
			if allComplete {
				if debugMode {
					ui.Global.Messages.AllOutputFilesReady(time.Since(start))
				}
				return nil
			}
		}
	}
}

// collectExpectedOutputFiles gathers the list of output files that tools should create
func collectExpectedOutputFiles(reportDir string, workflows map[string]*Workflow) []string {
	var files []string
	
	for _, workflow := range workflows {
		for _, step := range workflow.Steps {
			// Look for output file specifications in arguments
			for i, arg := range step.ArgsSudo {
				if (arg == "-o" || arg == "-oX" || arg == "-oN" || arg == "-oG") && i+1 < len(step.ArgsSudo) {
					outputFile := step.ArgsSudo[i+1]
					// Replace placeholders with actual values
					outputFile = strings.ReplaceAll(outputFile, "{{report_dir}}", reportDir)
					files = append(files, outputFile)
				}
			}
			for i, arg := range step.ArgsNormal {
				if (arg == "-o" || arg == "-oX" || arg == "-oN" || arg == "-oG") && i+1 < len(step.ArgsNormal) {
					outputFile := step.ArgsNormal[i+1]
					// Replace placeholders with actual values
					outputFile = strings.ReplaceAll(outputFile, "{{report_dir}}", reportDir)
					files = append(files, outputFile)
				}
			}
		}
	}
	
	return files
}

// isFileCompletelyWritten checks if a file exists and appears to be completely written
func isFileCompletelyWritten(filePath string) bool {
	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	
	// Check if file has content
	if info.Size() == 0 {
		return false
	}
	
	// Wait a brief moment and check if size changed (indicates ongoing write)
	initialSize := info.Size()
	time.Sleep(50 * time.Millisecond)
	
	info2, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	
	// If size changed, file is still being written
	if info2.Size() != initialSize {
		return false
	}
	
	// Additional check: try to open file for reading to ensure it's not locked
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	file.Close()
	
	return true
}

// ExtractProvidedData extracts provided data from scan output files
func ExtractProvidedData(reportDir string, provides []string) (map[string]string, error) {
	extractedData := make(map[string]string)
	
	for _, provided := range provides {
		switch provided {
		case "discovered_ports":
			ports, err := extractDiscoveredPortsFromFiles(".")
			if err != nil {
				return nil, fmt.Errorf("failed to extract discovered_ports: %w", err)
			}
			extractedData[provided] = ports
		default:
			return nil, fmt.Errorf("unsupported provided data type: %s", provided)
		}
	}
	
	return extractedData, nil
}

// extractDiscoveredPortsFromFiles extracts open ports from naabu JSON files in current directory
func extractDiscoveredPortsFromFiles(dir string) (string, error) {
	// Look for naabu JSON files in the current directory
	files, err := filepath.Glob(filepath.Join(dir, "naabu_*.json"))
	if err != nil {
		return "", fmt.Errorf("failed to search for naabu files: %w", err)
	}
	
	if len(files) == 0 {
		return "", fmt.Errorf("no naabu JSON files found in %s", dir)
	}
	
	// Read the first naabu file found
	data, err := os.ReadFile(files[0])
	if err != nil {
		return "", fmt.Errorf("failed to read naabu file: %w", err)
	}
	
	// Parse naabu JSON output - each line is a separate JSON object
	content := string(data)
	lines := strings.Split(content, "\n")
	var openPorts []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		
		var naabuResult struct {
			Host     string `json:"host"`
			IP       string `json:"ip"`
			Port     int    `json:"port"`
			Protocol string `json:"protocol"`
		}
		
		if err := json.Unmarshal([]byte(line), &naabuResult); err != nil {
			continue // Skip malformed JSON lines
		}
		
		// Add port to list (naabu only reports open ports)
		openPorts = append(openPorts, strconv.Itoa(naabuResult.Port))
	}
	
	if len(openPorts) == 0 {
		return "", fmt.Errorf("no open ports found in naabu JSON output")
	}
	
	return strings.Join(openPorts, ","), nil
}

// ShowNaabuResults displays the results from naabu port discovery
func ShowNaabuResults(results *ScanResults) {
	ShowPortDiscoveryResults(results)
}

// ShowNmapResults displays the results from nmap scans
func ShowNmapResults(results *ScanResults) {
	ShowDeepScanResults(results)
}
