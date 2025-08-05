package core

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ipcrawler/core/reporting/agents"
	"ipcrawler/internal/utils"
)

// RunReportingPipeline executes the complete reporting pipeline for scan results
func RunReportingPipeline(reportDir, target string, workflows map[string]*Workflow, config *Config, debugMode bool) error {
	if debugMode {
		log.Printf("Starting reporting pipeline for target: %s", target)
	}

	// Create pipeline with config
	pipelineConfig := agents.DefaultPipelineConfig()
	pipelineConfig.ReportDir = reportDir
	
	// Override with config settings if available
	if config.Reporting != nil && config.Reporting.Pipeline != nil {
		if config.Reporting.Pipeline.MaxRetries > 0 {
			pipelineConfig.MaxRetries = config.Reporting.Pipeline.MaxRetries
		}
		pipelineConfig.FailFast = config.Reporting.Pipeline.FailFast
		pipelineConfig.LogLevel = config.Reporting.Pipeline.LogLevel
	}

	// Create logger for pipeline
	logFilePath := fmt.Sprintf("%s/logs/reporting_pipeline.log", reportDir)
	logFile, err := os.Create(logFilePath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()
	
	// Fix permissions for log file
	if err := utils.FixFilePermissions(logFilePath); err != nil {
		if debugMode {
			log.Printf("Warning: Could not fix permissions for %s: %v", logFilePath, err)
		}
	}
	
	// Create error log file
	errorLogPath := fmt.Sprintf("%s/logs/error.log", reportDir)
	errorLogFile, err := os.Create(errorLogPath)
	if err != nil {
		return fmt.Errorf("failed to create error log file: %w", err)
	}
	defer errorLogFile.Close()
	
	// Fix permissions for error log file
	if err := utils.FixFilePermissions(errorLogPath); err != nil {
		if debugMode {
			log.Printf("Warning: Could not fix permissions for %s: %v", errorLogPath, err)
		}
	}
	
	logger := log.New(logFile, "", log.LstdFlags)
	
	// Create pipeline
	pipeline := agents.NewPipeline(pipelineConfig, logger)
	pipeline.SetTarget(target)
	pipeline.SetReportDir(reportDir)
	pipeline.SetDebugMode(debugMode)

	// Collect all unique agents specified in workflows  
	requiredAgents := collectRequiredAgents(workflows)
	if debugMode {
		log.Printf("Required agents from workflows: %v", requiredAgents)
	}
	
	// Add agents to pipeline in the correct order
	agentOrder := []string{"receiver", "nmap_processor", "universal_processor", "data_accumulator", "coordinator", "validator", "reporter"}
	
	for _, agentName := range agentOrder {
		if !requiredAgents[agentName] {
			continue // Skip if not required by any workflow
		}
		
		switch agentName {
		case "receiver":
			receiverConfig := agents.DefaultReceiverConfig()
			if config.Reporting != nil && config.Reporting.Agents != nil {
				if receiverAgentConfig, ok := config.Reporting.Agents["receiver"]; ok {
					if cfg, ok := receiverAgentConfig.(map[string]interface{}); ok {
						if validateSchema, ok := cfg["validate_schema"].(bool); ok {
							receiverConfig.ValidateSchema = validateSchema
						}
						if errorHandling, ok := cfg["error_handling"].(string); ok {
							receiverConfig.ErrorHandling = errorHandling
						}
					}
				}
			}
			pipeline.AddAgent(agents.NewReceiverAgent(receiverConfig))
			
		case "nmap_processor":
			nmapConfig := agents.DefaultNmapProcessorConfig()
			pipeline.AddAgent(agents.NewNmapProcessor(nmapConfig))
			
		case "universal_processor":
			pipeline.AddAgent(agents.NewUniversalProcessor())
			
		case "data_accumulator":
			accumulatorAgent := &dataAccumulatorAgent{
				BaseAgent: agents.NewBaseAgent("data_accumulator", logger),
				cleanerOutputs: make(map[string]interface{}),
			}
			pipeline.AddAgent(accumulatorAgent)
			
		case "coordinator":
			coordinatorConfig := agents.DefaultCoordinatorConfig()
			pipeline.AddAgent(agents.NewCoordinatorAgent(coordinatorConfig))
			
		case "validator":
			validatorConfig := agents.DefaultValidatorConfig()
			if config.Reporting != nil && config.Reporting.Agents != nil {
				if validatorAgentConfig, ok := config.Reporting.Agents["validator"]; ok {
					if cfg, ok := validatorAgentConfig.(map[string]interface{}); ok {
						if strictMode, ok := cfg["strict_mode"].(bool); ok {
							validatorConfig.StrictMode = strictMode
						}
						if validationLevel, ok := cfg["validation_level"].(string); ok {
							validatorConfig.ValidationLevel = validationLevel
						}
					}
				}
			}
			pipeline.AddAgent(agents.NewValidatorAgent(validatorConfig))
			
		case "reporter":
			reporterConfig := agents.DefaultReporterConfig()
			if config.Reporting != nil && config.Reporting.Agents != nil {
				if reporterAgentConfig, ok := config.Reporting.Agents["reporter"]; ok {
					if cfg, ok := reporterAgentConfig.(map[string]interface{}); ok {
						if templates, ok := cfg["templates"].([]interface{}); ok {
							reporterConfig.Templates = make([]string, 0)
							for _, t := range templates {
								if templateStr, ok := t.(string); ok {
									reporterConfig.Templates = append(reporterConfig.Templates, templateStr)
								}
							}
						}
					}
				}
			}
			pipeline.AddAgent(agents.NewReporterAgent(reporterConfig))
		}
	}

	// Execute pipeline
	if debugMode {
		log.Printf("Executing reporting pipeline with %d agents", len(pipeline.GetAgents()))
	}
	
	startTime := time.Now()
	result, err := pipeline.Execute(target, nil)
	if err != nil {
		// Enhanced error context
		return fmt.Errorf("pipeline execution failed after %v: %w", time.Since(startTime), err)
	}

	duration := time.Since(startTime)
	if debugMode {
		log.Printf("Reporting pipeline completed in %v", duration)
	}

	// Check results - be more strict about failures
	if !result.Success {
		if debugMode {
			log.Printf("Pipeline completed with errors: %v", result.Error)
		}
		// This is now a hard failure instead of just a warning
		return fmt.Errorf("reporting pipeline failed: %w", result.Error)
	}

	// Verify that expected output files were actually created
	if err := verifyReportOutputs(reportDir, debugMode); err != nil {
		if debugMode {
			log.Printf("Report verification failed: %v", err)
		}
		return fmt.Errorf("report verification failed: %w", err)
	}

	if debugMode {
		log.Printf("Successfully generated reports in: %s", reportDir)
	}
	return nil
}

// collectRequiredAgents analyzes workflows to determine which agents are needed
func collectRequiredAgents(workflows map[string]*Workflow) map[string]bool {
	requiredAgents := make(map[string]bool)
	needsCoordination := false
	
	for _, workflow := range workflows {
		if !workflow.HasReporting() {
			continue
		}
		
		reportConfig := workflow.GetReportConfig()
		if reportConfig == nil || !reportConfig.Enabled {
			continue
		}
		
		// Add agents specified in the workflow
		for _, agentName := range reportConfig.Agents {
			requiredAgents[agentName] = true
		}
		
		// Check if coordination is needed
		if reportConfig.Coordination || len(workflow.Requires) > 0 || len(workflow.Provides) > 0 {
			needsCoordination = true
		}
		
		// Auto-add data_accumulator if we have processors
		for _, agentName := range reportConfig.Agents {
			if strings.HasSuffix(agentName, "_processor") {
				requiredAgents["data_accumulator"] = true
				break
			}
		}
	}
	
	// Auto-add coordinator if any workflow needs coordination
	if needsCoordination {
		requiredAgents["coordinator"] = true
	}
	
	return requiredAgents
}

// RunWorkflowReporting executes reporting pipeline for a single workflow
func RunWorkflowReporting(reportDir, target, workflowKey string, workflow *Workflow, config *Config, debugMode bool) error {
	if debugMode {
		log.Printf("Running reporting for workflow: %s", workflowKey)
	}

	// Create pipeline with config
	pipelineConfig := agents.DefaultPipelineConfig()
	pipelineConfig.ReportDir = reportDir
	
	// Override with config settings if available
	if config.Reporting != nil && config.Reporting.Pipeline != nil {
		if config.Reporting.Pipeline.MaxRetries > 0 {
			pipelineConfig.MaxRetries = config.Reporting.Pipeline.MaxRetries
		}
		pipelineConfig.FailFast = config.Reporting.Pipeline.FailFast
		pipelineConfig.LogLevel = config.Reporting.Pipeline.LogLevel
	}

	// Create logger for pipeline
	logFilePath := fmt.Sprintf("%s/logs/workflow_%s_reporting.log", reportDir, workflowKey)
	logFile, err := os.Create(logFilePath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()
	
	// Fix permissions for log file
	if err := utils.FixFilePermissions(logFilePath); err != nil {
		if debugMode {
			log.Printf("Warning: Could not fix permissions for %s: %v", logFilePath, err)
		}
	}
	
	logger := log.New(logFile, "", log.LstdFlags)
	
	// Create pipeline
	pipeline := agents.NewPipeline(pipelineConfig, logger)
	pipeline.SetTarget(target)
	pipeline.SetReportDir(reportDir)
	pipeline.SetDebugMode(debugMode)

	// Get report config
	reportConfig := workflow.GetReportConfig()
	if reportConfig == nil || !reportConfig.Enabled {
		return fmt.Errorf("reporting not enabled for workflow %s", workflowKey)
	}
	
	// Add agents specified in the workflow in the correct order
	agentOrder := []string{"receiver", "nmap_processor", "universal_processor", "data_accumulator", "coordinator", "validator", "reporter"}
	
	for _, agentName := range agentOrder {
		// Check if this agent is required by the workflow
		agentRequired := false
		for _, requiredAgent := range reportConfig.Agents {
			if requiredAgent == agentName {
				agentRequired = true
				break
			}
		}
		
		if !agentRequired {
			continue // Skip if not required by this workflow
		}
		
		switch agentName {
		case "receiver":
			receiverConfig := agents.DefaultReceiverConfig()
			if config.Reporting != nil && config.Reporting.Agents != nil {
				if receiverAgentConfig, ok := config.Reporting.Agents["receiver"]; ok {
					if cfg, ok := receiverAgentConfig.(map[string]interface{}); ok {
						if validateSchema, ok := cfg["validate_schema"].(bool); ok {
							receiverConfig.ValidateSchema = validateSchema
						}
						if errorHandling, ok := cfg["error_handling"].(string); ok {
							receiverConfig.ErrorHandling = errorHandling
						}
					}
				}
			}
			pipeline.AddAgent(agents.NewReceiverAgent(receiverConfig))
			
		case "nmap_processor":
			nmapConfig := agents.DefaultNmapProcessorConfig()
			pipeline.AddAgent(agents.NewNmapProcessor(nmapConfig))
			
		case "universal_processor":
			pipeline.AddAgent(agents.NewUniversalProcessor())
			
		case "data_accumulator":
			accumulatorAgent := &dataAccumulatorAgent{
				BaseAgent: agents.NewBaseAgent("data_accumulator", logger),
				cleanerOutputs: make(map[string]interface{}),
			}
			pipeline.AddAgent(accumulatorAgent)
			
		case "coordinator":
			coordinatorConfig := agents.DefaultCoordinatorConfig()
			pipeline.AddAgent(agents.NewCoordinatorAgent(coordinatorConfig))
		}
	}

	// Execute pipeline
	if debugMode {
		log.Printf("Executing workflow reporting pipeline with %d agents", len(pipeline.GetAgents()))
	}
	
	startTime := time.Now()
	result, err := pipeline.Execute(target, nil)
	if err != nil {
		return fmt.Errorf("workflow pipeline execution failed: %w", err)
	}

	duration := time.Since(startTime)
	if debugMode {
		log.Printf("Workflow reporting pipeline completed in %v", duration)
	}

	// Check results
	if !result.Success {
		if debugMode {
			log.Printf("Workflow pipeline completed with errors: %v", result.Error)
		}
		return fmt.Errorf("workflow pipeline completed with errors: %w", result.Error)
	}

	return nil
}

// ExtractProvidedData extracts provided data from reporting pipeline results
func ExtractProvidedData(reportDir string, provides []string) (map[string]string, error) {
	extractedData := make(map[string]string)
	
	for _, provided := range provides {
		switch provided {
		case "discovered_ports":
			ports, err := extractDiscoveredPortsFromReports(reportDir)
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

// extractDiscoveredPortsFromReports extracts open ports from scan reports (naabu, nmap, etc.)
func extractDiscoveredPortsFromReports(reportDir string) (string, error) {
	// First try to extract from naabu JSON files (most common for port discovery)
	rawDir := fmt.Sprintf("%s/raw", reportDir)
	if ports, err := extractPortsFromNaabuJSON(rawDir); err == nil && ports != "" {
		return ports, nil
	}
	
	// Try to read from processed JSON report 
	jsonPath := fmt.Sprintf("%s/processed/nmap_scan_results.json", reportDir)
	if _, err := os.Stat(jsonPath); err == nil {
		if ports, err := extractPortsFromJSON(jsonPath); err == nil && ports != "" {
			return ports, nil
		}
	}
	
	// Fallback to reading raw nmap output
	if ports, err := extractPortsFromRawNmap(rawDir); err == nil && ports != "" {
		return ports, nil
	}
	
	return "", fmt.Errorf("no port data found in any report format")
}

// extractPortsFromJSON extracts ports from processed JSON report
func extractPortsFromJSON(jsonPath string) (string, error) {
	data, err := ioutil.ReadFile(jsonPath)
	if err != nil {
		return "", fmt.Errorf("failed to read JSON file: %w", err)
	}
	
	var nmapData struct {
		Ports []struct {
			Number int    `json:"number"`
			State  string `json:"state"`
		} `json:"ports"`
	}
	
	if err := json.Unmarshal(data, &nmapData); err != nil {
		return "", fmt.Errorf("failed to parse JSON: %w", err)
	}
	
	var openPorts []string
	for _, port := range nmapData.Ports {
		if port.State == "open" {
			openPorts = append(openPorts, strconv.Itoa(port.Number))
		}
	}
	
	if len(openPorts) == 0 {
		return "", fmt.Errorf("no open ports found in JSON report")
	}
	
	return strings.Join(openPorts, ","), nil
}

// extractPortsFromRawNmap extracts ports from raw nmap output files
func extractPortsFromRawNmap(rawDir string) (string, error) {
	// Look for nmap .txt files in the raw directory
	files, err := filepath.Glob(filepath.Join(rawDir, "nmap_port_discovery_*.txt"))
	if err != nil {
		return "", fmt.Errorf("failed to search for nmap files: %w", err)
	}
	
	if len(files) == 0 {
		return "", fmt.Errorf("no nmap port discovery files found in %s", rawDir)
	}
	
	// Read the first file found
	data, err := ioutil.ReadFile(files[0])
	if err != nil {
		return "", fmt.Errorf("failed to read nmap file: %w", err)
	}
	
	// Parse open ports from nmap output using regex
	content := string(data)
	var openPorts []string
	
	// Pattern to match lines like "80/tcp   open  http"
	portRegex := regexp.MustCompile(`(?m)^(\d+)\/tcp\s+open\s+`)
	matches := portRegex.FindAllStringSubmatch(content, -1)
	
	for _, match := range matches {
		if len(match) > 1 {
			openPorts = append(openPorts, match[1])
		}
	}
	
	if len(openPorts) == 0 {
		return "", fmt.Errorf("no open ports found in nmap output")
	}
	
	return strings.Join(openPorts, ","), nil
}

// extractPortsFromNaabuJSON extracts ports from naabu JSON output files  
func extractPortsFromNaabuJSON(rawDir string) (string, error) {
	// Look for naabu JSON files in the raw directory
	files, err := filepath.Glob(filepath.Join(rawDir, "naabu_*.json"))
	if err != nil {
		return "", fmt.Errorf("failed to search for naabu files: %w", err)
	}
	
	if len(files) == 0 {
		return "", fmt.Errorf("no naabu JSON files found")
	}
	
	// Read the first naabu file found
	data, err := ioutil.ReadFile(files[0])
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

// dataAccumulatorAgent collects outputs from all cleaner agents
type dataAccumulatorAgent struct {
	*agents.BaseAgent
	cleanerOutputs map[string]interface{}
}

// Validate checks if the agent is properly configured
func (d *dataAccumulatorAgent) Validate() error {
	return nil
}

// Process accumulates processor outputs
func (d *dataAccumulatorAgent) Process(input *agents.AgentInput) (*agents.AgentOutput, error) {
	if err := d.ValidateInput(input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	
	// Check if this is output from a processor agent
	if input.PreviousAgent != "" && strings.Contains(input.PreviousAgent, "_processor") {
		// Extract tool name from agent name (e.g., "nmap_processor" -> "nmap")
		toolName := strings.TrimSuffix(input.PreviousAgent, "_processor")
		d.cleanerOutputs[toolName] = input.Data
		d.LogInfo("Accumulated output from %s", input.PreviousAgent)
	}
	
	// Pass accumulated outputs to next agent (coordinator)
	output := d.CreateOutput(d.cleanerOutputs, input.Metadata, true)
	return output, nil
}

// verifyReportOutputs checks that expected report files were created
func verifyReportOutputs(reportDir string, debugMode bool) error {
	expectedDirs := []string{
		filepath.Join(reportDir, "processed"),
		filepath.Join(reportDir, "summary"),
	}
	
	// Check that expected directories exist
	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("expected report directory missing: %s", dir)
		}
	}
	
	// Check for summary files (at least one should exist)
	summaryDir := filepath.Join(reportDir, "summary")
	files, err := ioutil.ReadDir(summaryDir)
	if err != nil {
		return fmt.Errorf("failed to read summary directory: %w", err)
	}
	
	if len(files) == 0 {
		return fmt.Errorf("no summary files generated in %s", summaryDir)
	}
	
	// Check that summary files have content
	hasValidSummary := false
	for _, file := range files {
		if file.Size() > 0 {
			hasValidSummary = true
			break
		}
	}
	
	if !hasValidSummary {
		return fmt.Errorf("all summary files are empty in %s", summaryDir)
	}
	
	if debugMode {
		log.Printf("Report verification passed: found %d files in summary directory", len(files))
	}
	
	return nil
}