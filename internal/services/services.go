package services

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Reference data manager for IPCrawler (NO EXECUTION CONTROL)
type Database struct {
	data map[string]interface{}
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

	// Load all YAML files from config and data folders
	for _, dir := range []string{"config", "data", "database"} {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue // Skip if folder doesn't exist
		}

		files, err := os.ReadDir(dir)
		if err != nil {
			fmt.Printf("Warning: Failed to read directory %s: %v\n", dir, err)
			continue
		}

		for _, file := range files {
			if !file.IsDir() && filepath.Ext(file.Name()) == ".yaml" {
				filePath := filepath.Join(dir, file.Name())
				if err := globalDB.loadYAMLFile(filePath); err != nil {
					fmt.Printf("Warning: Failed to load %s: %v\n", filePath, err)
				}
			}
		}
	}

	// Database now only contains reference data - no execution control

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


// GetData returns raw data for any database file
func (db *Database) GetData(key string) interface{} {
	return db.data[key]
}

// Reference data access methods (NO EXECUTION CONTROL)

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