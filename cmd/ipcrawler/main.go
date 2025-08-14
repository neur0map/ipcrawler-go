package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

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
	"github.com/neur0map/ipcrawler/internal/executor"
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

	// Execution engine components  
	executionEngine     *executor.ToolExecutionEngine
	workflowOrchestrator *executor.WorkflowOrchestrator

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

// Workflow execution message types for Bubble Tea async operations
type workflowStartedMsg struct {
	WorkflowName string
	Target       string
	QueueSize    int
}

type workflowProgressMsg struct {
	WorkflowName string
	Step         string
	Status       string
	Output       string
}

type workflowCompletedMsg struct {
	WorkflowName string
	Success      bool
	Error        string
	Duration     time.Duration
}

type workflowQueueUpdatedMsg struct {
	QueuedWorkflows []string
	CompletedCount  int
	TotalCount      int
}

// metricsTickMsg is sent by the metrics ticker to trigger regular updates
type metricsTickMsg struct{}

// readinessFallbackMsg is sent to ensure TUI becomes ready even if WindowSizeMsg is delayed
type readinessFallbackMsg struct{}

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

	// Initialize live output (raw tool execution output) - empty by default
	liveOutputLines := []string{
		"=== IPCrawler Live Tool Execution Output ===",
		"Ready to execute workflows...",
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

		// System startup log entries
		createLogEntry("DEBUG", "Configuration validation starting..."),
		func() string {
			count := 0
			if workflows != nil {
				count = len(workflows.Workflows)
			}
			return createLogEntry("INFO", "Loading workflow configurations", "count", count)
		}(),
		createLogEntry("WARN", "Using default configuration - custom config not found"),
		// Dynamically list available workflows from loaded configuration
		func() string {
			if workflows != nil && len(workflows.Workflows) > 0 {
				var workflowNames []string
				for name := range workflows.Workflows {
					workflowNames = append(workflowNames, name)
				}
				return createLogEntry("DEBUG", "Parsing available workflows", "workflows", strings.Join(workflowNames, ","))
			}
			return createLogEntry("WARN", "No workflows loaded - check configuration")
		}(),
		createLogEntry("INFO", "All workflow configurations validated successfully"),

		createLogEntry("DEBUG", "UI component initialization starting..."),
		createLogEntry("DEBUG", "Creating workflow tree list component"),
		createLogEntry("DEBUG", "Creating execution queue list component"),
		createLogEntry("DEBUG", "Creating live output viewport", "width", viewportWidth, "height", viewportHeight),
		createLogEntry("DEBUG", "Creating system logs viewport", "width", viewportWidth, "height", viewportHeight),
		createLogEntry("DEBUG", "Setting up key bindings", "focus_keys", "1,2,3,4,5,6"),
		createLogEntry("DEBUG", "Configuring scroll support", "scroll_keys", "up,down,page-up,page-down"),
		createLogEntry("INFO", "All UI components initialized and configured"),

		createLogEntry("DEBUG", "Security module initialization..."),
		createLogEntry("DEBUG", "Loading security policies"),
		createLogEntry("DEBUG", "Validating tool permissions"),
		createLogEntry("DEBUG", "Setting up sandboxing", "mode", "restricted"),
		createLogEntry("DEBUG", "Configuring audit logging", "log_level", "debug"),
		createLogEntry("INFO", "Security framework initialized successfully"),

		createLogEntry("DEBUG", "Performance monitoring setup..."),
		createLogEntry("DEBUG", "Initializing memory tracker"),
		createLogEntry("DEBUG", "Setting up goroutine monitor"),
		createLogEntry("DEBUG", "Configuring metrics collection", "interval", "1s"),
		createLogEntry("DEBUG", "Enabling real-time performance updates"),
		createLogEntry("INFO", "Performance monitoring active"),

		createLogEntry("DEBUG", "Network module configuration..."),
		createLogEntry("DEBUG", "Testing external connectivity"),
		createLogEntry("DEBUG", "Validating DNS resolution"),
		createLogEntry("DEBUG", "Checking proxy settings", "proxy", "none"),
		createLogEntry("DEBUG", "Configuring timeout values", "connect_timeout", "10s", "read_timeout", "30s"),
		createLogEntry("INFO", "Network connectivity verified"),

		createLogEntry("DEBUG", "Tool dependency verification..."),
		// Dynamically check tools from loaded workflows
		func() string {
			if workflows != nil && len(workflows.Workflows) > 0 {
				var allTools []string
				for _, workflow := range workflows.Workflows {
					for _, tool := range workflow.Tools {
						found := false
						for _, existing := range allTools {
							if existing == tool.Name {
								found = true
								break
							}
						}
						if !found {
							allTools = append(allTools, tool.Name)
						}
					}
				}
				if len(allTools) > 0 {
					return createLogEntry("DEBUG", "Required tools detected", "tools", strings.Join(allTools, ","))
				}
			}
			return createLogEntry("WARN", "No tools to verify - no workflows loaded")
		}(),
		createLogEntry("INFO", "Tool verification complete - system ready for operation"),

		// Database-related logs removed

		createLogEntry("DEBUG", "Session management initialization..."),
		createLogEntry("DEBUG", "Generating session ID"),
		createLogEntry("DEBUG", "Setting session timeout"),
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
		createLogEntry("DEBUG", "Scanning plugin directory"),
		createLogEntry("DEBUG", "Scanning for available plugins"),
		createLogEntry("INFO", "Plugin system ready"),

		createLogEntry("DEBUG", "Final system checks..."),
		createLogEntry("DEBUG", "Verifying file permissions"),
		createLogEntry("DEBUG", "Checking disk space"),
		createLogEntry("DEBUG", "Validating log rotation"),
		createLogEntry("DEBUG", "Testing emergency shutdown procedures"),
		createLogEntry("INFO", "All system checks passed - IPCrawler ready for operation"),

		createLogEntry("INFO", "=== SYSTEM STARTUP COMPLETE ==="),
		createLogEntry("DEBUG", "Initialization complete"),
		createLogEntry("INFO", "System health: operational"),
		createLogEntry("DEBUG", "Ready to accept workflow execution requests"),
		createLogEntry("INFO", "Use ↑↓ keys to scroll logs when focused on this window"),

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

	// Initialize execution engine components
	executionEngine := executor.NewToolExecutionEngine(cfg, "")
	workflowExecutor := executor.NewWorkflowExecutor(executionEngine)
	workflowOrchestrator := executor.NewWorkflowOrchestrator(workflowExecutor, cfg)
	
	// Create the model first so we can reference it in the callback
	m := &model{
		config:               cfg,
		executionEngine:      executionEngine,
		workflowOrchestrator: workflowOrchestrator,
		workflows:            workflows,
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
	
	// Set ready to true immediately since we're setting size manually in runTUI()
	// This prevents getting stuck on "Initializing..." screen
	m.ready = true

	// Add startup logging for debugging
	m.logSystemMessage("info", "=== IPCrawler TUI Started ===", "version", "debug", "timestamp", time.Now().Format("15:04:05"))
	m.logSystemMessage("info", "System initialized", "workflows_loaded", len(workflows.Workflows))
	m.logSystemMessage("debug", "Press TAB to cycle windows, SPACE to select workflows, ENTER to execute")

	// Set up workflow status callback for real-time updates
	workflowOrchestrator.SetStatusCallback(func(workflowName, target, status, message string) {
		// This callback runs in a separate goroutine, so we need to be careful about TUI updates
		m.logSystemMessage("info", "Workflow status update", 
			"workflow", workflowName, 
			"target", target, 
			"status", status, 
			"message", message)
		
		// Add to live output for immediate visibility
		if len(m.liveOutput) > 100 { // Prevent memory issues
			m.liveOutput = m.liveOutput[1:]
		}
		m.liveOutput = append(m.liveOutput, fmt.Sprintf("[%s] %s: %s", status, workflowName, message))
		m.updateLiveOutput()
	})

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
		// Fallback readiness timer - ensures TUI becomes ready even if WindowSizeMsg is delayed
		tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
			return readinessFallbackMsg{}
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
				m.logSystemMessage("info", "=== TARGET ENTERED ===", "input", inputValue)
				if err := m.validateTarget(inputValue); err != nil {
					m.targetError = err.Error()
					m.logSystemMessage("error", "Target validation failed", "error", err.Error())
				} else {
					m.scanTarget = inputValue
					m.showTargetModal = false
					m.targetError = ""
					m.focus = workflowTreeFocus // Set initial focus after modal closes
					m.logSystemMessage("info", "Target accepted, focus set to workflow tree", "target", inputValue)
					
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
					m.logSystemMessage("info", "Workflow selected", "name", workflowItem.name, "description", workflowItem.description)
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
						m.logSystemMessage("info", "Workflow deselected", "workflow", executionItem.name, "total_queued", selectedCount)
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
			
			// Enhanced logging for Enter key debugging - IMMEDIATE DEBUG
			m.logSystemMessage("info", "=== ENTER KEY PRESSED ===", "focus_window", m.focus, "has_selected_workflows", hasSelectedWorkflows)
			
			// Log which workflows are currently selected
			selectedList := []string{}
			for workflow, selected := range m.selectedWorkflows {
				if selected {
					selectedList = append(selectedList, workflow)
				}
			}
			m.logSystemMessage("info", "Currently selected workflows", "workflows", strings.Join(selectedList, ", "), "count", len(selectedList))
			
			if (m.focus == workflowTreeFocus || m.focus == scanOverviewFocus) && hasSelectedWorkflows {
				m.logSystemMessage("info", "Starting workflow execution via command")
				// Return the command instead of calling synchronously
				cmd := m.executeWorkflowsCmd()
				if cmd != nil {
					cmds = append(cmds, cmd)
					m.logSystemMessage("info", "Workflow command added to execution queue")
				} else {
					m.logSystemMessage("error", "Failed to create workflow command")
				}
			} else {
				focusName := []string{"targetInput", "workflowTree", "scanOverview", "output", "logs", "tools", "perf"}
				currentFocus := "unknown"
				if int(m.focus) < len(focusName) {
					currentFocus = focusName[m.focus]
				}
				m.logSystemMessage("warning", "Enter key ignored", "reason", "focus or workflow selection conditions not met", "current_focus", currentFocus, "required_focus", "workflowTree or scanOverview")
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
		
		// Remove verbose system metrics debug logging

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

	case workflowStartedMsg:
		// Handle workflow started message
		m.logSystemMessage("info", "Workflows started", "workflows", msg.WorkflowName, "target", msg.Target, "queue_size", msg.QueueSize)
		
		// Clear selected workflows and update UI
		m.selectedWorkflows = make(map[string]bool)
		m.updateWorkflowList()
		
		// Update live output with execution start
		m.liveOutput = []string{
			"=== IPCrawler Workflow Execution Starting ===",
			"",
			fmt.Sprintf("Target: %s", msg.Target),
			fmt.Sprintf("Workflows: %s", msg.WorkflowName),
			fmt.Sprintf("Queue Size: %d", msg.QueueSize),
			"",
			"=== Starting Workflow Execution ===",
			"",
		}
		m.updateLiveOutput()
		
		// Initialize tools list with queued workflows
		m.tools = []toolExecution{}
		workflowNames := strings.Split(msg.WorkflowName, ", ")
		for _, name := range workflowNames {
			m.tools = append(m.tools, toolExecution{
				Name:   fmt.Sprintf("[%s]", name),
				Status: "running",
				Output: "",
			})
		}
		
		// Start monitoring workflow progress
		cmds = append(cmds, m.monitorWorkflowProgressCmd())

	case workflowProgressMsg:
		// Handle workflow progress updates
		m.logSystemMessage("debug", "Workflow progress", "workflow", msg.WorkflowName, "step", msg.Step, "status", msg.Status)
		
		// Update live output with progress
		if msg.Output != "" {
			m.liveOutput = append(m.liveOutput, fmt.Sprintf("[%s] %s: %s", msg.WorkflowName, msg.Step, msg.Output))
			m.updateLiveOutput()
		}
		
		// Update tools list status
		for i, tool := range m.tools {
			if strings.Contains(tool.Name, msg.WorkflowName) {
				m.tools[i].Status = msg.Status
				m.tools[i].Output = msg.Output
				break
			}
		}

	case workflowCompletedMsg:
		// Handle workflow completion
		if msg.Success {
			m.logSystemMessage("info", "Workflow completed successfully", "workflow", msg.WorkflowName, "duration", msg.Duration)
			m.liveOutput = append(m.liveOutput, 
				"",
				fmt.Sprintf("Workflow '%s' completed successfully (Duration: %s)", msg.WorkflowName, msg.Duration),
				"",
			)
		} else {
			m.logSystemMessage("error", "Workflow failed", "workflow", msg.WorkflowName, "error", msg.Error)
			m.liveOutput = append(m.liveOutput,
				"",
				fmt.Sprintf("Workflow '%s' failed: %s", msg.WorkflowName, msg.Error),
				"",
			)
		}
		m.updateLiveOutput()
		
		// Update tools list status
		for i, tool := range m.tools {
			if strings.Contains(tool.Name, msg.WorkflowName) {
				if msg.Success {
					m.tools[i].Status = "completed"
				} else {
					m.tools[i].Status = "failed"
					m.tools[i].Output = msg.Error
				}
				break
			}
		}

	case workflowQueueUpdatedMsg:
		// Handle workflow queue updates
		m.logSystemMessage("debug", "Workflow queue updated", "completed", msg.CompletedCount, "total", msg.TotalCount, "queued", len(msg.QueuedWorkflows))
		
		// Get actual workflow status to update live output with real progress
		activeWorkflows := m.workflowOrchestrator.GetActiveWorkflows()
		
		// Update live output with current execution status
		if len(activeWorkflows) > 0 {
			m.liveOutput = append(m.liveOutput, 
				fmt.Sprintf("[%s] Active workflows: %d", time.Now().Format("15:04:05"), len(activeWorkflows)),
			)
			
			// Show details of each active workflow
			for workflowKey, execution := range activeWorkflows {
				status := "running"
				switch execution.Status {
				case 0: status = "queued"
				case 1: status = "running" 
				case 2: status = "completed"
				case 3: status = "failed"
				case 4: status = "cancelled"
				}
				
				progressPct := 0
				if execution.TotalSteps > 0 {
					progressPct = (execution.CompletedSteps * 100) / execution.TotalSteps
				}
				
				m.liveOutput = append(m.liveOutput,
					fmt.Sprintf("  • %s: %s (%d%% complete, step %d/%d)", 
						workflowKey, status, progressPct, execution.CompletedSteps, execution.TotalSteps),
				)
			}
			m.updateLiveOutput()
		}
		
		// Check if all workflows are completed
		if msg.TotalCount > 0 && len(activeWorkflows) == 0 && len(msg.QueuedWorkflows) == 0 {
			m.liveOutput = append(m.liveOutput,
				"",
				"=== All Workflows Completed ===",
				fmt.Sprintf("Total workflows executed: %d", msg.TotalCount),
				"Check workspace directory for output files.",
				"",
			)
			m.updateLiveOutput()
		}

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
		
		// Update resource monitor with current system metrics
		if m.workflowOrchestrator != nil && m.workflowOrchestrator.ResourceMonitor != nil {
			if err := m.workflowOrchestrator.ResourceMonitor.UpdateResourceUsageFromSystem(); err != nil {
				m.logSystemMessage("debug", "Failed to update resource usage", "error", err)
			}
		}
		
		// Log queue status for monitoring (only if there's activity)
		if m.workflowOrchestrator != nil {
			queuedCount, activeCount, queuedNames, activeNames := m.workflowOrchestrator.GetExecutionStatus()
			if queuedCount > 0 || activeCount > 0 {
				m.logSystemMessage("debug", "Queue status update", 
					"queued_count", queuedCount, 
					"active_count", activeCount, 
					"queued_workflows", strings.Join(queuedNames, ", "), 
					"active_workflows", strings.Join(activeNames, ", "))
			}
		}
		
		cmds = append(cmds, tea.Every(refreshInterval, func(t time.Time) tea.Msg {
			return metricsTickMsg{}
		}))

	case readinessFallbackMsg:
		// Fallback to ensure TUI is ready even if WindowSizeMsg is delayed
		if !m.ready && m.width > 0 && m.height > 0 {
			m.ready = true
		}
	}

	// Update animated values for smooth transitions on every update cycle
	m.updateAnimatedValues()

	// TODO: Replace with real tool execution when backend is implemented
	// Currently showing execution preparation only - no simulation


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
	// Simplified ready check - only check if we have minimal dimensions
	if m.width < 10 || m.height < 5 {
		return "Initializing... (Terminal too small)"
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

// executeSelectedWorkflows prepares execution of selected workflows (no backend yet)
func (m *model) executeSelectedWorkflows() {
	// Recover from any panics
	defer func() {
		if r := recover(); r != nil {
			m.logSystemMessage("error", "Panic in executeSelectedWorkflows", "panic", r)
			m.liveOutput = append(m.liveOutput, fmt.Sprintf("ERROR: Unexpected error in workflow execution: %v", r))
			m.updateLiveOutput()
		}
	}()
	
	// DEBUG: Confirm function is called
	m.logSystemMessage("debug", "executeSelectedWorkflows called")
	
	// DEBUG: Check if components are initialized
	if m.workflowOrchestrator == nil {
		m.logSystemMessage("error", "workflowOrchestrator is nil")
		return
	}
	if m.executionEngine == nil {
		m.logSystemMessage("error", "executionEngine is nil")
		return
	}
	
	ctx := context.Background()
	
	// Clear previous execution state
	items := []list.Item{}
	m.tools = []toolExecution{}
	m.liveOutput = []string{
		"=== IPCrawler Workflow Execution Starting ===",
		"",
	}

	// Get the current target from session
	target := m.getTarget()
	if target == "" {
		m.liveOutput = append(m.liveOutput, "ERROR: No target specified. Please restart and enter a target.")
		m.updateLiveOutput()
		return
	}

	m.liveOutput = append(m.liveOutput, fmt.Sprintf("Target: %s", target))
	m.liveOutput = append(m.liveOutput, "")

	// DEBUG: Show selected workflows
	m.logSystemMessage("debug", "Selected workflows count", "count", len(m.selectedWorkflows))
	
	// Queue selected workflows for execution
	selectedCount := 0
	for workflowName, selected := range m.selectedWorkflows {
		if selected {
			selectedCount++
			// Load the real workflow definition
			m.logSystemMessage("debug", "Loading workflow definition", "workflow", workflowName)
			workflow, err := m.loadWorkflowDefinition(workflowName)
			if err != nil {
				m.liveOutput = append(m.liveOutput, fmt.Sprintf("ERROR loading workflow %s: %v", workflowName, err))
				m.logSystemMessage("error", "Failed to load workflow", "workflow", workflowName, "error", err)
				continue
			}

			// Queue workflow for execution
			if err := m.workflowOrchestrator.QueueWorkflow(workflow, target); err != nil {
				m.liveOutput = append(m.liveOutput, fmt.Sprintf("ERROR queueing workflow %s: %v", workflowName, err))
				continue
			}

			m.liveOutput = append(m.liveOutput, fmt.Sprintf("✓ Queued workflow: %s", workflow.Name))
			
			// Add to execution overview
			items = append(items, executionItem{
				name:        workflowName,
				description: fmt.Sprintf("%s (Queued)", workflow.Name),
				status:      "queued",
			})

			// Add workflow header to tools list
			m.tools = append(m.tools, toolExecution{
				Name:   fmt.Sprintf("[%s]", workflow.Name),
				Status: "queued",
				Output: "",
			})

			// Add each step from the workflow to the tools list
			for _, step := range workflow.Steps {
				m.tools = append(m.tools, toolExecution{
					Name:   fmt.Sprintf("  → %s (%s)", step.Tool, strings.Join(step.Modes, ",")),
					Status: "queued",
					Output: "",
				})
			}
		}
	}

	if selectedCount == 0 {
		m.liveOutput = append(m.liveOutput, "No workflows selected for execution.")
		m.updateLiveOutput()
		return
	}

	m.liveOutput = append(m.liveOutput, "")
	m.liveOutput = append(m.liveOutput, fmt.Sprintf("Queued %d workflows for execution", selectedCount))
	m.liveOutput = append(m.liveOutput, "")
	m.liveOutput = append(m.liveOutput, "=== Starting Workflow Execution ===")
	m.liveOutput = append(m.liveOutput, "")

	// Update UI with queued state
	m.scanOverviewList.SetItems(items)
	m.updateLiveOutput()

	// Start workflow execution asynchronously
	go m.monitorWorkflowExecution(ctx)

	// Execute queued workflows
	if err := m.workflowOrchestrator.ExecuteQueuedWorkflows(ctx); err != nil {
		m.liveOutput = append(m.liveOutput, fmt.Sprintf("ERROR starting workflow execution: %v", err))
		m.updateLiveOutput()
	}

	// Clear selected workflows after execution starts
	m.selectedWorkflows = make(map[string]bool)
	m.updateWorkflowList()

	// Log the execution start
	m.logSystemMessage("info", "Workflow execution started", "workflows", selectedCount, "target", target)
}

// executeWorkflowsCmd creates a command to execute selected workflows asynchronously
func (m *model) executeWorkflowsCmd() tea.Cmd {
	// Get selected workflows
	selectedWorkflows := make(map[string]bool)
	for k, v := range m.selectedWorkflows {
		if v {
			selectedWorkflows[k] = v
		}
	}
	
	m.logSystemMessage("info", "executeWorkflowsCmd called", "selected_count", len(selectedWorkflows))
	
	if len(selectedWorkflows) == 0 {
		m.logSystemMessage("warning", "No workflows selected, returning nil command")
		return nil
	}

	// Get target from session
	target := m.getTarget()
	m.logSystemMessage("info", "Target retrieved", "target", target)
	
	if target == "" {
		m.logSystemMessage("error", "No target found in session")
		return func() tea.Msg {
			return workflowCompletedMsg{
				WorkflowName: "validation",
				Success:      false,
				Error:        "No target specified. Please restart and enter a target.",
			}
		}
	}

	return func() tea.Msg {
		// Initialize execution state
		queuedWorkflows := make([]string, 0, len(selectedWorkflows))
		
		ctx := context.Background()
		
		// Queue workflows for execution
		for workflowName := range selectedWorkflows {
			// Load workflow definition
			workflow, err := m.loadWorkflowDefinitionForCmd(workflowName)
			if err != nil {
				return workflowCompletedMsg{
					WorkflowName: workflowName,
					Success:      false,
					Error:        fmt.Sprintf("Failed to load workflow: %v", err),
				}
			}

			// Queue workflow
			if err := m.workflowOrchestrator.QueueWorkflow(workflow, target); err != nil {
				return workflowCompletedMsg{
					WorkflowName: workflowName,
					Success:      false,
					Error:        fmt.Sprintf("Failed to queue workflow: %v", err),
				}
			}

			queuedWorkflows = append(queuedWorkflows, workflowName)
		}

		// Actually start the workflow execution (this is the key fix!)
		go func() {
			// Execute queued workflows - this is the real backend execution
			if err := m.workflowOrchestrator.ExecuteQueuedWorkflows(ctx); err != nil {
				m.logSystemMessage("error", "Failed to start workflow execution", "error", err)
			}
		}()

		return workflowStartedMsg{
			WorkflowName: strings.Join(queuedWorkflows, ", "),
			Target:       target,
			QueueSize:    len(queuedWorkflows),
		}
	}
}

// loadWorkflowDefinitionForCmd is a safer version for use in commands
func (m *model) loadWorkflowDefinitionForCmd(workflowKey string) (*executor.Workflow, error) {
	// Map from description keys to actual workflow files
	workflowFiles := map[string]string{
		"reconnaissance": "workflows/reconnaissance/port-scanning.yaml",
		"dns-enumeration": "workflows/reconnaissance/dns-enumeration.yaml",
	}

	filePath, exists := workflowFiles[workflowKey]
	if !exists {
		return nil, fmt.Errorf("no workflow file mapped for key: %s", workflowKey)
	}

	// Check if file exists, try alternative locations if not
	possiblePaths := []string{
		filePath,
		filepath.Join(".", filePath),
		filepath.Join("..", filePath),
	}

	var data []byte
	var err error
	for _, path := range possiblePaths {
		data, err = os.ReadFile(path)
		if err == nil {
			break
		}
	}

	if err != nil {
		return nil, fmt.Errorf("workflow file not found in any location: %v", err)
	}

	// Define a temporary struct with proper YAML tags for unmarshaling
	type yamlWorkflowStep struct {
		Name               string            `yaml:"name"`
		Tool               string            `yaml:"tool"`
		Description        string            `yaml:"description"`
		Modes              []string          `yaml:"modes"`
		Concurrent         bool              `yaml:"concurrent"`
		CombineResults     bool              `yaml:"combine_results"`
		DependsOn          string            `yaml:"depends_on"`
		StepPriority       string            `yaml:"step_priority"`
		MaxConcurrentTools int               `yaml:"max_concurrent_tools"`
		Variables          map[string]string `yaml:"variables"`
	}
	
	type yamlWorkflow struct {
		Name                   string              `yaml:"name"`
		Description            string              `yaml:"description"`
		Category               string              `yaml:"category"`
		ParallelWorkflow       bool                `yaml:"parallel_workflow"`
		IndependentExecution   bool                `yaml:"independent_execution"`
		MaxConcurrentWorkflows int                 `yaml:"max_concurrent_workflows"`
		WorkflowPriority       string              `yaml:"workflow_priority"`
		Steps                  []yamlWorkflowStep  `yaml:"steps"`
	}

	var yamlWf yamlWorkflow
	if err := yaml.Unmarshal(data, &yamlWf); err != nil {
		return nil, fmt.Errorf("failed to parse workflow YAML: %v", err)
	}

	// Convert to executor.Workflow
	workflow := &executor.Workflow{
		Name:                    yamlWf.Name,
		Description:             yamlWf.Description,
		Category:                yamlWf.Category,
		ParallelWorkflow:        yamlWf.ParallelWorkflow,
		IndependentExecution:    yamlWf.IndependentExecution,
		MaxConcurrentWorkflows:  yamlWf.MaxConcurrentWorkflows,
		WorkflowPriority:        yamlWf.WorkflowPriority,
		Steps:                   make([]*executor.WorkflowStep, len(yamlWf.Steps)),
	}

	// Convert steps
	for i, yamlStep := range yamlWf.Steps {
		workflow.Steps[i] = &executor.WorkflowStep{
			Name:               yamlStep.Name,
			Tool:               yamlStep.Tool,
			Description:        yamlStep.Description,
			Modes:              yamlStep.Modes,
			Concurrent:         yamlStep.Concurrent,
			CombineResults:     yamlStep.CombineResults,
			DependsOn:          yamlStep.DependsOn,
			StepPriority:       yamlStep.StepPriority,
			MaxConcurrentTools: yamlStep.MaxConcurrentTools,
			Variables:          yamlStep.Variables,
		}
	}

	return workflow, nil
}

// monitorWorkflowProgressCmd creates a command to monitor workflow progress
func (m *model) monitorWorkflowProgressCmd() tea.Cmd {
	return tea.Every(2*time.Second, func(t time.Time) tea.Msg {
		// Get actual workflow status from orchestrator
		activeWorkflows := m.workflowOrchestrator.GetActiveWorkflows()
		queuedWorkflows := m.workflowOrchestrator.GetQueueStatus()
		
		// Return queue update message with real data
		return workflowQueueUpdatedMsg{
			QueuedWorkflows: func() []string {
				var workflows []string
				for _, qw := range queuedWorkflows {
					workflows = append(workflows, qw.Workflow.Name)
				}
				return workflows
			}(),
			CompletedCount: func() int {
				total := 0
				completed := 0
				for _, aw := range activeWorkflows {
					total++
					if aw.Status == 2 { // WorkflowStatusCompleted
						completed++
					}
				}
				return completed
			}(),
			TotalCount: len(activeWorkflows) + len(queuedWorkflows),
		}
	})
}

// loadWorkflowDefinition loads the actual workflow YAML definition
func (m *model) loadWorkflowDefinition(workflowKey string) (*executor.Workflow, error) {
	// Map from description keys to actual workflow files
	workflowFiles := map[string]string{
		"reconnaissance": "workflows/reconnaissance/port-scanning.yaml",
		"dns-enumeration": "workflows/reconnaissance/dns-enumeration.yaml",
	}

	filePath, exists := workflowFiles[workflowKey]
	if !exists {
		return nil, fmt.Errorf("no workflow file mapped for key: %s", workflowKey)
	}

	// Check if file exists, try alternative locations if not
	possiblePaths := []string{
		filePath,
		filepath.Join(".", filePath),
		filepath.Join("..", filePath),
	}

	var data []byte
	var err error
	for _, path := range possiblePaths {
		data, err = os.ReadFile(path)
		if err == nil {
			break
		}
	}

	if data == nil {
		return nil, fmt.Errorf("workflow file not found: %s", filePath)
	}

	// DEBUG: Show loaded YAML data
	m.logSystemMessage("debug", "YAML data loaded", "size", len(data))
	
	// Parse the YAML workflow definition
	type yamlWorkflow struct {
		Name                   string `yaml:"name"`
		Description            string `yaml:"description"`
		Category               string `yaml:"category"`
		ParallelWorkflow       bool   `yaml:"parallel_workflow"`
		IndependentExecution   bool   `yaml:"independent_execution"`
		MaxConcurrentWorkflows int    `yaml:"max_concurrent_workflows"`
		WorkflowPriority       string `yaml:"workflow_priority"`
		Steps                  []struct {
			Name            string            `yaml:"name"`
			Tool            string            `yaml:"tool"`
			Description     string            `yaml:"description"`
			Modes           []string          `yaml:"modes"`
			Concurrent      bool              `yaml:"concurrent"`
			CombineResults  bool              `yaml:"combine_results"`
			DependsOn       string            `yaml:"depends_on"`
			StepPriority    string            `yaml:"step_priority"`
			MaxConcurrent   int               `yaml:"max_concurrent_tools"`
			Variables       map[string]string `yaml:"variables"`
		} `yaml:"steps"`
	}

	var yamlWf yamlWorkflow
	if err := yaml.Unmarshal(data, &yamlWf); err != nil {
		return nil, fmt.Errorf("failed to parse workflow YAML: %w", err)
	}

	// Convert to executor.Workflow format
	workflow := &executor.Workflow{
		Name:                    yamlWf.Name,
		Description:             yamlWf.Description,
		Category:                yamlWf.Category,
		ParallelWorkflow:        yamlWf.ParallelWorkflow,
		IndependentExecution:    yamlWf.IndependentExecution,
		MaxConcurrentWorkflows:  yamlWf.MaxConcurrentWorkflows,
		WorkflowPriority:        yamlWf.WorkflowPriority,
		Steps:                   make([]*executor.WorkflowStep, len(yamlWf.Steps)),
	}

	// Convert steps
	for i, yamlStep := range yamlWf.Steps {
		workflow.Steps[i] = &executor.WorkflowStep{
			Name:                yamlStep.Name,
			Tool:                yamlStep.Tool,
			Description:         yamlStep.Description,
			Modes:               yamlStep.Modes,
			Concurrent:          yamlStep.Concurrent,
			CombineResults:      yamlStep.CombineResults,
			DependsOn:           yamlStep.DependsOn,
			StepPriority:        yamlStep.StepPriority,
			MaxConcurrentTools:  yamlStep.MaxConcurrent,
			Variables:           yamlStep.Variables,
		}
	}

	return workflow, nil
}

// getTarget retrieves the target from the model (runtime-only, no session files)
func (m *model) getTarget() string {
	return m.scanTarget
}

// monitorWorkflowExecution monitors ongoing workflow executions and updates the UI
func (m *model) monitorWorkflowExecution(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Get active workflows
			activeWorkflows := m.workflowOrchestrator.GetActiveWorkflows()
			queuedWorkflows := m.workflowOrchestrator.GetQueueStatus()

			// Update UI based on workflow states
			allCompleted := len(activeWorkflows) == 0 && len(queuedWorkflows) == 0

			if allCompleted && len(m.liveOutput) > 0 && !strings.Contains(m.liveOutput[len(m.liveOutput)-1], "=== Execution Complete ===") {
				m.liveOutput = append(m.liveOutput, "")
				m.liveOutput = append(m.liveOutput, "=== Execution Complete ===")
				m.liveOutput = append(m.liveOutput, "All workflows have finished executing.")
				m.liveOutput = append(m.liveOutput, "Check the workspace directory for output files.")
				m.updateLiveOutput()
				return
			}

			// Update workflow status in tools list
			for i, tool := range m.tools {
				// Update status based on execution state
				if strings.HasPrefix(tool.Name, "[") {
					// This is a workflow header
					workflowName := strings.Trim(strings.Split(tool.Name, "]")[0], "[")
					
					// Check if this workflow is running
					running := false
					for key := range activeWorkflows {
						if strings.Contains(key, workflowName) {
							running = true
							break
						}
					}
					
					if running {
						m.tools[i].Status = "running"
					} else if tool.Status == "running" {
						m.tools[i].Status = "completed"
					}
				}
			}
		}
	}
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
	
	// Keep debug logging only in TUI, not console
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

// loadWorkflowFromPath loads a workflow from a specific file path
func loadWorkflowFromPath(filePath string) (*executor.Workflow, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow file %s: %v", filePath, err)
	}

	// Define a temporary struct with proper YAML tags for unmarshaling
	type yamlWorkflowStep struct {
		Name               string            `yaml:"name"`
		Tool               string            `yaml:"tool"`
		Description        string            `yaml:"description"`
		Modes              []string          `yaml:"modes"`
		Concurrent         bool              `yaml:"concurrent"`
		CombineResults     bool              `yaml:"combine_results"`
		DependsOn          string            `yaml:"depends_on"`
		StepPriority       string            `yaml:"step_priority"`
		MaxConcurrentTools int               `yaml:"max_concurrent_tools"`
		Variables          map[string]string `yaml:"variables"`
	}
	
	type yamlWorkflow struct {
		Name                   string              `yaml:"name"`
		Description            string              `yaml:"description"`
		Category               string              `yaml:"category"`
		ParallelWorkflow       bool                `yaml:"parallel_workflow"`
		IndependentExecution   bool                `yaml:"independent_execution"`
		MaxConcurrentWorkflows int                 `yaml:"max_concurrent_workflows"`
		WorkflowPriority       string              `yaml:"workflow_priority"`
		Steps                  []yamlWorkflowStep  `yaml:"steps"`
	}

	var yamlWf yamlWorkflow
	if err := yaml.Unmarshal(data, &yamlWf); err != nil {
		return nil, fmt.Errorf("failed to parse workflow YAML %s: %v", filePath, err)
	}

	// Convert to executor.Workflow
	workflow := &executor.Workflow{
		Name:                    yamlWf.Name,
		Description:             yamlWf.Description,
		Category:                yamlWf.Category,
		ParallelWorkflow:        yamlWf.ParallelWorkflow,
		IndependentExecution:    yamlWf.IndependentExecution,
		MaxConcurrentWorkflows:  yamlWf.MaxConcurrentWorkflows,
		WorkflowPriority:        yamlWf.WorkflowPriority,
		Steps:                   make([]*executor.WorkflowStep, len(yamlWf.Steps)),
	}

	// Convert steps
	for i, yamlStep := range yamlWf.Steps {
		workflow.Steps[i] = &executor.WorkflowStep{
			Name:               yamlStep.Name,
			Tool:               yamlStep.Tool,
			Description:        yamlStep.Description,
			Modes:              yamlStep.Modes,
			Concurrent:         yamlStep.Concurrent,
			CombineResults:     yamlStep.CombineResults,
			DependsOn:          yamlStep.DependsOn,
			StepPriority:       yamlStep.StepPriority,
			MaxConcurrentTools: yamlStep.MaxConcurrentTools,
			Variables:          yamlStep.Variables,
		}
	}

	return workflow, nil
}

// discoverAllWorkflows automatically discovers all workflow files in the workflows directory
func discoverAllWorkflows() (map[string]*executor.Workflow, error) {
	workflows := make(map[string]*executor.Workflow)
	
	err := filepath.WalkDir("workflows", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		// Skip descriptions.yaml (metadata only)
		if d.Name() == "descriptions.yaml" {
			return nil
		}
		
		// Process .yaml files
		if strings.HasSuffix(d.Name(), ".yaml") {
			workflow, err := loadWorkflowFromPath(path)
			if err != nil {
				log.Warn("Failed to load workflow", "path", path, "error", err)
				return nil // Continue processing other files
			}
			
			workflowKey := strings.TrimSuffix(d.Name(), ".yaml")
			workflows[workflowKey] = workflow
		}
		
		return nil
	})
	
	return workflows, err
}

// runCLI executes all workflows in CLI mode without TUI
func runCLI(target string) error {
	// Initialize logger for CLI output
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    false,
		ReportTimestamp: true,
		TimeFormat:      time.Kitchen,
		Prefix:          "IPCrawler CLI",
	})
	
	logger.Info("=== IPCrawler CLI Mode ===", "target", target)
	
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %v", err)
	}
	
	// Validate target
	if target == "" {
		return fmt.Errorf("target cannot be empty")
	}
	
	// Create workspace directory
	sanitizedTarget := sanitizeTargetForPath(target)
	timestamp := time.Now().Unix()
	workspaceDir := filepath.Join(cfg.Output.WorkspaceBase, fmt.Sprintf("%s_%d", sanitizedTarget, timestamp))
	
	if err := createWorkspaceStructure(workspaceDir); err != nil {
		return fmt.Errorf("failed to create workspace: %v", err)
	}
	
	logger.Info("Workspace created", "path", workspaceDir)
	
	// Set up workspace file logging
	debugLogger, infoLogger, rawLogger, err := setupWorkspaceLogging(workspaceDir)
	if err != nil {
		return fmt.Errorf("failed to setup workspace logging: %v", err)
	}
	// Note: File handles will be closed when the function exits
	
	// Make loggers available globally for executors
	setGlobalLoggers(debugLogger, infoLogger, rawLogger)
	
	// Discover all workflows
	workflows, err := discoverAllWorkflows()
	if err != nil {
		return fmt.Errorf("failed to discover workflows: %v", err)
	}
	
	if len(workflows) == 0 {
		return fmt.Errorf("no workflows found in workflows directory")
	}
	
	// Log discovered workflows
	workflowNames := make([]string, 0, len(workflows))
	for name, workflow := range workflows {
		workflowNames = append(workflowNames, name)
		logger.Info("Discovered workflow", "name", name, "title", workflow.Name, "description", workflow.Description)
	}
	
	logger.Info("Starting workflow execution", "count", len(workflows), "workflows", strings.Join(workflowNames, ", "))
	
	// Initialize execution engine and orchestrator
	executionEngine := executor.NewToolExecutionEngine(cfg, "")
	
	// Set the workspace base directory for consistent path resolution
	executionEngine.SetWorkspaceBase(workspaceDir)
	
	// Set up workspace logging for tool execution engine
	if err := executionEngine.SetWorkspaceLoggers(workspaceDir); err != nil {
		return fmt.Errorf("failed to setup tool execution engine logging: %v", err)
	}
	
	workflowExecutor := executor.NewWorkflowExecutor(executionEngine)
	workflowOrchestrator := executor.NewWorkflowOrchestrator(workflowExecutor, cfg)
	
	// Set up workspace logging for workflow orchestrator
	if err := workflowOrchestrator.SetWorkspaceLoggers(workspaceDir); err != nil {
		return fmt.Errorf("failed to setup workflow orchestrator logging: %v", err)
	}
	
	// Set up status callback for CLI logging
	workflowOrchestrator.SetStatusCallback(func(workflowName, target, status, message string) {
		logger.Info("Workflow status", "workflow", workflowName, "target", target, "status", status, "message", message)
	})
	
	// Queue all workflows
	var ctx context.Context
	var cancel context.CancelFunc
	
	// Set timeout from configuration
	if cfg.Tools.CLIMode.ExecutionTimeoutSeconds > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(cfg.Tools.CLIMode.ExecutionTimeoutSeconds)*time.Second)
		logger.Info("CLI execution timeout set", "seconds", cfg.Tools.CLIMode.ExecutionTimeoutSeconds)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
		logger.Info("CLI execution timeout disabled (unlimited)")
	}
	defer cancel()
	
	for workflowName, workflow := range workflows {
		logger.Info("Queueing workflow", "name", workflowName, "title", workflow.Name)
		if err := workflowOrchestrator.QueueWorkflow(workflow, target); err != nil {
			logger.Error("Failed to queue workflow", "name", workflowName, "error", err)
			continue
		}
	}
	
	// Execute queued workflows
	logger.Info("Executing queued workflows...")
	if err := workflowOrchestrator.ExecuteQueuedWorkflows(ctx); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			logger.Warn("Workflow execution timed out", "timeout_seconds", cfg.Tools.CLIMode.ExecutionTimeoutSeconds)
		}
		return fmt.Errorf("failed to execute workflows: %v", err)
	}
	
	logger.Info("All workflows completed successfully")
	return nil
}

