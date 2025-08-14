package nmap

import (
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ResultCombiner handles combining results from multiple nmap scan modes
// This is ISOLATED tool-specific code for nmap result consolidation
type ResultCombiner struct{}

// ServiceInfo represents combined service information
type ServiceInfo struct {
	Port     int
	Protocol string
	State    string
	Service  string
	Product  string
	Version  string
	Sources  []string // Which scan modes found this service
}

// CombineResults merges multiple nmap XML output files into consolidated magic variables
// This method contains ALL nmap-specific combining logic
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
	hosts := make(map[string]bool)
	services := make(map[string]*ServiceInfo) // port:protocol -> ServiceInfo
	
	for i, outputPath := range outputPaths {
		data, err := os.ReadFile(outputPath)
		if err != nil {
			continue // Skip files that can't be read
		}

		var nmapRun NmapRun
		if err := xml.Unmarshal(data, &nmapRun); err != nil {
			continue // Skip invalid XML files
		}

		sourceMode := fmt.Sprintf("mode_%d", i+1)

		// Process each host
		for _, host := range nmapRun.Hosts {
			// Extract host addresses
			for _, addr := range host.Addresses {
				if addr.AddrType == "ipv4" || addr.AddrType == "ipv6" {
					hosts[addr.Addr] = true
				}
			}

			// Process ports and services
			for _, port := range host.Ports.Ports {
				key := fmt.Sprintf("%d:%s", port.PortID, port.Protocol)
				
				if existing, exists := services[key]; exists {
					// Merge information from multiple scans
					existing.Sources = append(existing.Sources, sourceMode)
					
					// Update service info if this scan has more details
					if port.Service.Name != "" && existing.Service == "" {
						existing.Service = port.Service.Name
					}
					if port.Service.Product != "" && existing.Product == "" {
						existing.Product = port.Service.Product
					}
					if port.Service.Version != "" && existing.Version == "" {
						existing.Version = port.Service.Version
					}
					
					// Keep the most "open" state (open > filtered > closed)
					if port.State.State == "open" || (existing.State != "open" && port.State.State == "filtered") {
						existing.State = port.State.State
					}
				} else {
					// New service discovery
					services[key] = &ServiceInfo{
						Port:     port.PortID,
						Protocol: port.Protocol,
						State:    port.State.State,
						Service:  port.Service.Name,
						Product:  port.Service.Product,
						Version:  port.Service.Version,
						Sources:  []string{sourceMode},
					}
				}
			}
		}
	}

	// Categorize and analyze results
	var openPorts []string
	var closedPorts []string
	var filteredPorts []string
	var tcpPorts []string
	var udpPorts []string
	var serviceNames []string
	var productNames []string
	var highConfidenceServices []string  // Found by multiple scans
	var uniqueDiscoveries []string       // Found by only one scan

	for _, svc := range services {
		portStr := strconv.Itoa(svc.Port)
		
		// Categorize by state
		switch strings.ToLower(svc.State) {
		case "open":
			openPorts = append(openPorts, portStr)
		case "closed":
			closedPorts = append(closedPorts, portStr)
		case "filtered":
			filteredPorts = append(filteredPorts, portStr)
		}

		// Categorize by protocol
		switch strings.ToLower(svc.Protocol) {
		case "tcp":
			tcpPorts = append(tcpPorts, portStr)
		case "udp":
			udpPorts = append(udpPorts, portStr)
		}

		// Collect service information
		if svc.Service != "" {
			serviceNames = append(serviceNames, svc.Service)
		}
		if svc.Product != "" {
			productNames = append(productNames, svc.Product)
		}

		// Analyze discovery confidence
		uniqueSources := make(map[string]bool)
		for _, source := range svc.Sources {
			uniqueSources[source] = true
		}
		
		serviceDesc := fmt.Sprintf("%d/%s", svc.Port, svc.Protocol)
		if svc.Service != "" {
			serviceDesc += fmt.Sprintf("(%s)", svc.Service)
		}
		
		if len(uniqueSources) > 1 {
			highConfidenceServices = append(highConfidenceServices, serviceDesc)
		} else {
			uniqueDiscoveries = append(uniqueDiscoveries, serviceDesc)
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
		"combined_ports":                strings.Join(openPorts, ","),
		"combined_port_count":           strconv.Itoa(len(openPorts)),
		"combined_open_ports":           strings.Join(openPorts, ","),
		"combined_open_port_count":      strconv.Itoa(len(openPorts)),
		"combined_hosts":                strings.Join(hostList, ","),
		"combined_host_count":           strconv.Itoa(len(hostList)),
		
		// State-specific results
		"combined_closed_ports":         strings.Join(closedPorts, ","),
		"combined_closed_port_count":    strconv.Itoa(len(closedPorts)),
		"combined_filtered_ports":       strings.Join(filteredPorts, ","),
		"combined_filtered_port_count":  strconv.Itoa(len(filteredPorts)),
		
		// Protocol-specific results
		"combined_tcp_ports":            strings.Join(removeDuplicates(tcpPorts), ","),
		"combined_tcp_port_count":       strconv.Itoa(len(removeDuplicates(tcpPorts))),
		"combined_udp_ports":            strings.Join(removeDuplicates(udpPorts), ","),
		"combined_udp_port_count":       strconv.Itoa(len(removeDuplicates(udpPorts))),
		
		// Service information
		"combined_services":             strings.Join(removeDuplicates(serviceNames), ","),
		"combined_service_count":        strconv.Itoa(len(removeDuplicates(serviceNames))),
		"combined_products":             strings.Join(removeDuplicates(productNames), ","),
		"combined_product_count":        strconv.Itoa(len(removeDuplicates(productNames))),
		
		// Confidence analysis
		"combined_high_confidence_services":    strings.Join(highConfidenceServices, ","),
		"combined_high_confidence_count":       strconv.Itoa(len(highConfidenceServices)),
		"combined_unique_discoveries":          strings.Join(uniqueDiscoveries, ","),
		"combined_unique_discovery_count":      strconv.Itoa(len(uniqueDiscoveries)),
		
		// Scan statistics
		"combined_scan_count":           strconv.Itoa(len(outputPaths)),
		"combined_total_services":       strconv.Itoa(len(services)),
	}

	// Fallback if no results
	if len(openPorts) == 0 {
		combinedVars["combined_ports"] = ""
		combinedVars["combined_port_count"] = "0"
		combinedVars["combined_open_ports"] = ""
	}

	return combinedVars
}

// GetToolName returns the tool name for registration
func (rc *ResultCombiner) GetToolName() string {
	return "nmap"
}