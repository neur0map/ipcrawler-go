package reporting

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	
	"ipcrawler/internal/scanners"
)

// DefaultPortManager implements port file operations
func (p *DefaultPortManager) CombinePorts(outputFile string, inputFiles ...string) error {
	var allPorts []string
	portSet := make(map[string]bool) // Avoid duplicates
	
	for _, inputFile := range inputFiles {
		if _, err := os.Stat(inputFile); os.IsNotExist(err) {
			continue // Skip missing files
		}
		
		data, err := os.ReadFile(inputFile)
		if err != nil {
			continue // Skip unreadable files
		}
		
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !portSet[line] {
				allPorts = append(allPorts, line)
				portSet[line] = true
			}
		}
	}
	
	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	content := strings.Join(allPorts, "\n")
	return os.WriteFile(outputFile, []byte(content), 0644)
}

// ExtractPorts reads ports from a JSON file and saves as text
func (p *DefaultPortManager) ExtractPorts(jsonFile, outputFile string) error {
	// This would parse naabu JSON output and extract port numbers
	data, err := os.ReadFile(jsonFile)
	if err != nil {
		return fmt.Errorf("failed to read JSON file: %w", err)
	}
	
	var ports []string
	
	// Parse JSON lines (naabu format)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		
		// Simple JSON parsing - look for "port":number pattern
		if strings.Contains(line, `"port":`) {
			start := strings.Index(line, `"port":`) + 7
			end := start
			for end < len(line) && (line[end] >= '0' && line[end] <= '9') {
				end++
			}
			if end > start {
				portStr := line[start:end]
				if _, err := strconv.Atoi(portStr); err == nil {
					ports = append(ports, portStr)
				}
			}
		}
	}
	
	content := strings.Join(ports, "\n")
	return os.WriteFile(outputFile, []byte(content), 0644)
}

// ReadPorts reads ports from a text file
func (p *DefaultPortManager) ReadPorts(portFile string) ([]scanners.Port, error) {
	data, err := os.ReadFile(portFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read port file: %w", err)
	}
	
	var ports []scanners.Port
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		if portNum, err := strconv.Atoi(line); err == nil {
			ports = append(ports, scanners.Port{
				Number:   portNum,
				Protocol: "tcp",
				State:    "open",
			})
		}
	}
	
	return ports, nil
}

// WritePorts writes ports to a text file
func (p *DefaultPortManager) WritePorts(portFile string, ports []scanners.Port) error {
	var portStrs []string
	for _, port := range ports {
		portStrs = append(portStrs, strconv.Itoa(port.Number))
	}
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(portFile), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	content := strings.Join(portStrs, "\n")
	return os.WriteFile(portFile, []byte(content), 0644)
}