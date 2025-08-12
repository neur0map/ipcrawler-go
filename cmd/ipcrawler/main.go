package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/mattn/go-isatty"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	netutil "github.com/shirou/gopsutil/v3/net"

	"github.com/neur0map/ipcrawler/internal/config"
	"github.com/neur0map/ipcrawler/internal/tui/data/loader"
)

// focusState tracks which card is focused
type focusState int

const (
	targetInputFocus focusState = iota // Modal for target input
	workflowTreeFocus                  // Was overviewFocus - now for selecting workflows
	scanOverviewFocus                  // Was workflowsFocus - now shows selected workflow details
	outputFocus                        // Live raw tool output
	logsFocus                          // System logs, debug, errors, warnings
	toolsFocus
	perfFocus
)

type model struct {
	// Layout
	width  int
	height int
	focus  focusState
	ready  bool

	// Configuration
	config *config.Config

	// Data source
	workflows *loader.WorkflowData

	// Target modal state
	showTargetModal bool           // Controls modal visibility
	targetInput     textinput.Model // Text input component
	scanTarget      string          // Stored target value after validation
	targetError     string          // Validation error message

	// Interactive list components
	workflowTreeList list.Model // For selecting workflows
	scanOverviewList list.Model // Shows execution queue and status

	// Multi-select workflow tracking
	selectedWorkflows map[string]bool // Track which workflows are selected
	executedWorkflows map[string]bool // Track which workflows have been executed
	currentWorkflow   string          // Currently highlighted workflow
	executionQueue    []string        // Queue of workflows to execute

	// Live output (raw tool output)
	outputViewport viewport.Model
	liveOutput     []string // Raw tool execution output

	// System logs (debug, errors, warnings)
	logsViewport viewport.Model
	systemLogs   []string    // System messages, debug info, errors
	logger       *log.Logger // Charmbracelet structured logger

	// Running tools
	spinner spinner.Model
	tools   []toolExecution

	// Performance data
	perfData systemMetrics

	// Calculated card dimensions
	cardWidth  int
	cardHeight int

	// Styles for cards
	cardStyle        lipgloss.Style
	focusedCardStyle lipgloss.Style
	titleStyle       lipgloss.Style
	dimStyle         lipgloss.Style
	headerStyle      lipgloss.Style
}

type toolExecution struct {
	Name   string
	Status string
	Output string
}

type systemMetrics struct {
	CPUPercent         float64
	CPUCores           int
	MemoryUsed         uint64
	MemoryTotal        uint64
	MemoryPercent      float64
	DiskUsed           uint64
	DiskTotal          uint64
	DiskPercent        float64
	NetworkSent        uint64
	NetworkRecv        uint64
	NetworkSentRate    uint64   // Bytes per second
	NetworkRecvRate    uint64   // Bytes per second
	NetworkSentHistory []uint64 // Last 30 readings for sparkline
	NetworkRecvHistory []uint64 // Last 30 readings for sparkline
	Uptime             uint64
	Goroutines         int
	LastUpdate         time.Time
	Hostname           string
	Platform           string

	// Smooth animation values
	AnimatedCPU        float64
	AnimatedMemory     float64
	AnimatedDisk       float64
	AnimationStartTime time.Time
	BaselineGoroutines int // Baseline goroutine count (excluding monitoring)
	lastCPUTimes       *cpu.TimesStat // Previous CPU times for calculating usage
}

// systemMetricsMsg is sent when system metrics are updated asynchronously
type systemMetricsMsg systemMetrics

// metricsTickMsg is sent by the metrics ticker to trigger regular updates
type metricsTickMsg struct{}

// List item implementations for bubbles/list component

// systemItem represents a system overview item
type systemItem struct {
	title string
	desc  string
}

func (i systemItem) Title() string       { return i.title }
func (i systemItem) Description() string { return i.desc }
func (i systemItem) FilterValue() string { return i.title }

// workflowItem represents a workflow in the tree with selection status
type workflowItem struct {
	name        string
	description string
	toolCount   int
	selected    bool
	executed    bool // Track if workflow has been executed
}

func (i workflowItem) Title() string {
	checkbox := "[ ]"
	if i.selected {
		checkbox = "[X]"
	}

	// Add executed mark with color
	executedMark := ""
	if i.executed {
		// Use a checkmark for executed workflows
		// Executed workflow styling (success color from ui.yaml)
		executedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
		executedMark = " " + executedStyle.Render("✓")
	}

	return fmt.Sprintf("%s %s%s", checkbox, i.name, executedMark)
}
func (i workflowItem) Description() string {
	// Show tool count and executed status more concisely
	if i.executed {
		// Executed workflow styling (success color from ui.yaml)
		executedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
		return fmt.Sprintf("%s (%d tools) %s", i.description, i.toolCount, executedStyle.Render("[EXECUTED]"))
	}
	return fmt.Sprintf("%s (%d tools)", i.description, i.toolCount)
}
func (i workflowItem) FilterValue() string { return i.name }

// toolItem represents a tool within a workflow
type toolItem struct {
	name     string
	workflow string
}

func (i toolItem) Title() string       { return i.name }
func (i toolItem) Description() string { return fmt.Sprintf("in %s workflow", i.workflow) }
func (i toolItem) FilterValue() string { return i.name }

// executionItem represents a workflow in the execution queue
type executionItem struct {
	name        string
	description string
	status      string // "queued", "running", "completed", "failed"
}

func (i executionItem) Title() string {
	statusIcon := "[WAIT]"
	switch i.status {
	case "queued":
		statusIcon = "[QUEUE]"
	case "running":
		statusIcon = "[RUN]"
	case "completed":
		statusIcon = "[DONE]"
	case "failed":
		statusIcon = "[FAIL]"
	case "ready":
		statusIcon = "[READY]"
	case "info":
		statusIcon = "[INFO]"
	default:
		statusIcon = "[WAIT]"
	}
	return fmt.Sprintf("%s %s", statusIcon, i.name)
}
func (i executionItem) Description() string {
	return fmt.Sprintf("%s - %s", i.description, i.status)
}
func (i executionItem) FilterValue() string { return i.name }

