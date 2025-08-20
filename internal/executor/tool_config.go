package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// ToolConfig represents a tool configuration loaded from tools/*/config.yaml
type ToolConfig struct {
	Tool              string                   `yaml:"tool"`
	Description       string                   `yaml:"description"`
	Format            string                   `yaml:"format"`
	File              string                   `yaml:"file"`
	Args              map[string][]string      `yaml:"args"`
	Overrides         []map[string]interface{} `yaml:"overrides"`
	
	// Output configuration for separator display
	ShowSeparator     bool `yaml:"show_separator"`     // Whether to show visual separator for this tool
	SeparatorPriority int  `yaml:"separator_priority"` // Priority for separator display (higher = shown first)
}

// ToolConfigLoader loads and manages tool configurations
type ToolConfigLoader struct {
	toolsPath string
	configs   map[string]*ToolConfig
	mutex     sync.RWMutex // Protect concurrent access to configs map
}

// NewToolConfigLoader creates a new tool configuration loader
func NewToolConfigLoader(toolsPath string) *ToolConfigLoader {
	return &ToolConfigLoader{
		toolsPath: toolsPath,
		configs:   make(map[string]*ToolConfig),
	}
}

// LoadToolConfig loads a specific tool's configuration
func (tcl *ToolConfigLoader) LoadToolConfig(toolName string) (*ToolConfig, error) {
	// Check if already loaded (read lock first)
	tcl.mutex.RLock()
	if config, exists := tcl.configs[toolName]; exists {
		tcl.mutex.RUnlock()
		return config, nil
	}
	tcl.mutex.RUnlock()

	// Construct path to tool config
	configPath := filepath.Join(tcl.toolsPath, toolName, "config.yaml")

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("tool config not found: %s", configPath)
	}

	// Read and parse the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tool config %s: %w", configPath, err)
	}

	var config ToolConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse tool config %s: %w", configPath, err)
	}

	// Validate required fields
	if config.Tool == "" {
		config.Tool = toolName // Default to directory name if not specified
	}

	// Cache the config (write lock)
	tcl.mutex.Lock()
	tcl.configs[toolName] = &config
	tcl.mutex.Unlock()

	return &config, nil
}

// LoadAllToolConfigs discovers and loads all tool configurations from the tools directory
func (tcl *ToolConfigLoader) LoadAllToolConfigs() (map[string]*ToolConfig, error) {
	// Check if tools directory exists
	if _, err := os.Stat(tcl.toolsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("tools directory not found: %s", tcl.toolsPath)
	}

	// Read tools directory
	entries, err := os.ReadDir(tcl.toolsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tools directory %s: %w", tcl.toolsPath, err)
	}

	loadedConfigs := make(map[string]*ToolConfig)

	// Load each tool's config
	for _, entry := range entries {
		if !entry.IsDir() {
			continue // Skip non-directories
		}

		toolName := entry.Name()
		config, err := tcl.LoadToolConfig(toolName)
		if err != nil {
			// Log error but continue with other tools - silently continue to avoid console spam
			continue
		}

		loadedConfigs[toolName] = config
	}

	return loadedConfigs, nil
}

// GetAvailableTools returns a list of available tool names
func (tcl *ToolConfigLoader) GetAvailableTools() ([]string, error) {
	configs, err := tcl.LoadAllToolConfigs()
	if err != nil {
		return nil, err
	}

	var tools []string
	for toolName := range configs {
		tools = append(tools, toolName)
	}

	return tools, nil
}

// GetToolArguments returns the argument templates for a specific execution mode
func (tc *ToolConfig) GetToolArguments(mode string) ([]string, error) {
	args, exists := tc.Args[mode]
	if !exists {
		// Return available modes for better error message
		var availableModes []string
		for mode := range tc.Args {
			availableModes = append(availableModes, mode)
		}
		return nil, fmt.Errorf("execution mode '%s' not found for tool '%s'. Available modes: %s",
			mode, tc.Tool, strings.Join(availableModes, ", "))
	}

	// Return a copy to prevent modifications
	result := make([]string, len(args))
	copy(result, args)
	return result, nil
}

// GetAvailableModes returns all available execution modes for this tool
func (tc *ToolConfig) GetAvailableModes() []string {
	var modes []string
	for mode := range tc.Args {
		modes = append(modes, mode)
	}
	return modes
}

