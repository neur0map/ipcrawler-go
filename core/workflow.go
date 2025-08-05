package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Workflow struct {
	Name          string       `yaml:"name"`
	Description   string       `yaml:"description"`
	Report        interface{}  `yaml:"report,omitempty"` // Can be bool or ReportConfig
	Requires      []string     `yaml:"requires,omitempty"` // Dependencies on other workflows
	Provides      []string     `yaml:"provides,omitempty"` // Data this workflow provides
	ParallelGroup string       `yaml:"parallel_group,omitempty"` // Workflows with same group can run in parallel
	Steps         []Step       `yaml:"steps"`
}

type Step struct {
	Tool       string   `yaml:"tool"`
	Args       []string `yaml:"args,omitempty"`       // Legacy single args (for backward compatibility)
	ArgsSudo   []string `yaml:"args_sudo,omitempty"`   // Arguments requiring sudo
	ArgsNormal []string `yaml:"args_normal,omitempty"` // Arguments for normal mode
}

// GetArgs returns the appropriate arguments based on sudo preference
func (s *Step) GetArgs(useSudo bool) []string {
	// If dual args are defined, use appropriate set
	if len(s.ArgsSudo) > 0 && len(s.ArgsNormal) > 0 {
		if useSudo {
			return s.ArgsSudo
		}
		return s.ArgsNormal
	}
	
	// Fall back to legacy args field for backward compatibility
	return s.Args
}

type ReportConfig struct {
	Enabled      bool     `yaml:"enabled"`
	OutputFormat []string `yaml:"output_format"`
	Agents       []string `yaml:"agents"`
	Coordination bool     `yaml:"coordination,omitempty"` // Whether this workflow needs coordination
}

type AgentConfig struct {
	Receiver *ReceiverAgentConfig `yaml:"receiver,omitempty"`
	Cleaner  *CleanerAgentConfig  `yaml:"cleaner,omitempty"`
	Reviewer *ReviewerAgentConfig `yaml:"reviewer,omitempty"`
	Reporter *ReporterAgentConfig `yaml:"reporter,omitempty"`
}

type ReceiverAgentConfig struct {
	ValidateSchema bool   `yaml:"validate_schema"`
	ErrorHandling  string `yaml:"error_handling"`
}

type CleanerAgentConfig struct {
	Type                string   `yaml:"type"`
	ExtractFields       []string `yaml:"extract_fields"`
	SeverityFilter      []string `yaml:"severity_filter,omitempty"`
	GroupByTemplate     bool     `yaml:"group_by_template,omitempty"`
	IncludeClosedPorts  bool     `yaml:"include_closed_ports,omitempty"`
	MinimumPorts        int      `yaml:"minimum_ports,omitempty"`
	FilterStatusCodes   []string `yaml:"filter_status_codes,omitempty"`
	MinResponseSize     int      `yaml:"min_response_size,omitempty"`
}

type ReviewerAgentConfig struct {
	ValidationRules []string `yaml:"validation_rules"`
}

type ReporterAgentConfig struct {
	Templates      []string `yaml:"templates"`
	IncludeRawData bool     `yaml:"include_raw_data"`
}

func LoadWorkflow(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow file: %w", err)
	}

	var workflow Workflow
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return nil, fmt.Errorf("failed to parse workflow file: %w", err)
	}

	return &workflow, nil
}

// extractToolFromWorkflow extracts the tool name from the first step of a workflow
func extractToolFromWorkflow(workflow *Workflow) string {
	if len(workflow.Steps) > 0 {
		return workflow.Steps[0].Tool
	}
	return "unknown"
}

// IsNucleiWorkflow checks if a workflow uses nuclei or is a vulnerability scan
func IsNucleiWorkflow(workflow *Workflow) bool {
	if len(workflow.Steps) > 0 && workflow.Steps[0].Tool == "nuclei" {
		return true
	}
	// Also check if it's a vulnerability scan (alternative detection)
	return strings.Contains(strings.ToLower(workflow.Name), "vulnerability") ||
		   strings.Contains(strings.ToLower(workflow.Description), "vulnerability")
}

// IsNmapDeepScan checks if a workflow is an nmap deep scan
func IsNmapDeepScan(workflow *Workflow) bool {
	if len(workflow.Steps) == 0 || workflow.Steps[0].Tool != "nmap" {
		return false
	}
	// Check if it's a deep scan by name or description
	return strings.Contains(strings.ToLower(workflow.Name), "deep") ||
		   strings.Contains(strings.ToLower(workflow.Description), "deep") ||
		   strings.Contains(strings.ToLower(workflow.Name), "service") ||
		   strings.Contains(strings.ToLower(workflow.Description), "service")
}