func newModel() *model {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		// Continue with defaults if config fails
		cfg = &config.Config{}
	}

	// Load workflows - try multiple paths
	var workflows *loader.WorkflowData

	// Try current directory first
	workflows, err = loader.LoadWorkflowDescriptions(".")
	if err != nil || workflows == nil || len(workflows.Workflows) == 0 {
		// Try from executable directory
		if execPath, err := os.Executable(); err == nil {
			execDir := filepath.Dir(execPath)
			workflows, _ = loader.LoadWorkflowDescriptions(execDir)

			// If that fails, try parent directory of executable (for bin/ case)
			if workflows == nil || len(workflows.Workflows) == 0 {
				parentDir := filepath.Dir(execDir)
				workflows, _ = loader.LoadWorkflowDescriptions(parentDir)
			}
		}
	}

	// Final fallback: empty workflows
	if workflows == nil || len(workflows.Workflows) == 0 {
		workflows = &loader.WorkflowData{Workflows: make(map[string]loader.WorkflowConfig)}
	}

	// Create viewports for output and logs using config settings
	viewportWidth := cfg.UI.Formatting.DebugViewportWidth
	viewportHeight := cfg.UI.Formatting.DebugViewportHeight
	if viewportWidth == 0 {
		viewportWidth = 50 // Default fallback
	}
	if viewportHeight == 0 {
		viewportHeight = 10 // Default fallback
	}
	liveOutputVp := viewport.New(viewportWidth, viewportHeight)
	// Apply viewport config settings
	if cfg.UI.Components.Viewport.MouseWheelDelta > 0 {
		liveOutputVp.MouseWheelDelta = cfg.UI.Components.Viewport.MouseWheelDelta
	} else {
		liveOutputVp.MouseWheelDelta = 3 // Fallback if not configured
	}
	if cfg.UI.Components.Viewport.HighPerformance {
		liveOutputVp.HighPerformanceRendering = true
	}

	logsVp := viewport.New(viewportWidth, viewportHeight)
	// Apply viewport config settings
	if cfg.UI.Components.Viewport.MouseWheelDelta > 0 {
		logsVp.MouseWheelDelta = cfg.UI.Components.Viewport.MouseWheelDelta
	} else {
		logsVp.MouseWheelDelta = 3 // Fallback if not configured
	}
	if cfg.UI.Components.Viewport.HighPerformance {
		logsVp.HighPerformanceRendering = true
	}

	// Initialize live output (raw tool execution output) - extensive content for scrolling demo
	liveOutputLines := []string{
		"=== IPCrawler Live Tool Execution Output ===",
		"Session started: 2024-08-09 12:34:56",
		"",
		">>> Executing Reconnaissance Workflow <<<",
		"",
		"[12:34:56] [NMAP] Starting network reconnaissance",
		"[12:34:56] nmap -sS -sV -O --top-ports 1000 target.com",
		"[12:34:57] Starting Nmap 7.95 ( https://nmap.org ) at 2024-08-09 12:34",
		"[12:34:58] Nmap scan report for target.com (192.168.1.100)",
		"[12:34:59] Host is up (0.050s latency).",
		"[12:35:00] Not shown: 997 closed tcp ports (reset)",
		"[12:35:01] PORT     STATE SERVICE    VERSION",
		"[12:35:02] 22/tcp   open  ssh        OpenSSH 8.9p1 Ubuntu 3ubuntu0.1",
		"[12:35:03] 80/tcp   open  http       Apache httpd 2.4.52",
		"[12:35:04] 443/tcp  open  ssl/http   Apache httpd 2.4.52",
		"[12:35:05] 3306/tcp open  mysql      MySQL 8.0.33-0ubuntu0.22.04.2",
		"[12:35:06] 8080/tcp open  http-proxy Squid http proxy 5.2",
		"[12:35:07] Device type: general purpose",
		"[12:35:08] Running: Linux 5.X",
		"[12:35:09] OS CPE: cpe:/o:linux:linux_kernel:5",
		"[12:35:10] OS details: Linux 5.0 - 5.4",
		"[12:35:11] Network Distance: 2 hops",
		"[12:35:12] Nmap done: 1 IP address (1 host up) scanned in 15.42 seconds",
		"",
		"[12:35:13] [SUBDOMAIN] Starting subdomain enumeration",
		"[12:35:13] subfinder -d target.com -all -recursive",
		"[12:35:14] Found subdomain: www.target.com",
		"[12:35:15] Found subdomain: api.target.com",
		"[12:35:16] Found subdomain: admin.target.com",
		"[12:35:17] Found subdomain: dev.target.com",
		"[12:35:18] Found subdomain: staging.target.com",
		"[12:35:19] Found subdomain: mail.target.com",
		"[12:35:20] Found subdomain: ftp.target.com",
		"[12:35:21] Found subdomain: blog.target.com",
		"[12:35:22] Found subdomain: shop.target.com",
		"[12:35:23] Found subdomain: support.target.com",
		"[12:35:24] amass enum -d target.com -brute",
		"[12:35:25] Found subdomain: vpn.target.com",
		"[12:35:26] Found subdomain: jenkins.target.com",
		"[12:35:27] Found subdomain: gitlab.target.com",
		"[12:35:28] Found subdomain: jira.target.com",
		"[12:35:29] Total subdomains found: 14",
		"",
		"[12:35:30] [DIRB] Starting directory brute force",
		"[12:35:30] gobuster dir -u http://target.com -w /usr/share/wordlists/dirbuster/directory-list-2.3-medium.txt",
		"[12:35:31] /.htaccess            (Status: 403) [Size: 278]",
		"[12:35:32] /.htpasswd            (Status: 403) [Size: 278]",
		"[12:35:33] /admin                (Status: 200) [Size: 1432]",
		"[12:35:34] /api                  (Status: 200) [Size: 89]",
		"[12:35:35] /backup               (Status: 301) [Size: 315]",
		"[12:35:36] /cgi-bin              (Status: 403) [Size: 278]",
		"[12:35:37] /config               (Status: 301) [Size: 315]",
		"[12:35:38] /css                  (Status: 301) [Size: 312]",
		"[12:35:39] /dashboard            (Status: 302) [Size: 0]",
		"[12:35:40] /docs                 (Status: 301) [Size: 313]",
		"[12:35:41] /downloads            (Status: 301) [Size: 318]",
		"[12:35:42] /files                (Status: 301) [Size: 314]",
		"[12:35:43] /images               (Status: 301) [Size: 315]",
		"[12:35:44] /js                   (Status: 301) [Size: 311]",
		"[12:35:45] /login                (Status: 200) [Size: 2156]",
		"[12:35:46] /logs                 (Status: 403) [Size: 278]",
		"[12:35:47] /phpinfo              (Status: 200) [Size: 95912]",
		"[12:35:48] /robots.txt           (Status: 200) [Size: 45]",
		"[12:35:49] /server-info          (Status: 403) [Size: 278]",
		"[12:35:50] /server-status        (Status: 403) [Size: 278]",
		"[12:35:51] /uploads              (Status: 301) [Size: 316]",
		"[12:35:52] /wp-admin             (Status: 301) [Size: 317]",
		"[12:35:53] /wp-content           (Status: 301) [Size: 319]",
		"[12:35:54] /wp-includes          (Status: 301) [Size: 320]",
		"",
		">>> Executing Web Application Workflow <<<",
		"",
		"[12:35:55] [NIKTO] Starting web vulnerability scan",
		"[12:35:55] nikto -h http://target.com -Format txt",
		"[12:35:56] - Nikto v2.5.0",
		"[12:35:57] + Target IP:          192.168.1.100",
		"[12:35:58] + Target Hostname:    target.com",
		"[12:35:59] + Target Port:        80",
		"[12:36:00] + Start Time:         2024-08-09 12:35:55",
		"[12:36:01] + Server: Apache/2.4.52 (Ubuntu)",
		"[12:36:02] + /: The anti-clickjacking X-Frame-Options header is not present.",
		"[12:36:03] + /: The X-Content-Type-Options header is not set.",
		"[12:36:04] + /: Server may leak inodes via ETags, header found with file /robots.txt",
		"[12:36:05] + /admin/: This might be interesting... has been seen in web logs from an unknown scanner.",
		"[12:36:06] + /backup/: Backup folder found. This can contain sensitive files.",
		"[12:36:07] + /config/: Configuration file found. May contain sensitive information.",
		"[12:36:08] + /phpinfo.php: Output from the phpinfo() function was found.",
		"[12:36:09] + /wp-admin/: Admin login page/section found.",
		"[12:36:10] + /wp-login.php: Wordpress login found",
		"[12:36:11] + 7967 requests: 0 error(s) and 10 item(s) reported",
		"",
		"[12:36:12] [SQLMAP] Testing for SQL injection",
		"[12:36:12] sqlmap -u 'http://target.com/login.php' --data='username=admin&password=admin' --batch",
		"[12:36:13] [INFO] testing connection to the target URL",
		"[12:36:14] [INFO] checking if the target is protected by some kind of WAF/IPS",
		"[12:36:15] [INFO] testing if the target URL content is stable",
		"[12:36:16] [INFO] target URL content is stable",
		"[12:36:17] [INFO] testing if POST parameter 'username' is dynamic",
		"[12:36:18] [WARNING] POST parameter 'username' does not appear to be dynamic",
		"[12:36:19] [INFO] heuristic (basic) test shows that POST parameter 'username' might be injectable",
		"[12:36:20] [INFO] testing for SQL injection on POST parameter 'username'",
		"[12:36:21] [INFO] testing 'AND boolean-based blind - WHERE or HAVING clause'",
		"[12:36:22] [INFO] POST parameter 'username' appears to be 'AND boolean-based blind - WHERE or HAVING clause' injectable",
		"[12:36:23] [INFO] testing 'Generic inline queries'",
		"[12:36:24] [INFO] testing 'MySQL >= 5.5 AND error-based - WHERE, HAVING, ORDER BY or GROUP BY clause (BIGINT UNSIGNED)'",
		"[12:36:25] [INFO] testing 'MySQL >= 5.5 OR error-based - WHERE or HAVING clause (BIGINT UNSIGNED)'",
		"[12:36:26] POST parameter 'username' is vulnerable. Do you want to keep testing the others?",
		"",
		">>> Executing Vulnerability Scanning Workflow <<<",
		"",
		"[12:36:27] [NESSUS] Starting comprehensive vulnerability scan",
		"[12:36:27] Starting Nessus scan ID: 12345",
		"[12:36:28] Scan policy: Full Network Scan",
		"[12:36:29] Target: 192.168.1.100",
		"[12:36:30] [HIGH] CVE-2023-1234: Apache HTTP Server vulnerability",
		"[12:36:31] [MEDIUM] CVE-2023-5678: MySQL privilege escalation",
		"[12:36:32] [LOW] Missing security headers detected",
		"[12:36:33] [INFO] Service enumeration complete",
		"[12:36:34] [CRITICAL] SSH brute force protection not enabled",
		"[12:36:35] [HIGH] Outdated WordPress installation detected",
		"[12:36:36] [MEDIUM] Weak SSL/TLS configuration",
		"[12:36:37] [LOW] Directory listing enabled in /uploads/",
		"[12:36:38] Total vulnerabilities found: 43",
		"[12:36:39] Critical: 1, High: 8, Medium: 15, Low: 19",
		"",
		"[12:36:40] [OPENVAS] Secondary vulnerability validation",
		"[12:36:40] Launching OpenVAS scan...",
		"[12:36:41] Scanning 192.168.1.100 with NVT feed 20240809",
		"[12:36:42] Port scanning (Nmap) completed",
		"[12:36:43] Host details: Linux target.com 5.15.0-72-generic",
		"[12:36:44] Service detection completed",
		"[12:36:45] Vulnerability tests: 98765 NVTs loaded",
		"[12:36:46] Found: Weak SSH encryption algorithms",
		"[12:36:47] Found: Apache version disclosure",
		"[12:36:48] Found: MySQL default configuration",
		"[12:36:49] Found: Missing HTTPS enforcement",
		"[12:36:50] Scan progress: 100% complete",
		"",
		"=== SCAN SUMMARY ===",
		"Total execution time: 16 minutes 54 seconds",
		"Hosts scanned: 1",
		"Subdomains found: 14",
		"Directories discovered: 25",
		"Vulnerabilities identified: 43",
		"Critical security issues: 1",
		"Recommendations generated: 12",
		"",
		"=== END OF LIVE OUTPUT ===",
		"",
		"Use ↑↓ arrow keys to scroll through this content",
		"Focus this window with key '3' then scroll",
		"Switch to Logs window with key '4'",
	}

	// Define color styles for direct log formatting (no buffer to avoid losing colors)
	debugStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.UI.Theme.Colors["debug"]))
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.UI.Theme.Colors["info"]))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.UI.Theme.Colors["warning"]))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.UI.Theme.Colors["error"]))

	timestampStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.UI.Theme.Colors["timestamp"]))
	prefixStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(cfg.UI.Theme.Colors["prefix"]))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.UI.Theme.Colors["key"]))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.UI.Theme.Colors["value"]))
	workflowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.UI.Theme.Colors["workflow"]))
	countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.UI.Theme.Colors["count"]))
	progressStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.UI.Theme.Colors["progress"]))

	// Helper function to create colored log entries
	createLogEntry := func(level, message string, keyvals ...interface{}) string {
		timeFormat := cfg.UI.Formatting.TimeFormat
		if timeFormat == "" {
			timeFormat = "15:04:05" // Fallback
		}
		timestamp := timestampStyle.Render(time.Now().Format(timeFormat))
		prefix := prefixStyle.Render("IPCrawler")

		var levelStyled string
		switch level {
		case "DEBUG":
			levelStyled = debugStyle.Bold(true).Render("DEBUG")
		case "INFO":
			levelStyled = infoStyle.Bold(true).Render("INFO ")
		case "WARN":
			levelStyled = warnStyle.Bold(true).Render("WARN ")
		case "ERROR":
			levelStyled = errorStyle.Bold(true).Render("ERROR")
		}

		// Format key-value pairs with colors
		var kvPairs []string
		for i := 0; i < len(keyvals); i += 2 {
			if i+1 < len(keyvals) {
				key := fmt.Sprintf("%v", keyvals[i])
				value := fmt.Sprintf("%v", keyvals[i+1])

				// Apply special styling for specific keys
				var styledKey, styledValue string
				switch key {
				case "workflow":
					styledKey = workflowStyle.Render(key)
					styledValue = workflowStyle.Render(value)
				case "count", "total_queued":
					styledKey = countStyle.Render(key)
					styledValue = countStyle.Render(value)
				case "progress":
					styledKey = progressStyle.Render(key)
					styledValue = progressStyle.Render(value)
				case "err":
					styledKey = errorStyle.Render(key)
					styledValue = errorStyle.Render(value)
				default:
					styledKey = keyStyle.Render(key)
					styledValue = valueStyle.Render(value)
				}
				kvPairs = append(kvPairs, styledKey+"="+styledValue)
			}
		}

		// Combine all parts
		logLine := timestamp + " " + prefix + " " + levelStyled + " " + message
		if len(kvPairs) > 0 {
			logLine += " " + strings.Join(kvPairs, " ")
		}
		return logLine
	}

	// Build initial system logs with colored formatting
	systemLogLines := []string{
		createLogEntry("INFO", "System initialized"),
		createLogEntry("INFO", "Loading workflows from descriptions.yaml"),
	}

	// Add workflow loading result
	if len(workflows.Workflows) > 0 {
		systemLogLines = append(systemLogLines, createLogEntry("INFO", "Workflows loaded successfully", "count", len(workflows.Workflows)))
		for name := range workflows.Workflows {
			systemLogLines = append(systemLogLines, createLogEntry("DEBUG", "Found workflow", "name", name))
		}
	} else {
		systemLogLines = append(systemLogLines, createLogEntry("WARN", "No workflows loaded - check workflows/descriptions.yaml"))
		if err != nil {
			systemLogLines = append(systemLogLines, createLogEntry("ERROR", "Workflow loading failed", "err", err))
		}
	}

	// Build extensive system logs with colored formatting
	systemLogLines = append(systemLogLines, []string{
		createLogEntry("INFO", "TUI ready - Use Tab to navigate cards"),
		createLogEntry("INFO", "Press 1-6 for direct card focus"),
		createLogEntry("DEBUG", "Available focus states: workflow, queue, output, logs, tools, performance"),
		createLogEntry("DEBUG", "Workflow selection: Use SPACE to add/remove from queue"),
		createLogEntry("DEBUG", "Execution: Press ENTER when workflows are selected"),
		createLogEntry("INFO", "Viewport scrolling enabled - use arrow keys when focused"),
		createLogEntry("DEBUG", "Live output shows raw tool execution"),
		createLogEntry("DEBUG", "Logs show system events and debug information"),
		createLogEntry("INFO", "Memory usage optimal - ready for workflow execution"),
		createLogEntry("DEBUG", "All components initialized successfully"),

		// Add extensive system log entries for scrolling demo with varied levels and colors
		createLogEntry("DEBUG", "Configuration validation starting..."),
		createLogEntry("INFO", "Loading workflow configurations", "count", 5),
		createLogEntry("WARN", "Using default configuration - custom config not found"),
		createLogEntry("DEBUG", "Parsing reconnaissance workflow", "tools", "nmap,subfinder,amass,gobuster"),
		createLogEntry("DEBUG", "Parsing web-application workflow", "tools", "nikto,sqlmap,wpscan,burpsuite"),
		createLogEntry("DEBUG", "Parsing vulnerability-scanning workflow", "tools", "nessus,openvas,nuclei,zap"),
		createLogEntry("DEBUG", "Parsing network-scanning workflow", "tools", "masscan,zmap,rustscan,unicornscan"),
		createLogEntry("DEBUG", "Parsing post-exploitation workflow", "tools", "metasploit,empire,cobalt-strike,bloodhound"),
		createLogEntry("INFO", "All workflow configurations validated successfully"),

		createLogEntry("DEBUG", "UI component initialization starting..."),
		createLogEntry("DEBUG", "Creating workflow tree list component", "width", 35, "height", 12),
		createLogEntry("DEBUG", "Creating execution queue list component", "width", 35, "height", 12),
		createLogEntry("DEBUG", "Creating live output viewport", "width", viewportWidth, "height", viewportHeight),
		createLogEntry("DEBUG", "Creating system logs viewport", "width", viewportWidth, "height", viewportHeight),
		createLogEntry("DEBUG", "Setting up key bindings", "focus_keys", "1,2,3,4,5,6"),
		createLogEntry("DEBUG", "Configuring scroll support", "scroll_keys", "up,down,page-up,page-down"),
		createLogEntry("INFO", "All UI components initialized and configured"),

		createLogEntry("DEBUG", "Security module initialization..."),
		createLogEntry("DEBUG", "Loading security policies", "policy_count", 15),
		createLogEntry("DEBUG", "Validating tool permissions", "executable_tools", 25),
		createLogEntry("DEBUG", "Setting up sandboxing", "mode", "restricted"),
		createLogEntry("DEBUG", "Configuring audit logging", "log_level", "debug"),
		createLogEntry("INFO", "Security framework initialized successfully"),

		createLogEntry("DEBUG", "Performance monitoring setup..."),
		createLogEntry("DEBUG", "Initializing memory tracker", "initial_memory", "12.5MB"),
		createLogEntry("DEBUG", "Setting up goroutine monitor", "initial_goroutines", 5),
		createLogEntry("DEBUG", "Configuring metrics collection", "interval", "1s"),
		createLogEntry("DEBUG", "Enabling real-time performance updates"),
		createLogEntry("INFO", "Performance monitoring active"),

		createLogEntry("DEBUG", "Network module configuration..."),
		createLogEntry("DEBUG", "Testing external connectivity", "target", "8.8.8.8"),
		createLogEntry("DEBUG", "Validating DNS resolution", "test_domain", "google.com"),
		createLogEntry("DEBUG", "Checking proxy settings", "proxy", "none"),
		createLogEntry("DEBUG", "Configuring timeout values", "connect_timeout", "10s", "read_timeout", "30s"),
		createLogEntry("INFO", "Network connectivity verified"),

		createLogEntry("DEBUG", "Tool dependency verification..."),
		createLogEntry("DEBUG", "Checking nmap installation", "version", "7.95", "status", "available"),
		createLogEntry("DEBUG", "Checking subfinder installation", "version", "2.6.3", "status", "available"),
		createLogEntry("DEBUG", "Checking gobuster installation", "version", "3.6", "status", "available"),
		createLogEntry("DEBUG", "Checking nikto installation", "version", "2.5.0", "status", "available"),
		createLogEntry("DEBUG", "Checking sqlmap installation", "version", "1.7.11", "status", "available"),
		createLogEntry("WARN", "Some optional tools not found", "missing", "burpsuite,metasploit"),
		createLogEntry("INFO", "Core tools verified - system ready for operation"),
		createLogEntry("ERROR", "Tool validation failed for deprecated scanner", "err", "version incompatible"),

		// Database-related logs removed

		createLogEntry("DEBUG", "Session management initialization..."),
		createLogEntry("DEBUG", "Generating session ID", "session", "abc123def456"),
		createLogEntry("DEBUG", "Setting session timeout", "timeout", "2h"),
		// Auto-save log removed
		createLogEntry("DEBUG", "Loading previous session state", "found", false),
		createLogEntry("INFO", "New session created successfully"),

		createLogEntry("DEBUG", "Workflow engine startup..."),
		createLogEntry("DEBUG", "Loading workflow executor", "max_concurrent", cfg.Tools.ToolExecution.MaxConcurrentExecutions),
		createLogEntry("DEBUG", "Initializing task queue", "max_size", cfg.Tools.ToolExecution.MaxParallelExecutions*50), // Estimated queue size
		createLogEntry("DEBUG", "Setting up progress tracking", "granularity", "step"),
		createLogEntry("DEBUG", "Configuring error handling", "retry_attempts", cfg.Tools.RetryAttempts),
		createLogEntry("INFO", "Workflow execution engine ready"),

		createLogEntry("DEBUG", "Output processing initialization..."),
		createLogEntry("DEBUG", "Setting up result parsers", "formats", "json,xml,txt"),
		createLogEntry("DEBUG", "Configuring data sanitization", "mode", "strict"),
		createLogEntry("DEBUG", "Setting up export handlers", "formats", "pdf,html,csv"),
		createLogEntry("INFO", "Output processing pipeline configured"),

		createLogEntry("DEBUG", "Plugin system loading..."),
		createLogEntry("DEBUG", "Scanning plugin directory", "path", "/usr/local/share/ipcrawler/plugins"),
		createLogEntry("DEBUG", "Loading custom reconnaissance plugin", "name", "advanced-recon"),
		createLogEntry("DEBUG", "Loading reporting enhancement plugin", "name", "executive-summary"),
		createLogEntry("WARN", "Plugin signature verification failed", "plugin", "untrusted-scanner"),
		createLogEntry("INFO", "Trusted plugins loaded successfully", "count", 2),

		createLogEntry("DEBUG", "Final system checks..."),
		createLogEntry("DEBUG", "Verifying file permissions", "config_dir", "/etc/ipcrawler"),
		createLogEntry("DEBUG", "Checking disk space", "available", "15.2GB", "required", "1GB"),
		createLogEntry("DEBUG", "Validating log rotation", "max_size", "100MB", "retention", "30d"),
		createLogEntry("DEBUG", "Testing emergency shutdown procedures"),
		createLogEntry("INFO", "All system checks passed - IPCrawler ready for operation"),

		createLogEntry("INFO", "=== SYSTEM STARTUP COMPLETE ==="),
		createLogEntry("DEBUG", "Total initialization time: 2.34 seconds"),
		createLogEntry("INFO", "System health: 100% operational", "progress", "100%"),
		createLogEntry("DEBUG", "Ready to accept workflow execution requests"),
		createLogEntry("INFO", "Use ↑↓ keys to scroll logs when focused on this window"),

		createLogEntry("ERROR", "Demo mode active - some features limited", "err", "no production license"),
	}...)

	// Set content for both viewports and auto-scroll to bottom
	liveOutputVp.SetContent(strings.Join(liveOutputLines, "\n"))
	liveOutputVp.GotoBottom() // Start at bottom for live updates

	logsVp.SetContent(strings.Join(systemLogLines, "\n"))
	logsVp.GotoBottom() // Start at bottom for live updates

	// Create spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	// Spinner styling
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.UI.Theme.Colors["spinner"]))

	// Create list delegates with custom styling
	delegate := list.NewDefaultDelegate()
	// List selection styling
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color(cfg.UI.Theme.Colors["list_selected"])).
		BorderLeftForeground(lipgloss.Color(cfg.UI.Theme.Colors["list_selected"]))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle.Copy()

	// Build workflow tree items (for multi-selection)
	workflowTreeItems := []list.Item{}
	for name, workflow := range workflows.Workflows {
		workflowTreeItems = append(workflowTreeItems, workflowItem{
			name:        name,
			description: workflow.Name,
			toolCount:   len(workflow.Tools),
			selected:    false, // Initially not selected
			executed:    false, // Initially not executed
		})
	}

	// Add fallback items if no workflows
	if len(workflows.Workflows) == 0 {
		workflowTreeItems = []list.Item{
			workflowItem{name: "No workflows found", description: "Check workflows/descriptions.yaml", toolCount: 0, selected: false},
		}
	}

	// Initialize with empty execution queue
	executionQueueItems := []list.Item{
		executionItem{name: "No workflows selected", description: "Select workflows and press Enter to execute", status: ""},
	}

	// Create the lists with config settings
	workflowTreeList := list.New(workflowTreeItems, delegate, 0, 0)
	if cfg.UI.Components.List.Title != "" {
		workflowTreeList.Title = "Available " + cfg.UI.Components.List.Title
	} else {
		workflowTreeList.Title = "Available Workflows"
	}
	workflowTreeList.SetShowStatusBar(cfg.UI.Components.List.ShowStatusBar)
	workflowTreeList.SetShowPagination(false)
	workflowTreeList.SetFilteringEnabled(cfg.UI.Components.List.FilteringEnabled)
	// Remove background from title
	workflowTreeList.Styles.Title = workflowTreeList.Styles.Title.Background(lipgloss.NoColor{})

	scanOverviewList := list.New(executionQueueItems, delegate, 0, 0)
	scanOverviewList.Title = "Execution Queue"
	scanOverviewList.SetShowStatusBar(cfg.UI.Components.List.ShowStatusBar)
	scanOverviewList.SetShowPagination(false)
	scanOverviewList.SetFilteringEnabled(cfg.UI.Components.List.FilteringEnabled)
	// Remove background from title
	scanOverviewList.Styles.Title = scanOverviewList.Styles.Title.Background(lipgloss.NoColor{})

	// Session persistence removed - fresh start each time
	
	// Initialize target input (fresh session each time)
	ti := textinput.New()
	ti.Placeholder = "Enter IP, hostname, or CIDR (e.g., 192.168.1.0/30)"
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 60

	m := &model{
		config:            cfg,
		workflows:         workflows,
		// No session persistence - fresh start each time
		showTargetModal:   true, // Always show modal on startup
		targetInput:       ti,
		scanTarget:        "", // Empty target for fresh start
		workflowTreeList:  workflowTreeList,
		scanOverviewList:  scanOverviewList,
		selectedWorkflows: make(map[string]bool),
		executedWorkflows: make(map[string]bool),
		executionQueue:    []string{},
		outputViewport:    liveOutputVp,
		liveOutput:        liveOutputLines,
		logsViewport:      logsVp,
		systemLogs:        systemLogLines,
		spinner:           s,
		tools:             []toolExecution{},
		perfData:          systemMetrics{},

		// Box card styles using config colors with extra rounded borders
		cardStyle: lipgloss.NewStyle().
			Border(lipgloss.Border{
				Top:         "─",
				Bottom:      "─",
				Left:        "│",
				Right:       "│",
				TopLeft:     "╭",
				TopRight:    "╮",
				BottomLeft:  "╰",
				BottomRight: "╯",
			}).
			// Card border styling
			BorderForeground(lipgloss.Color(cfg.UI.Theme.Colors["border"])).
			Padding(0, 1),
		focusedCardStyle: lipgloss.NewStyle().
			Border(lipgloss.Border{
				Top:         "═",
				Bottom:      "═",
				Left:        "║",
				Right:       "║",
				TopLeft:     "╔",
				TopRight:    "╗",
				BottomLeft:  "╚",
				BottomRight: "╝",
			}).
			// Focused card border styling
			BorderForeground(lipgloss.Color(cfg.UI.Theme.Colors["focused"])). // Configurable focused color
			Padding(0, 1),
		titleStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cfg.UI.Theme.Colors["accent"])).
			Bold(true).
			Align(lipgloss.Center),
		headerStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cfg.UI.Theme.Colors["secondary"])).
			Align(lipgloss.Right),
		dimStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cfg.UI.Theme.Colors["secondary"])),
	}

	// Initialize system metrics with zero timestamp to force immediate first update
	m.perfData.LastUpdate = time.Time{} // Zero time ensures immediate first update

	return m
}

