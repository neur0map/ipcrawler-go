package tool

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/carlosm/ipcrawler/internal/services"
	"github.com/jmespath/go-jmespath"
	"gopkg.in/yaml.v3"
)

type Tool interface {
	Name() string
	Execute(ctx context.Context, args []string, target string) (*Result, error)
}

type Result struct {
	ToolName       string
	Output         string
	Format         string
	Data           interface{}
	ParsedResults  []GenericResult
	Error          error
}

type GenericResult struct {
	Type   string
	Fields map[string]interface{}
}

type Config struct {
	Name     string                 `yaml:"name"`
	Command  string                 `yaml:"command"`
	Output   string                 `yaml:"output"`
	Args     ArgsConfig             `yaml:"args"`
	Mappings []MappingConfig        `yaml:"mappings"`
}

type ArgsConfig struct {
	Default []string            `yaml:"default"`
	Flags   map[string][]string `yaml:"flags"`
}

type MappingConfig struct {
	Type   string            `yaml:"type"`
	Query  string            `yaml:"query"`
	Fields map[string]string `yaml:"fields"`
}

type GenericAdapter struct {
	config Config
	path   string
	db     *services.Database
}

func NewGenericAdapter(configPath string) (*GenericAdapter, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading tool config: %w", err)
	}
	
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing tool config: %w", err)
	}
	
	// Load database for enhanced validation and configuration
	db, _ := services.LoadDatabase()
	
	return &GenericAdapter{
		config: config,
		path:   configPath,
		db:     db,
	}, nil
}

func (g *GenericAdapter) Name() string {
	return g.config.Name
}

func (g *GenericAdapter) Execute(ctx context.Context, args []string, target string) (*Result, error) {
	// Validate tool execution against database security policies
	if g.db != nil {
		if !g.db.IsToolAllowed(g.config.Name) {
			return nil, fmt.Errorf("tool '%s' not allowed by security policy", g.config.Name)
		}
		
		if err := g.db.ValidateArguments(args); err != nil {
			return nil, fmt.Errorf("argument validation failed: %w", err)
		}
	}
	
	finalArgs := make([]string, 0)
	finalArgs = append(finalArgs, g.config.Args.Default...)
	finalArgs = append(finalArgs, args...)
	
	for i, arg := range finalArgs {
		finalArgs[i] = strings.ReplaceAll(arg, "{{target}}", target)
	}
	
	cmd := exec.CommandContext(ctx, g.config.Command, finalArgs...)
	output, err := cmd.CombinedOutput()
	
	result := &Result{
		ToolName: g.config.Name,
		Output:   string(output),
		Format:   g.config.Output,
	}
	
	if err != nil {
		result.Error = err
		return result, nil
	}
	
	switch g.config.Output {
	case "json":
		var data interface{}
		if err := json.Unmarshal(output, &data); err == nil {
			result.Data = data
			result.ParsedResults = g.parseWithMappings(data)
		}
	case "xml":
		var data interface{}
		if err := xml.Unmarshal(output, &data); err == nil {
			result.Data = data
			result.ParsedResults = g.parseWithMappings(data)
		}
	default:
		result.Data = string(output)
	}
	
	return result, nil
}

func (g *GenericAdapter) parseWithMappings(data interface{}) []GenericResult {
	var results []GenericResult
	
	for _, mapping := range g.config.Mappings {
		queryResult, err := jmespath.Search(mapping.Query, data)
		if err != nil {
			continue
		}
		
		if queryResult == nil {
			continue
		}
		
		switch items := queryResult.(type) {
		case []interface{}:
			for _, item := range items {
				if itemMap, ok := item.(map[string]interface{}); ok {
					result := GenericResult{
						Type:   mapping.Type,
						Fields: make(map[string]interface{}),
					}
					
					for fieldName, fieldPath := range mapping.Fields {
						if value, exists := itemMap[fieldPath]; exists {
							result.Fields[fieldName] = value
						}
					}
					
					if len(result.Fields) > 0 {
						results = append(results, result)
					}
				}
			}
		case map[string]interface{}:
			result := GenericResult{
				Type:   mapping.Type,
				Fields: make(map[string]interface{}),
			}
			
			for fieldName, fieldPath := range mapping.Fields {
				if value, exists := items[fieldPath]; exists {
					result.Fields[fieldName] = value
				}
			}
			
			if len(result.Fields) > 0 {
				results = append(results, result)
			}
		}
	}
	
	return results
}

func LoadToolConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	
	return &config, nil
}