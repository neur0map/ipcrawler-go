package agents

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ipcrawler/internal/utils"
)

// NmapProcessor processes and cleans nmap scan results
type NmapProcessor struct {
	*BaseAgent
	config *NmapProcessorConfig
}

// NmapProcessorConfig holds configuration for the nmap processor
type NmapProcessorConfig struct {
	ExtractFields    []string `yaml:"extract_fields"`
	IncludeClosedPorts bool   `yaml:"include_closed_ports"`
	MinimumPorts     int      `yaml:"minimum_ports"`
}

// DefaultNmapProcessorConfig returns default configuration
func DefaultNmapProcessorConfig() *NmapProcessorConfig {
	return &NmapProcessorConfig{
		ExtractFields:      []string{"ports", "services", "os", "vulnerabilities", "scripts"},
		IncludeClosedPorts: false,
		MinimumPorts:       1,
	}
}

// NewNmapProcessor creates a new nmap processor agent
func NewNmapProcessor(config *NmapProcessorConfig) *NmapProcessor {
	if config == nil {
		config = DefaultNmapProcessorConfig()
	}
	
	return &NmapProcessor{
		BaseAgent: NewBaseAgent("nmap_processor", nil),
		config:    config,
	}
}

// Validate checks if the nmap cleaner is properly configured
func (n *NmapProcessor) Validate() error {
	if n.config == nil {
		return fmt.Errorf("nmap cleaner config is required")
	}
	if len(n.config.ExtractFields) == 0 {
		return fmt.Errorf("at least one extract field must be specified")
	}
	return nil
}

