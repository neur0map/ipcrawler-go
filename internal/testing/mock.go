package simulator

import (
	"fmt"
	"math/rand"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// MockSimulator implements the Simulator interface for demonstration
type MockSimulator struct {
	workflows   []WorkflowItem
	tools       []ToolItem
	logs        []LogEntry
	status      SystemStatus
	metrics     Metrics
	startTime   time.Time
	logTicker   *time.Ticker
	statusTicker *time.Ticker
}

// NewMockSimulator creates a new mock simulator with demo data
func NewMockSimulator() *MockSimulator {
	startTime := time.Now()
	
	return &MockSimulator{
		workflows: generateMockWorkflows(),
		tools:     generateMockTools(),
		logs:      []LogEntry{},
		startTime: startTime,
		status: SystemStatus{
			Status:      "running",
			Uptime:      0,
			ActiveTasks: 2,
			Completed:   15,
			Failed:      1,
			Version:     "1.0.0",
		},
		metrics: Metrics{
			CPU:         45.2,
			Memory:      62.8,
			ActiveTasks: 2,
			Throughput:  89.5,
			ErrorRate:   0.02,
		},
	}
}

// GetWorkflows returns mock workflows for the list component
func (ms *MockSimulator) GetWorkflows() []WorkflowItem {
	return ms.workflows
}

// ExecuteWorkflow simulates workflow execution
func (ms *MockSimulator) ExecuteWorkflow(id string) tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(time.Time) tea.Msg {
		return WorkflowExecutionMsg{
			WorkflowID: id,
			Status:     "started",
			Progress:   0.0,
			Message:    fmt.Sprintf("Starting workflow: %s", id),
		}
	})
}

// GetWorkflowStatus returns the status of a specific workflow
func (ms *MockSimulator) GetWorkflowStatus(id string) WorkflowStatus {
	// Simulate different statuses based on workflow ID
	statuses := map[string]string{
		"port-scan-basic":     "completed",
		"subdomain-discovery": "running", 
		"web-app-scan":        "pending",
		"network-discovery":   "ready",
		"vulnerability-scan":  "ready",
	}
	
	status, exists := statuses[id]
	if !exists {
		status = "unknown"
	}
	
	return WorkflowStatus{
		ID:        id,
		Status:    status,
		Progress:  rand.Float64(),
		StartTime: ms.startTime,
	}
}

// GetTools returns mock tools for the list component
func (ms *MockSimulator) GetTools() []ToolItem {
	return ms.tools
}

// ExecuteTool simulates tool execution
func (ms *MockSimulator) ExecuteTool(id string, args map[string]interface{}) tea.Cmd {
	return tea.Tick(time.Millisecond*200, func(time.Time) tea.Msg {
		return ToolExecutionMsg{
			ToolID:   id,
			Status:   "running",
			Progress: rand.Float64(),
			Output:   fmt.Sprintf("Executing %s with args: %v", id, args),
		}
	})
}

// GetLogs returns current log entries
func (ms *MockSimulator) GetLogs() []LogEntry {
	return ms.logs
}

// StreamLogs starts log streaming simulation
func (ms *MockSimulator) StreamLogs() tea.Cmd {
	return tea.Tick(time.Millisecond*500, func(time.Time) tea.Msg {
		return ms.generateLogMessage()
	})
}

// GetSystemStatus returns current system status
func (ms *MockSimulator) GetSystemStatus() SystemStatus {
	ms.status.Uptime = time.Since(ms.startTime)
	return ms.status
}

// GetMetrics returns current system metrics
func (ms *MockSimulator) GetMetrics() Metrics {
	// Simulate changing metrics
	ms.metrics.CPU = 30 + rand.Float64()*40        // 30-70%
	ms.metrics.Memory = 50 + rand.Float64()*30     // 50-80%
	ms.metrics.Throughput = 80 + rand.Float64()*40 // 80-120
	ms.metrics.ErrorRate = rand.Float64() * 0.05   // 0-5%
	
	return ms.metrics
}

