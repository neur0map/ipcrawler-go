package services

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// SecurityValidator validates YAML files for malicious content
type SecurityValidator struct {
	rules *SecurityValidationRules
}

// SecurityValidationRules contains all validation rules from config/security.yaml
type SecurityValidationRules struct {
	SecurityValidation struct {
		YAMLSecurity struct {
			ForbiddenYAMLTags     []string `yaml:"forbidden_yaml_tags"`
			ForbiddenYAMLPatterns []string `yaml:"forbidden_yaml_patterns"`
		} `yaml:"yaml_security"`
		
		ToolConfigValidation struct {
			RequiredFields    []string `yaml:"required_fields"`
			CommandValidation struct {
				AllowedCommands            []string `yaml:"allowed_commands"`
				ForbiddenCommandPatterns   []string `yaml:"forbidden_command_patterns"`
			} `yaml:"command_validation"`
			ArgumentValidation struct {
				MaxLengths struct {
					ToolName       int `yaml:"tool_name"`
					Command        int `yaml:"command"`
					SingleArgument int `yaml:"single_argument"`
					TotalArguments int `yaml:"total_arguments"`
				} `yaml:"max_lengths"`
				ForbiddenArgumentPatterns []string `yaml:"forbidden_argument_patterns"`
			} `yaml:"argument_validation"`
			PathValidation struct {
				AllowedOutputDirs []string `yaml:"allowed_output_dirs"`
				ForbiddenPaths    []string `yaml:"forbidden_paths"`
			} `yaml:"path_validation"`
		} `yaml:"tool_config_validation"`
		
		WorkflowConfigValidation struct {
			RequiredFields []string `yaml:"required_fields"`
			StepValidation struct {
				MaxLimits struct {
					StepsPerWorkflow     int `yaml:"steps_per_workflow"`
					DependenciesPerStep  int `yaml:"dependencies_per_step"`
					OverrideArgsPerStep  int `yaml:"override_args_per_step"`
				} `yaml:"max_limits"`
				RequiredStepFields     []string `yaml:"required_step_fields"`
				AllowedStepTypes       []string `yaml:"allowed_step_types"`
				AllowedToolsInSteps    []string `yaml:"allowed_tools_in_steps"`
			} `yaml:"step_validation"`
		} `yaml:"workflow_config_validation"`
		
		GeneralSecurity struct {
			MaxFileSizes struct {
				ToolConfig     string `yaml:"tool_config"`
				WorkflowConfig string `yaml:"workflow_config"`
			} `yaml:"max_file_sizes"`
			RequiredEncoding string `yaml:"required_encoding"`
			MaxLineLength    int    `yaml:"max_line_length"`
			MaxLinesPerFile  int    `yaml:"max_lines_per_file"`
			MaxYAMLDepth     int    `yaml:"max_yaml_depth"`
			MaxArrayLength   int    `yaml:"max_array_length"`
			MaxObjectKeys    int    `yaml:"max_object_keys"`
		} `yaml:"general_security"`
	} `yaml:"security_validation"`
	
	SeverityLevels struct {
		Critical []string `yaml:"critical"`
		High     []string `yaml:"high"`
		Medium   []string `yaml:"medium"`
		Low      []string `yaml:"low"`
	} `yaml:"severity_levels"`
}

// ValidationResult contains the result of security validation
type ValidationResult struct {
	Valid    bool                    `json:"valid"`
	Errors   []ValidationError       `json:"errors"`
	Warnings []ValidationError       `json:"warnings"`
	Info     []ValidationError       `json:"info"`
}

// ValidationError represents a security validation issue
type ValidationError struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Message     string `json:"message"`
	Line        int    `json:"line,omitempty"`
	Column      int    `json:"column,omitempty"`
	FilePath    string `json:"file_path"`
}

// NewSecurityValidator creates a new security validator
func NewSecurityValidator() (*SecurityValidator, error) {
	rules, err := loadSecurityValidationRules()
	if err != nil {
		return nil, fmt.Errorf("loading security validation rules: %w", err)
	}
	
	return &SecurityValidator{
		rules: rules,
	}, nil
}