// Process cleans and formats nmap scan results
func (n *NmapProcessor) Process(input *AgentInput) (*AgentOutput, error) {
	if err := n.ValidateInput(input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	
	n.LogInfo("Processing nmap output for target: %s", input.Target)
	
	output := n.CreateOutput(nil, input.Metadata, true)
	
	// Extract tool outputs from input
	toolOutputs, ok := input.Data.(map[string]*ToolOutput)
	if !ok {
		return nil, fmt.Errorf("invalid input data format")
	}
	
	// Find nmap output
	nmapOutput, exists := toolOutputs["nmap"]
	if !exists {
		n.LogWarning("No nmap output found")
		output.AddWarning("No nmap output found")
		return output, nil
	}
	
	// Process nmap data
	cleanedData, err := n.processNmapData(nmapOutput)
	if err != nil {
		n.LogError("Failed to process nmap data: %v", err)
		output.AddError(fmt.Errorf("failed to process nmap data: %w", err))
		return output, nil
	}
	
	// Generate text report
	textReport, err := n.generateTextReport(cleanedData)
	if err != nil {
		n.LogError("Failed to generate text report: %v", err)
		output.AddError(fmt.Errorf("failed to generate text report: %w", err))
	}
	
	// Save outputs
	processedDir := filepath.Join(input.ReportDir, "processed")
	
	// Save cleaned JSON
	if err := n.saveCleanedJSON(processedDir, cleanedData); err != nil {
		n.LogError("Failed to save cleaned JSON: %v", err)
		output.AddError(fmt.Errorf("failed to save cleaned JSON: %w", err))
	}
	
	// Save text report
	if err := n.saveTextReport(processedDir, textReport); err != nil {
		n.LogError("Failed to save text report: %v", err)
		output.AddError(fmt.Errorf("failed to save text report: %w", err))
	}
	
	result := &CleanedNmapData{
		CleanedData: cleanedData,
		TextReport:  textReport,
		Statistics:  n.generateStatistics(cleanedData),
	}
	
	output.Data = result
	output.Metadata["ports_found"] = fmt.Sprintf("%d", len(cleanedData.Ports))
	output.Metadata["services_identified"] = fmt.Sprintf("%d", len(cleanedData.Services))
	
	n.LogInfo("Nmap cleaning completed. Found %d ports, %d services", 
		len(cleanedData.Ports), len(cleanedData.Services))
	
	return output, nil
}

// NmapData represents cleaned nmap scan results
type NmapData struct {
	Target        string         `json:"target"`
	ScanTime      time.Time      `json:"scan_time"`
	Ports         []Port         `json:"ports"`
	Services      []Service      `json:"services"`
	OSDetection   *OSInfo        `json:"os_detection,omitempty"`
	Scripts       []ScriptResult `json:"scripts,omitempty"`
	Statistics    ScanStatistics `json:"statistics"`
}

// Port represents a network port
type Port struct {
	Number     int    `json:"number"`
	Protocol   string `json:"protocol"`
	State      string `json:"state"`
	Service    string `json:"service"`
	Version    string `json:"version,omitempty"`
	Product    string `json:"product,omitempty"`
	ExtraInfo  string `json:"extra_info,omitempty"`
	Confidence int    `json:"confidence"`
}

// Service represents a detected service
type Service struct {
	Name       string `json:"name"`
	Port       int    `json:"port"`
	Protocol   string `json:"protocol"`
	Version    string `json:"version,omitempty"`
	Product    string `json:"product,omitempty"`
	CPE        string `json:"cpe,omitempty"`
}

// OSInfo represents OS detection results
type OSInfo struct {
	Name       string  `json:"name"`
	Family     string  `json:"family"`
	Generation string  `json:"generation,omitempty"`
	Vendor     string  `json:"vendor,omitempty"`
	Accuracy   int     `json:"accuracy"`
}

// ScriptResult represents nmap script results
type ScriptResult struct {
	ID     string `json:"id"`
	Output string `json:"output"`
	Port   int    `json:"port,omitempty"`
}

// ScanStatistics represents scan statistics
type ScanStatistics struct {
	TotalHosts     int           `json:"total_hosts"`
	UpHosts        int           `json:"up_hosts"`
	TotalPorts     int           `json:"total_ports"`
	OpenPorts      int           `json:"open_ports"`
	ClosedPorts    int           `json:"closed_ports"`
	FilteredPorts  int           `json:"filtered_ports"`
	ScanDuration   time.Duration `json:"scan_duration"`
}

// CleanedNmapData represents the complete cleaned nmap output
type CleanedNmapData struct {
	CleanedData *NmapData      `json:"cleaned_data"`
	TextReport  string         `json:"text_report"`
	Statistics  ScanStatistics `json:"statistics"`
}

// XML parsing structures for nmap output
type NmapRun struct {
	XMLName xml.Name `xml:"nmaprun"`
	Scanner string   `xml:"scanner,attr"`
	Args    string   `xml:"args,attr"`
	Start   string   `xml:"start,attr"`
	StartStr string  `xml:"startstr,attr"`
	Version string   `xml:"version,attr"`
	Hosts   []Host   `xml:"host"`
}

type Host struct {
	StartTime string     `xml:"starttime,attr"`
	EndTime   string     `xml:"endtime,attr"`
	Status    HostStatus `xml:"status"`
	Address   []Address  `xml:"address"`
	Hostnames Hostnames  `xml:"hostnames"`
	Ports     Ports      `xml:"ports"`
	OS        *OS        `xml:"os"`
}

type HostStatus struct {
	State  string `xml:"state,attr"`
	Reason string `xml:"reason,attr"`
}

type Address struct {
	Addr     string `xml:"addr,attr"`
	AddrType string `xml:"addrtype,attr"`
}

type Hostnames struct {
	Hostname []Hostname `xml:"hostname"`
}

type Hostname struct {
	Name string `xml:"name,attr"`
	Type string `xml:"type,attr"`
}

type Ports struct {
	Port []NmapPort `xml:"port"`
}

type NmapPort struct {
	Protocol string      `xml:"protocol,attr"`
	PortID   string      `xml:"portid,attr"`
	State    PortState   `xml:"state"`
	Service  PortService `xml:"service"`
}

type PortState struct {
	State  string `xml:"state,attr"`
	Reason string `xml:"reason,attr"`
}

type PortService struct {
	Name      string `xml:"name,attr"`
	Product   string `xml:"product,attr"`
	Version   string `xml:"version,attr"`
	ExtraInfo string `xml:"extrainfo,attr"`
	Conf      string `xml:"conf,attr"`
}

type OS struct {
	OSMatch []OSMatch `xml:"osmatch"`
}

type OSMatch struct {
	Name     string    `xml:"name,attr"`
	Accuracy string    `xml:"accuracy,attr"`
	OSClass  []OSClass `xml:"osclass"`
}

type OSClass struct {
	Type     string `xml:"type,attr"`
	Vendor   string `xml:"vendor,attr"`
	OSFamily string `xml:"osfamily,attr"`
	OSGen    string `xml:"osgen,attr"`
}

// processNmapData processes raw nmap XML data
func (n *NmapProcessor) processNmapData(toolOutput *ToolOutput) (*NmapData, error) {
	// For nmap, we need to parse XML data
	if len(toolOutput.RawData) == 0 {
		return nil, fmt.Errorf("no raw data available")
	}
	
	// Check if it's XML data
	if !strings.Contains(string(toolOutput.RawData), "<?xml") {
		return nil, fmt.Errorf("nmap output is not in XML format")
	}
	
	// Parse XML data
	var nmaprun NmapRun
	if err := xml.Unmarshal(toolOutput.RawData, &nmaprun); err != nil {
		return nil, fmt.Errorf("failed to parse nmap XML: %w", err)
	}
	
	result := &NmapData{
		Ports:    make([]Port, 0),
		Services: make([]Service, 0),
		Scripts:  make([]ScriptResult, 0),
	}
	
	// Extract scan time
	if nmaprun.StartStr != "" {
		if parsedTime, err := time.Parse("Mon Jan 2 15:04:05 2006", nmaprun.StartStr); err == nil {
			result.ScanTime = parsedTime
		}
	}
	
	// Process hosts
	if len(nmaprun.Hosts) > 0 {
		host := nmaprun.Hosts[0] // Process first host
		
		// Extract target address
		for _, addr := range host.Address {
			if addr.AddrType == "ipv4" || addr.AddrType == "ipv6" {
				result.Target = addr.Addr
				break
			}
		}
		
		// Extract ports
		for _, nmapPort := range host.Ports.Port {
			port := Port{
				Number:     n.parseIntFromString(nmapPort.PortID),
				Protocol:   nmapPort.Protocol,
				State:      nmapPort.State.State,
				Service:    nmapPort.Service.Name,
				Version:    nmapPort.Service.Version,
				Product:    nmapPort.Service.Product,
				ExtraInfo:  nmapPort.Service.ExtraInfo,
				Confidence: n.parseIntFromString(nmapPort.Service.Conf),
			}
			
			if port.State == "open" || n.config.IncludeClosedPorts {
				result.Ports = append(result.Ports, port)
				
				// Add to services if it's a service
				if port.Service != "" {
					service := Service{
						Name:     port.Service,
						Port:     port.Number,
						Protocol: port.Protocol,
						Version:  port.Version,
						Product:  port.Product,
					}
					result.Services = append(result.Services, service)
				}
			}
		}
		
		// Extract OS detection
		if host.OS != nil && len(host.OS.OSMatch) > 0 {
			osMatch := host.OS.OSMatch[0]
			osInfo := &OSInfo{
				Name:     osMatch.Name,
				Accuracy: n.parseIntFromString(osMatch.Accuracy),
			}
			
			if len(osMatch.OSClass) > 0 {
				osClass := osMatch.OSClass[0]
				osInfo.Family = osClass.OSFamily
				osInfo.Vendor = osClass.Vendor
				osInfo.Generation = osClass.OSGen
			}
			
			result.OSDetection = osInfo
		}
	}
	
	result.Statistics = n.calculateStatistics(result)
	return result, nil
}

// parseIntFromString safely parses integer from string
func (n *NmapProcessor) parseIntFromString(s string) int {
	if s == "" {
		return 0
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return val
}


// generateTextReport creates a human-readable text report
func (n *NmapProcessor) generateTextReport(data *NmapData) (string, error) {
	var report strings.Builder
	
	report.WriteString("NMAP SCAN REPORT\n")
	report.WriteString("================\n\n")
	
	report.WriteString(fmt.Sprintf("Target: %s\n", data.Target))
	report.WriteString(fmt.Sprintf("Scan Time: %s\n\n", data.ScanTime.Format("2006-01-02 15:04:05")))
	
	// Port summary
	report.WriteString("PORT SUMMARY\n")
	report.WriteString("------------\n")
	report.WriteString(fmt.Sprintf("Open Ports: %d\n", data.Statistics.OpenPorts))
	report.WriteString(fmt.Sprintf("Closed Ports: %d\n", data.Statistics.ClosedPorts))
	report.WriteString(fmt.Sprintf("Filtered Ports: %d\n\n", data.Statistics.FilteredPorts))
	
	// Open ports details
	if len(data.Ports) > 0 {
		report.WriteString("OPEN PORTS\n")
		report.WriteString("----------\n")
		for _, port := range data.Ports {
			if port.State == "open" {
				report.WriteString(fmt.Sprintf("%d/%s\t%s\t%s", 
					port.Number, port.Protocol, port.State, port.Service))
				if port.Version != "" {
					report.WriteString(fmt.Sprintf("\t%s", port.Version))
				}
				report.WriteString("\n")
			}
		}
		report.WriteString("\n")
	}
	
	// Services
	if len(data.Services) > 0 {
		report.WriteString("DETECTED SERVICES\n")
		report.WriteString("-----------------\n")
		for _, service := range data.Services {
			report.WriteString(fmt.Sprintf("%s on port %d/%s", 
				service.Name, service.Port, service.Protocol))
			if service.Version != "" {
				report.WriteString(fmt.Sprintf(" - %s", service.Version))
			}
			report.WriteString("\n")
		}
		report.WriteString("\n")
	}
	
	// OS Detection
	if data.OSDetection != nil {
		report.WriteString("OS DETECTION\n")
		report.WriteString("------------\n")
		report.WriteString(fmt.Sprintf("OS: %s\n", data.OSDetection.Name))
		report.WriteString(fmt.Sprintf("Family: %s\n", data.OSDetection.Family))
		report.WriteString(fmt.Sprintf("Accuracy: %d%%\n\n", data.OSDetection.Accuracy))
	}
	
	return report.String(), nil
}


func (n *NmapProcessor) calculateStatistics(data *NmapData) ScanStatistics {
	stats := ScanStatistics{}
	
	for _, port := range data.Ports {
		stats.TotalPorts++
		switch port.State {
		case "open":
			stats.OpenPorts++
		case "closed":
			stats.ClosedPorts++
		case "filtered":
			stats.FilteredPorts++
		}
	}
	
	return stats
}

func (n *NmapProcessor) generateStatistics(data *NmapData) ScanStatistics {
	return n.calculateStatistics(data)
}

func (n *NmapProcessor) saveCleanedJSON(processedDir string, data *NmapData) error {
	// Create processed directory if it doesn't exist
	if err := os.MkdirAll(processedDir, 0755); err != nil {
		return fmt.Errorf("failed to create processed directory: %w", err)
	}
	
	// Marshal data to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal nmap data to JSON: %w", err)
	}
	
	// Write to file
	filePath := filepath.Join(processedDir, "nmap_cleaned.json")
	if err := utils.WriteFileWithPermissions(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write nmap JSON file: %w", err)
	}
	
	n.LogInfo("Saved cleaned nmap JSON to: %s", filePath)
	return nil
}

func (n *NmapProcessor) saveTextReport(processedDir string, report string) error {
	// Create processed directory if it doesn't exist
	if err := os.MkdirAll(processedDir, 0755); err != nil {
		return fmt.Errorf("failed to create processed directory: %w", err)
	}
	
	// Write to file
	filePath := filepath.Join(processedDir, "nmap_report.txt")
	if err := utils.WriteFileWithPermissions(filePath, []byte(report), 0644); err != nil {
		return fmt.Errorf("failed to write nmap text report: %w", err)
	}
	
	n.LogInfo("Saved nmap text report to: %s", filePath)
	return nil
}