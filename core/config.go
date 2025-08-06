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
	Reporting       *ReportingConfig `yaml:"reporting,omitempty"`
	Concurrency     *ConcurrencyConfig `yaml:"concurrency,omitempty"`
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

// SetReportDir overrides the report directory (used by CLI flag)
func (c *Config) SetReportDir(dir string) {
	if dir != "" {
		c.ReportDir = dir
	}
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

// ConcurrencyConfig holds parallel execution configuration
type ConcurrencyConfig struct {
	MaxConcurrentWorkflows int  `yaml:"max_concurrent_workflows"`
	EnableAutoLimit        bool `yaml:"enable_auto_limit"`
	UseErrGroup           bool  `yaml:"use_errgroup"`
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

// GetMaxConcurrency returns the maximum concurrent workflows with sensible defaults
func (c *Config) GetMaxConcurrency(workflowCount int) int {
	// If no concurrency config, use sensible defaults
	if c.Concurrency == nil {
		if workflowCount <= 4 {
			return workflowCount
		}
		return 4
	}
	
	// If auto-limit is enabled, adapt based on workflow count and system resources
	if c.Concurrency.EnableAutoLimit {
		maxLimit := c.Concurrency.MaxConcurrentWorkflows
		if maxLimit <= 0 {
			maxLimit = 4 // Default fallback
		}
		
		if workflowCount < maxLimit {
			return workflowCount
		}
		return maxLimit
	}
	
	// Use configured value with fallback
	if c.Concurrency.MaxConcurrentWorkflows > 0 {
		return c.Concurrency.MaxConcurrentWorkflows
	}
	
	// Fallback to sensible default
	return 4
}

// UseErrGroup returns whether to use errgroup for parallel execution
func (c *Config) UseErrGroup() bool {
	if c.Concurrency == nil {
		return true // Default to using errgroup
	}
	return c.Concurrency.UseErrGroup
}