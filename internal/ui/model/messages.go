package model

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/carlosm/ipcrawler/internal/ui/layout"
)

// Core message types for the IPCrawler TUI

// WorkflowUpdateMsg represents a workflow state change
type WorkflowUpdateMsg struct {
	WorkflowID  string
	Status      string  // running, completed, failed, pending
	Progress    float64 // 0.0 to 1.0
	Description string
	Error       error
	Duration    time.Duration
	StartTime   time.Time
}

// ToolExecutionMsg represents a tool execution event
type ToolExecutionMsg struct {
	ToolName   string
	WorkflowID string
	Status     string // running, completed, failed
	Progress   float64
	Output     string
	Error      error
	Duration   time.Duration
	Args       []string
}

// ToolStartMsg represents a tool start event
type ToolStartMsg struct {
	ToolName   string
	WorkflowID string
	Args       []string
}

// LogMsg represents a log entry
type LogMsg struct {
	Level     string // debug, info, warn, error
	Message   string
	Timestamp time.Time
	Category  string // workflow, tool, system
	Data      map[string]interface{}
}

// SystemStatsMsg represents system resource information
type SystemStatsMsg struct {
	CPUUsage    float64
	MemoryUsage float64
	DiskUsage   float64
	ActiveTasks int
	CompletedTasks int
	FailedTasks int
	Timestamp   time.Time
}

// ResizeMsg is an alias for terminal resize events
type ResizeMsg = tea.WindowSizeMsg

// FocusMsg represents focus changes between components
type FocusMsg struct {
	Component FocusedComponent
}

// HelpToggleMsg toggles the help panel
type HelpToggleMsg struct{}

// LayoutChangeMsg triggers a layout recalculation
type LayoutChangeMsg struct {
	NewLayout layout.LayoutMode
}

// ErrorMsg represents application errors
type ErrorMsg struct {
	Error   error
	Context string
	Fatal   bool
}

// QuitMsg represents application termination
type QuitMsg struct{}

// TickMsg represents periodic updates (for spinners, progress, etc.)
type TickMsg struct {
	Time time.Time
}

// SimulatorMsg represents messages from the workflow simulator
type SimulatorMsg struct {
	Type string // start, progress, complete, error
	Data interface{}
}

// WorkflowStartMsg indicates a workflow has started
type WorkflowStartMsg struct {
	WorkflowID string
	Description string
	Steps      []string
}

// WorkflowCompleteMsg indicates a workflow has completed
type WorkflowCompleteMsg struct {
	WorkflowID string
	Success    bool
	Duration   time.Duration
	Results    map[string]interface{}
}

// ProgressUpdateMsg represents progress bar updates
type ProgressUpdateMsg struct {
	ID       string
	Progress float64
	Message  string
}

// StatusUpdateMsg represents status panel updates
type StatusUpdateMsg struct {
	Running   int
	Completed int
	Failed    int
	Total     int
}

// ComponentStateMsg represents internal component state changes
type ComponentStateMsg struct {
	Component string
	State     interface{}
}

// KeyMapUpdateMsg updates the current keymap based on context
type KeyMapUpdateMsg struct {
	KeyMap KeyMapContext
}

// ConfigUpdateMsg represents configuration changes
type ConfigUpdateMsg struct {
	Key   string
	Value interface{}
}

// DataRefreshMsg triggers a refresh of displayed data
type DataRefreshMsg struct {
	Component string
}

// NotificationMsg represents user notifications
type NotificationMsg struct {
	Level   string // info, warning, error, success
	Title   string
	Message string
	Timeout time.Duration
}

// ViewportScrollMsg represents viewport scrolling events
type ViewportScrollMsg struct {
	ViewportID string
	Direction  string // up, down, top, bottom
	Amount     int
}

// TableUpdateMsg represents table data updates
type TableUpdateMsg struct {
	TableID string
	Rows    [][]string
	Headers []string
}

// ListUpdateMsg represents list updates
type ListUpdateMsg struct {
	ListID string
	Items  []interface{}
}

// ThemeChangeMsg represents theme switching
type ThemeChangeMsg struct {
	ThemeName string
}

// DebugMsg represents debug information
type DebugMsg struct {
	Component string
	Message   string
	Data      interface{}
}

// BatchMsg allows batching multiple messages
type BatchMsg struct {
	Messages []tea.Msg
}

// InitCompleteMsg indicates initialization is complete
type InitCompleteMsg struct{}

// ShutdownMsg initiates graceful shutdown
type ShutdownMsg struct {
	Reason string
}