// updateSystemMetricsAsync starts a background goroutine to collect system metrics
func (m *model) updateSystemMetricsAsync() tea.Cmd {
	return func() tea.Msg {
		// Create a copy of current metrics to work with
		newMetrics := m.perfData

		// CPU information (use current CPU times for calculation)
		cpuTimes, err := cpu.Times(false)
		if err == nil && len(cpuTimes) > 0 {
			// Calculate CPU usage from times if we have previous data
			if newMetrics.LastUpdate.IsZero() {
				// First time - just store the times, usage will be 0
				newMetrics.CPUPercent = 0
			} else {
				// Calculate usage based on time differences
				currentTime := cpuTimes[0]
				timeDelta := time.Since(newMetrics.LastUpdate).Seconds()
				
				if timeDelta > 0 && newMetrics.lastCPUTimes != nil {
					prevTime := *newMetrics.lastCPUTimes
					
					totalDelta := (currentTime.User + currentTime.System + currentTime.Nice + 
								  currentTime.Iowait + currentTime.Irq + currentTime.Softirq + 
								  currentTime.Steal + currentTime.Idle) - 
								 (prevTime.User + prevTime.System + prevTime.Nice + 
								  prevTime.Iowait + prevTime.Irq + prevTime.Softirq + 
								  prevTime.Steal + prevTime.Idle)
					
					idleDelta := currentTime.Idle - prevTime.Idle
					
					if totalDelta > 0 {
						newMetrics.CPUPercent = 100.0 * (1.0 - idleDelta/totalDelta)
					}
				}
			}
			// Store current times for next calculation
			newMetrics.lastCPUTimes = &cpuTimes[0]
		}

		cpuCounts, err := cpu.Counts(true)
		if err == nil {
			newMetrics.CPUCores = cpuCounts
		}

		// Memory information
		memInfo, err := mem.VirtualMemory()
		if err == nil {
			newMetrics.MemoryUsed = memInfo.Used
			newMetrics.MemoryTotal = memInfo.Total
			newMetrics.MemoryPercent = memInfo.UsedPercent
		}

		// Disk information (root filesystem)
		diskInfo, err := disk.Usage("/")
		if err == nil {
			newMetrics.DiskUsed = diskInfo.Used
			newMetrics.DiskTotal = diskInfo.Total
			newMetrics.DiskPercent = diskInfo.UsedPercent
		}

		// Network information (simple rates only)
		netIO, err := netutil.IOCounters(false)
		if err == nil && len(netIO) > 0 {
			newSent := netIO[0].BytesSent
			newRecv := netIO[0].BytesRecv

			// Calculate simple rates if we have previous data and this isn't the first reading
			if !newMetrics.LastUpdate.IsZero() && newMetrics.NetworkSent > 0 && newMetrics.NetworkRecv > 0 {
				timeDiff := time.Since(newMetrics.LastUpdate).Seconds()
				if timeDiff > 0 && newSent >= newMetrics.NetworkSent && newRecv >= newMetrics.NetworkRecv {
					newMetrics.NetworkSentRate = uint64(float64(newSent-newMetrics.NetworkSent) / timeDiff)
					newMetrics.NetworkRecvRate = uint64(float64(newRecv-newMetrics.NetworkRecv) / timeDiff)
				}
			} else {
				// First reading - rates are 0
				newMetrics.NetworkSentRate = 0
				newMetrics.NetworkRecvRate = 0
			}

			newMetrics.NetworkSent = newSent
			newMetrics.NetworkRecv = newRecv
			// Clear history for simpler display (no sparklines)
			newMetrics.NetworkSentHistory = nil
			newMetrics.NetworkRecvHistory = nil
		}

		// Host information
		hostInfo, err := host.Info()
		if err == nil {
			newMetrics.Uptime = hostInfo.Uptime
			newMetrics.Hostname = hostInfo.Hostname
			newMetrics.Platform = hostInfo.Platform
		}

		// Goroutines - establish baseline if not set, then show stable count
		currentGoroutines := runtime.NumGoroutine()
		if newMetrics.BaselineGoroutines == 0 {
			// First measurement - establish baseline (subtract 1 for this goroutine)
			newMetrics.BaselineGoroutines = currentGoroutines - 1
			newMetrics.Goroutines = newMetrics.BaselineGoroutines
		} else {
			// Show stable baseline count (filter out temporary monitoring goroutines)
			newMetrics.Goroutines = newMetrics.BaselineGoroutines
		}
		newMetrics.LastUpdate = time.Now()

		// Metrics collection completed

		return systemMetricsMsg(newMetrics)
	}
}

