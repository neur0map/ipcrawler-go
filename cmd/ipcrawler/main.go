package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/mattn/go-isatty"

	"github.com/your-org/ipcrawler/internal/config"
	"github.com/your-org/ipcrawler/internal/tui/data/loader"
)

// focusState tracks which card is focused
type focusState int

const (
	workflowTreeFocus focusState = iota // Was overviewFocus - now for selecting workflows
	scanOverviewFocus                   // Was workflowsFocus - now shows selected workflow details
	outputFocus                         // Live raw tool output
	logsFocus                           // System logs, debug, errors, warnings
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
	MemoryMB   float64
	Goroutines int
	LastUpdate string
}

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
		executedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82")) // Green
		executedMark = " " + executedStyle.Render("✓")
	}

	return fmt.Sprintf("%s %s%s", checkbox, i.name, executedMark)
}
func (i workflowItem) Description() string {
	// Show tool count and executed status more concisely
	if i.executed {
		executedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82")) // Green
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
	liveOutputVp := viewport.New(50, 10)
	// Apply config settings
	if cfg.UI.Components.Viewport.ScrollSpeed > 0 {
		liveOutputVp.MouseWheelDelta = cfg.UI.Components.Viewport.ScrollSpeed
	} else {
		liveOutputVp.MouseWheelDelta = 3 // Default faster scrolling
	}
	if cfg.UI.Components.Viewport.HighPerformance {
		liveOutputVp.HighPerformanceRendering = true
	}

	logsVp := viewport.New(50, 10)
	// Apply config settings
	if cfg.UI.Components.Viewport.ScrollSpeed > 0 {
		logsVp.MouseWheelDelta = cfg.UI.Components.Viewport.ScrollSpeed
	} else {
		logsVp.MouseWheelDelta = 3 // Default faster scrolling
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
	debugStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Gray
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))   // Blue
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))  // Orange
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red

	timestampStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))        // Gray
	prefixStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69")) // Purple
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("75"))               // Light blue
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))            // White
	workflowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("120"))         // Green
	countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("220"))            // Yellow
	progressStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))          // Bright green

	// Helper function to create colored log entries
	createLogEntry := func(level, message string, keyvals ...interface{}) string {
		timestamp := timestampStyle.Render(time.Now().Format("15:04:05"))
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
		createLogEntry("DEBUG", "Creating live output viewport", "width", 50, "height", 10),
		createLogEntry("DEBUG", "Creating system logs viewport", "width", 50, "height", 10),
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

		createLogEntry("DEBUG", "Database connection testing..."),
		createLogEntry("DEBUG", "Connecting to results database", "host", "localhost", "port", 5432),
		createLogEntry("DEBUG", "Validating schema version", "current", "1.2.3", "required", "1.2.0"),
		createLogEntry("DEBUG", "Testing write permissions", "table", "scan_results"),
		createLogEntry("DEBUG", "Testing read permissions", "table", "workflow_history"),
		createLogEntry("INFO", "Database connectivity established"),

		createLogEntry("DEBUG", "Session management initialization..."),
		createLogEntry("DEBUG", "Generating session ID", "session", "abc123def456"),
		createLogEntry("DEBUG", "Setting session timeout", "timeout", "2h"),
		createLogEntry("DEBUG", "Configuring auto-save", "interval", "5m"),
		createLogEntry("DEBUG", "Loading previous session state", "found", false),
		createLogEntry("INFO", "New session created successfully"),

		createLogEntry("DEBUG", "API integration testing..."),
		createLogEntry("DEBUG", "Testing VirusTotal API", "status", "authenticated"),
		createLogEntry("DEBUG", "Testing Shodan API", "status", "authenticated"),
		createLogEntry("DEBUG", "Testing SecurityTrails API", "status", "rate_limited"),
		createLogEntry("WARN", "Some API services have limitations", "limited_apis", 1),
		createLogEntry("INFO", "API integrations configured"),

		createLogEntry("DEBUG", "Workflow engine startup..."),
		createLogEntry("DEBUG", "Loading workflow executor", "max_concurrent", 3),
		createLogEntry("DEBUG", "Initializing task queue", "max_size", 100),
		createLogEntry("DEBUG", "Setting up progress tracking", "granularity", "step"),
		createLogEntry("DEBUG", "Configuring error handling", "retry_attempts", 3),
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
		createLogEntry("WARN", "Remember to configure API keys for enhanced functionality"),
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
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Create list delegates with custom styling
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("69")).
		BorderLeftForeground(lipgloss.Color("69"))
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

	scanOverviewList := list.New(executionQueueItems, delegate, 0, 0)
	scanOverviewList.Title = "Execution Queue"
	scanOverviewList.SetShowStatusBar(cfg.UI.Components.List.ShowStatusBar)
	scanOverviewList.SetShowPagination(false)
	scanOverviewList.SetFilteringEnabled(cfg.UI.Components.List.FilteringEnabled)

	m := &model{
		config:            cfg,
		workflows:         workflows,
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
		perfData:          systemMetrics{MemoryMB: 12.5, Goroutines: 5, LastUpdate: "12:34:56"},

		// Box card styles using config colors
		cardStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1),
		focusedCardStyle: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color("86")). // Bright cyan for maximum visibility
			Padding(0, 1),
		titleStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cfg.UI.Theme.Colors["accent"])).
			Bold(true).
			Align(lipgloss.Center),
		headerStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(cfg.UI.Theme.Colors["secondary"])).
			Align(lipgloss.Right),
		dimStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")),
	}

	return m
}