func LoadTemplateWorkflows(workflowsDir, templateName string) (map[string]*Workflow, error) {
	templateDir := filepath.Join(workflowsDir, templateName)
	
	// Check if template directory exists
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("template directory not found: %s", templateDir)
	}

	workflows := make(map[string]*Workflow)

	// Recursively walk all directories and find YAML files
	err := filepath.WalkDir(templateDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip files/directories we can't access
		}
		
		// Skip directories and non-YAML files
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}
		
		// Load workflow and extract tool name from YAML content
		workflow, err := LoadWorkflow(path)
		if err != nil {
			return fmt.Errorf("failed to load workflow %s: %w", path, err)
		}
		
		// Get tool name from first step (more reliable than directory name)
		toolName := extractToolFromWorkflow(workflow)
		workflowBaseName := strings.TrimSuffix(d.Name(), ".yaml")
		workflowKey := fmt.Sprintf("%s_%s", toolName, workflowBaseName)
		
		workflows[workflowKey] = workflow
		return nil
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to walk template directory: %w", err)
	}

	return workflows, nil
}

func (w *Workflow) ReplaceVars(vars map[string]string) {
	for i, step := range w.Steps {
		// Replace placeholders in legacy Args field
		for j, arg := range step.Args {
			w.Steps[i].Args[j] = replacePlaceholders(arg, vars)
		}
		
		// Replace placeholders in ArgsSudo field
		for j, arg := range step.ArgsSudo {
			w.Steps[i].ArgsSudo[j] = replacePlaceholders(arg, vars)
		}
		
		// Replace placeholders in ArgsNormal field
		for j, arg := range step.ArgsNormal {
			w.Steps[i].ArgsNormal[j] = replacePlaceholders(arg, vars)
		}
	}
}

// HasReporting returns true if the workflow has reporting enabled
func (w *Workflow) HasReporting() bool {
	if w.Report == nil {
		return false
	}
	
	// Handle simple boolean format
	if enabled, ok := w.Report.(bool); ok {
		return enabled
	}
	
	// Handle map format (YAML parsed as map[string]interface{})
	if reportMap, ok := w.Report.(map[string]interface{}); ok {
		if enabled, ok := reportMap["enabled"].(bool); ok {
			return enabled
		}
	}
	
	// Handle full config format
	if config, ok := w.Report.(*ReportConfig); ok {
		return config.Enabled
	}
	
	return false
}

// GetReportConfig returns the report configuration or default if using simple format
func (w *Workflow) GetReportConfig() *ReportConfig {
	if w.Report == nil {
		return nil
	}
	
	// If it's a simple boolean, return default config
	if enabled, ok := w.Report.(bool); ok && enabled {
		return &ReportConfig{
			Enabled:      true,
			OutputFormat: []string{"json", "txt"},
			Agents:       []string{"receiver", "validator", "reporter"}, // Default agents
		}
	}
	
	// If it's a map (YAML parsed as map[string]interface{}), convert it
	if reportMap, ok := w.Report.(map[string]interface{}); ok {
		config := &ReportConfig{}
		if enabled, ok := reportMap["enabled"].(bool); ok {
			config.Enabled = enabled
		}
		if agents, ok := reportMap["agents"].([]interface{}); ok {
			config.Agents = make([]string, 0, len(agents))
			for _, agent := range agents {
				if agentStr, ok := agent.(string); ok {
					config.Agents = append(config.Agents, agentStr)
				}
			}
		}
		if outputFormat, ok := reportMap["output_format"].([]interface{}); ok {
			config.OutputFormat = make([]string, 0, len(outputFormat))
			for _, format := range outputFormat {
				if formatStr, ok := format.(string); ok {
					config.OutputFormat = append(config.OutputFormat, formatStr)
				}
			}
		} else {
			config.OutputFormat = []string{"json", "txt"}
		}
		return config
	}
	
	// If it's already a config, return it
	if config, ok := w.Report.(*ReportConfig); ok {
		return config
	}
	
	return nil
}

func replacePlaceholders(s string, vars map[string]string) string {
	result := s
	for key, value := range vars {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

func (w *Workflow) GetCommand(step Step) string {
	return fmt.Sprintf("%s %s", step.Tool, strings.Join(step.Args, " "))
}

// GetCommandWithArgs creates a command string with specific args
func (w *Workflow) GetCommandWithArgs(tool string, args []string) string {
	return fmt.Sprintf("%s %s", tool, strings.Join(args, " "))
}