package ui

import (
	"encoding/json"
	"encoding/xml"
	"regexp"
	"strconv"
	"strings"
	
	"ipcrawler/internal/database"
	"ipcrawler/internal/parsers"
)

// OutputParser interface for different tool output parsers
type OutputParser interface {
	ParseLine(line string) (*ParseResult, error)
	GetToolName() string
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

// NaabuParser parses naabu JSON output
type NaabuParser struct{}

func NewNaabuParser() *NaabuParser {
	return &NaabuParser{}
}

func (p *NaabuParser) GetToolName() string {
	return "naabu"
}

func (p *NaabuParser) ParseLine(line string) (*ParseResult, error) {
	line = strings.TrimSpace(line)
	
	// Skip empty lines or non-JSON lines
	if line == "" || !strings.HasPrefix(line, "{") {
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
	
	var naabuResult struct {
		Host      string `json:"host"`
		IP        string `json:"ip"`
		Port      int    `json:"port"`
		Protocol  string `json:"protocol"`
		Timestamp string `json:"timestamp"`
		TLS       bool   `json:"tls"`
	}
	
	if err := json.Unmarshal([]byte(line), &naabuResult); err != nil {
		// Not valid JSON, ignore
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
	
	// Enhance port info using database
	db := database.Global()
	portStr := strconv.Itoa(naabuResult.Port)
	service := db.GetServiceByPort(portStr)
	
	// Create port info from naabu result with database enhancement
	port := &PortInfo{
		Number:  naabuResult.Port,
		State:   "open", // naabu only reports open ports
		Service: service,
		Version: "", // naabu doesn't provide version info
	}
	
	return &ParseResult{
		Type: ParseResultPort,
		Port: port,
		ShouldDisplay: true,
	}, nil
}

// NmapXMLStructures for parsing XML output
type NmapXMLPort struct {
	Protocol string `xml:"protocol,attr"`
	PortID   string `xml:"portid,attr"`
	State    struct {
		State string `xml:"state,attr"`
	} `xml:"state"`
	Service struct {
		Name    string `xml:"name,attr"`
		Product string `xml:"product,attr"`
		Version string `xml:"version,attr"`
		Tunnel  string `xml:"tunnel,attr"`
	} `xml:"service"`
}

// NmapParser parses nmap output (both XML and text)
type NmapParser struct {
	portRegex    *regexp.Regexp
	serviceRegex *regexp.Regexp
	versionRegex *regexp.Regexp
	xmlBuffer    strings.Builder
	inXMLMode    bool
}

func NewNmapParser() *NmapParser {
	return &NmapParser{
		// Match port lines: "22/tcp open ssh OpenSSH 7.4"
		portRegex: regexp.MustCompile(`^(\d+)/(tcp|udp)\s+(\w+)\s+(.*)$`),
		// Match service detection lines
		serviceRegex: regexp.MustCompile(`^(\d+)/(tcp|udp)\s+(\w+)\s+(\S+)\s+(.*)$`),
		// Match version detection lines with more detail
		versionRegex: regexp.MustCompile(`^(\d+)/(tcp|udp)\s+(\w+)\s+(\S+)\s+(.+)$`),
	}
}

func (p *NmapParser) GetToolName() string {
	return "nmap"
}

func (p *NmapParser) ParseLine(line string) (*ParseResult, error) {
	line = strings.TrimSpace(line)
	
	// Skip empty lines
	if line == "" {
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
	
	// Detect XML mode
	if strings.HasPrefix(line, "<?xml") || strings.HasPrefix(line, "<nmaprun") {
		p.inXMLMode = true
		p.xmlBuffer.WriteString(line + "\n")
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
	
	// Handle XML mode
	if p.inXMLMode {
		p.xmlBuffer.WriteString(line + "\n")
		
		// Check if this line contains a complete port entry
		if strings.Contains(line, "<port protocol=") && strings.Contains(line, "</port>") {
			return p.parseXMLPort(line)
		}
		
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
	
	// Handle text mode (legacy)
	if p.isNmapNoise(line) {
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
	
	// Try to parse as port/service information (text mode)
	if matches := p.versionRegex.FindStringSubmatch(line); matches != nil {
		return p.parseTextPort(matches)
	}
	
	// Check if it's an interesting message we should display
	if p.isInterestingMessage(line) {
		return &ParseResult{
			Type: ParseResultMessage,
			Message: line,
			ShouldDisplay: true,
		}, nil
	}
	
	return &ParseResult{Type: ParseResultIgnore}, nil
}

// parseXMLPort parses a port entry from XML output
func (p *NmapParser) parseXMLPort(line string) (*ParseResult, error) {
	var port NmapXMLPort
	if err := xml.Unmarshal([]byte(line), &port); err != nil {
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
	
	// Only process open ports
	if port.State.State != "open" {
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
	
	portNum, err := strconv.Atoi(port.PortID)
	if err != nil {
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
	
	// Create descriptive service name from nmap detection
	serviceName := port.Service.Name
	productInfo := port.Service.Product
	versionInfo := port.Service.Version
	
	// Build version display
	var displayVersion string
	if productInfo != "" {
		displayVersion = productInfo
		if versionInfo != "" {
			displayVersion += " " + versionInfo
		}
	} else if versionInfo != "" {
		displayVersion = versionInfo
	}
	
	// Use nmap's detected service name, not database lookup
	finalService := serviceName
	if finalService == "" {
		db := database.Global()
		finalService = db.GetServiceByPort(port.PortID)
	}
	
	portInfo := &PortInfo{
		Number:  portNum,
		State:   port.State.State,
		Service: finalService,
		Version: displayVersion,
	}
	
	serviceInfo := &ServiceInfo{
		Port:    portNum,
		Service: finalService,
		Version: displayVersion,
	}
	
	return &ParseResult{
		Type: ParseResultService,
		Port: portInfo,
		Service: serviceInfo,
		ShouldDisplay: true,
	}, nil
}

// parseTextPort parses port information from text output (legacy)
func (p *NmapParser) parseTextPort(matches []string) (*ParseResult, error) {
	portNum, err := strconv.Atoi(matches[1])
	if err != nil {
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
	
	state := matches[3]
	serviceName := matches[4]
	versionInfo := matches[5]
	
	// Only show open ports
	if state != "open" {
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
	
	// Enhance service info using database
	db := database.Global()
	portStr := strconv.Itoa(portNum)
	
	// Try to identify service from banner/version info
	finalService := serviceName
	if versionInfo != "" {
		bannerService := db.GetServiceByBanner(versionInfo)
		if bannerService != "unknown-service" {
			finalService = bannerService
		}
	}
	
	// If still unknown, fall back to database port lookup
	if finalService == "" || finalService == "unknown" {
		finalService = db.GetServiceByPort(portStr)
	}
	
	port := &PortInfo{
		Number:  portNum,
		State:   state,
		Service: finalService,
		Version: versionInfo,
	}
	
	service := &ServiceInfo{
		Port:    portNum,
		Service: finalService,
		Version: versionInfo,
	}
	
	// If we have detailed service info, return as service detection
	if versionInfo != "" && serviceName != "unknown" {
		return &ParseResult{
			Type: ParseResultService,
			Port: port,
			Service: service,
			ShouldDisplay: true,
		}, nil
	}
	
	// Otherwise, return as port discovery
	return &ParseResult{
		Type: ParseResultPort,
		Port: port,
		ShouldDisplay: true,
	}, nil
}

// isNmapNoise filters out uninteresting nmap output
func (p *NmapParser) isNmapNoise(line string) bool {
	noisePatterns := []string{
		"Starting Nmap",
		"Nmap scan report",
		"Host is up",
		"Not shown:",
		"PORT STATE SERVICE",
		"Nmap done:",
		"# Nmap",
		"Warning:",
		"NSOCK ERROR",
		"Bug in",
		"Assertion failed",
		"scanned in",
		"raw packets",
		"Scanning",
	}
	
	lineLower := strings.ToLower(line)
	for _, pattern := range noisePatterns {
		if strings.Contains(lineLower, strings.ToLower(pattern)) {
			return true
		}
	}
	
	return false
}

// isInterestingMessage determines if a message is worth displaying
func (p *NmapParser) isInterestingMessage(line string) bool {
	interestingPatterns := []string{
		"OS detection performed",
		"Aggressive OS guesses",
		"Service Info:",
	}
	
	lineLower := strings.ToLower(line)
	for _, pattern := range interestingPatterns {
		if strings.Contains(lineLower, strings.ToLower(pattern)) {
			return true
		}
	}
	
	return false
}

// ParserConfig holds configuration for parser behavior
type ParserConfig struct {
	ToolName string
	OutputFormat string // "json", "text", "xml", etc.
	Patterns map[string]string // Custom regex patterns
	EnableGeneric bool // Whether to fall back to generic parsing
}

// ParserRegistry manages all available output parsers
type ParserRegistry struct {
	parsers map[string]func() OutputParser
	configs map[string]*ParserConfig
}

// Global parser registry
var GlobalParserRegistry = NewParserRegistry()

func NewParserRegistry() *ParserRegistry {
	registry := &ParserRegistry{
		parsers: make(map[string]func() OutputParser),
		configs: make(map[string]*ParserConfig),
	}
	
	// No hardcoded parsers - everything is dynamic now
	
	return registry
}

// RegisterParser adds a new parser to the registry
func (r *ParserRegistry) RegisterParser(toolName string, parserFunc func() OutputParser) {
	r.parsers[toolName] = parserFunc
}

// RegisterConfig adds configuration for a tool
func (r *ParserRegistry) RegisterConfig(toolName string, config *ParserConfig) {
	r.configs[toolName] = config
}

// GetParser returns a dynamic parser that works with any tool
func (r *ParserRegistry) GetParser(toolName string) OutputParser {
	// Always use dynamic parser - no hardcoded tool knowledge
	return NewDynamicParserWrapper(toolName)
}

// GetConfig returns the configuration for a tool
func (r *ParserRegistry) GetConfig(toolName string) *ParserConfig {
	if config, exists := r.configs[toolName]; exists {
		return config
	}
	return &ParserConfig{
		ToolName: toolName,
		OutputFormat: "text",
		EnableGeneric: true,
	}
}

// StreamingOutputProcessor processes tool output in real-time
type StreamingOutputProcessor struct {
	parser   OutputParser
	progress *ProgressDisplay
	toolName string
}

func NewStreamingOutputProcessor(toolName string, target string) *StreamingOutputProcessor {
	parser := GlobalParserRegistry.GetParser(toolName)
	progress := NewProgressDisplay(target, toolName)
	
	return &StreamingOutputProcessor{
		parser:   parser,
		progress: progress,
		toolName: toolName,
	}
}

// ProcessLine processes a single line of output and updates the display
func (s *StreamingOutputProcessor) ProcessLine(line string) error {
	if s.parser == nil || s.progress == nil {
		return nil
	}
	
	result, err := s.parser.ParseLine(line)
	if err != nil || !result.ShouldDisplay {
		return err
	}
	
	switch result.Type {
	case ParseResultPort:
		if result.Port != nil {
			s.progress.AddDiscoveredPort(*result.Port)
		}
	case ParseResultService:
		if result.Service != nil {
			s.progress.AddDetectedService(*result.Service)
		}
	case ParseResultMessage:
		if result.Message != "" {
			s.progress.UpdateStatus(result.Message)
		}
	}
	
	return nil
}

// Start begins the progress display
func (s *StreamingOutputProcessor) Start() {
	if s.progress != nil {
		s.progress.Start()
	}
}

// Complete marks the processing as completed
func (s *StreamingOutputProcessor) Complete() {
	if s.progress != nil {
		s.progress.Complete()
		s.progress.ShowFinalSummary()
	}
}

// Fail marks the processing as failed
func (s *StreamingOutputProcessor) Fail(errorMsg string) {
	if s.progress != nil {
		s.progress.Fail(errorMsg)
	}
}

// GetResults returns all discovered results
func (s *StreamingOutputProcessor) GetResults() ([]PortInfo, []ServiceInfo) {
	if s.progress == nil {
		return nil, nil
	}
	return s.progress.GetDiscoveredPorts(), s.progress.GetDetectedServices()
}

// GenericParser handles output from unknown tools using pattern matching
type GenericParser struct {
	toolName string
	portRegex *regexp.Regexp
	serviceRegex *regexp.Regexp
	jsonRegex *regexp.Regexp
}

func NewGenericParser(toolName string) *GenericParser {
	return &GenericParser{
		toolName: toolName,
		// Generic patterns to catch common port/service formats
		portRegex: regexp.MustCompile(`(?i)(?:port|found|open|listening)[:\s]*?(\d+)(?:/(tcp|udp))?`),
		serviceRegex: regexp.MustCompile(`(?i)(\d+)(?:/(tcp|udp))?[\s:]+(?:open|running|listening)[\s:]+([a-zA-Z0-9_-]+)(?:[\s:]+(.+))?`),
		jsonRegex: regexp.MustCompile(`^\s*\{.*\}\s*$`),
	}
}

func (p *GenericParser) GetToolName() string {
	return p.toolName
}

func (p *GenericParser) ParseLine(line string) (*ParseResult, error) {
	line = strings.TrimSpace(line)
	
	// Skip empty lines
	if line == "" {
		return &ParseResult{Type: ParseResultIgnore}, nil
	}
	
	// Try to parse as JSON first (many modern tools output JSON)
	if p.jsonRegex.MatchString(line) {
		if result := p.tryParseJSON(line); result != nil {
			return result, nil
		}
	}
	
	// Try to match service detection pattern
	if matches := p.serviceRegex.FindStringSubmatch(line); matches != nil {
		portNum, err := strconv.Atoi(matches[1])
		if err == nil {
			protocol := matches[2]
			if protocol == "" {
				protocol = "tcp" // Default to TCP
			}
			
			service := matches[3]
			version := matches[4]
			
			if service != "" {
				// Enhance service using database
				db := database.Global()
				portStr := strconv.Itoa(portNum)
				
				// Try banner detection first, then port lookup
				finalService := service
				if version != "" {
					bannerService := db.GetServiceByBanner(version)
					if bannerService != "unknown-service" {
						finalService = bannerService
					}
				}
				
				// If still generic, use port lookup
				if finalService == service {
					dbService := db.GetServiceByPort(portStr)
					if !strings.HasPrefix(dbService, "port-") {
						finalService = dbService
					}
				}
				
				return &ParseResult{
					Type: ParseResultService,
					Port: &PortInfo{
						Number:  portNum,
						State:   "open",
						Service: finalService,
						Version: version,
					},
					Service: &ServiceInfo{
						Port:    portNum,
						Service: finalService,
						Version: version,
					},
					ShouldDisplay: true,
				}, nil
			}
		}
	}
	
	// Try to match simple port discovery pattern
	if matches := p.portRegex.FindStringSubmatch(line); matches != nil {
		portNum, err := strconv.Atoi(matches[1])
		if err == nil {
			protocol := matches[2]
			if protocol == "" {
				protocol = "tcp" // Default to TCP
			}
			
			// Enhance with database service info
			db := database.Global()
			portStr := strconv.Itoa(portNum)
			service := db.GetServiceByPort(portStr)
			
			return &ParseResult{
				Type: ParseResultPort,
				Port: &PortInfo{
					Number:  portNum,
					State:   "open",
					Service: service,
					Version: "",
				},
				ShouldDisplay: true,
			}, nil
		}
	}
	
	// Check if it looks like an interesting informational message
	if p.isInterestingLine(line) {
		return &ParseResult{
			Type: ParseResultMessage,
			Message: line,
			ShouldDisplay: true,
		}, nil
	}
	
	return &ParseResult{Type: ParseResultIgnore}, nil
}

// tryParseJSON attempts to parse a JSON line for common fields
func (p *GenericParser) tryParseJSON(line string) *ParseResult {
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(line), &jsonData); err != nil {
		return nil
	}
	
	// Try to extract port information from common JSON fields
	if port := p.extractPortFromJSON(jsonData); port != nil {
		return &ParseResult{
			Type: ParseResultPort,
			Port: port,
			ShouldDisplay: true,
		}
	}
	
	return nil
}

// extractPortFromJSON tries to extract port info from JSON data
func (p *GenericParser) extractPortFromJSON(data map[string]interface{}) *PortInfo {
	// Common field names for ports
	portFields := []string{"port", "Port", "PORT", "p", "portNumber"}
	serviceFields := []string{"service", "Service", "SERVICE", "name", "protocol"}
	
	var portNum int
	var service string
	
	// Extract port number
	for _, field := range portFields {
		if val, exists := data[field]; exists {
			switch v := val.(type) {
			case float64:
				portNum = int(v)
			case int:
				portNum = v
			case string:
				if p, err := strconv.Atoi(v); err == nil {
					portNum = p
				}
			}
			if portNum > 0 {
				break
			}
		}
	}
	
	// Extract service (optional)
	for _, field := range serviceFields {
		if val, exists := data[field]; exists {
			if s, ok := val.(string); ok {
				service = s
				break
			}
		}
	}
	
	if portNum > 0 && portNum <= 65535 {
		return &PortInfo{
			Number:  portNum,
			State:   "open",
			Service: service,
			Version: "",
		}
	}
	
	return nil
}

// isInterestingLine determines if a line contains potentially useful information
func (p *GenericParser) isInterestingLine(line string) bool {
	lineLower := strings.ToLower(line)
	
	// Skip common noise patterns
	noisePatterns := []string{
		"starting", "initializing", "loading", "warning:", "error:",
		"debug:", "info:", "verbose:", "trace:", "#", "//",
		"usage:", "help:", "version", "copyright", "license",
	}
	
	for _, pattern := range noisePatterns {
		if strings.Contains(lineLower, pattern) {
			return false
		}
	}
	
	// Look for potentially interesting keywords
	interestingPatterns := []string{
		"found", "detected", "discovered", "identified", "vulnerable",
		"open", "listening", "running", "active", "alive",
		"service", "version", "banner", "response", "result",
	}
	
	for _, pattern := range interestingPatterns {
		if strings.Contains(lineLower, pattern) {
			return true
		}
	}
	
	return false
}

// NewConfiguredGenericParser creates a generic parser with custom configuration
func NewConfiguredGenericParser(toolName string, config *ParserConfig) *GenericParser {
	parser := NewGenericParser(toolName)
	
	// Apply custom patterns if provided
	if config.Patterns != nil {
		if portPattern, exists := config.Patterns["port"]; exists {
			if regex, err := regexp.Compile(portPattern); err == nil {
				parser.portRegex = regex
			}
		}
		if servicePattern, exists := config.Patterns["service"]; exists {
			if regex, err := regexp.Compile(servicePattern); err == nil {
				parser.serviceRegex = regex
			}
		}
	}
	
	return parser
}

// UniversalParserWrapper adapts the new universal parser to the existing interface
type UniversalParserWrapper struct {
	parser *parsers.UniversalParser
}

// NewUniversalParserWrapper creates a wrapper for the universal parser
func NewUniversalParserWrapper(toolName string) (*UniversalParserWrapper, error) {
	parser, err := parsers.NewUniversalParser(toolName)
	if err != nil {
		return nil, err
	}
	
	return &UniversalParserWrapper{parser: parser}, nil
}

func (w *UniversalParserWrapper) GetToolName() string {
	return w.parser.GetToolName()
}

func (w *UniversalParserWrapper) ParseLine(line string) (*ParseResult, error) {
	result, err := w.parser.ParseLine(line)
	if err != nil {
		return nil, err
	}
	
	// Convert types from parsers package to ui package
	uiResult := &ParseResult{
		Type:          ParseResultType(result.Type),
		ShouldDisplay: result.ShouldDisplay,
		Message:       result.Message,
	}
	
	// Convert port info if present
	if result.Port != nil {
		uiResult.Port = &PortInfo{
			Number:  result.Port.Number,
			State:   result.Port.State,
			Service: result.Port.Service,
			Version: result.Port.Version,
		}
	}
	
	// Convert service info if present
	if result.Service != nil {
		uiResult.Service = &ServiceInfo{
			Port:    result.Service.Port,
			Service: result.Service.Service,
			Version: result.Service.Version,
		}
	}
	
	return uiResult, nil
}

// DynamicParserWrapper adapts the dynamic parser to the existing interface
type DynamicParserWrapper struct {
	parser *parsers.DynamicParser
}

// NewDynamicParserWrapper creates a wrapper for the dynamic parser
func NewDynamicParserWrapper(toolName string) *DynamicParserWrapper {
	return &DynamicParserWrapper{
		parser: parsers.NewDynamicParser(toolName),
	}
}

func (w *DynamicParserWrapper) GetToolName() string {
	return w.parser.ToolName
}

func (w *DynamicParserWrapper) ParseLine(line string) (*ParseResult, error) {
	result, err := w.parser.ParseLine(line)
	if err != nil {
		return nil, err
	}
	
	// Convert types from parsers package to ui package
	uiResult := &ParseResult{
		Type:          ParseResultType(result.Type),
		Message:       result.Message,
		ShouldDisplay: result.ShouldDisplay,
	}
	
	// Convert port info if present
	if result.Port != nil {
		uiResult.Port = &PortInfo{
			Number:  result.Port.Number,
			State:   result.Port.State,
			Service: result.Port.Service,
			Version: result.Port.Version,
		}
	}
	
	// Convert service info if present
	if result.Service != nil {
		uiResult.Service = &ServiceInfo{
			Port:    result.Service.Port,
			Service: result.Service.Service,
			Version: result.Service.Version,
		}
	}
	
	return uiResult, nil
}