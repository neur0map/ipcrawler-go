package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/carlosm/ipcrawler/internal/tool"
)

type Registry struct {
	mu    sync.RWMutex
	tools map[string]func() (tool.Tool, error)
}

var globalRegistry = &Registry{
	tools: make(map[string]func() (tool.Tool, error)),
}

func Register(name string, factory func() (tool.Tool, error)) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.tools[name] = factory
}

func Get(name string) (tool.Tool, error) {
	globalRegistry.mu.RLock()
	factory, exists := globalRegistry.tools[name]
	globalRegistry.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("tool %s not found in registry", name)
	}
	
	return factory()
}

func LoadAllTools() error {
	toolsDir := "tools"
	
	if _, err := os.Stat(toolsDir); os.IsNotExist(err) {
		return nil
	}
	
	entries, err := os.ReadDir(toolsDir)
	if err != nil {
		return fmt.Errorf("reading tools directory: %w", err)
	}
	
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		
		toolDir := filepath.Join(toolsDir, entry.Name())
		configPath := filepath.Join(toolDir, "config.yaml")
		
		if _, err := os.Stat(configPath); err != nil {
			continue
		}
		
		cfg, err := tool.LoadToolConfig(configPath)
		if err != nil {
			fmt.Printf("Warning: failed to load tool config %s: %v\n", configPath, err)
			continue
		}
		
		// Create closure with proper variable capture to avoid closure bug
		Register(cfg.Name, func(path string) func() (tool.Tool, error) {
			return func() (tool.Tool, error) {
				return tool.NewGenericAdapter(path)
			}
		}(configPath))
		
		fmt.Printf("Registered tool: %s\n", cfg.Name)
	}
	
	return nil
}

func GetConfig(name string) (*tool.Config, error) {
	toolsDir := "tools"
	toolDir := filepath.Join(toolsDir, name)
	configPath := filepath.Join(toolDir, "config.yaml")
	
	return tool.LoadToolConfig(configPath)
}

func ListTools() []string {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	
	tools := make([]string, 0, len(globalRegistry.tools))
	for name := range globalRegistry.tools {
		tools = append(tools, name)
	}
	return tools
}

// LoadSpecificTools loads only the specified tools instead of all tools
func LoadSpecificTools(requiredTools []string) error {
	if len(requiredTools) == 0 {
		return nil // No tools needed
	}

	toolsDir := "tools"
	
	if _, err := os.Stat(toolsDir); os.IsNotExist(err) {
		return fmt.Errorf("tools directory not found: %s", toolsDir)
	}
	
	for _, toolName := range requiredTools {
		toolDir := filepath.Join(toolsDir, toolName)
		configPath := filepath.Join(toolDir, "config.yaml")
		
		if _, err := os.Stat(configPath); err != nil {
			fmt.Printf("Warning: tool config not found for %s: %s\n", toolName, configPath)
			continue
		}
		
		cfg, err := tool.LoadToolConfig(configPath)
		if err != nil {
			fmt.Printf("Warning: failed to load tool config %s: %v\n", configPath, err)
			continue
		}
		
		// Ensure the tool name matches what we expect
		if cfg.Name != toolName {
			fmt.Printf("Warning: tool config name mismatch: expected %s, got %s\n", toolName, cfg.Name)
		}
		
		finalPath := configPath
		Register(cfg.Name, func() (tool.Tool, error) {
			return tool.NewGenericAdapter(finalPath)
		})
		
		fmt.Printf("Registered tool: %s\n", cfg.Name)
	}
	
	return nil
}