// updateSystemMetrics collects real system information using gopsutil (DEPRECATED - use async version)
func (m *model) updateSystemMetrics() {
	// CPU information
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err == nil && len(cpuPercent) > 0 {
		m.perfData.CPUPercent = cpuPercent[0]
	}

	cpuCounts, err := cpu.Counts(true)
	if err == nil {
		m.perfData.CPUCores = cpuCounts
	}

	// Memory information
	memInfo, err := mem.VirtualMemory()
	if err == nil {
		m.perfData.MemoryUsed = memInfo.Used
		m.perfData.MemoryTotal = memInfo.Total
		m.perfData.MemoryPercent = memInfo.UsedPercent
	}

	// Disk information (root filesystem)
	diskInfo, err := disk.Usage("/")
	if err == nil {
		m.perfData.DiskUsed = diskInfo.Used
		m.perfData.DiskTotal = diskInfo.Total
		m.perfData.DiskPercent = diskInfo.UsedPercent
	}

	// Network information
	netIO, err := netutil.IOCounters(false)
	if err == nil && len(netIO) > 0 {
		newSent := netIO[0].BytesSent
		newRecv := netIO[0].BytesRecv

		// Calculate rates if we have previous data
		if m.perfData.NetworkSent > 0 && m.perfData.NetworkRecv > 0 {
			timeDiff := time.Since(m.perfData.LastUpdate).Seconds()
			if timeDiff > 0 {
				m.perfData.NetworkSentRate = uint64(float64(newSent-m.perfData.NetworkSent) / timeDiff)
				m.perfData.NetworkRecvRate = uint64(float64(newRecv-m.perfData.NetworkRecv) / timeDiff)

				// Add to history (keep last 30 readings)
				m.perfData.NetworkSentHistory = append(m.perfData.NetworkSentHistory, m.perfData.NetworkSentRate)
				m.perfData.NetworkRecvHistory = append(m.perfData.NetworkRecvHistory, m.perfData.NetworkRecvRate)

				// Trim history to last 30 entries
				if len(m.perfData.NetworkSentHistory) > 30 {
					m.perfData.NetworkSentHistory = m.perfData.NetworkSentHistory[1:]
				}
				if len(m.perfData.NetworkRecvHistory) > 30 {
					m.perfData.NetworkRecvHistory = m.perfData.NetworkRecvHistory[1:]
				}
			}
		}

		m.perfData.NetworkSent = newSent
		m.perfData.NetworkRecv = newRecv
	}

	// Host information
	hostInfo, err := host.Info()
	if err == nil {
		m.perfData.Uptime = hostInfo.Uptime
		m.perfData.Hostname = hostInfo.Hostname
		m.perfData.Platform = hostInfo.Platform
	}

	// Goroutines
	m.perfData.Goroutines = runtime.NumGoroutine()
	m.perfData.LastUpdate = time.Now()
}

// formatBytes converts bytes to human-readable format
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// createProgressBar creates a visual progress bar
func createProgressBar(percent float64, width int, filled, empty string) string {
	if width <= 0 {
		return ""
	}

	filledWidth := int(percent / 100.0 * float64(width))
	if filledWidth > width {
		filledWidth = width
	}

	bar := strings.Repeat(filled, filledWidth) + strings.Repeat(empty, width-filledWidth)
	return bar
}

// smoothHistoryData applies a simple moving average to reduce sparkline flicker
func smoothHistoryData(data []uint64, windowSize int) []uint64 {
	if len(data) < windowSize || windowSize <= 1 {
		return data
	}

	smoothed := make([]uint64, len(data))

	// Copy first few elements as-is
	for i := 0; i < windowSize-1; i++ {
		smoothed[i] = data[i]
	}

	// Apply moving average
	for i := windowSize - 1; i < len(data); i++ {
		var sum uint64
		for j := i - windowSize + 1; j <= i; j++ {
			sum += data[j]
		}
		smoothed[i] = sum / uint64(windowSize)
	}

	return smoothed
}

// createSparkline generates a sparkline from historical data
func createSparkline(data []uint64, width int) string {
	if len(data) == 0 || width <= 0 {
		return strings.Repeat(" ", width)
	}

	// Apply smoothing to reduce flicker (3-point moving average)
	smoothedData := smoothHistoryData(data, 3)

	// Sparkline characters from lowest to highest
	sparkChars := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	// Find min and max values for scaling
	var min, max uint64 = smoothedData[0], smoothedData[0]
	for _, val := range smoothedData {
		if val < min {
			min = val
		}
		if val > max {
			max = val
		}
	}

	// If all values are the same, return flat line
	if max == min {
		return strings.Repeat(string(sparkChars[0]), width)
	}

	// Take the last 'width' data points
	startIdx := 0
	if len(smoothedData) > width {
		startIdx = len(smoothedData) - width
	}

	var result strings.Builder
	for i := startIdx; i < len(smoothedData) && result.Len() < width; i++ {
		// Scale the value to sparkline character range
		normalized := float64(smoothedData[i]-min) / float64(max-min)
		charIndex := int(normalized * float64(len(sparkChars)-1))
		if charIndex >= len(sparkChars) {
			charIndex = len(sparkChars) - 1
		}
		result.WriteRune(sparkChars[charIndex])
	}

	// Pad with spaces if needed
	for result.Len() < width {
		result.WriteRune(' ')
	}

	return result.String()
}

// formatNetworkRate converts bytes per second to human-readable format
func formatNetworkRate(bytesPerSec uint64) string {
	if bytesPerSec == 0 {
		return "0 B/s"
	}

	const unit = 1024
	if bytesPerSec < unit {
		return fmt.Sprintf("%d B/s", bytesPerSec)
	}

	div, exp := uint64(unit), 0
	for n := bytesPerSec / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB/s", float64(bytesPerSec)/float64(div), "KMGTPE"[exp])
}

// smoothInterpolate provides smooth transitions between values using easing
func smoothInterpolate(start, end, progress float64) float64 {
	if progress >= 1.0 {
		return end
	}
	if progress <= 0.0 {
		return start
	}

	// Use easeOutCubic for smooth deceleration
	// Formula: 1 - (1 - t)^3
	easedProgress := 1 - (1-progress)*(1-progress)*(1-progress)
	return start + (end-start)*easedProgress
}

// smoothInterpolateUint64 provides smooth transitions for uint64 values (like network rates)
func smoothInterpolateUint64(start, end uint64, factor float64) uint64 {
	if factor >= 1.0 {
		return end
	}
	if factor <= 0.0 {
		return start
	}

	// Use exponential smoothing for network rates to reduce flicker
	// Formula: newValue = oldValue + factor * (newValue - oldValue)
	diff := int64(end) - int64(start)
	smoothed := int64(start) + int64(float64(diff)*factor)

	if smoothed < 0 {
		return 0
	}
	return uint64(smoothed)
}

// updateAnimatedValues smoothly transitions metrics to new values
func (m *model) updateAnimatedValues() {
	// Animation speed for CPU/Memory/Disk (smooth and stable)
	animationFactor := m.config.UI.Performance.AnimationFactor
	if animationFactor == 0 {
		animationFactor = 0.15 // Default fallback
	}

	// Update animated values with smooth interpolation
	m.perfData.AnimatedCPU = smoothInterpolate(m.perfData.AnimatedCPU, m.perfData.CPUPercent, animationFactor)
	m.perfData.AnimatedMemory = smoothInterpolate(m.perfData.AnimatedMemory, m.perfData.MemoryPercent, animationFactor)
	m.perfData.AnimatedDisk = smoothInterpolate(m.perfData.AnimatedDisk, m.perfData.DiskPercent, animationFactor)

	// Network rates removed for simplicity (no longer animated)
}

// getThemeColor gets a color from the theme config with fallback
func (m *model) getThemeColor(colorName, fallback string) lipgloss.Color {
	if m.config != nil && m.config.UI.Theme.Colors != nil {
		if color, exists := m.config.UI.Theme.Colors[colorName]; exists && color != "" {
			return lipgloss.Color(color)
		}
	}
	return lipgloss.Color(fallback)
}

func (m *model) Init() tea.Cmd {
	// Get refresh interval from config
	refreshMs := m.config.UI.Components.Status.RefreshMs
	if refreshMs == 0 {
		refreshMs = 1000 // Default to 1 second
	}
	refreshInterval := time.Duration(refreshMs) * time.Millisecond

	return tea.Batch(
		m.spinner.Tick,
		textinput.Blink,
		// Dedicated metrics ticker
		tea.Every(refreshInterval, func(t time.Time) tea.Msg {
			return metricsTickMsg{}
		}),
	)
}

// validateTarget validates the input target (IP, hostname, or CIDR)
func (m *model) validateTarget(input string) error {
	input = strings.TrimSpace(input)
	if input == "" {
		return fmt.Errorf("target cannot be empty")
	}

	// Split by comma for multiple targets
	targets := strings.Split(input, ",")
	
	for _, target := range targets {
		target = strings.TrimSpace(target)
		
		// Try to parse as IP address
		if ip := net.ParseIP(target); ip != nil {
			continue // Valid IP
		}
		
		// Try to parse as CIDR
		if _, _, err := net.ParseCIDR(target); err == nil {
			continue // Valid CIDR
		}
		
		// Try to resolve as hostname
		if _, err := net.LookupHost(target); err == nil {
			continue // Valid hostname
		}
		
		// Check if it looks like a hostname (basic validation)
		if isValidHostname(target) {
			continue // Likely valid hostname (will be resolved at execution time)
		}
		
		return fmt.Errorf("invalid target: %s (must be IP, hostname, or CIDR)", target)
	}
	
	return nil
}

// isValidHostname performs basic hostname validation
func isValidHostname(hostname string) bool {
	// Basic hostname validation
	if len(hostname) > 253 {
		return false
	}
	
	// Must contain only valid characters
	for _, r := range hostname {
		if !((r >= 'a' && r <= 'z') || 
			 (r >= 'A' && r <= 'Z') || 
			 (r >= '0' && r <= '9') || 
			 r == '.' || r == '-') {
			return false
		}
	}
	
	// Must not start or end with dot or hyphen
	if strings.HasPrefix(hostname, ".") || strings.HasPrefix(hostname, "-") ||
		strings.HasSuffix(hostname, ".") || strings.HasSuffix(hostname, "-") {
		return false
	}
	
	return true
}

// sanitizeTargetForPath converts a target (IP, hostname, CIDR) to a safe directory name
func (m *model) sanitizeTargetForPath(target string) string {
	// Handle multiple targets (take first one for main directory)
	targets := strings.Split(target, ",")
	if len(targets) > 0 {
		target = strings.TrimSpace(targets[0])
	}
	
	// Replace problematic characters for filesystem
	sanitized := target
	sanitized = strings.ReplaceAll(sanitized, "/", "_")  // CIDR notation
	sanitized = strings.ReplaceAll(sanitized, ":", "_")  // IPv6
	sanitized = strings.ReplaceAll(sanitized, " ", "_")  // Spaces
	sanitized = strings.ReplaceAll(sanitized, "..", "_") // Double dots
	
	// Ensure it's not empty
	if sanitized == "" {
		sanitized = "unknown_target"
	}
	
	return sanitized
}

// getProjectDirectory returns the directory where the project files are located
func getProjectDirectory() (string, error) {
	// Try to get executable directory first (for built binaries)
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		// Check if this looks like a project directory by looking for key files
		if _, err := os.Stat(filepath.Join(execDir, "go.mod")); err == nil {
			return execDir, nil
		}
		// If go.mod not found, try parent directory (common when binary is in bin/)
		parentDir := filepath.Dir(execDir)
		if _, err := os.Stat(filepath.Join(parentDir, "go.mod")); err == nil {
			return parentDir, nil
		}
	}
	
	// Fallback: try current working directory
	if cwd, err := os.Getwd(); err == nil {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return cwd, nil
		}
	}
	
	// Last resort: use current working directory anyway
	return os.Getwd()
}

// createWorkspaceStructure creates the complete workspace directory structure with all subdirectories and initial files
func (m *model) createWorkspaceStructure(workspacePath string) error {
	// Get the project directory (where go.mod, Makefile, configs are)
	projectDir, err := getProjectDirectory()
	if err != nil {
		return fmt.Errorf("failed to determine project directory: %w", err)
	}
	
	// Create absolute workspace path in the project directory
	absoluteWorkspacePath := filepath.Join(projectDir, workspacePath)
	
	// Create main workspace directory in project directory
	if err := os.MkdirAll(absoluteWorkspacePath, 0755); err != nil {
		return fmt.Errorf("failed to create workspace directory %s: %w", absoluteWorkspacePath, err)
	}
	
	// Define directory structure
	directories := []string{
		"logs/info",
		"logs/error",
		"logs/warning",
		"logs/debug",
		"scans",
		"reports",
		"raw",
	}
	
	// Create all subdirectories
	for _, dir := range directories {
		dirPath := filepath.Join(absoluteWorkspacePath, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			m.logSystemMessage("warn", "Failed to create directory", "path", dirPath, "err", err.Error())
		}
	}
	
	// Create README.md
	readmePath := filepath.Join(absoluteWorkspacePath, "README.md")
	readmeContent := fmt.Sprintf(`# IPCrawler Workspace

## Target Information
- **Target**: %s
- **Created**: %s
- **Session ID**: %s

## Directory Structure
- **logs/** - Application and tool logs
  - **info/** - Informational messages
  - **error/** - Error logs
  - **warning/** - Warning messages
  - **debug/** - Debug information
- **scans/** - Tool scan results
- **reports/** - Generated reports and summaries  
- **raw/** - Raw tool output

## Usage
All scan outputs for this target will be stored in this workspace.
Each tool execution will create timestamped files in the appropriate directories.

## Template Variables
When tools are executed, these paths are available:
- {{workspace}} - This directory
- {{logs_dir}} - logs/
- {{scans_dir}} - scans/
- {{reports_dir}} - reports/
- {{raw_dir}} - raw/
`, m.scanTarget, time.Now().Format("2006-01-02 15:04:05"), m.getSessionID())
	
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
		m.logSystemMessage("warn", "Failed to create README", "err", err.Error())
	}
	
	// Create session_info.json
	sessionPath := filepath.Join(absoluteWorkspacePath, "session_info.json")
	sessionInfo := map[string]interface{}{
		"target":      m.scanTarget,
		"workspace":   absoluteWorkspacePath,
		"created":     time.Now().Format(time.RFC3339),
		"session_id":  m.getSessionID(),
		"directories": directories,
	}
	
	if data, err := json.MarshalIndent(sessionInfo, "", "  "); err == nil {
		if err := os.WriteFile(sessionPath, data, 0644); err != nil {
			m.logSystemMessage("warn", "Failed to create session info", "err", err.Error())
		} else {
			m.logSystemMessage("info", "Created session_info.json", "path", sessionPath)
		}
	}
	
	m.logSystemMessage("info", "Workspace structure created", 
		"workspace", absoluteWorkspacePath,
		"directories", len(directories),
		"target", m.scanTarget)
	
	return nil
}

