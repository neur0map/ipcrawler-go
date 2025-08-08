package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Enhanced database manager for all IPCrawler configuration
type Database struct {
	data map[string]interface{}
	system *SystemConfig
	workflow *WorkflowConfig
	toolDefaults *ToolDefaults
	errorMessages *ErrorMessages
}

// Configuration structures for type-safe access
type SystemConfig struct {
	System struct {
		Defaults         map[string]interface{} `yaml:"defaults"`
		Directories      map[string]interface{} `yaml:"directories"`
		Timeouts         map[string]int         `yaml:"timeouts"`
		Limits           map[string]int         `yaml:"limits"`
		Logging          map[string]interface{} `yaml:"logging"`
		StatusIndicators map[string]string      `yaml:"status_indicators"`
	} `yaml:"system"`
}

type WorkflowConfig struct {
	Workflow struct {
		StepTypes       map[string]interface{} `yaml:"step_types"`
		FileFormats     map[string]interface{} `yaml:"file_formats"`
		ErrorHandling   map[string]interface{} `yaml:"error_handling"`
		ExecutionModes  map[string]interface{} `yaml:"execution_modes"`
		StatusMessages  map[string]string      `yaml:"status_messages"`
	} `yaml:"workflow"`
}

type ToolDefaults struct {
	ToolDefaults struct {
		Categories       map[string]interface{} `yaml:"categories"`
		CommonFlags      map[string]interface{} `yaml:"common_flags"`
		ArgumentPatterns map[string]interface{} `yaml:"argument_patterns"`
		Security         map[string]interface{} `yaml:"security"`
		Execution        map[string]interface{} `yaml:"execution"`
		ExitCodes        map[string]interface{} `yaml:"exit_codes"`
		OutputValidation map[string]interface{} `yaml:"output_validation"`
	} `yaml:"tool_defaults"`
}

type ErrorMessages struct {
	ErrorMessages struct {
		System    map[string]string `yaml:"system"`
		Tools     map[string]string `yaml:"tools"`
		Workflows map[string]string `yaml:"workflows"`
		Files     map[string]string `yaml:"files"`
		Network   map[string]string `yaml:"network"`
		Database  map[string]string `yaml:"database"`
	} `yaml:"error_messages"`
	Warnings map[string]interface{} `yaml:"warnings"`
	Success  map[string]interface{} `yaml:"success"`
	Info     map[string]interface{} `yaml:"info"`
	Help     map[string]interface{} `yaml:"help"`
	Prompts  map[string]interface{} `yaml:"prompts"`
}

var globalDB *Database

// LoadDatabase loads all YAML files from the database folder
func LoadDatabase() (*Database, error) {
	if globalDB != nil {
		return globalDB, nil
	}

	globalDB = &Database{
		data: make(map[string]interface{}),
	}

	// Load all YAML files from database folder
	databaseDir := "database"
	if _, err := os.Stat(databaseDir); os.IsNotExist(err) {
		return globalDB, nil // Return empty database if folder doesn't exist
	}

	files, err := os.ReadDir(databaseDir)
	if err != nil {
		return globalDB, err
	}

	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".yaml" {
			filePath := filepath.Join(databaseDir, file.Name())
			if err := globalDB.loadYAMLFile(filePath); err != nil {
				fmt.Printf("Warning: Failed to load %s: %v\n", filePath, err)
			}
		}
	}

	// Load enhanced configurations after all files are loaded
	if err := globalDB.loadEnhancedConfigs(); err != nil {
		fmt.Printf("Warning: Failed to load enhanced configs: %v\n", err)
	}

	return globalDB, nil
}

func (db *Database) loadYAMLFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var content map[string]interface{}
	if err := yaml.Unmarshal(data, &content); err != nil {
		return err
	}

	// Use filename (without extension) as the key
	fileName := filepath.Base(filePath)
	key := fileName[:len(fileName)-len(filepath.Ext(fileName))]
	db.data[key] = content

	return nil
}

// loadEnhancedConfigs loads and parses the enhanced configuration files
func (db *Database) loadEnhancedConfigs() error {
	// Load system configuration
	if systemData, ok := db.data["system_config"].(map[string]interface{}); ok {
		db.system = &SystemConfig{}
		data, _ := yaml.Marshal(systemData)
		yaml.Unmarshal(data, db.system)
	}
	
	// Load workflow configuration
	if workflowData, ok := db.data["workflow_config"].(map[string]interface{}); ok {
		db.workflow = &WorkflowConfig{}
		data, _ := yaml.Marshal(workflowData)
		yaml.Unmarshal(data, db.workflow)
	}
	
	// Load tool defaults
	if toolData, ok := db.data["tool_defaults"].(map[string]interface{}); ok {
		db.toolDefaults = &ToolDefaults{}
		data, _ := yaml.Marshal(toolData)
		yaml.Unmarshal(data, db.toolDefaults)
	}
	
	// Load error messages
	if errorData, ok := db.data["error_messages"].(map[string]interface{}); ok {
		db.errorMessages = &ErrorMessages{}
		data, _ := yaml.Marshal(errorData)
		yaml.Unmarshal(data, db.errorMessages)
	}
	
	return nil
}