// loadSecurityValidationRules loads validation rules from config/security.yaml
func loadSecurityValidationRules() (*SecurityValidationRules, error) {
	data, err := os.ReadFile("config/security.yaml")
	if err != nil {
		return nil, fmt.Errorf("reading security config: %w", err)
	}
	
	var rules SecurityValidationRules
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("parsing security config: %w", err)
	}
	
	return &rules, nil
}

// ValidateToolConfig validates a tool configuration file
func (sv *SecurityValidator) ValidateToolConfig(filePath string) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []ValidationError{},
		Info:     []ValidationError{},
	}
	
	// Read and validate file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}
	
	// Basic security checks
	if err := sv.validateBasicSecurity(filePath, content, result); err != nil {
		return result, err
	}
	
	// Parse YAML
	var toolConfig map[string]interface{}
	if err := yaml.Unmarshal(content, &toolConfig); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Type:     "yaml_parse_error",
			Severity: "critical",
			Message:  fmt.Sprintf("Failed to parse YAML: %v", err),
			FilePath: filePath,
		})
		return result, nil
	}
	
	// Validate tool-specific fields
	sv.validateToolFields(toolConfig, filePath, result)
	sv.validateToolCommand(toolConfig, filePath, result)
	sv.validateToolArguments(toolConfig, filePath, result)
	
	return result, nil
}

// ValidateWorkflowConfig validates a workflow configuration file
func (sv *SecurityValidator) ValidateWorkflowConfig(filePath string) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []ValidationError{},
		Info:     []ValidationError{},
	}
	
	// Read and validate file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}
	
	// Basic security checks
	if err := sv.validateBasicSecurity(filePath, content, result); err != nil {
		return result, err
	}
	
	// Parse YAML
	var workflowConfig map[string]interface{}
	if err := yaml.Unmarshal(content, &workflowConfig); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Type:     "yaml_parse_error",
			Severity: "critical",
			Message:  fmt.Sprintf("Failed to parse YAML: %v", err),
			FilePath: filePath,
		})
		return result, nil
	}
	
	// Validate workflow-specific fields
	sv.validateWorkflowFields(workflowConfig, filePath, result)
	sv.validateWorkflowSteps(workflowConfig, filePath, result)
	
	return result, nil
}

// validateBasicSecurity performs basic security validation
func (sv *SecurityValidator) validateBasicSecurity(filePath string, content []byte, result *ValidationResult) error {
	contentStr := string(content)
	
	// Check for forbidden YAML tags
	for _, tag := range sv.rules.SecurityValidation.YAMLSecurity.ForbiddenYAMLTags {
		if strings.Contains(contentStr, tag) {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Type:     "forbidden_yaml_tag",
				Severity: "critical",
				Message:  fmt.Sprintf("Forbidden YAML tag detected: %s", tag),
				FilePath: filePath,
			})
		}
	}
	
	// Check for forbidden YAML patterns
	for _, pattern := range sv.rules.SecurityValidation.YAMLSecurity.ForbiddenYAMLPatterns {
		matched, err := regexp.MatchString(pattern, contentStr)
		if err != nil {
			continue // Skip invalid regex patterns
		}
		if matched {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Type:     "forbidden_yaml_pattern",
				Severity: "high",
				Message:  fmt.Sprintf("Forbidden YAML pattern detected: %s", pattern),
				FilePath: filePath,
			})
		}
	}
	
	// Validate file encoding
	if !utf8.Valid(content) {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Type:     "invalid_encoding",
			Severity: "high",
			Message:  "File is not valid UTF-8",
			FilePath: filePath,
		})
	}
	
	// Check line limits
	lines := strings.Split(contentStr, "\n")
	maxLines := sv.rules.SecurityValidation.GeneralSecurity.MaxLinesPerFile
	if len(lines) > maxLines {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Type:     "too_many_lines",
			Severity: "medium",
			Message:  fmt.Sprintf("Too many lines: %d > %d", len(lines), maxLines),
			FilePath: filePath,
		})
	}
	
	// Check line length limits
	maxLineLength := sv.rules.SecurityValidation.GeneralSecurity.MaxLineLength
	for i, line := range lines {
		if len(line) > maxLineLength {
			result.Warnings = append(result.Warnings, ValidationError{
				Type:     "line_too_long",
				Severity: "low",
				Message:  fmt.Sprintf("Line too long: %d > %d", len(line), maxLineLength),
				Line:     i + 1,
				FilePath: filePath,
			})
		}
	}
	
	return nil
}