// Helper functions for CLI mode
func sanitizeTargetForPath(target string) string {
	// Replace special characters for safe directory names
	sanitized := strings.ReplaceAll(target, ".", "_")
	sanitized = strings.ReplaceAll(sanitized, ":", "_")
	sanitized = strings.ReplaceAll(sanitized, "/", "_")
	sanitized = strings.ReplaceAll(sanitized, "\\", "_")
	return sanitized
}

func createWorkspaceStructure(workspaceDir string) error {
	// Create base workspace directory
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return err
	}
	
	// Create subdirectories
	subdirs := []string{"logs/info", "logs/debug", "logs/error", "logs/warning", "raw", "scans", "reports"}
	for _, subdir := range subdirs {
		if err := os.MkdirAll(filepath.Join(workspaceDir, subdir), 0755); err != nil {
			return err
		}
	}
	
	return nil
}

// setupWorkspaceLogging creates file loggers for the workspace
func setupWorkspaceLogging(workspaceDir string) (*log.Logger, *log.Logger, *log.Logger, error) {
	// Create debug logger
	debugFile, err := os.OpenFile(filepath.Join(workspaceDir, "logs/debug/execution.log"), 
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create debug log file: %v", err)
	}
	
	debugLogger := log.NewWithOptions(debugFile, log.Options{
		ReportCaller:    false,
		ReportTimestamp: true,
		TimeFormat:      time.RFC3339,
		Prefix:          "DEBUG",
	})
	
	// Create info logger
	infoFile, err := os.OpenFile(filepath.Join(workspaceDir, "logs/info/workflow.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create info log file: %v", err)
	}
	
	infoLogger := log.NewWithOptions(infoFile, log.Options{
		ReportCaller:    false,
		ReportTimestamp: true,
		TimeFormat:      time.RFC3339,
		Prefix:          "INFO",
	})
	
	// Create raw output logger
	rawFile, err := os.OpenFile(filepath.Join(workspaceDir, "raw/tool_output.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create raw output file: %v", err)
	}
	
	rawLogger := log.NewWithOptions(rawFile, log.Options{
		ReportCaller:    false,
		ReportTimestamp: true,
		TimeFormat:      time.RFC3339,
		Prefix:          "RAW",
	})
	
	return debugLogger, infoLogger, rawLogger, nil
}

