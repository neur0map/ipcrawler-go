package loader

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ToolConfig represents a tool with its requirements
type ToolConfig struct {
	Name         string `yaml:"name"`
	RequiresSudo bool   `yaml:"requires_sudo"`
	Reason       string `yaml:"reason,omitempty"`
}

// WorkflowConfig represents a workflow configuration
type WorkflowConfig struct {
	Name        string       `yaml:"name"`
	Description string       `yaml:"description"`
	Tools       []ToolConfig `yaml:"tools"`
}

// WorkflowData holds all workflow configurations
type WorkflowData struct {
	Workflows map[string]WorkflowConfig
}

// LoadWorkflowDescriptions loads and parses the workflows/descriptions.yaml file
func LoadWorkflowDescriptions(basePath string) (*WorkflowData, error) {
	// Try multiple possible locations for the workflow file
	possiblePaths := []string{
		filepath.Join(basePath, "workflows", "descriptions.yaml"),
		filepath.Join(basePath, "..", "workflows", "descriptions.yaml"),
		filepath.Join("workflows", "descriptions.yaml"),
		filepath.Join(".", "workflows", "descriptions.yaml"),
	}
	
	// If basePath is empty, start with current directory
	if basePath == "" {
		basePath = "."
	}
	
	var data []byte
	var err error
	var foundPath string
	
	// Try each path until we find the file
	for _, path := range possiblePaths {
		data, err = os.ReadFile(path)
		if err == nil {
			foundPath = path
			break
		}
	}
	
	if data == nil {
		return nil, fmt.Errorf("failed to find workflows/descriptions.yaml in any expected location")
	}

	var workflows map[string]WorkflowConfig
	if err := yaml.Unmarshal(data, &workflows); err != nil {
		return nil, fmt.Errorf("failed to parse workflows YAML from %s: %w", foundPath, err)
	}

	return &WorkflowData{Workflows: workflows}, nil
}

// GetWorkflowNames returns all workflow names for display
func (wd *WorkflowData) GetWorkflowNames() []string {
	names := make([]string, 0, len(wd.Workflows))
	for key := range wd.Workflows {
		names = append(names, key)
	}
	return names
}

// GetWorkflow returns a specific workflow by key
func (wd *WorkflowData) GetWorkflow(key string) (WorkflowConfig, bool) {
	workflow, exists := wd.Workflows[key]
	return workflow, exists
}

// GetAllTools returns all unique tools across all workflows
func (wd *WorkflowData) GetAllTools() []string {
	toolSet := make(map[string]bool)
	for _, workflow := range wd.Workflows {
		for _, tool := range workflow.Tools {
			toolSet[tool.Name] = true
		}
	}

	tools := make([]string, 0, len(toolSet))
	for tool := range toolSet {
		tools = append(tools, tool)
	}
	return tools
}

// GetToolsRequiringSudo returns all tools that require sudo privileges
func (wd *WorkflowData) GetToolsRequiringSudo() []ToolConfig {
	var sudoTools []ToolConfig
	toolSet := make(map[string]ToolConfig)
	
	for _, workflow := range wd.Workflows {
		for _, tool := range workflow.Tools {
			if tool.RequiresSudo {
				toolSet[tool.Name] = tool
			}
		}
	}
	
	for _, tool := range toolSet {
		sudoTools = append(sudoTools, tool)
	}
	return sudoTools
}

// HasToolsRequiringSudo checks if any workflow contains tools requiring sudo
func (wd *WorkflowData) HasToolsRequiringSudo() bool {
	for _, workflow := range wd.Workflows {
		for _, tool := range workflow.Tools {
			if tool.RequiresSudo {
				return true
			}
		}
	}
	return false
}

// GetWorkflowSummary returns a summary string for overview
func (wd *WorkflowData) GetWorkflowSummary() string {
	totalWorkflows := len(wd.Workflows)
	totalTools := len(wd.GetAllTools())
	
	return fmt.Sprintf("Total Workflows: %d | Total Tools: %d", totalWorkflows, totalTools)
}