// validateToolFields validates required tool configuration fields
func (sv *SecurityValidator) validateToolFields(config map[string]interface{}, filePath string, result *ValidationResult) {
	requiredFields := sv.rules.SecurityValidation.ToolConfigValidation.RequiredFields
	
	for _, field := range requiredFields {
		if !sv.hasNestedField(config, field) {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Type:     "missing_required_field",
				Severity: "high",
				Message:  fmt.Sprintf("Required field missing: %s", field),
				FilePath: filePath,
			})
		}
	}
}

// validateToolCommand validates tool command security
func (sv *SecurityValidator) validateToolCommand(config map[string]interface{}, filePath string, result *ValidationResult) {
	command, ok := config["command"].(string)
	if !ok {
		return // Already caught by required fields validation
	}
	
	// Check if command is in allowed list
	allowed := sv.rules.SecurityValidation.ToolConfigValidation.CommandValidation.AllowedCommands
	isAllowed := false
	for _, allowedCmd := range allowed {
		if command == allowedCmd {
			isAllowed = true
			break
		}
	}
	
	if !isAllowed {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Type:     "forbidden_command",
			Severity: "critical",
			Message:  fmt.Sprintf("Command not in allowed list: %s", command),
			FilePath: filePath,
		})
	}
	
	// Check for forbidden command patterns
	forbidden := sv.rules.SecurityValidation.ToolConfigValidation.CommandValidation.ForbiddenCommandPatterns
	for _, pattern := range forbidden {
		matched, err := regexp.MatchString(pattern, command)
		if err != nil {
			continue // Skip invalid regex patterns
		}
		if matched {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Type:     "forbidden_command",
				Severity: "critical",
				Message:  fmt.Sprintf("Forbidden command pattern detected: %s", pattern),
				FilePath: filePath,
			})
		}
	}
}

// validateToolArguments validates tool arguments
func (sv *SecurityValidator) validateToolArguments(config map[string]interface{}, filePath string, result *ValidationResult) {
	args, ok := config["args"].(map[string]interface{})
	if !ok {
		return
	}
	
	// Validate argument patterns
	forbidden := sv.rules.SecurityValidation.ToolConfigValidation.ArgumentValidation.ForbiddenArgumentPatterns
	
	// Check default args
	if defaultArgs, ok := args["default"].([]interface{}); ok {
		for _, arg := range defaultArgs {
			if argStr, ok := arg.(string); ok {
				sv.validateSingleArgument(argStr, forbidden, filePath, result)
			}
		}
	}
	
	// Check flag args
	if flags, ok := args["flags"].(map[string]interface{}); ok {
		for _, flagArgs := range flags {
			if flagArgList, ok := flagArgs.([]interface{}); ok {
				for _, arg := range flagArgList {
					if argStr, ok := arg.(string); ok {
						sv.validateSingleArgument(argStr, forbidden, filePath, result)
					}
				}
			}
		}
	}
}

// validateSingleArgument validates a single argument string
func (sv *SecurityValidator) validateSingleArgument(arg string, forbidden []string, filePath string, result *ValidationResult) {
	for _, pattern := range forbidden {
		matched, err := regexp.MatchString(pattern, arg)
		if err != nil {
			continue // Skip invalid regex patterns
		}
		if matched {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Type:     "forbidden_argument",
				Severity: "high",
				Message:  fmt.Sprintf("Forbidden argument pattern detected: %s in %s", pattern, arg),
				FilePath: filePath,
			})
		}
	}
}

