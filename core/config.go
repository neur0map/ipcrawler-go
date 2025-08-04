package core

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DefaultTemplate string          `yaml:"default_template"`
	Templates       []string        `yaml:"templates"`
	Reporting       *ReportingConfig `yaml:"reporting,omitempty"`
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

	// Set default report directory if not specified
	if config.Reporting != nil && config.Reporting.BaseDir != "" {
		config.ReportDir = config.Reporting.BaseDir
	} else {
		config.ReportDir = "reports"
	}

	return &config, nil
}

// ReportingConfig holds the reporting configuration
type ReportingConfig struct {
	Enabled   bool              `yaml:"enabled"`
	BaseDir   string            `yaml:"base_dir"`
	Formats   []string          `yaml:"formats"`
	Pipeline  *PipelineConfig   `yaml:"pipeline,omitempty"`
	Agents    map[string]interface{} `yaml:"agents,omitempty"`
}

// PipelineConfig holds pipeline configuration
type PipelineConfig struct {
	MaxRetries    int    `yaml:"max_retries"`
	RetryDelay    string `yaml:"retry_delay"`
	Timeout       string `yaml:"timeout"`
	ParallelMode  bool   `yaml:"parallel_mode"`
	FailFast      bool   `yaml:"fail_fast"`
	LogLevel      string `yaml:"log_level"`
}

func (c *Config) Validate() error {
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