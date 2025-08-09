package model

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/carlosm/ipcrawler/internal/ui/theme"
)


// FocusedComponent defines which UI component currently has focus
type FocusedComponent int

const (
	FocusWorkflowList FocusedComponent = iota
	FocusWorkflowTable
	FocusOutputViewport
	FocusStatusPanel
	FocusHelpPanel
	FocusMainContent
)

// String returns the string representation of FocusedComponent
func (f FocusedComponent) String() string {
	switch f {
	case FocusWorkflowList:
		return "workflow_list"
	case FocusWorkflowTable:
		return "workflow_table"
	case FocusOutputViewport:
		return "output_viewport"
	case FocusStatusPanel:
		return "status_panel"
	case FocusHelpPanel:
		return "help_panel"
	case FocusMainContent:
		return "main_content"
	default:
		return "unknown"
	}
}

// KeyMapContext defines different keyboard contexts
type KeyMapContext int

const (
	GlobalContext KeyMapContext = iota
	NavigationContext
	TableContext
	ListContext
	ViewportContext
	HelpContext
)

// AppState represents the overall application state
type AppState int

const (
	StateInitializing AppState = iota
	StateReady
	StateRunning
	StateCompleted
	StateError
	StateShuttingDown
)

// WorkflowStatus represents the state of a workflow
type WorkflowStatus struct {
	ID          string
	Status      string
	Description string
	Progress    float64
	Duration    time.Duration
	StartTime   time.Time
	EndTime     time.Time
	Error       error
	Steps       []StepStatus
}

// StepStatus represents the state of a workflow step
type StepStatus struct {
	ID          string
	Name        string
	Status      string
	Progress    float64
	Duration    time.Duration
	Output      string
	Error       error
}

// ToolStatus represents the state of a tool execution
type ToolStatus struct {
	Name       string
	WorkflowID string
	Status     string
	Progress   float64
	Duration   time.Duration
	Output     string
	Error      error
	Args       []string
}

// LogEntry represents a log entry
type LogEntry struct {
	Level     string
	Message   string
	Timestamp time.Time
	Category  string
	Data      map[string]interface{}
}

// SystemStats represents system resource information
type SystemStats struct {
	CPUUsage       float64
	MemoryUsage    float64
	DiskUsage      float64
	ActiveTasks    int
	CompletedTasks int
	FailedTasks    int
	Timestamp      time.Time
}


// NotificationLevel represents notification severity
type NotificationLevel int

const (
	NotificationInfo NotificationLevel = iota
	NotificationWarning
	NotificationError
	NotificationSuccess
)

// Notification represents a user notification
type Notification struct {
	Level   NotificationLevel
	Title   string
	Message string
	Timeout time.Duration
	Created time.Time
}

// AppConfig represents application configuration
type AppConfig struct {
	Theme           string
	AnimationsEnabled bool
	RefreshInterval time.Duration
	LogLevel        string
	MaxLogEntries   int
	ShowHelp        bool
	ColorDisabled   bool
}

// ComponentState represents the state of individual UI components
type ComponentState struct {
	// Bubbles components
	WorkflowList    list.Model
	WorkflowTable   table.Model
	OutputViewport  viewport.Model
	ProgressBar     progress.Model
	Spinner         spinner.Model
	Help            help.Model

	// Custom components
	StatusPanel  StatusPanelState
	NavPanel     NavPanelState
	MainPanel    MainPanelState
}

// StatusPanelState represents the status panel component state
type StatusPanelState struct {
	SystemStats   SystemStats
	ActiveTasks   int
	CompletedTasks int
	FailedTasks   int
	LastUpdate    time.Time
}

// NavPanelState represents the navigation panel component state
type NavPanelState struct {
	SelectedWorkflow string
	FilterText       string
	ShowCompleted    bool
	ShowFailed       bool
}

// MainPanelState represents the main content panel state
type MainPanelState struct {
	CurrentView    string // table, logs, details
	SelectedTool   string
	LogFilter      string
	AutoScroll     bool
}

// WorkflowListItem implements the list.Item interface for workflow display
type WorkflowListItem struct {
	Workflow WorkflowStatus
}

// FilterValue implements list.Item interface
func (w WorkflowListItem) FilterValue() string {
	return w.Workflow.ID
}

// Title implements list.DefaultItem interface
func (w WorkflowListItem) Title() string {
	icon := theme.StatusIcon(w.Workflow.Status)
	return icon + " " + w.Workflow.ID
}

// Description implements list.DefaultItem interface
func (w WorkflowListItem) Description() string {
	progress := ""
	if w.Workflow.Status == "running" {
		progress = formatProgress(w.Workflow.Progress)
	} else if w.Workflow.Duration > 0 {
		progress = formatDuration(w.Workflow.Duration)
	}
	
	if progress != "" {
		return w.Workflow.Description + " • " + progress
	}
	return w.Workflow.Description
}

// ToolTableRow represents a row in the tool execution table
type ToolTableRow struct {
	Tool      string
	Workflow  string
	Status    string
	Duration  string
	Output    string
}

// ToTableRow converts ToolStatus to table row
func (t ToolStatus) ToTableRow() []string {
	icon := theme.StatusIcon(t.Status)
	duration := formatDuration(t.Duration)
	output := truncateString(t.Output, 30)
	
	return []string{
		t.Name,
		t.WorkflowID,
		icon + " " + t.Status,
		duration,
		output,
	}
}

// Helper functions

// formatProgress formats progress as percentage
func formatProgress(progress float64) string {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	return fmt.Sprintf("%.0f%%", progress*100)
}

// formatDuration formats time duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."
	}
	return s[:maxLen-3] + "..."
}

