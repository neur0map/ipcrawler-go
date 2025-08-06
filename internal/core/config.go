package core

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version         string          `yaml:"version"`
	DefaultTemplate string          `yaml:"default_template"`
	Templates       []string        `yaml:"templates"`
	ReportDir       string          // Calculated field
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set default report directory (just current directory now)
	config.ReportDir = "."

	return &config, nil
}

// SetReportDir overrides the report directory (used by CLI flag)
func (c *Config) SetReportDir(dir string) {
	if dir != "" {
		c.ReportDir = dir
	}
}



func (c *Config) Validate() error {
	if c.Version == "" {
		return fmt.Errorf("version is required in config")
	}
	
	if c.DefaultTemplate == "" {
		return fmt.Errorf("default_template is required in config")
	}

	// Check if default template exists in templates list
	found := false
	for _, template := range c.Templates {
		if template == c.DefaultTemplate {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("default_template '%s' not found in templates list", c.DefaultTemplate)
	}

	return nil
}

