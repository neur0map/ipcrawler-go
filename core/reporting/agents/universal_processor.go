package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ipcrawler/core/database"
	"ipcrawler/internal/utils"
)

// UniversalProcessor processes any tool output using simple conventions
type UniversalProcessor struct {
	*BaseAgent
	database *database.DatabaseManager
}

// NewUniversalProcessor creates a new universal processor
func NewUniversalProcessor() *UniversalProcessor {
	return &UniversalProcessor{
		BaseAgent: NewBaseAgent("universal_processor", nil),
		database:  database.GetGlobalDatabase(),
	}
}

// Process processes tool outputs using filename-based detection and simple parsing
func (up *UniversalProcessor) Process(input *AgentInput) (*AgentOutput, error) {
	if err := up.ValidateInput(input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	up.LogInfo("Processing tool outputs with universal processor for target: %s", input.Target)
	
	output := up.CreateOutput(nil, input.Metadata, true)
	processedData := make(map[string]*UniversalOutput)

	// Extract tool outputs from input
	toolOutputs, ok := input.Data.(map[string]*ToolOutput)
	if !ok {
		return nil, fmt.Errorf("invalid input data format")
	}

	// Process each tool output
	for toolName, toolOutput := range toolOutputs {
		up.LogInfo("Processing output from tool: %s", toolName)
		
		processed, err := up.processToolOutput(toolName, toolOutput, input.ReportDir)
		if err != nil {
			up.LogError("Failed to process %s output: %v", toolName, err)
			output.AddError(fmt.Errorf("failed to process %s output: %w", toolName, err))
			continue
		}
		
		processedData[toolName] = processed
		up.LogInfo("Successfully processed %s output", toolName)
	}

	output.Data = processedData
	output.Metadata["tools_processed"] = fmt.Sprintf("%d", len(processedData))

	up.LogInfo("Universal processing completed. Processed %d tool outputs", len(processedData))
	return output, nil
}

// UniversalOutput represents processed output in a simple, common format
type UniversalOutput struct {
	ToolName   string                 `json:"tool_name"`
	Target     string                 `json:"target"`
	ScanTime   time.Time              `json:"scan_time"`
	Format     string                 `json:"format"`
	Ports      []UniversalPort        `json:"ports,omitempty"`
	Vulnerabilities []UniversalVuln   `json:"vulnerabilities,omitempty"`
	Services   []UniversalService     `json:"services,omitempty"`
	TextReport string                 `json:"text_report"`
	Statistics map[string]int         `json:"statistics"`
	RawData    interface{}            `json:"raw_data,omitempty"`
}

type UniversalPort struct {
	Number   int    `json:"number"`
	Protocol string `json:"protocol"`
	State    string `json:"state"`
	Service  string `json:"service,omitempty"`
	Version  string `json:"version,omitempty"`
}

type UniversalVuln struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Severity string `json:"severity"`
	URL      string `json:"url,omitempty"`
}

