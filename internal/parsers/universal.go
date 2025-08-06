package parsers

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ToolMapping defines how to parse output from a specific tool
type ToolMapping struct {
	OutputFormat string            `yaml:"output_format"`
	StreamParse  bool              `yaml:"stream_parse"`
	Fields       map[string]string `yaml:"fields"`
	ResultType   string            `yaml:"result_type"`
}

// DisplayTemplate defines how to display results
type DisplayTemplate struct {
	Icon    string `yaml:"icon"`
	Message string `yaml:"message"`
}


// ParseResult represents the result of parsing a single line of output
type ParseResult struct {
	Type    ParseResultType
	Port    *PortInfo
	Service *ServiceInfo
	Message string
	ShouldDisplay bool
}

// ParseResultType indicates the type of parsed result
type ParseResultType int

const (
	ParseResultPort ParseResultType = iota
	ParseResultService
	ParseResultMessage
	ParseResultIgnore
)

// PortInfo represents discovered port information
type PortInfo struct {
	Number  int
	State   string
	Service string
	Version string
}

// ServiceInfo represents detected service information  
type ServiceInfo struct {
	Port    int
	Service string
	Version string
}

// UniversalParser parses output from any tool using configuration
type UniversalParser struct {
	toolName    string
	mapping     ToolMapping
	template    DisplayTemplate
}

// UniversalResult represents parsed data from any tool
type UniversalResult struct {
	Type        string
	Port        int
	Protocol    string
	Service     string
	Version     string
	Host        string
	State       string
	RawData     map[string]interface{}
	ShouldDisplay bool
}


// NewUniversalParser creates a parser for any tool using workflow configuration
func NewUniversalParser(toolName string) (*UniversalParser, error) {
	// Instead of loading separate config, we'll use the workflow output config
	// For now, create a default parser since the workflow is passed at runtime
	return &UniversalParser{
		toolName: toolName,
		mapping: ToolMapping{
			OutputFormat: "json",
			StreamParse:  true,
			Fields: map[string]string{
				"port": "port",
				"host": "host",
			},
			ResultType: "generic",
		},
		template: DisplayTemplate{
			Icon:    "📡",
			Message: "Result: {data}",
		},
	}, nil
}

// NewUniversalParserWithWorkflow creates a parser using workflow configuration
func NewUniversalParserWithWorkflow(outputConfig map[string]interface{}, toolName string) (*UniversalParser, error) {
	mapping := ToolMapping{
		OutputFormat: "json",
		StreamParse:  true,
		Fields: map[string]string{
			"port": "port",
			"host": "host",
		},
		ResultType: "generic",
	}
	
	template := DisplayTemplate{
		Icon:    "📡",
		Message: "Result: {data}",
	}
	
	// Extract from output config if provided
	if outputConfig != nil {
		if format, ok := outputConfig["format"].(string); ok {
			mapping.OutputFormat = format
		}
		if streamParse, ok := outputConfig["stream_parse"].(bool); ok {
			mapping.StreamParse = streamParse
		}
		if fields, ok := outputConfig["fields"].(map[string]interface{}); ok {
			mapping.Fields = make(map[string]string)
			for k, v := range fields {
				if s, ok := v.(string); ok {
					mapping.Fields[k] = s
				}
			}
		}
		if resultType, ok := outputConfig["result_type"].(string); ok {
			mapping.ResultType = resultType
		}
		
		if display, ok := outputConfig["display"].(map[string]interface{}); ok {
			if icon, ok := display["icon"].(string); ok {
				template.Icon = icon
			}
			if message, ok := display["message"].(string); ok {
				template.Message = message
			}
		}
	}
	
	return &UniversalParser{
		toolName: toolName,
		mapping:  mapping,
		template: template,
	}, nil
}

// GetToolName returns the tool name
func (p *UniversalParser) GetToolName() string {
	return p.toolName
}

// ParseLine processes a single line of tool output
func (p *UniversalParser) ParseLine(line string) (*ParseResult, error) {
	line = strings.TrimSpace(line)
	
	// Skip empty lines
	if line == "" {
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
	
	// Only parse if we're doing stream parsing
	if !p.mapping.StreamParse {
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
	
	switch p.mapping.OutputFormat {
	case "json":
		return p.parseJSONLine(line)
	default:
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
}

// parseJSONLine parses a JSON line using field mappings
func (p *UniversalParser) parseJSONLine(line string) (*ParseResult, error) {
	// Must be valid JSON
	if !strings.HasPrefix(line, "{") {
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
	
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(line), &data); err != nil {
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
	
	result := p.extractDataFromJSON(data)
	if result == nil {
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
	
	// Convert to appropriate result type
	switch p.mapping.ResultType {
	case "port_discovery":
		return &ParseResult{
			Type: ParseResultPort,
			Port: &PortInfo{
				Number:  result.Port,
				State:   "open", // Most tools only report open ports
				Service: result.Service,
				Version: result.Version,
			},
			ShouldDisplay: true,
		}, nil
		
	case "service_analysis":
		return &ParseResult{
			Type: ParseResultService,
			Service: &ServiceInfo{
				Port:    result.Port,
				Service: result.Service,
				Version: result.Version,
			},
			ShouldDisplay: true,
		}, nil
		
	default:
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
}

// extractDataFromJSON extracts relevant data using field mappings
func (p *UniversalParser) extractDataFromJSON(data map[string]interface{}) *UniversalResult {
	result := &UniversalResult{
		Type:    p.mapping.ResultType,
		RawData: data,
	}
	
	// Extract port
	if portField, exists := p.mapping.Fields["port"]; exists {
		if val, ok := data[portField]; ok {
			switch v := val.(type) {
			case float64:
				result.Port = int(v)
			case int:
				result.Port = v
			case string:
				if p, err := strconv.Atoi(v); err == nil {
					result.Port = p
				}
			}
		}
	}
	
	// Extract protocol
	if protocolField, exists := p.mapping.Fields["protocol"]; exists {
		if val, ok := data[protocolField]; ok {
			if s, ok := val.(string); ok {
				result.Protocol = s
			}
		}
	}
	
	// Extract host
	if hostField, exists := p.mapping.Fields["host"]; exists {
		if val, ok := data[hostField]; ok {
			if s, ok := val.(string); ok {
				result.Host = s
			}
		}
	}
	
	// Only return if we found a valid port
	if result.Port > 0 && result.Port <= 65535 {
		return result
	}
	
	return nil
}

// ParseFile processes a complete file (for XML and other non-streaming formats)  
func (p *UniversalParser) ParseFile(filePath string) ([]*UniversalResult, error) {
	if p.mapping.StreamParse {
		return nil, fmt.Errorf("tool %s is configured for stream parsing, not file parsing", p.toolName)
	}
	
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	
	switch p.mapping.OutputFormat {
	case "xml":
		return p.parseXMLFile(data)
	default:
		return nil, fmt.Errorf("unsupported output format: %s", p.mapping.OutputFormat)
	}
}

// parseXMLFile parses XML output (like nmap)
func (p *UniversalParser) parseXMLFile(data []byte) ([]*UniversalResult, error) {
	// For now, return empty - this will be implemented when needed
	return []*UniversalResult{}, nil
}