func (m *model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSizes()

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

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

		// Simulate tool execution progress
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
								m.liveOutput = append(m.liveOutput,
									fmt.Sprintf("[%s] Starting %s", time.Now().Format("15:04:05"), m.tools[j].Name))
								m.outputViewport.SetContent(strings.Join(m.liveOutput, "\n"))
								m.outputViewport.GotoBottom()
								foundNext = true
								break
							}
						}
						// Add completion to live output
						m.liveOutput = append(m.liveOutput,
							fmt.Sprintf("[%s] Completed %s", time.Now().Format("15:04:05"), m.tools[i].Name))
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
	}

	return m, tea.Batch(cmds...)
}

func (m *model) View() string {
	if !m.ready || m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	// Create title
	title := m.titleStyle.Render("IPCrawler TUI - Dynamic Cards Dashboard")
	title = lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(title)

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
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, workflowSelection, "  ", executionQueue, "  ", tools, "  ", perf)

	// Bottom Row: Live Output and Logs (side by side)
	liveOutput := m.renderOutputCard()
	logs := m.renderLogsCard()
	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, liveOutput, "  ", logs)

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
	titleColor := lipgloss.Color("240")
	if m.focus == workflowTreeFocus {
		style = m.focusedCardStyle
		titleColor = lipgloss.Color("86") // Bright cyan when focused
	}

	// Update list title color dynamically
	m.workflowTreeList.Styles.Title = m.workflowTreeList.Styles.Title.Foreground(titleColor)

	// Use the list component's view
	return style.Width(m.cardWidth).Height(m.cardHeight).Render(m.workflowTreeList.View())
}