// GetServiceName dynamically looks up service name from database/services.yaml
func (db *Database) GetServiceName(port int) string {
	if servicesData, ok := db.data["services"].(map[string]interface{}); ok {
		// Access the "services" map - it's type map[interface{}]interface{}
		if portToServiceMap, ok := servicesData["services"].(map[interface{}]interface{}); ok {
			// Look for the port as an integer key
			if service, exists := portToServiceMap[port]; exists {
				if serviceName, ok := service.(string); ok {
					return serviceName
				}
			}
		}
		// Check for default service
		if defaultService, ok := servicesData["default_service"].(string); ok {
			return defaultService
		}
	}
	return "Unknown"
}

// GetServiceCategories returns service categories from database/service_categories.yaml
func (db *Database) GetServiceCategories() map[string]interface{} {
	if categories, ok := db.data["service_categories"].(map[string]interface{}); ok {
		if categoriesMap, ok := categories["categories"].(map[string]interface{}); ok {
			return categoriesMap
		}
	}
	return make(map[string]interface{})
}

// GetRiskLevel determines risk level for a port from database/service_categories.yaml
// Returns the highest risk level if a port appears in multiple categories
func (db *Database) GetRiskLevel(port int) string {
	categories := db.GetServiceCategories()
	
	// Risk level priority: critical > high > medium > low > unknown
	riskPriority := map[string]int{
		"critical": 5,
		"high":     4,
		"medium":   3,
		"low":      2,
		"unknown":  1,
	}
	
	highestRisk := "unknown"
	highestPriority := 0
	
	for _, categoryData := range categories {
		if catMap, ok := categoryData.(map[string]interface{}); ok {
			if ports, ok := catMap["ports"].([]interface{}); ok {
				for _, p := range ports {
					if portInt, ok := p.(int); ok && portInt == port {
						if riskLevel, ok := catMap["risk_level"].(string); ok {
							if priority, exists := riskPriority[riskLevel]; exists && priority > highestPriority {
								highestRisk = riskLevel
								highestPriority = priority
							}
						}
					}
				}
			}
		}
	}
	return highestRisk
}

// IsToolAllowed checks if a tool is in the security whitelist
func (db *Database) IsToolAllowed(toolName string) bool {
	security := db.GetToolSecurity()
	if allowedCommands, ok := security["allowed_commands"].(map[string]interface{}); ok {
		if whitelist, ok := allowedCommands["whitelist"].([]interface{}); ok {
			for _, allowed := range whitelist {
				if allowedStr, ok := allowed.(string); ok && allowedStr == toolName {
					return true
				}
			}
		}
	}
	// If no whitelist is configured, allow all tools (backward compatibility)
	return len(db.GetToolSecurity()) == 0
}

// ValidateArguments checks if tool arguments are safe
func (db *Database) ValidateArguments(args []string) error {
	security := db.GetToolSecurity()
	if argValidation, ok := security["argument_validation"].(map[string]interface{}); ok {
		// Check forbidden characters
		if forbiddenChars, ok := argValidation["forbidden_chars"].([]interface{}); ok {
			for _, arg := range args {
				for _, forbidden := range forbiddenChars {
					if forbiddenStr, ok := forbidden.(string); ok {
						if strings.Contains(arg, forbiddenStr) {
							return fmt.Errorf("forbidden character '%s' in argument: %s", forbiddenStr, arg)
						}
					}
				}
			}
		}
		
		// Check maximum length
		if maxLength, ok := argValidation["max_length"].(int); ok {
			for _, arg := range args {
				if len(arg) > maxLength {
					return fmt.Errorf("argument too long (%d > %d): %s", len(arg), maxLength, arg)
				}
			}
		}
	}
	return nil
}

// GetData returns raw data for any database file
func (db *Database) GetData(key string) interface{} {
	return db.data[key]
}

// Enhanced getter methods for type-safe access

// GetSystemConfig returns the system configuration
func (db *Database) GetSystemConfig() *SystemConfig {
	return db.system
}

// GetWorkflowConfig returns the workflow configuration
func (db *Database) GetWorkflowConfig() *WorkflowConfig {
	return db.workflow
}

// GetToolDefaults returns the tool defaults configuration
func (db *Database) GetToolDefaults() *ToolDefaults {
	return db.toolDefaults
}

