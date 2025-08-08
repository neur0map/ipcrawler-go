package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type GlobalConfig struct {
	AutoRunAllWorkflows    bool   `yaml:"auto_run_all_workflows"`
	MaxConcurrentWorkflows int    `yaml:"max_concurrent_workflows"`
	DefaultOutputDir       string `yaml:"default_output_dir"`
	DefaultReportDir       string `yaml:"default_report_dir"`
}

func LoadGlobalConfig(path string) (*GlobalConfig, error) {
	if path == "" {
		path = findConfigFile()
	}
	
	cfg := &GlobalConfig{
		AutoRunAllWorkflows:    true,
		MaxConcurrentWorkflows: 3,
		DefaultOutputDir:       "out",
		DefaultReportDir:       "reports",
	}
	
	if path == "" {
		return cfg, nil
	}
	
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}
	
	if cfg.MaxConcurrentWorkflows <= 0 {
		cfg.MaxConcurrentWorkflows = 3
	}
	
	if cfg.DefaultOutputDir == "" {
		cfg.DefaultOutputDir = "out"
	}
	
	if cfg.DefaultReportDir == "" {
		cfg.DefaultReportDir = "reports"
	}
	
	
	return cfg, nil
}

func findConfigFile() string {
	candidates := []string{
		"global.yaml",
		".ipcrawlerrc.yaml",
		filepath.Join(os.Getenv("HOME"), ".ipcrawlerrc.yaml"),
		filepath.Join(os.Getenv("HOME"), "global.yaml"),
	}
	
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	
	return ""
}