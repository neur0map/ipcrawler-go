package parsers

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
)

// DynamicParser parses any tool output without predefined field mappings
type DynamicParser struct {
	ToolName string // Exported for access from ui package
}

// NewDynamicParser creates a parser that handles any JSON/XML output
func NewDynamicParser(toolName string) *DynamicParser {
	return &DynamicParser{
		ToolName: toolName,
	}
}

// ParseLine dynamically parses output based on content detection
func (p *DynamicParser) ParseLine(line string) (*ParseResult, error) {
	line = strings.TrimSpace(line)
	
	// Skip empty lines
	if line == "" {
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
	
	// Detect JSON
	if strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
		return p.parseJSON(line)
	}
	
	// Detect XML (for complete documents, not streaming)
	if strings.HasPrefix(line, "<?xml") || strings.HasPrefix(line, "<") {
		// XML needs complete document, not line-by-line
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
	
	// Skip noise (common tool output patterns to ignore)
	if p.isNoise(line) {
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
	
	// If it looks like useful data, show it as a message
	return &ParseResult{
		Type:    ParseResultMessage,
		Message: line,
		ShouldDisplay: true,
	}, nil
}

// parseJSON extracts ALL fields from JSON dynamically
func (p *DynamicParser) parseJSON(line string) (*ParseResult, error) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(line), &data); err != nil {
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
	
	// Extract common fields that might exist
	port := p.extractNumber(data, "port", "Port", "PORT")
	service := p.extractString(data, "service", "Service", "name", "protocol")
	version := p.extractString(data, "version", "Version", "product")
	// host := p.extractString(data, "host", "Host", "ip", "IP", "address") // Reserved for future use
	state := p.extractString(data, "state", "State", "status")
	
	// If we found a port, it's port discovery
	if port > 0 {
		// Build a description from ALL available fields
		var details []string
		for k, v := range data {
			// Skip already processed fields
			if k == "port" || k == "Port" || k == "PORT" {
				continue
			}
			// Add any other fields as details
			if v != nil && v != "" && v != 0 && v != false {
				details = append(details, fmt.Sprintf("%s:%v", k, v))
			}
		}
		
		result := &PortInfo{
			Number:  port,
			State:   state,
			Service: service,
			Version: version,
		}
		
		// If we have service info, treat as service detection
		if service != "" || version != "" {
			return &ParseResult{
				Type: ParseResultService,
				Service: &ServiceInfo{
					Port:    port,
					Service: service,
					Version: version,
				},
				ShouldDisplay: true,
			}, nil
		}
		
		// Otherwise it's port discovery
		return &ParseResult{
			Type: ParseResultPort,
			Port: result,
			ShouldDisplay: true,
		}, nil
	}
	
	// If no port but has other data, show as message
	if len(data) > 0 {
		// Format all fields for display
		var fields []string
		for k, v := range data {
			if v != nil && v != "" {
				fields = append(fields, fmt.Sprintf("%s:%v", k, v))
			}
		}
		return &ParseResult{
			Type:    ParseResultMessage,
			Message: strings.Join(fields, " "),
			ShouldDisplay: true,
		}, nil
	}
	
	return &ParseResult{Type: ParseResultIgnore}, nil
}

// extractNumber tries to find a numeric value from multiple possible keys
func (p *DynamicParser) extractNumber(data map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		if val, exists := data[key]; exists {
			switch v := val.(type) {
			case float64:
				return int(v)
			case int:
				return v
			case string:
				var num int
				fmt.Sscanf(v, "%d", &num)
				return num
			}
		}
	}
	return 0
}

// extractString tries to find a string value from multiple possible keys
func (p *DynamicParser) extractString(data map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, exists := data[key]; exists {
			if s, ok := val.(string); ok {
				return s
			}
		}
	}
	return ""
}

// isNoise filters out common tool output that isn't actual results
func (p *DynamicParser) isNoise(line string) bool {
	lower := strings.ToLower(line)
	
	noisePatterns := []string{
		"starting", "scanning", "initializing", "loading",
		"nmap ", "nuclei ", "naabu ", "masscan ",  // Tool headers
		"# ", "//", "/*", "*/",  // Comments
		"warning:", "error:", "debug:", "info:",  // Log levels
		"usage:", "help:", "version:", "copyright:",
		"[*]", "[+]", "[-]", "[!]",  // Common status prefixes
		"----", "====", "****",  // Separators
	}
	
	for _, pattern := range noisePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	
	// Also filter lines that are too short to be useful
	return len(strings.TrimSpace(line)) < 3
}

// ParseXMLDocument parses a complete XML document (for tools like nmap)
func (p *DynamicParser) ParseXMLDocument(data []byte) ([]*ParseResult, error) {
	// For nmap XML, we need a simple structure to capture ports
	type Port struct {
		XMLName  xml.Name `xml:"port"`
		Protocol string   `xml:"protocol,attr"`
		PortID   string   `xml:"portid,attr"`
		State    struct {
			State string `xml:"state,attr"`
		} `xml:"state"`
		Service struct {
			Name    string `xml:"name,attr"`
			Product string `xml:"product,attr"`
			Version string `xml:"version,attr"`
		} `xml:"service"`
	}
	
	type NmapRun struct {
		XMLName xml.Name `xml:"nmaprun"`
		Host    struct {
			Ports []Port `xml:"ports>port"`
		} `xml:"host"`
	}
	
	var nmapResult NmapRun
	if err := xml.Unmarshal(data, &nmapResult); err != nil {
		// If it's not nmap format, try generic XML parsing
		return p.parseGenericXML(data)
	}
	
	var results []*ParseResult
	for _, port := range nmapResult.Host.Ports {
		if port.State.State != "open" {
			continue
		}
		
		portNum := 0
		fmt.Sscanf(port.PortID, "%d", &portNum)
		
		if portNum > 0 {
			// Build version string
			version := port.Service.Product
			if port.Service.Version != "" {
				if version != "" {
					version += " " + port.Service.Version
				} else {
					version = port.Service.Version
				}
			}
			
			result := &ParseResult{
				Type: ParseResultService,
				Service: &ServiceInfo{
					Port:    portNum,
					Service: port.Service.Name,
					Version: version,
				},
				ShouldDisplay: true,
			}
			results = append(results, result)
		}
	}
	
	return results, nil
}

// parseGenericXML handles any XML format dynamically
func (p *DynamicParser) parseGenericXML(data []byte) ([]*ParseResult, error) {
	// For now, return empty - could implement generic XML parsing if needed
	return []*ParseResult{}, nil
}