type UniversalService struct {
	Name     string `json:"name"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Version  string `json:"version,omitempty"`
}

// processToolOutput processes a single tool output
func (up *UniversalProcessor) processToolOutput(toolName string, toolOutput *ToolOutput, reportDir string) (*UniversalOutput, error) {
	result := &UniversalOutput{
		ToolName:        toolName,
		ScanTime:        time.Now(),
		Ports:           make([]UniversalPort, 0),
		Vulnerabilities: make([]UniversalVuln, 0),
		Services:        make([]UniversalService, 0),
		Statistics:      make(map[string]int),
	}

	// Detect format from filename
	result.Format = up.detectFormat(toolOutput.FilePath)
	
	// Parse based on format and tool
	if err := up.parseOutput(result, toolOutput.RawData, toolName); err != nil {
		return nil, err
	}

	// Generate statistics
	up.calculateStatistics(result)

	// Generate text report
	result.TextReport = up.generateTextReport(result)

	// Save processed output
	if err := up.saveProcessedOutput(reportDir, toolName, result); err != nil {
		up.LogWarning("Failed to save processed output: %v", err)
	}

	return result, nil
}

// detectFormat detects output format from filename
func (up *UniversalProcessor) detectFormat(filePath string) string {
	lowerPath := strings.ToLower(filePath)
	if strings.HasSuffix(lowerPath, ".json") {
		return "json"
	} else if strings.HasSuffix(lowerPath, ".xml") {
		return "xml"
	}
	return "text"
}

// parseOutput parses tool output based on format and tool type
func (up *UniversalProcessor) parseOutput(result *UniversalOutput, rawData []byte, toolName string) error {
	switch result.Format {
	case "json":
		return up.parseJSON(result, rawData, toolName)
	case "xml":
		return up.parseXML(result, rawData, toolName)
	default:
		return up.parseText(result, rawData, toolName)
	}
}

// parseJSON handles JSON output with tool-specific logic
func (up *UniversalProcessor) parseJSON(result *UniversalOutput, rawData []byte, toolName string) error {
	// Handle JSON Lines format (multiple JSON objects)
	if strings.Contains(string(rawData), "\n{") {
		return up.parseJSONLines(result, rawData, toolName)
	}

	// Handle single JSON object
	var data interface{}
	if err := json.Unmarshal(rawData, &data); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	result.RawData = data

	// Tool-specific parsing
	switch strings.ToLower(toolName) {
	case "naabu":
		up.parseNaabuJSON(result, data)
	case "nuclei":
		up.parseNucleiJSON(result, data)
	}

	return nil
}

// parseJSONLines handles JSON Lines format
func (up *UniversalProcessor) parseJSONLines(result *UniversalOutput, rawData []byte, toolName string) error {
	lines := strings.Split(string(rawData), "\n")
	var objects []interface{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var obj interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue // Skip invalid lines
		}
		objects = append(objects, obj)
	}

	result.RawData = objects

	// Process each object based on tool
	for _, obj := range objects {
		switch strings.ToLower(toolName) {
		case "naabu":
			up.parseNaabuObject(result, obj)
		case "nuclei":
			up.parseNucleiObject(result, obj)
		}
	}

	return nil
}

// parseNaabuObject processes a single naabu JSON object
func (up *UniversalProcessor) parseNaabuObject(result *UniversalOutput, obj interface{}) {
	if objMap, ok := obj.(map[string]interface{}); ok {
		port := UniversalPort{State: "open", Protocol: "tcp"}

		if hostVal, exists := objMap["host"]; exists {
			result.Target = fmt.Sprintf("%v", hostVal)
		}
		if portVal, exists := objMap["port"]; exists {
			if portFloat, ok := portVal.(float64); ok {
				port.Number = int(portFloat)
			}
		}

		if port.Number > 0 {
			result.Ports = append(result.Ports, port)
		}
	}
}

// parseNucleiObject processes a single nuclei JSON object
func (up *UniversalProcessor) parseNucleiObject(result *UniversalOutput, obj interface{}) {
	if objMap, ok := obj.(map[string]interface{}); ok {
		vuln := UniversalVuln{}

		if templateID, exists := objMap["template-id"]; exists {
			vuln.ID = fmt.Sprintf("%v", templateID)
		}
		if info, exists := objMap["info"]; exists {
			if infoMap, ok := info.(map[string]interface{}); ok {
				if name, exists := infoMap["name"]; exists {
					vuln.Name = fmt.Sprintf("%v", name)
				}
				if severity, exists := infoMap["severity"]; exists {
					vuln.Severity = fmt.Sprintf("%v", severity)
				}
			}
		}
		if matchedAt, exists := objMap["matched-at"]; exists {
			vuln.URL = fmt.Sprintf("%v", matchedAt)
		}

		if vuln.ID != "" {
			result.Vulnerabilities = append(result.Vulnerabilities, vuln)
		}
	}
}

// Simplified parsers for other formats
func (up *UniversalProcessor) parseNaabuJSON(result *UniversalOutput, data interface{}) {
	// Handle single object case
	up.parseNaabuObject(result, data)
}

func (up *UniversalProcessor) parseNucleiJSON(result *UniversalOutput, data interface{}) {
	// Handle single object case
	up.parseNucleiObject(result, data)
}

func (up *UniversalProcessor) parseXML(result *UniversalOutput, rawData []byte, toolName string) error {
	result.RawData = string(rawData)
	return nil
}

func (up *UniversalProcessor) parseText(result *UniversalOutput, rawData []byte, toolName string) error {
	result.RawData = string(rawData)
	return nil
}

// calculateStatistics generates basic statistics
func (up *UniversalProcessor) calculateStatistics(result *UniversalOutput) {
	result.Statistics["total_ports"] = len(result.Ports)
	result.Statistics["total_vulns"] = len(result.Vulnerabilities)
	result.Statistics["total_services"] = len(result.Services)

	openPorts := 0
	for _, port := range result.Ports {
		if port.State == "open" {
			openPorts++
		}
	}
	result.Statistics["open_ports"] = openPorts
}

// generateTextReport creates a simple text report
func (up *UniversalProcessor) generateTextReport(result *UniversalOutput) string {
	var report strings.Builder
	
	report.WriteString(fmt.Sprintf("%s SCAN REPORT\n", strings.ToUpper(result.ToolName)))
	report.WriteString(strings.Repeat("=", len(result.ToolName)+13) + "\n\n")
	report.WriteString(fmt.Sprintf("Target: %s\n", result.Target))
	report.WriteString(fmt.Sprintf("Scan Time: %s\n\n", result.ScanTime.Format("2006-01-02 15:04:05")))

	if len(result.Ports) > 0 {
		report.WriteString("OPEN PORTS\n----------\n")
		for _, port := range result.Ports {
			report.WriteString(fmt.Sprintf("%d/%s %s\n", port.Number, port.Protocol, port.State))
		}
		report.WriteString("\n")
	}

	if len(result.Vulnerabilities) > 0 {
		report.WriteString("VULNERABILITIES\n---------------\n")
		for _, vuln := range result.Vulnerabilities {
			report.WriteString(fmt.Sprintf("[%s] %s - %s\n", vuln.Severity, vuln.ID, vuln.Name))
		}
		report.WriteString("\n")
	}

	return report.String()
}

// saveProcessedOutput saves the processed data
func (up *UniversalProcessor) saveProcessedOutput(reportDir, toolName string, result *UniversalOutput) error {
	processedDir := filepath.Join(reportDir, "processed")
	if err := os.MkdirAll(processedDir, 0755); err != nil {
		return err
	}

	// Save JSON
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	jsonPath := filepath.Join(processedDir, fmt.Sprintf("%s_processed.json", toolName))
	if err := utils.WriteFileWithPermissions(jsonPath, jsonData, 0644); err != nil {
		return err
	}

	// Save text report
	reportPath := filepath.Join(processedDir, fmt.Sprintf("%s_report.txt", toolName))
	if err := utils.WriteFileWithPermissions(reportPath, []byte(result.TextReport), 0644); err != nil {
		return err
	}

	return nil
}

// Validate checks if the processor is properly configured
func (up *UniversalProcessor) Validate() error {
	return nil // Universal processor needs no special validation
}