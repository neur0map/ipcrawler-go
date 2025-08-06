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

// LoadWorkflows loads workflows from a directory - alias for LoadTemplateWorkflows
func LoadWorkflows(workflowsDir string) (map[string]*Workflow, error) {
	// Extract template name from the path (assume it's the parent directory)
	parts := strings.Split(workflowsDir, "/")
	if len(parts) >= 2 {
		templateName := parts[len(parts)-2] // Get the second-to-last part (template name)
		baseDir := strings.Join(parts[:len(parts)-2], "/") // Everything except "scanning" and template
		return LoadTemplateWorkflows(baseDir, templateName)
	}
	
	// Fallback - try to load from current directory structure
	return LoadTemplateWorkflows("workflows", "basic")
}

// ExtractToolsFromWorkflows extracts all tools used in the workflows
func ExtractToolsFromWorkflows(workflows map[string]*Workflow) []string {
	toolsMap := make(map[string]bool)
	var tools []string
	
	for _, workflow := range workflows {
		for _, step := range workflow.Steps {
			if !toolsMap[step.Tool] {
				toolsMap[step.Tool] = true
				tools = append(tools, step.Tool)
			}
		}
	}
	
	return tools
}

// ExtractToolsAndArgsFromWorkflows extracts tools and their arguments from workflows
func ExtractToolsAndArgsFromWorkflows(workflows map[string]*Workflow) ([]string, [][]string) {
	toolsMap := make(map[string][]string)
	var tools []string
	var allArgs [][]string
	
	for _, workflow := range workflows {
		for _, step := range workflow.Steps {
			if _, exists := toolsMap[step.Tool]; !exists {
				tools = append(tools, step.Tool)
				// Use sudo args if available, otherwise normal args, otherwise legacy args
				if len(step.ArgsSudo) > 0 {
					toolsMap[step.Tool] = step.ArgsSudo
				} else if len(step.ArgsNormal) > 0 {
					toolsMap[step.Tool] = step.ArgsNormal
				} else {
					toolsMap[step.Tool] = step.Args
				}
			}
		}
	}
	
	// Build the args array in the same order as tools
	for _, tool := range tools {
		allArgs = append(allArgs, toolsMap[tool])
	}
	
	return tools, allArgs
}

// extractToolFromWorkflow extracts the tool name from the first step of a workflow
func extractToolFromWorkflow(workflow *Workflow) string {
	if len(workflow.Steps) > 0 {
		return workflow.Steps[0].Tool
	}
	return "unknown"
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