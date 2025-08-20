package naabu

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

// OutputParser handles naabu-specific output parsing
// This is ISOLATED tool-specific code that implements the ToolOutputParser interface
type OutputParser struct{}

// GetToolName returns the tool name for registration
func (p *OutputParser) GetToolName() string {
	return "naabu"
}

// NaabuResult represents a single result from naabu JSON output
type NaabuResult struct {
	IP        string `json:"ip"`
	Port      int    `json:"port"`
	Protocol  string `json:"protocol"`
	Timestamp string `json:"timestamp"`
	TLS       bool   `json:"tls,omitempty"`
}

// ParseOutput extracts useful data from naabu JSON output and creates magic variables
// This method contains ALL naabu-specific logic, isolated from the main executor
func (p *OutputParser) ParseOutput(outputPath string) map[string]string {
	// Read the output file
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return map[string]string{
			"ports":      "",
			"port_count": "0",
			"error":      "failed to read output file",
		}
	}

	// Parse JSONL (JSON Lines) format that naabu produces
	lines := strings.Split(string(data), "\n")
	var results []NaabuResult
	var ports []string
	var tlsPorts []string
	hosts := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var result NaabuResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			continue // Skip invalid lines
		}

		results = append(results, result)
		ports = append(ports, strconv.Itoa(result.Port))
		hosts[result.IP] = true

		if result.TLS {
			tlsPorts = append(tlsPorts, strconv.Itoa(result.Port))
		}
	}

	// Extract unique hosts
	var hostList []string
	for host := range hosts {
		hostList = append(hostList, host)
	}

	// Create magic variables that other tools can use
	magicVars := map[string]string{
		"ports":          strings.Join(ports, ","),
		"port_count":     strconv.Itoa(len(ports)),
		"unique_ports":   strings.Join(removeDuplicates(ports), ","),
		"tls_ports":      strings.Join(tlsPorts, ","),
		"tls_port_count": strconv.Itoa(len(tlsPorts)),
		"hosts":          strings.Join(hostList, ","),
		"host_count":     strconv.Itoa(len(hostList)),
	}

	// Add protocol-specific port lists
	tcpPorts := []string{}
	udpPorts := []string{}
	for _, result := range results {
		portStr := strconv.Itoa(result.Port)
		switch strings.ToLower(result.Protocol) {
		case "tcp":
			tcpPorts = append(tcpPorts, portStr)
		case "udp":
			udpPorts = append(udpPorts, portStr)
		}
	}

	magicVars["tcp_ports"] = strings.Join(removeDuplicates(tcpPorts), ",")
	magicVars["udp_ports"] = strings.Join(removeDuplicates(udpPorts), ",")

	// If no ports found, provide fallback
	if len(ports) == 0 {
		magicVars["ports"] = "80,443"
		magicVars["port_count"] = "0"
		magicVars["unique_ports"] = "80,443"
	}

	return magicVars
}

// removeDuplicates removes duplicate strings from a slice
func removeDuplicates(slice []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}
