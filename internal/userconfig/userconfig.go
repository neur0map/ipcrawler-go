package userconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// UserConfig represents the persistent user configuration
type UserConfig struct {
	DefaultOutputDirectory string    `yaml:"default_output_directory,omitempty"`
	LastUpdated            time.Time `yaml:"last_updated"`
}

// getConfigDir returns the user's IPCrawler config directory
func getConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %v", err)
	}
	
	configDir := filepath.Join(homeDir, ".ipcrawler")
	return configDir, nil
}

// getConfigPath returns the full path to the user config file
func getConfigPath() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	
	return filepath.Join(configDir, "config.yaml"), nil
}

// ensureConfigDir creates the config directory if it doesn't exist
func ensureConfigDir() error {
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}
	
	return os.MkdirAll(configDir, 0755)
}

// LoadUserConfig loads the user configuration from disk
func LoadUserConfig() (*UserConfig, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}
	
	// If config file doesn't exist, return default config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &UserConfig{
			LastUpdated: time.Now(),
		}, nil
	}
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}
	
	var config UserConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}
	
	return &config, nil
}

// SaveUserConfig saves the user configuration to disk
func (uc *UserConfig) Save() error {
	if err := ensureConfigDir(); err != nil {
		return err
	}
	
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}
	
	uc.LastUpdated = time.Now()
	
	data, err := yaml.Marshal(uc)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}
	
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}
	
	return nil
}

// SetDefaultOutputDirectory sets the default output directory
func (uc *UserConfig) SetDefaultOutputDirectory(dir string) error {
	// Validate directory path
	if dir == "" {
		return fmt.Errorf("output directory cannot be empty")
	}
	
	// Convert to absolute path
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("invalid directory path: %v", err)
	}
	
	// Check if directory exists or can be created
	if err := os.MkdirAll(absDir, 0755); err != nil {
		return fmt.Errorf("cannot create directory %s: %v", absDir, err)
	}
	
	// Check if directory is writable
	testFile := filepath.Join(absDir, ".ipcrawler_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("directory %s is not writable: %v", absDir, err)
	}
	os.Remove(testFile) // Clean up test file
	
	uc.DefaultOutputDirectory = absDir
	return uc.Save()
}

// ClearDefaultOutputDirectory clears the default output directory
func (uc *UserConfig) ClearDefaultOutputDirectory() error {
	uc.DefaultOutputDirectory = ""
	return uc.Save()
}

// GetEffectiveOutputDirectory returns the effective output directory based on priority:
// 1. outputFlag (if provided)
// 2. DefaultOutputDirectory (if set)
// 3. fallback (if provided)
// 4. "./ipcrawler_results" (final fallback)
func (uc *UserConfig) GetEffectiveOutputDirectory(outputFlag, fallback string) string {
	// Priority 1: Command line flag
	if outputFlag != "" {
		return outputFlag
	}
	
	// Priority 2: User's default setting
	if uc.DefaultOutputDirectory != "" {
		return uc.DefaultOutputDirectory
	}
	
	// Priority 3: Provided fallback
	if fallback != "" {
		return fallback
	}
	
	// Priority 4: Final fallback
	return "./ipcrawler_results"
}

// GetConfigInfo returns a formatted string with current configuration
func (uc *UserConfig) GetConfigInfo() string {
	configPath, _ := getConfigPath()
	
	info := fmt.Sprintf("IPCrawler Configuration:\n")
	info += fmt.Sprintf("  Config file: %s\n", configPath)
	
	if uc.DefaultOutputDirectory != "" {
		info += fmt.Sprintf("  Default output directory: %s\n", uc.DefaultOutputDirectory)
	} else {
		info += fmt.Sprintf("  Default output directory: Not set (using ./ipcrawler_results)\n")
	}
	
	info += fmt.Sprintf("  Last updated: %s\n", uc.LastUpdated.Format("2006-01-02 15:04:05"))
	
	return info
}