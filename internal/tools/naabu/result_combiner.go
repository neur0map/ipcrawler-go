package naabu

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ResultCombiner handles combining results from multiple naabu scan modes
// This is ISOLATED tool-specific code for naabu result consolidation
type ResultCombiner struct{}

// CombineResults merges multiple naabu JSON output files into consolidated magic variables
// This method contains ALL naabu-specific combining logic
func (rc *ResultCombiner) CombineResults(outputPaths []string) map[string]string {
	if len(outputPaths) == 0 {
		return map[string]string{
			"combined_ports":      "",
			"combined_port_count": "0",
			"error":               "no output files provided",
		}
	}

	// If only one file, parse it normally
	if len(outputPaths) == 1 {
		parser := &OutputParser{}
		vars := parser.ParseOutput(outputPaths[0])

		// Add "combined_" prefix to variables for consistency
		combined := make(map[string]string)
		for key, value := range vars {
			combined["combined_"+key] = value
		}
		return combined
	}

	// Parse all files and collect results
	var allResults []NaabuResult
	hosts := make(map[string]bool)
	sources := make(map[string]string) // Track which file each port came from

	for i, outputPath := range outputPaths {
		data, err := os.ReadFile(outputPath)
		if err != nil {
			continue // Skip files that can't be read
		}

		// Parse JSONL format
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var result NaabuResult
			if err := json.Unmarshal([]byte(line), &result); err != nil {
				continue // Skip invalid lines
			}

			allResults = append(allResults, result)
			hosts[result.IP] = true

			// Track source of this port discovery
			portKey := fmt.Sprintf("%s:%d", result.IP, result.Port)
			if _, exists := sources[portKey]; !exists {
				sources[portKey] = fmt.Sprintf("mode_%d", i+1)
			}
		}
	}

	// Deduplicate and categorize results
	uniquePorts := make(map[string]bool)
	var ports []string
	var tlsPorts []string
	var tcpPorts []string
	var udpPorts []string
	coverage := make(map[string][]string) // port -> list of modes that found it

	for _, result := range allResults {
		portStr := strconv.Itoa(result.Port)
		portKey := fmt.Sprintf("%s:%d", result.IP, result.Port)

		if !uniquePorts[portStr] {
			uniquePorts[portStr] = true
			ports = append(ports, portStr)
		}

		// Track which modes found this port
		sourceMode := sources[portKey]
		coverage[portStr] = append(coverage[portStr], sourceMode)

		// Categorize by protocol and features
		switch strings.ToLower(result.Protocol) {
		case "tcp":
			if !contains(tcpPorts, portStr) {
				tcpPorts = append(tcpPorts, portStr)
			}
		case "udp":
			if !contains(udpPorts, portStr) {
				udpPorts = append(udpPorts, portStr)
			}
		}

		if result.TLS && !contains(tlsPorts, portStr) {
			tlsPorts = append(tlsPorts, portStr)
		}
	}

	// Calculate coverage statistics
	var highCoveragePorts []string // Found by multiple modes
	var uniqueDiscoveries []string // Found by only one mode

	for port, modes := range coverage {
		modeSet := make(map[string]bool)
		for _, mode := range modes {
			modeSet[mode] = true
		}

		if len(modeSet) > 1 {
			highCoveragePorts = append(highCoveragePorts, port)
		} else {
			uniqueDiscoveries = append(uniqueDiscoveries, port)
		}
	}

	// Convert host map to slice
	var hostList []string
	for host := range hosts {
		hostList = append(hostList, host)
	}

	// Create combined magic variables
	combinedVars := map[string]string{
		// Core combined results
		"combined_ports":        strings.Join(ports, ","),
		"combined_port_count":   strconv.Itoa(len(ports)),
		"combined_unique_ports": strings.Join(ports, ","), // Already deduplicated
		"combined_hosts":        strings.Join(hostList, ","),
		"combined_host_count":   strconv.Itoa(len(hostList)),

		// Protocol-specific results
		"combined_tcp_ports":      strings.Join(tcpPorts, ","),
		"combined_tcp_port_count": strconv.Itoa(len(tcpPorts)),
		"combined_udp_ports":      strings.Join(udpPorts, ","),
		"combined_udp_port_count": strconv.Itoa(len(udpPorts)),
		"combined_tls_ports":      strings.Join(tlsPorts, ","),
		"combined_tls_port_count": strconv.Itoa(len(tlsPorts)),

		// Coverage analysis
		"combined_high_coverage_ports":    strings.Join(highCoveragePorts, ","),
		"combined_high_coverage_count":    strconv.Itoa(len(highCoveragePorts)),
		"combined_unique_discoveries":     strings.Join(uniqueDiscoveries, ","),
		"combined_unique_discovery_count": strconv.Itoa(len(uniqueDiscoveries)),

		// Scan statistics
		"combined_scan_count":    strconv.Itoa(len(outputPaths)),
		"combined_total_results": strconv.Itoa(len(allResults)),
	}

	// Fallback if no results
	if len(ports) == 0 {
		combinedVars["combined_ports"] = ""
		combinedVars["combined_port_count"] = "0"
	}

	return combinedVars
}

// GetToolName returns the tool name for registration
func (rc *ResultCombiner) GetToolName() string {
	return "naabu"
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