// getSessionID returns the current session ID or generates a new one
func (m *model) getSessionID() string {
	// No session persistence - return empty session ID
	return fmt.Sprintf("session_%d", time.Now().Unix())
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Handle target modal input first if it's showing
	if m.showTargetModal {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				// Validate and accept target
				inputValue := strings.TrimSpace(m.targetInput.Value())
				if err := m.validateTarget(inputValue); err != nil {
					m.targetError = err.Error()
				} else {
					m.scanTarget = inputValue
					m.showTargetModal = false
					m.targetError = ""
					m.focus = workflowTreeFocus // Set initial focus after modal closes
					
					// Create workspace/target directory structure immediately
					sanitizedTarget := m.sanitizeTargetForPath(m.scanTarget)
					// Use workspace_base from configuration (no hardcoded paths)
					outputDir := filepath.Join(m.config.Output.WorkspaceBase, sanitizedTarget)
					
					if err := m.createWorkspaceStructure(outputDir); err != nil {
						m.logSystemMessage("error", "Failed to create workspace", "err", err.Error())
					} else {
						m.logSystemMessage("info", "Workspace initialized successfully", "path", outputDir)
					}
					
					m.logSystemMessage("info", "Target configured", "target", m.scanTarget, "workspace", outputDir)
				}
				return m, nil
			case "esc":
				// Exit the application if no target is set
				if m.scanTarget == "" {
					return m, tea.Quit
				}
				m.showTargetModal = false
				return m, nil
			case "ctrl+c":
				return m, tea.Quit
			default:
				// Update text input
				m.targetInput, cmd = m.targetInput.Update(msg)
				m.targetError = "" // Clear error on new input
				return m, cmd
			}
		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
			m.updateSizes()
			if !m.ready {
				m.ready = true
			}
			return m, nil
		case metricsTickMsg, systemMetricsMsg, spinner.TickMsg:
			// Allow system messages through even when modal is showing
			// Fall through to main message processing
		default:
			// For other messages while modal is showing, return early
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSizes()
		if !m.ready {
			m.ready = true
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "tab":
			m.focus = focusState((int(m.focus) + 1) % 6)
		case "shift+tab":
			m.focus = focusState((int(m.focus) + 5) % 6)
		case "1":
			m.focus = workflowTreeFocus
		case "2":
			m.focus = scanOverviewFocus
		case "3":
			m.focus = outputFocus
		case "4":
			m.focus = logsFocus
		case "5":
			m.focus = toolsFocus
		case "6":
			m.focus = perfFocus
		case " ":
			// Handle workflow selection/deselection with spacebar
			if m.focus == workflowTreeFocus {
				// Move from workflow tree to execution queue
				selectedItem := m.workflowTreeList.SelectedItem()
				if workflowItem, ok := selectedItem.(workflowItem); ok {
					m.selectedWorkflows[workflowItem.name] = true
					m.updateWorkflowList()
					m.updateExecutionQueue()

					// Log workflow selection and update logs viewport
					selectedCount := 0
					for _, selected := range m.selectedWorkflows {
						if selected {
							selectedCount++
						}
					}
					m.logSystemMessage("info", "Workflow added to queue", "workflow", workflowItem.name, "total_queued", selectedCount)
				}
			} else if m.focus == scanOverviewFocus {
				// Move from execution queue back to workflow tree
				selectedItem := m.scanOverviewList.SelectedItem()
				if executionItem, ok := selectedItem.(executionItem); ok {
					// Only deselect if it's an actual workflow (not summary items)
					if _, exists := m.workflows.Workflows[executionItem.name]; exists {
						m.selectedWorkflows[executionItem.name] = false
						m.updateWorkflowList()
						m.updateExecutionQueue()

						// Log workflow deselection
						selectedCount := 0
						for _, selected := range m.selectedWorkflows {
							if selected {
								selectedCount++
							}
						}
						m.logSystemMessage("info", "Workflow removed from queue", "workflow", executionItem.name, "total_queued", selectedCount)
					}
				}
			}
		case "enter":
			// Execute selected workflows from either card
			hasSelectedWorkflows := false
			for _, selected := range m.selectedWorkflows {
				if selected {
					hasSelectedWorkflows = true
					break
				}
			}
			if (m.focus == workflowTreeFocus || m.focus == scanOverviewFocus) && hasSelectedWorkflows {
				m.executeSelectedWorkflows()
			}
		case "up":
			// Handle faster scrolling for viewport windows
			switch m.focus {
			case outputFocus:
				m.outputViewport.ScrollUp(3) // Scroll 3 lines at a time for faster navigation
			case logsFocus:
				m.logsViewport.ScrollUp(3) // Scroll 3 lines at a time for faster navigation
			default:
				// Pass to default handlers for other components
				switch m.focus {
				case workflowTreeFocus:
					m.workflowTreeList, cmd = m.workflowTreeList.Update(msg)
					cmds = append(cmds, cmd)
				case scanOverviewFocus:
					m.scanOverviewList, cmd = m.scanOverviewList.Update(msg)
					cmds = append(cmds, cmd)
				}
			}
		case "down":
			// Handle faster scrolling for viewport windows
			switch m.focus {
			case outputFocus:
				m.outputViewport.ScrollDown(3) // Scroll 3 lines at a time for faster navigation
			case logsFocus:
				m.logsViewport.ScrollDown(3) // Scroll 3 lines at a time for faster navigation
			default:
				// Pass to default handlers for other components
				switch m.focus {
				case workflowTreeFocus:
					m.workflowTreeList, cmd = m.workflowTreeList.Update(msg)
					cmds = append(cmds, cmd)
				case scanOverviewFocus:
					m.scanOverviewList, cmd = m.scanOverviewList.Update(msg)
					cmds = append(cmds, cmd)
				}
			}
		default:
			// Handle focused component input for other keys
			switch m.focus {
			case workflowTreeFocus:
				m.workflowTreeList, cmd = m.workflowTreeList.Update(msg)
				cmds = append(cmds, cmd)
			case scanOverviewFocus:
				m.scanOverviewList, cmd = m.scanOverviewList.Update(msg)
				cmds = append(cmds, cmd)
			case outputFocus:
				// Only pass non-arrow keys to viewport
				if msg.String() != "up" && msg.String() != "down" {
					m.outputViewport, cmd = m.outputViewport.Update(msg)
					cmds = append(cmds, cmd)
				}
			case logsFocus:
				// Only pass non-arrow keys to viewport
				if msg.String() != "up" && msg.String() != "down" {
					m.logsViewport, cmd = m.logsViewport.Update(msg)
					cmds = append(cmds, cmd)
				}
			}
		}

	case systemMetricsMsg:
		// Received updated system metrics from background goroutine
		newMetrics := systemMetrics(msg)
		
		// Debug: Log that we received the metrics update
		m.logSystemMessage("debug", "Received system metrics update", 
			"cpu", fmt.Sprintf("%.1f%%", newMetrics.CPUPercent),
			"memory", fmt.Sprintf("%.1f%%", newMetrics.MemoryPercent),
			"timestamp", newMetrics.LastUpdate.Format("15:04:05"))

		// Initialize animated values if this is the first update
		if m.perfData.AnimationStartTime.IsZero() {
			newMetrics.AnimatedCPU = newMetrics.CPUPercent
			newMetrics.AnimatedMemory = newMetrics.MemoryPercent
			newMetrics.AnimatedDisk = newMetrics.DiskPercent
		} else {
			// Preserve current animated values for smooth transition
			newMetrics.AnimatedCPU = m.perfData.AnimatedCPU
			newMetrics.AnimatedMemory = m.perfData.AnimatedMemory
			newMetrics.AnimatedDisk = m.perfData.AnimatedDisk
			// Preserve baseline goroutine count
			if newMetrics.BaselineGoroutines == 0 && m.perfData.BaselineGoroutines > 0 {
				newMetrics.BaselineGoroutines = m.perfData.BaselineGoroutines
				newMetrics.Goroutines = m.perfData.BaselineGoroutines
			}
		}
		newMetrics.AnimationStartTime = time.Now()
		m.perfData = newMetrics

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case metricsTickMsg:
		// Trigger metrics update and return next tick command
		refreshMs := m.config.UI.Components.Status.RefreshMs
		if refreshMs == 0 {
			refreshMs = 1000 // Default to 1 second
		}
		refreshInterval := time.Duration(refreshMs) * time.Millisecond
		
		cmds = append(cmds, m.updateSystemMetricsAsync())
		cmds = append(cmds, tea.Every(refreshInterval, func(t time.Time) tea.Msg {
			return metricsTickMsg{}
		}))
	}

	// Update animated values for smooth transitions on every update cycle
	m.updateAnimatedValues()

	// Simulate tool execution progress - moved outside switch to run on every update
	if len(m.tools) > 0 {
		// Find current running tool and advance it
		for i := range m.tools {
			if m.tools[i].Status == "running" {
				// Randomly complete tools after some ticks (simulation)
				if time.Now().UnixNano()%7 == 0 { // Random completion
					m.tools[i].Status = "done"
					// Find next pending tool and start it
					foundNext := false
					for j := i + 1; j < len(m.tools); j++ {
						if m.tools[j].Status == "pending" {
							m.tools[j].Status = "running"
							// Add to live output
							timeFormat := m.config.UI.Formatting.TimeFormat
							if timeFormat == "" {
								timeFormat = "15:04:05" // Fallback
							}
							m.liveOutput = append(m.liveOutput,
								fmt.Sprintf("[%s] Starting %s", time.Now().Format(timeFormat), m.tools[j].Name))
							m.outputViewport.SetContent(strings.Join(m.liveOutput, "\n"))
							m.outputViewport.GotoBottom()
							foundNext = true
							break
						}
					}
					// Add completion to live output
					timeFormat := m.config.UI.Formatting.TimeFormat
					if timeFormat == "" {
						timeFormat = "15:04:05" // Fallback
					}
					m.liveOutput = append(m.liveOutput,
						fmt.Sprintf("[%s] Completed %s", time.Now().Format(timeFormat), m.tools[i].Name))
					m.outputViewport.SetContent(strings.Join(m.liveOutput, "\n"))
					m.outputViewport.GotoBottom()

					// Check if all tools are done
					if !foundNext {
						allDone := true
						for _, tool := range m.tools {
							if tool.Status == "pending" || tool.Status == "running" {
								allDone = false
								break
							}
						}

						if allDone {
							// Mark workflows as executed and move them back
							m.completeWorkflowExecution()
						}
					}
				}
				break
			}
		}
	}


	return m, tea.Batch(cmds...)
}

// renderTargetModal renders the target input modal
func (m *model) renderTargetModal() string {
	// Modal styles
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.config.UI.Theme.Colors["accent"])).
		Padding(2, 4).
		Width(70).
		Align(lipgloss.Center)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.config.UI.Theme.Colors["primary"])).
		Bold(true).
		MarginBottom(1)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.config.UI.Theme.Colors["secondary"])).
		Italic(true).
		MarginTop(1).
		MarginBottom(1)

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.config.UI.Theme.Colors["error"])).
		Bold(true).
		MarginTop(1)

	instructionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.config.UI.Theme.Colors["info"])).
		MarginTop(2)

	// Build modal content
	var content strings.Builder
	
	content.WriteString(titleStyle.Render("Target Configuration"))
	content.WriteString("\n\n")
	
	if m.targetInput.Value() != "" {
		content.WriteString(helpStyle.Render("Previous target loaded. Press Enter to use or modify:"))
	} else {
		content.WriteString(helpStyle.Render("Enter the target for scanning:"))
	}
	content.WriteString("\n\n")
	
	content.WriteString(m.targetInput.View())
	content.WriteString("\n")
	
	if m.targetError != "" {
		content.WriteString("\n")
		content.WriteString(errorStyle.Render("⚠ " + m.targetError))
	}
	
	content.WriteString("\n")
	content.WriteString(instructionStyle.Render("Press Enter to confirm, Esc to cancel"))
	content.WriteString("\n\n")
	content.WriteString(helpStyle.Render("Supported formats:"))
	content.WriteString("\n")
	content.WriteString(helpStyle.Render("• IP Address: 192.168.1.1"))
	content.WriteString("\n")
	content.WriteString(helpStyle.Render("• Hostname: example.com"))
	content.WriteString("\n")
	content.WriteString(helpStyle.Render("• CIDR: 192.168.1.0/24"))
	content.WriteString("\n")
	content.WriteString(helpStyle.Render("• Multiple targets: 192.168.1.1, example.com"))

	// Center the modal
	modal := modalStyle.Render(content.String())
	
	// Place modal in center of screen
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		modal,
		lipgloss.WithWhitespaceChars("░"),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#333333")),
	)
}