// Global loggers for executor modules
var (
	globalDebugLogger *log.Logger
	globalInfoLogger  *log.Logger
	globalRawLogger   *log.Logger
)

// setGlobalLoggers makes the workspace loggers available to executor modules
func setGlobalLoggers(debugLogger, infoLogger, rawLogger *log.Logger) {
	globalDebugLogger = debugLogger
	globalInfoLogger = infoLogger
	globalRawLogger = rawLogger
}

// logDebug writes debug messages to both console and file
func logDebug(msg string, args ...interface{}) {
	// Always show on console for CLI mode
	if len(args) > 0 {
		fmt.Printf("[DEBUG] "+msg+"\n", args...)
	} else {
		fmt.Printf("[DEBUG] %s\n", msg)
	}
	
	// Also write to file if available
	if globalDebugLogger != nil {
		if len(args) > 0 {
			globalDebugLogger.Debugf(msg, args...)
		} else {
			globalDebugLogger.Debug(msg)
		}
	}
}

// logRaw writes raw tool output to both console and file
func logRaw(toolName, mode, output string) {
	// Always show on console for CLI mode
	fmt.Printf("\n=== RAW OUTPUT: %s %s ===\n", toolName, mode)
	fmt.Print(output)
	fmt.Printf("=== END OUTPUT ===\n\n")
	
	// Also write to file if available
	if globalRawLogger != nil {
		globalRawLogger.Infof("=== %s %s ===\n%s", toolName, mode, output)
	}
}