// generateLogMessage creates a random log message
func (ms *MockSimulator) generateLogMessage() LogStreamMsg {
	messages := []string{
		"Initializing port scan on target network 192.168.1.0/24",
		"Found open port 22/tcp (SSH) on 192.168.1.100",
		"Found open port 80/tcp (HTTP) on 192.168.1.100", 
		"Found open port 443/tcp (HTTPS) on 192.168.1.100",
		"Found open port 3306/tcp (MySQL) on 192.168.1.101",
		"Subdomain discovery completed: 15 subdomains found",
		"Running nuclei security checks on discovered services",
		"Vulnerability CVE-2023-1234 detected on 192.168.1.100:80",
		"Web application scan completed with 3 findings",
		"Host discovery found 25 active hosts in network",
		"DNS enumeration completed successfully",
		"SSL certificate analysis complete",
		"Directory brute force found 8 hidden paths",
		"Technology stack identified: Apache 2.4.41, PHP 7.4.3",
		"Rate limiting detected, reducing scan speed",
	}
	
	levels := []string{"info", "warn", "error", "debug"}
	sources := []string{"nmap", "subfinder", "nuclei", "gobuster", "masscan"}
	
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     levels[rand.Intn(len(levels))],
		Source:    sources[rand.Intn(len(sources))],
		Message:   messages[rand.Intn(len(messages))],
	}
	
	// Add to internal log storage (with size limit)
	ms.logs = append(ms.logs, entry)
	if len(ms.logs) > 1000 {
		ms.logs = ms.logs[1:] // Keep only last 1000 entries
	}
	
	return LogStreamMsg{Entry: entry}
}

// generateMockWorkflows creates sample workflow data
func generateMockWorkflows() []WorkflowItem {
	return []WorkflowItem{
		{
			ID:          "port-scan-basic",
			Title:       "Basic Port Scan",
			Description: "Standard TCP port enumeration",
			Status:      "Ready",
			Tools:       []string{"nmap", "masscan"},
		},
		{
			ID:          "subdomain-discovery",
			Title:       "Subdomain Discovery",
			Description: "Comprehensive subdomain enumeration",
			Status:      "Running",
			Tools:       []string{"subfinder", "amass", "dnsgen"},
		},
		{
			ID:          "web-app-scan", 
			Title:       "Web Application Scan",
			Description: "Automated web app security testing",
			Status:      "Pending",
			Tools:       []string{"gobuster", "feroxbuster", "nuclei"},
		},
		{
			ID:          "network-discovery",
			Title:       "Network Discovery",
			Description: "Host discovery and network mapping",
			Status:      "Ready",
			Tools:       []string{"nmap", "masscan", "zmap"},
		},
		{
			ID:          "vulnerability-scan",
			Title:       "Vulnerability Assessment",
			Description: "Comprehensive vulnerability scanning",
			Status:      "Ready", 
			Tools:       []string{"nuclei", "nessus", "openvas"},
		},
	}
}

// generateMockTools creates sample tool data
func generateMockTools() []ToolItem {
	return []ToolItem{
		{
			ID:          "nmap",
			Name:        "Nmap",
			Description: "Network exploration and port scanner",
			Category:    "Port Scanning",
			Status:      "Available",
		},
		{
			ID:          "subfinder",
			Name:        "Subfinder",
			Description: "Passive subdomain discovery",
			Category:    "Reconnaissance",
			Status:      "Running",
		},
		{
			ID:          "masscan",
			Name:        "Masscan",
			Description: "High-speed port scanner",
			Category:    "Port Scanning", 
			Status:      "Available",
		},
		{
			ID:          "nuclei",
			Name:        "Nuclei",
			Description: "Vulnerability scanner with templates",
			Category:    "Vulnerability Scanning",
			Status:      "Available",
		},
		{
			ID:          "gobuster",
			Name:        "Gobuster",
			Description: "Directory and file brute-forcer",
			Category:    "Web Scanning",
			Status:      "Available",
		},
		{
			ID:          "feroxbuster",
			Name:        "Feroxbuster",
			Description: "Fast directory brute-forcer",
			Category:    "Web Scanning",
			Status:      "Available",
		},
		{
			ID:          "amass",
			Name:        "OWASP Amass",
			Description: "Network mapping of attack surfaces",
			Category:    "Reconnaissance",
			Status:      "Available",
		},
	}
}