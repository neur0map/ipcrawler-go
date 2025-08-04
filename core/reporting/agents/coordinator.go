package agents

import (
	"fmt"
	"strconv"
	"strings"
)

// CoordinatorAgent manages workflow dependencies and information flow
type CoordinatorAgent struct {
	*BaseAgent
	config      *CoordinatorConfig
	workflowData map[string]interface{}
}

// CoordinatorConfig holds configuration for the coordinator agent
type CoordinatorConfig struct {
	EnableDependencies bool     `yaml:"enable_dependencies"`
	WorkflowOrder      []string `yaml:"workflow_order"`
	DataExtraction     bool     `yaml:"data_extraction"`
}

// DefaultCoordinatorConfig returns default configuration
func DefaultCoordinatorConfig() *CoordinatorConfig {
	return &CoordinatorConfig{
		EnableDependencies: true,
		WorkflowOrder:      []string{"port-discovery", "deep-scan"},
		DataExtraction:     true,
	}
}

// NewCoordinatorAgent creates a new coordinator agent
func NewCoordinatorAgent(config *CoordinatorConfig) *CoordinatorAgent {
	if config == nil {
		config = DefaultCoordinatorConfig()
	}
	
	return &CoordinatorAgent{
		BaseAgent:    NewBaseAgent("coordinator", nil),
		config:       config,
		workflowData: make(map[string]interface{}),
	}
}

// Validate checks if the coordinator is properly configured
func (c *CoordinatorAgent) Validate() error {
	if c.config == nil {
		return fmt.Errorf("coordinator config is required")
	}
	return nil
}

// Process coordinates workflow dependencies and extracts information
func (c *CoordinatorAgent) Process(input *AgentInput) (*AgentOutput, error) {
	if err := c.ValidateInput(input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	
	c.LogInfo("Coordinating workflow dependencies for target: %s", input.Target)
	
	output := c.CreateOutput(nil, input.Metadata, true)
	
	// Extract processed data from previous agents
	if processorData, ok := input.Data.(map[string]interface{}); ok {
		// Check if we have nmap data from port discovery
		if nmapData, exists := processorData["nmap"]; exists {
			// Extract discovered ports for use in deep scan
			discoveredPorts, err := c.extractDiscoveredPorts(nmapData)
			if err != nil {
				c.LogError("Failed to extract discovered ports: %v", err)
				output.AddError(fmt.Errorf("failed to extract discovered ports: %w", err))
			} else {
				// Store discovered ports for use in subsequent workflows
				c.workflowData["discovered_ports"] = discoveredPorts
				output.Metadata["discovered_ports"] = discoveredPorts
				c.LogInfo("Extracted %s for deep scan", discoveredPorts)
			}
		}
	}
	
	// Pass through input data with coordination metadata added to output metadata
	output.Data = input.Data
	output.Metadata["coordination_completed"] = "true"
	output.Metadata["workflow_stage"] = "coordinated"
	output.Metadata["dependencies"] = "resolved"
	
	// Store coordination results separately if needed
	c.workflowData["coordination_meta"] = map[string]string{
		"workflow_stage": "coordinated",
		"dependencies":   "resolved",
	}
	
	c.LogInfo("Workflow coordination completed")
	return output, nil
}

// CoordinationResult represents the result of workflow coordination
type CoordinationResult struct {
	WorkflowData     map[string]interface{} `json:"workflow_data"`
	ProcessedData    interface{}            `json:"processed_data"`
	CoordinationMeta map[string]string      `json:"coordination_meta"`
}

// extractDiscoveredPorts extracts open ports from nmap data for use in deep scan
func (c *CoordinatorAgent) extractDiscoveredPorts(nmapData interface{}) (string, error) {
	// Handle different nmap data formats
	switch data := nmapData.(type) {
	case map[string]interface{}:
		// If it's a map, look for cleaned nmap data
		if cleanedData, exists := data["cleaned_data"]; exists {
			return c.extractPortsFromCleanedData(cleanedData)
		}
	case *NmapData:
		// If it's directly NmapData struct
		return c.extractPortsFromNmapData(data), nil
	}
	
	return "", fmt.Errorf("unsupported nmap data format")
}

// extractPortsFromCleanedData extracts ports from cleaned nmap data structure
func (c *CoordinatorAgent) extractPortsFromCleanedData(cleanedData interface{}) (string, error) {
	// Try to cast to map and extract ports
	if dataMap, ok := cleanedData.(map[string]interface{}); ok {
		if portsInterface, exists := dataMap["ports"]; exists {
			if portsList, ok := portsInterface.([]interface{}); ok {
				var openPorts []string
				for _, portInterface := range portsList {
					if portMap, ok := portInterface.(map[string]interface{}); ok {
						if state, exists := portMap["state"]; exists && state == "open" {
							if number, exists := portMap["number"]; exists {
								openPorts = append(openPorts, fmt.Sprintf("%v", number))
							}
						}
					}
				}
				if len(openPorts) > 0 {
					return strings.Join(openPorts, ","), nil
				}
			}
		}
	}
	
	return "", fmt.Errorf("no open ports found in cleaned data")
}

// extractPortsFromNmapData extracts ports from NmapData struct
func (c *CoordinatorAgent) extractPortsFromNmapData(data *NmapData) string {
	var openPorts []string
	for _, port := range data.Ports {
		if port.State == "open" {
			openPorts = append(openPorts, strconv.Itoa(port.Number))
		}
	}
	return strings.Join(openPorts, ",")
}