func (m *model) renderScanOverviewCard() string {
	style := m.cardStyle
	titleColor := lipgloss.Color("240")
	if m.focus == scanOverviewFocus {
		style = m.focusedCardStyle
		titleColor = lipgloss.Color("86") // Bright cyan when focused
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

	// Calculate width for side-by-side layout (split bottom row)
	outputWidth := (m.width - 6) / 2 // Split width, account for spacing

	// Card header with title and scroll info
	titleText := "Live Output"
	if m.focus == outputFocus {
		titleText = "Live Output (FOCUSED - Use ↑↓ to scroll)"
	}
	title := m.titleStyle.Render(titleText)
	scrollInfo := m.headerStyle.Render(fmt.Sprintf("%.1f%%", m.outputViewport.ScrollPercent()*100))
	titleWidth := outputWidth - 4
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

	// Calculate width for side-by-side layout (split bottom row)
	logsWidth := (m.width - 6) / 2 // Split width, account for spacing

	// Card header with title and scroll info
	titleText := "Logs"
	if m.focus == logsFocus {
		titleText = "Logs (FOCUSED - Use ↑↓ to scroll)"
	}
	title := m.titleStyle.Render(titleText)
	scrollInfo := m.headerStyle.Render(fmt.Sprintf("%.1f%%", m.logsViewport.ScrollPercent()*100))
	titleWidth := logsWidth - 4
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
	titleColor := lipgloss.Color("240")
	if m.focus == toolsFocus {
		titleColor = lipgloss.Color("86") // Bright cyan when focused
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
				statusColor := lipgloss.Color("86") // Cyan for completed

				if tool.Status == "running" {
					status = "[RUN] " + m.spinner.View()
					statusColor = lipgloss.Color("214") // Orange for running
				} else if tool.Status == "pending" {
					status = "[WAIT]"
					statusColor = lipgloss.Color("240") // Gray for pending
				} else if tool.Status == "failed" {
					status = "[FAIL]"
					statusColor = lipgloss.Color("196") // Red for failed
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
	if m.focus == perfFocus {
		style = m.focusedCardStyle
	}

	// Card header with title
	title := m.titleStyle.Width(m.cardWidth - 2).Render("Performance Monitor")

	// Card content
	content := fmt.Sprintf(`Memory: %.1f MB
Goroutines: %d
Last Update: %s

System: Operational
Status: Active`,
		m.perfData.MemoryMB,
		m.perfData.Goroutines,
		m.perfData.LastUpdate,
	)

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

	// Calculate card dimensions for horizontal layout (4 cards across)
	// Account for borders and spacing
	m.cardWidth = (m.width - 8) / 4    // 4 cards horizontal with spacing
	m.cardHeight = (m.height - 10) / 2 // 2 rows: top cards + output

	// Ensure reasonable minimums for readability
	if m.cardWidth < 35 {
		m.cardWidth = 35
	}
	if m.cardHeight < 12 {
		m.cardHeight = 12
	}

	// Update list sizes (account for borders and padding)
	listWidth := m.cardWidth - 4   // Subtract border and padding
	listHeight := m.cardHeight - 4 // Subtract border and padding

	if listWidth > 0 && listHeight > 0 {
		m.workflowTreeList.SetSize(listWidth, listHeight)
		m.scanOverviewList.SetSize(listWidth, listHeight)
	}

	// Update viewport sizes for side-by-side layout (live output and logs)
	bottomCardWidth := (m.width - 6) / 2 // Split bottom row
	viewportWidth := bottomCardWidth - 8 // Account for card borders and padding
	viewportHeight := m.cardHeight - 4

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

// executeSelectedWorkflows starts execution of selected workflows (UI simulation only)
func (m *model) executeSelectedWorkflows() {
	// This is UI-only for now - no actual backend execution
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
				m.liveOutput = append(m.liveOutput, fmt.Sprintf("[%s] Starting %s", time.Now().Format("15:04:05"), tool.Name))
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
	// Define color styles
	debugStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Gray
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))   // Blue
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))  // Orange
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red

	timestampStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))        // Gray
	prefixStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69")) // Purple
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("75"))               // Light blue
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255"))            // White
	workflowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("120"))         // Green
	countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("220"))            // Yellow
	progressStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("82"))          // Bright green

	timestamp := timestampStyle.Render(time.Now().Format("15:04:05"))
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

	// Create model with optimal size
	model := newModel()

	// Get actual terminal size for responsive design
	termWidth, termHeight := getTerminalSize()
	model.width = termWidth
	model.height = termHeight
	model.updateSizes()
	model.ready = true

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
