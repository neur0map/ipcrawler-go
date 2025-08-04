package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReceiverAgent collects and validates JSON output from security tools
type ReceiverAgent struct {
	*BaseAgent
	config *ReceiverConfig
}

// ReceiverConfig holds configuration for the receiver agent
type ReceiverConfig struct {
	ValidateSchema   bool     `yaml:"validate_schema"`
	ErrorHandling    string   `yaml:"error_handling"` // "continue", "fail", "skip"
	OutputFormats    []string `yaml:"output_formats"`
}

// DefaultReceiverConfig returns default configuration for receiver
func DefaultReceiverConfig() *ReceiverConfig {
	return &ReceiverConfig{
		ValidateSchema: true,
		ErrorHandling:  "continue",
		OutputFormats:  []string{"json", "xml"},
	}
}

// NewReceiverAgent creates a new receiver agent
func NewReceiverAgent(config *ReceiverConfig) *ReceiverAgent {
	if config == nil {
		config = DefaultReceiverConfig()
	}
	
	return &ReceiverAgent{
		BaseAgent: NewBaseAgent("receiver", nil),
		config:    config,
	}
}

// Validate checks if the receiver agent is properly configured
func (r *ReceiverAgent) Validate() error {
	if r.config == nil {
		return fmt.Errorf("receiver config is required")
	}
	
	validErrorHandling := []string{"continue", "fail", "skip"}
	found := false
	for _, valid := range validErrorHandling {
		if r.config.ErrorHandling == valid {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("invalid error handling mode: %s", r.config.ErrorHandling)
	}
	
	return nil
}

// Process collects and validates tool outputs
func (r *ReceiverAgent) Process(input *AgentInput) (*AgentOutput, error) {
	if err := r.ValidateInput(input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	
	r.LogInfo("Processing tool outputs for target: %s", input.Target)
	
	output := r.CreateOutput(nil, input.Metadata, true)
	
	// Collect all tool outputs from the raw directory
	rawDir := filepath.Join(input.ReportDir, "raw")
	toolOutputs, err := r.collectToolOutputs(rawDir)
	if err != nil {
		r.LogError("Failed to collect tool outputs: %v", err)
		output.AddError(fmt.Errorf("failed to collect tool outputs: %w", err))
		return output, nil
	}
	
	if len(toolOutputs) == 0 {
		r.LogWarning("No tool outputs found in %s", rawDir)
		output.AddWarning("No tool outputs found")
	}
	
	// Validate and route each tool output
	routedData := make(map[string]*ToolOutput)
	for toolName, toolOutput := range toolOutputs {
		r.LogInfo("Processing output from tool: %s", toolName)
		
		if r.config.ValidateSchema {
			if err := r.validateToolOutput(toolName, toolOutput); err != nil {
				r.LogError("Schema validation failed for %s: %v", toolName, err)
				
				switch r.config.ErrorHandling {
				case "fail":
					output.AddError(fmt.Errorf("schema validation failed for %s: %w", toolName, err))
					continue
				case "skip":
					r.LogWarning("Skipping %s due to validation failure", toolName)
					continue
				case "continue":
					output.AddWarning(fmt.Sprintf("Schema validation failed for %s: %v", toolName, err))
				}
			}
		}
		
		// Route to appropriate cleaner
		cleanerType := r.determineCleanerType(toolName)
		toolOutput.CleanerType = cleanerType
		routedData[toolName] = toolOutput
		
		r.LogInfo("Successfully processed %s output (%d bytes)", toolName, len(toolOutput.RawData))
	}
	
	output.Data = routedData
	output.Metadata["tools_processed"] = fmt.Sprintf("%d", len(routedData))
	output.Metadata["raw_directory"] = rawDir
	
	r.LogInfo("Receiver processing completed. Processed %d tool outputs", len(routedData))
	return output, nil
}

// ToolOutput represents output from a security tool
type ToolOutput struct {
	ToolName     string      `json:"tool_name"`
	FilePath     string      `json:"file_path"`
	RawData      []byte      `json:"raw_data"`
	ParsedData   interface{} `json:"parsed_data"`
	CleanerType  string      `json:"cleaner_type"`
	Schema       string      `json:"schema"`
	Valid        bool        `json:"valid"`
	ErrorMessage string      `json:"error_message,omitempty"`
}

// collectToolOutputs scans the raw directory for tool outputs
func (r *ReceiverAgent) collectToolOutputs(rawDir string) (map[string]*ToolOutput, error) {
	toolOutputs := make(map[string]*ToolOutput)
	
	// Check if raw directory exists
	if _, err := os.Stat(rawDir); os.IsNotExist(err) {
		return toolOutputs, nil // Empty but not an error
	}
	
	// Read directory contents
	entries, err := os.ReadDir(rawDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read raw directory: %w", err)
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		fileName := entry.Name()
		filePath := filepath.Join(rawDir, fileName)
		
		// Determine tool name from filename using simple pattern matching
		toolName := r.extractToolNameFromFile(fileName)
		if toolName == "" {
			r.LogWarning("Could not determine tool name from file: %s", fileName)
			continue
		}
		
		// Read file content
		rawData, err := os.ReadFile(filePath)
		if err != nil {
			r.LogError("Failed to read file %s: %v", filePath, err)
			continue
		}
		
		// Parse JSON if possible
		var parsedData interface{}
		if r.isJSONFile(fileName) {
			if err := json.Unmarshal(rawData, &parsedData); err != nil {
				r.LogWarning("Failed to parse JSON from %s: %v", fileName, err)
				parsedData = nil
			}
		} else if r.isXMLFile(fileName) && toolName == "nmap" {
			// For nmap XML files, we'll pass the raw XML data
			// The nmap cleaner will handle XML parsing
			parsedData = nil // Let the cleaner handle XML parsing
		}
		
		toolOutput := &ToolOutput{
			ToolName:   toolName,
			FilePath:   filePath,
			RawData:    rawData,
			ParsedData: parsedData,
			Valid:      true,
		}
		
		toolOutputs[toolName] = toolOutput
	}
	
	return toolOutputs, nil
}


// isJSONFile checks if a file is a JSON file
func (r *ReceiverAgent) isJSONFile(fileName string) bool {
	return strings.HasSuffix(strings.ToLower(fileName), ".json")
}

// isXMLFile checks if a file is an XML file
func (r *ReceiverAgent) isXMLFile(fileName string) bool {
	return strings.HasSuffix(strings.ToLower(fileName), ".xml")
}

// extractToolNameFromFile extracts tool name from filename using simple patterns
func (r *ReceiverAgent) extractToolNameFromFile(fileName string) string {
	// Remove extension
	baseName := fileName
	if idx := strings.LastIndex(fileName, "."); idx != -1 {
		baseName = fileName[:idx]
	}
	
	// Common tool patterns
	baseNameLower := strings.ToLower(baseName)
	toolNames := []string{"nmap", "naabu", "nuclei", "masscan", "gobuster", "ffuf", "nikto"}
	
	for _, tool := range toolNames {
		if strings.Contains(baseNameLower, tool) {
			return tool
		}
	}
	
	return ""
}

// determineCleanerType determines which processor should process the tool output
func (r *ReceiverAgent) determineCleanerType(toolName string) string {
	// Use tool-specific processor if available, otherwise universal
	switch toolName {
	case "nmap":
		return "nmap_processor"
	default:
		return "universal_processor"
	}
}

// validateToolOutput performs basic schema validation on tool output
func (r *ReceiverAgent) validateToolOutput(toolName string, toolOutput *ToolOutput) error {
	if toolOutput.ParsedData == nil {
		return fmt.Errorf("no parsed data available for validation")
	}
	
	// Tool-specific validation
	switch toolName {
	case "nmap":
		return r.validateNmapOutput(toolOutput.ParsedData)
	default:
		// Generic validation - just ensure it's valid data
		return nil
	}
}

// validateNmapOutput validates nmap JSON output structure
func (r *ReceiverAgent) validateNmapOutput(data interface{}) error {
	// Basic structure validation for nmap JSON
	if dataMap, ok := data.(map[string]interface{}); ok {
		if _, hasNmaprun := dataMap["nmaprun"]; !hasNmaprun {
			return fmt.Errorf("missing 'nmaprun' element in nmap output")
		}
		return nil
	}
	return fmt.Errorf("invalid nmap output structure")
}

