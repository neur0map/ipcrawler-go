package nmap

import (
	"encoding/xml"
	"os"
	"strconv"
	"strings"
)

// OutputParser handles nmap-specific XML output parsing
// This is ISOLATED tool-specific code that implements the ToolOutputParser interface
type OutputParser struct{}

// GetToolName returns the tool name for registration
func (p *OutputParser) GetToolName() string {
	return "nmap"
}

// NmapRun represents the root element of nmap XML output
type NmapRun struct {
	XMLName xml.Name `xml:"nmaprun"`
	Hosts   []Host   `xml:"host"`
	Stats   RunStats `xml:"runstats"`
}

// Host represents a scanned host
type Host struct {
	Addresses []Address `xml:"address"`
	Ports     Ports     `xml:"ports"`
	Status    Status    `xml:"status"`
}

// Address represents host address information
type Address struct {
	Addr     string `xml:"addr,attr"`
	AddrType string `xml:"addrtype,attr"`
}

// Status represents host status
type Status struct {
	State string `xml:"state,attr"`
}

// Ports represents the ports section
type Ports struct {
	Ports []Port `xml:"port"`
}

// Port represents a single port
type Port struct {
	Protocol string  `xml:"protocol,attr"`
	PortID   int     `xml:"portid,attr"`
	State    State   `xml:"state"`
	Service  Service `xml:"service"`
}

// State represents port state
type State struct {
	State string `xml:"state,attr"`
}

// Service represents service information
type Service struct {
	Name    string `xml:"name,attr"`
	Product string `xml:"product,attr"`
	Version string `xml:"version,attr"`
}

// RunStats represents scan statistics
type RunStats struct {
	Finished Finished `xml:"finished"`
}

// Finished represents completion information
type Finished struct {
	Time string `xml:"time,attr"`
}

// ParseOutput extracts useful data from nmap XML output and creates magic variables
// This method contains ALL nmap-specific logic, isolated from the main executor
func (p *OutputParser) ParseOutput(outputPath string) map[string]string {
	// Read the XML output file
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return map[string]string{
			"ports":        "",
			"port_count":   "0",
			"error":        "failed to read output file",
		}
	}

	// Parse XML
	var nmapRun NmapRun
	if err := xml.Unmarshal(data, &nmapRun); err != nil {
		return map[string]string{
			"ports":        "",
			"port_count":   "0", 
			"error":        "failed to parse XML",
		}
	}

	// Extract port and service information
	var openPorts []string
	var closedPorts []string
	var filteredPorts []string
	var tcpPorts []string
	var udpPorts []string
	var services []string
	var products []string
	hosts := make(map[string]bool)

	for _, host := range nmapRun.Hosts {
		// Extract host addresses
		for _, addr := range host.Addresses {
			if addr.AddrType == "ipv4" || addr.AddrType == "ipv6" {
				hosts[addr.Addr] = true
			}
		}

		// Extract port information
		for _, port := range host.Ports.Ports {
			portStr := strconv.Itoa(port.PortID)
			
			// Categorize by state
			switch strings.ToLower(port.State.State) {
			case "open":
				openPorts = append(openPorts, portStr)
			case "closed":
				closedPorts = append(closedPorts, portStr)
			case "filtered":
				filteredPorts = append(filteredPorts, portStr)
			}

			// Categorize by protocol
			switch strings.ToLower(port.Protocol) {
			case "tcp":
				tcpPorts = append(tcpPorts, portStr)
			case "udp":
				udpPorts = append(udpPorts, portStr)
			}

			// Extract service information
			if port.Service.Name != "" {
				services = append(services, port.Service.Name)
			}
			if port.Service.Product != "" {
				products = append(products, port.Service.Product)
			}
		}
	}

	// Convert host map to slice
	var hostList []string
	for host := range hosts {
		hostList = append(hostList, host)
	}

	// Create magic variables that other tools can use
	magicVars := map[string]string{
		"ports":            strings.Join(openPorts, ","),
		"port_count":       strconv.Itoa(len(openPorts)),
		"open_ports":       strings.Join(openPorts, ","),
		"open_port_count":  strconv.Itoa(len(openPorts)),
		"closed_ports":     strings.Join(closedPorts, ","),
		"closed_port_count": strconv.Itoa(len(closedPorts)),
		"filtered_ports":   strings.Join(filteredPorts, ","),
		"filtered_port_count": strconv.Itoa(len(filteredPorts)),
		"tcp_ports":        strings.Join(removeDuplicates(tcpPorts), ","),
		"udp_ports":        strings.Join(removeDuplicates(udpPorts), ","),
		"services":         strings.Join(removeDuplicates(services), ","),
		"service_count":    strconv.Itoa(len(removeDuplicates(services))),
		"products":         strings.Join(removeDuplicates(products), ","),
		"hosts":            strings.Join(hostList, ","),
		"host_count":       strconv.Itoa(len(hostList)),
	}

	// If no open ports found, provide fallback
	if len(openPorts) == 0 {
		magicVars["ports"] = ""
		magicVars["port_count"] = "0"
		magicVars["open_ports"] = ""
	}

	return magicVars
}

// removeDuplicates removes duplicate strings from a slice
func removeDuplicates(slice []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if item != "" && !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}