// GetErrorMessages returns the error messages configuration
func (db *Database) GetErrorMessages() *ErrorMessages {
	return db.errorMessages
}

// GetStatusIndicator returns a status indicator from system config
func (db *Database) GetStatusIndicator(key string) string {
	if db.system != nil {
		if indicator, ok := db.system.System.StatusIndicators[key]; ok {
			return indicator
		}
	}
	// Fallback defaults
	fallbacks := map[string]string{
		"starting":  "🔨",
		"progress":  "━",
		"success":   "✓",
		"warning":   "⚠️",
		"error":     "✗",
		"completed": "🎉",
	}
	if fallback, ok := fallbacks[key]; ok {
		return fallback
	}
	return ""
}

// GetErrorMessage returns a formatted error message
func (db *Database) GetErrorMessage(category, key string, params map[string]string) string {
	if db.errorMessages == nil {
		return fmt.Sprintf("Error in %s: %s", category, key)
	}
	
	var template string
	switch category {
	case "system":
		template = db.errorMessages.ErrorMessages.System[key]
	case "tools":
		template = db.errorMessages.ErrorMessages.Tools[key]
	case "workflows":
		template = db.errorMessages.ErrorMessages.Workflows[key]
	case "files":
		template = db.errorMessages.ErrorMessages.Files[key]
	case "network":
		template = db.errorMessages.ErrorMessages.Network[key]
	case "database":
		template = db.errorMessages.ErrorMessages.Database[key]
	}
	
	if template == "" {
		return fmt.Sprintf("Unknown error: %s.%s", category, key)
	}
	
	// Replace parameters in template
	result := template
	for param, value := range params {
		result = strings.ReplaceAll(result, fmt.Sprintf("{%s}", param), value)
	}
	
	return result
}

// GetStatusMessage returns a formatted status message from workflow config
func (db *Database) GetStatusMessage(key string, params map[string]string) string {
	if db.workflow == nil {
		return key
	}
	
	template, ok := db.workflow.Workflow.StatusMessages[key]
	if !ok {
		return key
	}
	
	// Replace parameters in template
	result := template
	for param, value := range params {
		result = strings.ReplaceAll(result, fmt.Sprintf("{%s}", param), value)
	}
	
	return result
}

// GetToolSecurity returns security configuration for tools
func (db *Database) GetToolSecurity() map[string]interface{} {
	if db.toolDefaults != nil {
		return db.toolDefaults.ToolDefaults.Security
	}
	return make(map[string]interface{})
}

// GetExecutionDefaults returns execution defaults for tools
func (db *Database) GetExecutionDefaults() map[string]interface{} {
	if db.toolDefaults != nil {
		return db.toolDefaults.ToolDefaults.Execution
	}
	return make(map[string]interface{})
}

// GetWorkflowLegend returns the complete workflow and tool reference guide
func (db *Database) GetWorkflowLegend() map[string]interface{} {
	if legendData, ok := db.data["workflow_legend"].(map[string]interface{}); ok {
		return legendData
	}
	return make(map[string]interface{})
}

// GetStepTypeInfo returns information about a specific workflow step type
func (db *Database) GetStepTypeInfo(stepType string) map[string]interface{} {
	legend := db.GetWorkflowLegend()
	if workflowLegend, ok := legend["workflow_legend"].(map[string]interface{}); ok {
		if stepTypes, ok := workflowLegend["step_types"].(map[string]interface{}); ok {
			if stepInfo, ok := stepTypes[stepType].(map[string]interface{}); ok {
				return stepInfo
			}
		}
	}
	return make(map[string]interface{})
}

// GetToolFlagInfo returns information about tool flags and their usage
func (db *Database) GetToolFlagInfo(toolName, flagName string) map[string]interface{} {
	legend := db.GetWorkflowLegend()
	if workflowLegend, ok := legend["workflow_legend"].(map[string]interface{}); ok {
		if toolFlags, ok := workflowLegend["tool_flags"].(map[string]interface{}); ok {
			if toolInfo, ok := toolFlags[toolName].(map[string]interface{}); ok {
				if flags, ok := toolInfo["flags"].(map[string]interface{}); ok {
					if flagInfo, ok := flags[flagName].(map[string]interface{}); ok {
						return flagInfo
					}
				}
			}
		}
	}
	return make(map[string]interface{})
}

// GetBestPractices returns workflow and tool best practices
func (db *Database) GetBestPractices() map[string]interface{} {
	legend := db.GetWorkflowLegend()
	if workflowLegend, ok := legend["workflow_legend"].(map[string]interface{}); ok {
		if practices, ok := workflowLegend["best_practices"].(map[string]interface{}); ok {
				return practices
		}
	}
	return make(map[string]interface{})
}