func main() {
	// Check for registry command line arguments first
	if len(os.Args) > 1 && os.Args[1] == "registry" {
		if err := runRegistryCommand(os.Args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Registry command failed: %v\n", err)
			os.Exit(1)
		}
		return
	}
	
	// Check for no-tui CLI mode
	if len(os.Args) >= 3 && os.Args[1] == "no-tui" {
		target := os.Args[2]
		if err := runCLI(target); err != nil {
			fmt.Fprintf(os.Stderr, "CLI execution failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// IPCrawler TUI - Main Entry Point
	// This should only be called via shell script launcher from 'make run'
	runTUI()
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
		
		// Get current executable path
		execPath, err := os.Executable()
		if err != nil {
			fmt.Printf("\nFailed to get executable path: %v\n", err)
			fmt.Println("Fallback: Please run 'sudo make run' manually")
			os.Exit(1)
		}
		
		// Restart with sudo using the direct binary path (avoid recursive make run)
		cmd := exec.Command("sudo", execPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		// Set environment variable to indicate this is a sudo restart
		cmd.Env = append(os.Environ(), "IPCRAWLER_SUDO_RESTART=1")
		
		fmt.Print("\nStarting with elevated privileges...")
		time.Sleep(1 * time.Second)
		
		// Replace current process with sudo version
		err = cmd.Run()
		if err != nil {
			fmt.Printf("\nFailed to restart with sudo: %v\n", err)
			fmt.Println("Fallback: Please run 'sudo make run' manually")
			
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