func (m *model) View() string {
	if !m.ready || m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	// Show modal if active
	if m.showTargetModal {
		return m.renderTargetModal()
	}

	// Create stylized title with enhanced formatting
	titleText := "IPCrawler"
	subtitleText := "smart automatic reconnaissance"

	// Main title with gradient-like styling
	// Main title styling
	mainTitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.config.UI.Theme.Colors["info"])).
		Bold(true).
		Render(titleText)

	// Subtitle styling
	subtitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.config.UI.Theme.Colors["timestamp"])).
		Italic(true).
		Render(subtitleText)

	// Separator styling
	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color(m.config.UI.Theme.Colors["secondary"])).
		Render(" • ")

	// Full title line
	fullTitle := lipgloss.JoinHorizontal(lipgloss.Left, mainTitle, separator, subtitle)
	
	// Add target display if configured
	var titleWithTarget string
	if m.scanTarget != "" {
		targetStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.config.UI.Theme.Colors["success"])).
			Bold(true)
		targetLine := targetStyle.Render(fmt.Sprintf("Target: %s", m.scanTarget))
		titleWithTarget = lipgloss.JoinVertical(lipgloss.Center, fullTitle, targetLine)
	} else {
		titleWithTarget = fullTitle
	}
	
	title := lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(titleWithTarget)

	// Create help with focus-specific instructions
	var helpText string
	selectedCount := 0
	for _, selected := range m.selectedWorkflows {
		if selected {
			selectedCount++
		}
	}

	switch m.focus {
	case workflowTreeFocus:
		if selectedCount > 0 {
			helpText = fmt.Sprintf("SPACE: add to queue • ENTER: execute %d queued • ↑↓: navigate • Tab: view queue", selectedCount)
		} else {
			helpText = "SPACE: add to queue • ↑↓: navigate • Tab: view queue • q: quit"
		}
	case scanOverviewFocus:
		if selectedCount > 0 {
			helpText = fmt.Sprintf("SPACE: remove from queue • ENTER: execute %d queued • ↑↓: navigate • Tab: add more", selectedCount)
		} else {
			helpText = "No workflows queued • Tab: go back to select workflows • q: quit"
		}
	case outputFocus:
		helpText = "Tab/Shift+Tab: cycle focus • 1-6: direct focus • ↑↓: scroll live output (3x speed) • q: quit"
	case logsFocus:
		helpText = "Tab/Shift+Tab: cycle focus • 1-6: direct focus • ↑↓: scroll system logs (3x speed) • q: quit"
	default:
		helpText = "Tab/Shift+Tab: cycle focus • 1-6: direct focus • Arrow keys: navigate • q: quit"
	}
	help := m.dimStyle.Render(helpText)
	help = lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(help)

	// Render cards with proper list components
	workflowSelection := m.renderWorkflowTreeCard()
	executionQueue := m.renderScanOverviewCard()
	tools := m.renderToolsCard()
	perf := m.renderPerfCard()
	gap := strings.Repeat(" ", m.config.UI.Layout.Cards.Spacing)
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, workflowSelection, gap, executionQueue, gap, tools, gap, perf)

	// Bottom Row: Live Output and Logs (side by side)
	liveOutput := m.renderOutputCard()
	logs := m.renderLogsCard()
	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, liveOutput, gap, logs)

	// Combine all
	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		topRow,
		"",
		bottomRow,
		"",
		help,
	)

	return content
}

func (m *model) renderWorkflowTreeCard() string {
	style := m.cardStyle
	// Card title styling
	titleColor := m.getThemeColor("secondary", "240")
	if m.focus == workflowTreeFocus {
		style = m.focusedCardStyle
		titleColor = m.getThemeColor("focused", "86") // Bright cyan when focused
	}

	// Update list title color dynamically
	m.workflowTreeList.Styles.Title = m.workflowTreeList.Styles.Title.Foreground(titleColor)

	// Use the list component's view
	return style.Width(m.cardWidth).Height(m.cardHeight).Render(m.workflowTreeList.View())
}

func (m *model) renderScanOverviewCard() string {
	style := m.cardStyle
	// Card title styling
	titleColor := m.getThemeColor("secondary", "240")
	if m.focus == scanOverviewFocus {
		style = m.focusedCardStyle
		titleColor = m.getThemeColor("focused", "86") // Bright cyan when focused
	}

	// Update list title color dynamically
	m.scanOverviewList.Styles.Title = m.scanOverviewList.Styles.Title.Foreground(titleColor)

	// Use the list component's view
	return style.Width(m.cardWidth).Height(m.cardHeight).Render(m.scanOverviewList.View())
}

func (m *model) renderOutputCard() string {
	style := m.cardStyle
	if m.focus == outputFocus {
		style = m.focusedCardStyle
	}

	// Calculate width for side-by-side layout based on top-row card width to avoid rounding drift
	gapWidth := m.config.UI.Layout.Cards.Spacing
	// Set bottom-left content width so its TOTAL width equals: card1 + gap + card2 (top row)
	// Each card total = content(m.cardWidth) + 4 (borders+padding)
	// So left total = 2*(m.cardWidth+4) + gapWidth → content width = left total - 4
	outputWidth := 2*m.cardWidth + gapWidth + 2
	// Width is derived from top-row geometry to maintain perfect alignment

	// Card header with title and scroll info
	titleText := "Live Output"
	if m.focus == outputFocus {
		titleText = "Live Output (FOCUSED - Use ↑↓ to scroll)"
	}
	title := m.titleStyle.Render(titleText)
	scrollInfo := m.headerStyle.Render(fmt.Sprintf("%.1f%%", m.outputViewport.ScrollPercent()*100))
	// Inner content width equals card width minus 2 border columns
	titleWidth := outputWidth - 2
	header := lipgloss.JoinHorizontal(lipgloss.Left, title,
		strings.Repeat(" ", titleWidth-lipgloss.Width(title)-lipgloss.Width(scrollInfo)), scrollInfo)

	// Card content
	content := m.outputViewport.View()

	// Combine header and content
	cardContent := lipgloss.JoinVertical(lipgloss.Left,
		header,
		strings.Repeat("─", titleWidth),
		content,
	)

	return style.Width(outputWidth).Height(m.cardHeight).Render(cardContent)
}

func (m *model) renderLogsCard() string {
	style := m.cardStyle
	if m.focus == logsFocus {
		style = m.focusedCardStyle
	}

	// Calculate complementary width to the left card so seams align with the top row
	gapWidth := m.config.UI.Layout.Cards.Spacing
	// Mirror the same calculation for the right card to keep totals consistent
	// Right content width = 2*(m.cardWidth+4) + gapWidth - 4
	logsWidth := 2*m.cardWidth + gapWidth + 2

	// Card header with title and scroll info
	titleText := "Logs"
	if m.focus == logsFocus {
		titleText = "Logs (FOCUSED - Use ↑↓ to scroll)"
	}
	title := m.titleStyle.Render(titleText)
	scrollInfo := m.headerStyle.Render(fmt.Sprintf("%.1f%%", m.logsViewport.ScrollPercent()*100))
	// Inner content width equals card width minus 2 border columns
	titleWidth := logsWidth - 2
	header := lipgloss.JoinHorizontal(lipgloss.Left, title,
		strings.Repeat(" ", titleWidth-lipgloss.Width(title)-lipgloss.Width(scrollInfo)), scrollInfo)

	// Card content
	content := m.logsViewport.View()

	// Combine header and content
	cardContent := lipgloss.JoinVertical(lipgloss.Left,
		header,
		strings.Repeat("─", titleWidth),
		content,
	)

	return style.Width(logsWidth).Height(m.cardHeight).Render(cardContent)
}

func (m *model) renderToolsCard() string {
	style := m.cardStyle
	if m.focus == toolsFocus {
		style = m.focusedCardStyle
	}

	// Card header with title
	// Card title styling
	titleColor := m.getThemeColor("secondary", "240")
	if m.focus == toolsFocus {
		titleColor = m.getThemeColor("focused", "86") // Bright cyan when focused
	}
	title := m.titleStyle.Foreground(titleColor).Width(m.cardWidth - 2).Render("Executing")

	// Card content
	content := strings.Builder{}
	if len(m.tools) == 0 {
		content.WriteString("No workflows executing\n")
		content.WriteString("Select workflows and press Enter to execute")
	} else {
		for _, tool := range m.tools {
			if tool.Status == "header" {
				// Workflow header - show in bold/different style
				content.WriteString(fmt.Sprintf("\n%s\n", tool.Name))
			} else {
				// Tool entry with status indicator
				status := "[DONE]"
				// Tool status styling
				statusColor := m.getThemeColor("completed", "86")

				if tool.Status == "running" {
					status = "[RUN] " + m.spinner.View()
					statusColor = m.getThemeColor("running", "214")
				} else if tool.Status == "pending" {
					status = "[WAIT]"
					statusColor = m.getThemeColor("pending", "240")
				} else if tool.Status == "failed" {
					status = "[FAIL]"
					statusColor = m.getThemeColor("failed", "196")
				}

				statusStyled := lipgloss.NewStyle().Foreground(statusColor).Render(status)
				content.WriteString(fmt.Sprintf("%s %s\n", statusStyled, tool.Name))
			}
		}
	}

	// Combine title and content
	cardContent := lipgloss.JoinVertical(lipgloss.Left,
		title,
		strings.Repeat("─", m.cardWidth-2),
		content.String(),
	)

	return style.Width(m.cardWidth).Height(m.cardHeight).Render(cardContent)
}

func (m *model) renderPerfCard() string {
	style := m.cardStyle
	// Card title styling
	titleColor := m.getThemeColor("secondary", "240")
	if m.focus == perfFocus {
		style = m.focusedCardStyle
		titleColor = m.getThemeColor("focused", "86") // Bright cyan when focused
	}

	// Card header with title
	title := m.titleStyle.Foreground(titleColor).Width(m.cardWidth - 2).Render("System Monitor")

	// System monitoring styling
	cpuStyle := lipgloss.NewStyle().Foreground(m.getThemeColor("cpu", "214"))
	memStyle := lipgloss.NewStyle().Foreground(m.getThemeColor("memory", "39"))
	diskStyle := lipgloss.NewStyle().Foreground(m.getThemeColor("disk", "120"))
	netStyle := lipgloss.NewStyle().Foreground(m.getThemeColor("network", "201"))
	infoStyle := lipgloss.NewStyle().Foreground(m.getThemeColor("secondary", "243"))

	// Progress bar width (adjust based on card width)
	barWidth := m.cardWidth - 12 // Leave space for labels and percentages
	if barWidth < 8 {
		barWidth = 8
	}

	// Create progress bars with colors using animated values for smooth transitions
	cpuBar := createProgressBar(m.perfData.AnimatedCPU, barWidth, "█", "░")
	memBar := createProgressBar(m.perfData.AnimatedMemory, barWidth, "█", "░")
	diskBar := createProgressBar(m.perfData.AnimatedDisk, barWidth, "█", "░")

	// Format uptime
	uptimeHours := m.perfData.Uptime / 3600
	uptimeMinutes := (m.perfData.Uptime % 3600) / 60

	// Build content with visual elements
	var contentLines []string

	// CPU section with smooth animated values
	contentLines = append(contentLines,
		cpuStyle.Render(fmt.Sprintf("CPU (%d cores)", m.perfData.CPUCores)),
		fmt.Sprintf("%s %5.1f%%", cpuStyle.Render(cpuBar), m.perfData.AnimatedCPU),
		"",
	)

	// Memory section with smooth animated values
	contentLines = append(contentLines,
		memStyle.Render("Memory"),
		fmt.Sprintf("%s %5.1f%%", memStyle.Render(memBar), m.perfData.AnimatedMemory),
		infoStyle.Render(fmt.Sprintf("%s / %s", formatBytes(m.perfData.MemoryUsed), formatBytes(m.perfData.MemoryTotal))),
		"",
	)

	// Disk section with smooth animated values
	contentLines = append(contentLines,
		diskStyle.Render("Disk"),
		fmt.Sprintf("%s %5.1f%%", diskStyle.Render(diskBar), m.perfData.AnimatedDisk),
		infoStyle.Render(fmt.Sprintf("%s / %s", formatBytes(m.perfData.DiskUsed), formatBytes(m.perfData.DiskTotal))),
		"",
	)

	// Network section (current rates)
	contentLines = append(contentLines,
		netStyle.Render("Network"),
		infoStyle.Render(fmt.Sprintf("UP:   %s", formatNetworkRate(m.perfData.NetworkSentRate))),
		infoStyle.Render(fmt.Sprintf("DOWN: %s", formatNetworkRate(m.perfData.NetworkRecvRate))),
		"",
	)

	// System info
	contentLines = append(contentLines,
		infoStyle.Render(fmt.Sprintf("Host: %s", m.perfData.Hostname)),
		infoStyle.Render(fmt.Sprintf("OS: %s", m.perfData.Platform)),
		infoStyle.Render(fmt.Sprintf("Uptime: %dh %dm", uptimeHours, uptimeMinutes)),
		infoStyle.Render(fmt.Sprintf("Goroutines: %d", m.perfData.Goroutines)),
	)

	content := strings.Join(contentLines, "\n")

	// Combine title and content
	cardContent := lipgloss.JoinVertical(lipgloss.Left,
		title,
		strings.Repeat("─", m.cardWidth-2),
		content,
	)

	return style.Width(m.cardWidth).Height(m.cardHeight).Render(cardContent)
}