// validateWorkflowFields validates required workflow configuration fields
func (sv *SecurityValidator) validateWorkflowFields(config map[string]interface{}, filePath string, result *ValidationResult) {
	requiredFields := sv.rules.SecurityValidation.WorkflowConfigValidation.RequiredFields
	
	for _, field := range requiredFields {
		if !sv.hasNestedField(config, field) {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Type:     "missing_required_field",
				Severity: "high",
				Message:  fmt.Sprintf("Required field missing: %s", field),
				FilePath: filePath,
			})
		}
	}
}

// validateWorkflowSteps validates workflow steps
func (sv *SecurityValidator) validateWorkflowSteps(config map[string]interface{}, filePath string, result *ValidationResult) {
	steps, ok := config["steps"].([]interface{})
	if !ok {
		return
	}
	
	// Check maximum number of steps
	maxSteps := sv.rules.SecurityValidation.WorkflowConfigValidation.StepValidation.MaxLimits.StepsPerWorkflow
	if len(steps) > maxSteps {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Type:     "too_many_steps",
			Severity: "medium",
			Message:  fmt.Sprintf("Too many steps: %d > %d", len(steps), maxSteps),
			FilePath: filePath,
		})
	}
	
	// Validate each step
	allowedTools := sv.rules.SecurityValidation.WorkflowConfigValidation.StepValidation.AllowedToolsInSteps
	for i, step := range steps {
		if stepMap, ok := step.(map[string]interface{}); ok {
			// Check if tool is allowed
			if tool, ok := stepMap["tool"].(string); ok {
				isAllowed := false
				for _, allowedTool := range allowedTools {
					if tool == allowedTool {
						isAllowed = true
						break
					}
				}
				if !isAllowed {
					result.Valid = false
					result.Errors = append(result.Errors, ValidationError{
						Type:     "forbidden_tool_in_step",
						Severity: "high",
						Message:  fmt.Sprintf("Tool not allowed in step %d: %s", i+1, tool),
						FilePath: filePath,
					})
				}
			}
		}
	}
}

// hasNestedField checks if a nested field exists (e.g., "execution.allowed")
func (sv *SecurityValidator) hasNestedField(config map[string]interface{}, fieldPath string) bool {
	parts := strings.Split(fieldPath, ".")
	current := config
	
	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part - check if it exists
			_, exists := current[part]
			return exists
		} else {
			// Intermediate part - navigate deeper
			if next, ok := current[part].(map[string]interface{}); ok {
				current = next
			} else {
				return false
			}
		}
	}
	
	return false
}

// ValidateAllToolConfigs validates all tool configurations in the tools directory
func (sv *SecurityValidator) ValidateAllToolConfigs(toolsDir string) (map[string]*ValidationResult, error) {
	results := make(map[string]*ValidationResult)
	
	err := filepath.Walk(toolsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Only validate config.yaml files
		if info.Name() == "config.yaml" {
			result, err := sv.ValidateToolConfig(path)
			if err != nil {
				return fmt.Errorf("validating %s: %w", path, err)
			}
			results[path] = result
		}
		
		return nil
	})
	
	return results, err
}

// ValidateAllWorkflowConfigs validates all workflow configurations
func (sv *SecurityValidator) ValidateAllWorkflowConfigs(workflowsDir string) (map[string]*ValidationResult, error) {
	results := make(map[string]*ValidationResult)
	
	err := filepath.Walk(workflowsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Only validate .yaml files
		if strings.HasSuffix(info.Name(), ".yaml") && !info.IsDir() {
			result, err := sv.ValidateWorkflowConfig(path)
			if err != nil {
				return fmt.Errorf("validating %s: %w", path, err)
			}
			results[path] = result
		}
		
		return nil
	})
	
	return results, err
}