func (m *model) updateSizes() {
	if m.width <= 10 || m.height <= 10 {
		return
	}

	// Calculate card dimensions using config values
	cards := m.config.UI.Layout.Cards
	scrollBarSpace := cards.ScrollBarSpace
	spacing := cards.Spacing
	columns := cards.Columns
	rows := cards.Rows
	verticalOffset := cards.VerticalOffset

	// Guard against invalid or zero values from config
	if columns <= 0 {
		columns = 1
	}
	if rows <= 0 {
		rows = 1
	}
	if spacing < 0 {
		spacing = 0
	}
	if scrollBarSpace < 0 {
		scrollBarSpace = 0
	}
	if verticalOffset < 0 {
		verticalOffset = 0
	}

	// Compute available space safely (account for gaps between columns)
	// For 4 cards with 2-space gaps: need 3 gaps * 2 spaces = 6 total spacing
	totalSpacing := spacing * (columns - 1)
	// Account for per-card horizontal chrome: 1 left pad + 1 right pad + 1 left border + 1 right border = 4
	perCardChrome := 4
	availableWidth := m.width - scrollBarSpace - totalSpacing - (columns * perCardChrome)
	availableHeight := m.height - verticalOffset

	// Ensure we have at least something to work with
	if availableWidth < columns*cards.MinWidth {
		availableWidth = columns * cards.MinWidth
	}
	if availableHeight < rows*cards.MinHeight {
		availableHeight = rows * cards.MinHeight
	}

	// Use config-driven layout calculations with guards
	m.cardWidth = availableWidth / columns
	// Ensure non-negative after chrome subtraction
	if m.cardWidth < 1 {
		m.cardWidth = 1
	}
	m.cardHeight = availableHeight / rows

	// Ensure reasonable minimums from config
	if m.cardWidth < cards.MinWidth {
		m.cardWidth = cards.MinWidth
	}
	if m.cardHeight < cards.MinHeight {
		m.cardHeight = cards.MinHeight
	}

	// Update list sizes using config values
	listConfig := m.config.UI.Components.List
	listWidth := m.cardWidth - listConfig.BorderPadding
	listHeight := m.cardHeight - listConfig.BorderPadding

	if listWidth > 0 && listHeight > 0 {
		m.workflowTreeList.SetSize(listWidth, listHeight)
		m.scanOverviewList.SetSize(listWidth, listHeight)
	}

	// Update viewport sizes using config values
	viewportConfig := m.config.UI.Components.Viewport
	splitWidth := int(float64(m.width-m.config.UI.Layout.Cards.Spacing-scrollBarSpace) * viewportConfig.SplitRatio)
	viewportWidth := splitWidth - viewportConfig.BorderPadding
	viewportHeight := m.cardHeight - viewportConfig.ContentPadding

	// Reasonable minimums for viewport functionality
	if viewportHeight < 8 {
		viewportHeight = 8
	}
	if viewportWidth < 20 {
		viewportWidth = 20
	}

	// Update both viewports with same dimensions
	m.outputViewport.Width = viewportWidth
	m.outputViewport.Height = viewportHeight
	m.logsViewport.Width = viewportWidth
	m.logsViewport.Height = viewportHeight
}

// updateWorkflowList refreshes the workflow list to show only unselected workflows
func (m *model) updateWorkflowList() {
	items := []list.Item{}
	availableCount := 0

	for name, workflow := range m.workflows.Workflows {
		// Only show workflows that are NOT selected
		if !m.selectedWorkflows[name] {
			items = append(items, workflowItem{
				name:        name,
				description: workflow.Name,
				toolCount:   len(workflow.Tools),
				selected:    false,                     // Always false since we only show unselected
				executed:    m.executedWorkflows[name], //
			})
			availableCount++
		}
	}

	if len(m.workflows.Workflows) == 0 {
		items = []list.Item{
			workflowItem{name: "No workflows found", description: "Check workflows/descriptions.yaml", toolCount: 0, selected: false},
		}
	} else if availableCount == 0 {
		items = []list.Item{
			workflowItem{name: "All workflows selected", description: "Go to Execution Queue to deselect workflows", toolCount: 0, selected: false},
		}
	}

	m.workflowTreeList.SetItems(items)
}

// updateExecutionQueue builds the execution queue display
func (m *model) updateExecutionQueue() {
	items := []list.Item{}

	// Add target information at the top if configured
	if m.scanTarget != "" {
		items = append(items, executionItem{
			name:        "Target: " + m.scanTarget,
			description: "Configured scanning target",
			status:      "info",
		})
	}

	if len(m.selectedWorkflows) == 0 {
		items = append(items, executionItem{
			name:        "No workflows selected",
			description: "Select workflows with SPACE and press ENTER to execute",
			status:      "",
		})
	} else {
		selectedCount := 0
		for workflowName, selected := range m.selectedWorkflows {
			if selected {
				selectedCount++
				if workflow, exists := m.workflows.Workflows[workflowName]; exists {
					items = append(items, executionItem{
						name:        workflowName,
						description: workflow.Name,
						status:      "queued",
					})
				}
			}
		}

		// Add summary
		if selectedCount > 0 {
			items = append(items, executionItem{
				name:        fmt.Sprintf("Total: %d workflows", selectedCount),
				description: "Press ENTER to execute all selected workflows",
				status:      "ready",
			})
		}
	}

	m.scanOverviewList.SetItems(items)
}

// completeWorkflowExecution moves completed workflows back to workflow tree with executed mark
func (m *model) completeWorkflowExecution() {
	// Mark all executed workflows
	for _, tool := range m.tools {
		if tool.Status == "header" {
			// Extract workflow name from header format "[WorkflowName]"
			workflowName := strings.Trim(tool.Name, "[]")
			// Find the actual workflow key by matching the description
			for key, workflow := range m.workflows.Workflows {
				if workflow.Name == workflowName {
					m.executedWorkflows[key] = true
					m.logSystemMessage("info", "Workflow completed", "workflow", key)
					break
				}
			}
		}
	}

	// Clear execution queue
	m.scanOverviewList.SetItems([]list.Item{
		executionItem{
			name:        "All workflows completed",
			description: fmt.Sprintf("%d workflows executed successfully", len(m.executedWorkflows)),
			status:      "done",
		},
	})

	// Clear tools list
	m.tools = []toolExecution{}

	// Update workflow tree to show executed status
	m.updateWorkflowList()

	// Add completion message to live output
	m.liveOutput = append(m.liveOutput,
		"",
		"=== All Workflows Completed Successfully ===",
		fmt.Sprintf("Total workflows executed: %d", len(m.executedWorkflows)),
		"",
		"Select more workflows to continue execution",
	)
	m.outputViewport.SetContent(strings.Join(m.liveOutput, "\n"))
	m.outputViewport.GotoBottom()

	// Log completion
	m.logSystemMessage("success", "All workflows completed", "total", len(m.executedWorkflows))
}

// getExecutionContext creates an ExecutionContext for tool execution
func (m *model) getExecutionContext() map[string]string {
	// Create context that can be used by executor
	ctx := make(map[string]string)
	ctx["target"] = m.scanTarget
	
	// Use workspace directory from configuration - no session persistence
	outputDir := ""
	// Create workspace/target structure using config
	sanitizedTarget := m.sanitizeTargetForPath(m.scanTarget)
	outputDir = filepath.Join(m.config.Output.WorkspaceBase, sanitizedTarget)
	
	// Set different output paths for different types
	ctx["output_dir"] = outputDir
	ctx["workspace"] = outputDir
	ctx["logs_dir"] = filepath.Join(outputDir, "logs")
	ctx["scans_dir"] = filepath.Join(outputDir, "scans")
	ctx["reports_dir"] = filepath.Join(outputDir, "reports")
	ctx["raw_dir"] = filepath.Join(outputDir, "raw")
	ctx["timestamp"] = time.Now().Format("20060102_150405")
	
	// Add session ID - no persistence, generate new ID each time
	ctx["session_id"] = fmt.Sprintf("session_%d", time.Now().Unix())
	
	return ctx
}

// executeSelectedWorkflows starts execution of selected workflows (UI simulation only)
func (m *model) executeSelectedWorkflows() {
	// This is UI-only for now - no actual backend execution
	// When implementing real execution, use getExecutionContext():
	// execCtx := m.getExecutionContext()
	// ctx := &executor.ExecutionContext{
	//     Target:     execCtx["target"],
	//     Workspace:  execCtx["workspace"],
	//     OutputDir:  execCtx["output_dir"],
	//     LogsDir:    execCtx["logs_dir"],
	//     ScansDir:   execCtx["scans_dir"],
	//     ReportsDir: execCtx["reports_dir"],
	//     RawDir:     execCtx["raw_dir"],
	//     Timestamp:  execCtx["timestamp"],
	//     SessionID:  execCtx["session_id"],
	// }
	items := []list.Item{}

	// Clear and populate the tools execution list
	m.tools = []toolExecution{}

	for workflowName, selected := range m.selectedWorkflows {
		if selected {
			if workflow, exists := m.workflows.Workflows[workflowName]; exists {
				items = append(items, executionItem{
					name:        workflowName,
					description: workflow.Name,
					status:      "running",
				})

				// Add workflow header to tools
				m.tools = append(m.tools, toolExecution{
					Name:   fmt.Sprintf("[%s]", workflow.Name),
					Status: "header",
					Output: "",
				})

				// Add each tool from the workflow to the execution list
				for _, tool := range workflow.Tools {
					m.tools = append(m.tools, toolExecution{
						Name:   fmt.Sprintf("  → %s", tool),
						Status: "pending",
						Output: "",
					})
				}
			}
		}
	}

	// If tools were added, set the first actual tool (not header) to running
	for i := range m.tools {
		if m.tools[i].Status == "pending" {
			m.tools[i].Status = "running"
			break
		}
	}

	// Add execution summary
	executedCount := len(items)
	items = append(items, executionItem{
		name:        fmt.Sprintf("Executing %d workflows", executedCount),
		description: fmt.Sprintf("Running %d tools total", len(m.tools)-executedCount),
		status:      "info",
	})

	m.scanOverviewList.SetItems(items)

	// Update live output with tool execution simulation
	m.liveOutput = []string{
		"=== IPCrawler Tool Execution Started ===",
		fmt.Sprintf("=== Executing %d Workflows ===", executedCount),
		fmt.Sprintf("=== Total Tools to Execute: %d ===", len(m.tools)-executedCount),
		"",
	}

	// Log the start of execution
	m.logSystemMessage("info", "Workflow execution started", "workflows", executedCount, "tools", len(m.tools)-executedCount)

	// Start the first tool's output
	if len(m.tools) > 0 {
		for i, tool := range m.tools {
			if tool.Status == "running" {
				timeFormat := m.config.UI.Formatting.TimeFormat
				if timeFormat == "" {
					timeFormat = "15:04:05" // Fallback
				}
				m.liveOutput = append(m.liveOutput, fmt.Sprintf("[%s] Starting %s", time.Now().Format(timeFormat), tool.Name))
				break
			} else if tool.Status == "header" && i+1 < len(m.tools) {
				m.liveOutput = append(m.liveOutput, fmt.Sprintf("\n=== %s ===", tool.Name))
			}
		}
	}

	// Update viewport with initial output
	m.outputViewport.SetContent(strings.Join(m.liveOutput, "\n"))
	m.outputViewport.GotoBottom()

	// Clear selected workflows after execution starts
	m.selectedWorkflows = make(map[string]bool)
	m.updateWorkflowList()

	m.liveOutput = append(m.liveOutput, "=== Execution in progress ===")

	// Update live output viewport with auto-scroll
	m.updateLiveOutput()

	// In production, this is where live tool output would be continuously updated
	// and the auto-scroll would keep the latest output visible
}

// updateLiveOutput refreshes the live output viewport with auto-scroll behavior
func (m *model) updateLiveOutput() {
	// Check if user has manually scrolled up from bottom
	wasAtBottom := m.outputViewport.AtBottom()

	// Update content
	liveOutputContent := strings.Join(m.liveOutput, "\n")
	m.outputViewport.SetContent(liveOutputContent)

	// Auto-scroll to bottom only if user was already at bottom (following live updates)
	if wasAtBottom {
		m.outputViewport.GotoBottom()
	}
}

// createColoredLogEntry creates a colored log entry for the model
func (m *model) createColoredLogEntry(level, message string, keyvals ...interface{}) string {
	// Log styling with theme colors
	debugStyle := lipgloss.NewStyle().Foreground(m.getThemeColor("debug", "240"))
	infoStyle := lipgloss.NewStyle().Foreground(m.getThemeColor("info", "39"))
	warnStyle := lipgloss.NewStyle().Foreground(m.getThemeColor("warning", "214"))
	errorStyle := lipgloss.NewStyle().Foreground(m.getThemeColor("error", "196"))

	timestampStyle := lipgloss.NewStyle().Foreground(m.getThemeColor("timestamp", "243"))
	prefixStyle := lipgloss.NewStyle().Bold(true).Foreground(m.getThemeColor("prefix", "69"))
	keyStyle := lipgloss.NewStyle().Foreground(m.getThemeColor("key", "75"))
	valueStyle := lipgloss.NewStyle().Foreground(m.getThemeColor("value", "255"))
	workflowStyle := lipgloss.NewStyle().Foreground(m.getThemeColor("workflow", "120"))
	countStyle := lipgloss.NewStyle().Foreground(m.getThemeColor("count", "220"))
	progressStyle := lipgloss.NewStyle().Foreground(m.getThemeColor("progress", "82"))

	timeFormat := m.config.UI.Formatting.TimeFormat
	if timeFormat == "" {
		timeFormat = "15:04:05" // Fallback
	}
	timestamp := timestampStyle.Render(time.Now().Format(timeFormat))
	prefix := prefixStyle.Render("IPCrawler")

	var levelStyled string
	switch strings.ToUpper(level) {
	case "DEBUG":
		levelStyled = debugStyle.Bold(true).Render("DEBUG")
	case "INFO":
		levelStyled = infoStyle.Bold(true).Render("INFO ")
	case "WARN":
		levelStyled = warnStyle.Bold(true).Render("WARN ")
	case "ERROR":
		levelStyled = errorStyle.Bold(true).Render("ERROR")
	}

	// Format key-value pairs with colors
	var kvPairs []string
	for i := 0; i < len(keyvals); i += 2 {
		if i+1 < len(keyvals) {
			key := fmt.Sprintf("%v", keyvals[i])
			value := fmt.Sprintf("%v", keyvals[i+1])

			// Apply special styling for specific keys
			var styledKey, styledValue string
			switch key {
			case "workflow":
				styledKey = workflowStyle.Render(key)
				styledValue = workflowStyle.Render(value)
			case "count", "total_queued":
				styledKey = countStyle.Render(key)
				styledValue = countStyle.Render(value)
			case "progress":
				styledKey = progressStyle.Render(key)
				styledValue = progressStyle.Render(value)
			case "err":
				styledKey = errorStyle.Render(key)
				styledValue = errorStyle.Render(value)
			default:
				styledKey = keyStyle.Render(key)
				styledValue = valueStyle.Render(value)
			}
			kvPairs = append(kvPairs, styledKey+"="+styledValue)
		}
	}

	// Combine all parts
	logLine := timestamp + " " + prefix + " " + levelStyled + " " + message
	if len(kvPairs) > 0 {
		logLine += " " + strings.Join(kvPairs, " ")
	}
	return logLine
}

// logSystemMessage adds a structured log message and updates the logs viewport
func (m *model) logSystemMessage(level, message string, keyvals ...interface{}) {
	// Check if user has manually scrolled up from bottom
	wasAtBottom := m.logsViewport.AtBottom()

	// Add new colored log entry
	coloredEntry := m.createColoredLogEntry(level, message, keyvals...)
	m.systemLogs = append(m.systemLogs, coloredEntry)
	logContent := strings.Join(m.systemLogs, "\n")
	m.logsViewport.SetContent(logContent)

	// Auto-scroll to bottom only if user was already at bottom (following live updates)
	if wasAtBottom {
		m.logsViewport.GotoBottom()
	}
}

// getTerminalSize returns the actual terminal dimensions
func getTerminalSize() (int, int) {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		// Fallback to tput if stty fails
		rowsCmd := exec.Command("tput", "lines")
		colsCmd := exec.Command("tput", "cols")

		rowsOut, err1 := rowsCmd.Output()
		colsOut, err2 := colsCmd.Output()

		if err1 == nil && err2 == nil {
			rows := strings.TrimSpace(string(rowsOut))
			cols := strings.TrimSpace(string(colsOut))

			var height, width int
			fmt.Sscanf(rows, "%d", &height)
			fmt.Sscanf(cols, "%d", &width)

			return width, height
		}

		// Final fallback
		return 80, 24
	}

	var height, width int
	fmt.Sscanf(string(out), "%d %d", &height, &width)
	return width, height
}

func main() {
	// Check for --new-window flag
	openNewWindow := len(os.Args) > 1 && os.Args[1] == "--new-window"

	if !openNewWindow {
		// Launch in new terminal window with optimal size
		launchInNewTerminal()
		return
	}

	// This is the actual TUI execution in the new terminal
	runTUI()
}

func launchInNewTerminal() {
	// Get the executable path
	executable, err := os.Executable()
	if err != nil {
		fmt.Printf("Error getting executable path: %v\n", err)
		os.Exit(1)
	}

	// Optimal dimensions for IPCrawler TUI (no overlaps, horizontal layout)
	width := 200
	height := 70

	// Try different terminal applications based on OS
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin": // macOS
		// Try Terminal.app first
		cmd = exec.Command("osascript", "-e", fmt.Sprintf(`
tell application "Terminal"
	activate
	set newWindow to do script "%s --new-window"
	tell newWindow's tab 1
		set the size to {%d, %d}
	end tell
end tell`, executable, width, height))

		if err := cmd.Run(); err != nil {
			// Fallback to iTerm2
			cmd = exec.Command("osascript", "-e", fmt.Sprintf(`
tell application "iTerm"
	create window with default profile
	tell current session of current window
		write text "%s --new-window"
		set rows to %d
		set columns to %d
	end tell
end tell`, executable, height, width))
		}

	case "linux":
		// Try gnome-terminal first
		cmd = exec.Command("gnome-terminal",
			"--geometry", fmt.Sprintf("%dx%d", width, height),
			"--title", "IPCrawler TUI",
			"--", executable, "--new-window")

		if err := cmd.Start(); err != nil {
			// Fallback to xterm
			cmd = exec.Command("xterm",
				"-geometry", fmt.Sprintf("%dx%d", width, height),
				"-title", "IPCrawler TUI",
				"-e", executable, "--new-window")
		}

	default:
		// Fallback: run in current terminal
		fmt.Println("Opening TUI in current terminal (new window not supported on this platform)")
		runTUI()
		return
	}

	// Execute the command
	if err := cmd.Run(); err != nil {
		fmt.Printf("Could not open new terminal window: %v\n", err)
		fmt.Println("Falling back to current terminal...")
		runTUI()
	}
}

func runTUI() {
	// Check TTY
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Println("IPCrawler TUI requires a terminal")
		os.Exit(1)
	}

	// Check terminal size and give user feedback
	actualWidth, actualHeight := getTerminalSize()
	if actualWidth < 200 || actualHeight < 70 {
		fmt.Printf("⚠️  Terminal size: %dx%d (detected)\n", actualWidth, actualHeight)
		fmt.Printf("💡 Optimal size: 200x70 for best experience\n")
		fmt.Printf("📖 See RESIZE_GUIDE.md for instructions\n")
		fmt.Printf("\nContinue anyway? (y/N): ")

		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Run 'make run' after resizing your terminal to 200x70")
			os.Exit(0)
		}
	}

	// Check for sudo requirements before creating model
	if !checkSudoRequirements() {
		return // Exit cleanly if user chose not to continue
	}

	// Create model with optimal size
	model := newModel()

	// Get actual terminal size for responsive design
	termWidth, termHeight := getTerminalSize()
	model.width = termWidth
	model.height = termHeight
	model.updateSizes()
	// ready flag will be set when WindowSizeMsg is received

	// Run TUI with proper window size handling
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		// Enable resize messages to handle window changes properly
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

// checkSudoRequirements checks if any workflows require sudo and warns the user
// Returns true if the program should continue, false if it should exit
func checkSudoRequirements() bool {
	// Check current privilege level first
	hasPrivileges, statusMsg := getPrivilegeStatus()
	
	// Load workflows to check for sudo requirements
	workflows, err := loader.LoadWorkflowDescriptions(".")
	if err != nil {
		// Try alternative paths
		if execPath, err := os.Executable(); err == nil {
			execDir := filepath.Dir(execPath)
			workflows, _ = loader.LoadWorkflowDescriptions(execDir)
			if workflows == nil || len(workflows.Workflows) == 0 {
				parentDir := filepath.Dir(execDir)
				workflows, _ = loader.LoadWorkflowDescriptions(parentDir)
			}
		}
	}

	// If no workflows loaded, skip check and continue
	if workflows == nil || len(workflows.Workflows) == 0 {
		return true
	}

	// Check for tools requiring sudo
	if workflows.HasToolsRequiringSudo() {
		// If we already have privileges, show success message and continue
		if hasPrivileges {
			fmt.Println("✅ Privilege Check Passed")
			fmt.Println("=========================")
			fmt.Printf("Status: %s\n", statusMsg)
			fmt.Println("All tools requiring elevated privileges will function properly.")
			fmt.Println("\nLoading IPCrawler TUI...")
			time.Sleep(1 * time.Second) // Brief pause to show message
			fmt.Print("\033[H\033[2J") // Clear screen
			return true
		}
		sudoTools := workflows.GetToolsRequiringSudo()
		
		fmt.Println("⚠️  IPCrawler Privilege Requirements")
		fmt.Println("=====================================")
		fmt.Printf("The following tools require sudo privileges for optimal functionality:\n\n")
		
		for _, tool := range sudoTools {
			fmt.Printf("  • %s", tool.Name)
			if tool.Reason != "" {
				fmt.Printf(" - %s", tool.Reason)
			}
			fmt.Println()
		}
		
		fmt.Println("\n💡 Solutions:")
		fmt.Println("  1. Restart IPCrawler with sudo privileges (recommended)")
		fmt.Println("  2. Continue with limited functionality (some scans may fail)")
		fmt.Printf("\nRestart with sudo privileges? (Y/n): ")
		
		var response string
		fmt.Scanln(&response)
		
		// Default to "yes" if empty or "y"/"yes", only "n"/"no" refuses
		if strings.ToLower(response) == "n" || strings.ToLower(response) == "no" {
			fmt.Println("\n⚠️  Continuing with limited functionality...")
			fmt.Println("Some tools requiring root privileges may fail.")
			fmt.Print("Press Enter to continue...")
			fmt.Scanln(&response)
			fmt.Print("\033[H\033[2J") // Clear screen
			return true // Continue without privileges
		}
		
		// User chose to escalate to sudo
		fmt.Println("\n🔐 Restarting IPCrawler with sudo privileges...")
		fmt.Println("You may be prompted for your password.")
		
		// Get the current executable path
		executable, err := os.Executable()
		if err != nil {
			fmt.Printf("Error getting executable path: %v\n", err)
			fmt.Println("Please run manually: sudo make run-here")
			return false
		}
		
		// Restart with sudo
		cmd := exec.Command("sudo", executable, "--new-window")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		
		fmt.Print("\nStarting with elevated privileges...")
		time.Sleep(1 * time.Second)
		
		// Replace current process with sudo version
		err = cmd.Run()
		if err != nil {
			fmt.Printf("\nFailed to restart with sudo: %v\n", err)
			fmt.Println("Fallback: Please run 'sudo make run-here' manually")
			
			fmt.Println("\n👋 Thank you for using IPCrawler!")
			fmt.Print("\nClosing in 3 seconds...")
			for i := 3; i > 0; i-- {
				time.Sleep(1 * time.Second)
				fmt.Printf(" %d", i)
			}
			fmt.Println()
		}
		
		return false // Exit current unprivileged process
	}
	return true // Continue with TUI
}

// isRunningAsRoot checks if the current process is running with root privileges
func isRunningAsRoot() bool {
	// Check if UID is 0 (root)
	if os.Geteuid() == 0 {
		return true
	}
	return false
}

// isRunningWithSudo checks if the process was started with sudo
func isRunningWithSudo() bool {
	// Check SUDO_UID environment variable (set by sudo)
	if os.Getenv("SUDO_UID") != "" {
		return true
	}
	
	// Check if we're root but SUDO_USER is set
	if isRunningAsRoot() && os.Getenv("SUDO_USER") != "" {
		return true
	}
	
	return false
}

// isRootlessEnvironment detects if we're in a rootless environment like containers/HTB
func isRootlessEnvironment() bool {
	// Check if we're running as root but in a container-like environment
	if !isRunningAsRoot() {
		return false
	}
	
	// Check for container indicators
	containerIndicators := []string{
		"/.dockerenv",                    // Docker
		"/run/.containerenv",            // Podman
		"/proc/1/cgroup",                // Check if we can read cgroup (container sign)
	}
	
	for _, indicator := range containerIndicators {
		if _, err := os.Stat(indicator); err == nil {
			return true
		}
	}
	
	// Check if we're in a limited root environment
	// HTB machines often have root but with limited capabilities
	if isRunningAsRoot() {
		// Check if we can access typical root-only files
		restrictedPaths := []string{
			"/etc/shadow",
			"/root/.ssh",
		}
		
		accessCount := 0
		for _, path := range restrictedPaths {
			if _, err := os.Stat(path); err == nil {
				accessCount++
			}
		}
		
		// If we're root but can't access typical root files, likely rootless
		if accessCount == 0 {
			return true
		}
	}
	
	return false
}

// getPrivilegeStatus returns a description of current privilege level
func getPrivilegeStatus() (bool, string) {
	if isRunningAsRoot() {
		if isRunningWithSudo() {
			return true, "Running with sudo privileges"
		} else if isRootlessEnvironment() {
			return true, "Running in rootless environment (container/sandbox)"
		} else {
			return true, "Running as root user"
		}
	}
	
	// Check if user might have capabilities without being root
	currentUser, err := user.Current()
	if err == nil && currentUser.Username != "" {
		// Check if user is in privileged groups
		groups := []string{"wheel", "admin", "sudo", "root"}
		for _, group := range groups {
			if checkUserInGroup(currentUser.Username, group) {
				return false, fmt.Sprintf("Running as %s (member of %s group)", currentUser.Username, group)
			}
		}
		return false, fmt.Sprintf("Running as %s (unprivileged)", currentUser.Username)
	}
	
	return false, "Running as unprivileged user"
}

// checkUserInGroup checks if a user is in a specific group (Unix-like systems)
func checkUserInGroup(username, groupname string) bool {
	if runtime.GOOS == "windows" {
		return false // Skip group checking on Windows
	}
	
	cmd := exec.Command("id", "-Gn", username)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	
	groups := strings.Fields(string(output))
	for _, group := range groups {
		if group == groupname {
			return true
		}
	}
	return false
}
