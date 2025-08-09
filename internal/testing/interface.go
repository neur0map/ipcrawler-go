package simulator

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Simulator provides demo content without actual tool execution
// This interface allows the TUI to demonstrate functionality
// without requiring real backend tools or network access
type Simulator interface {
	// Workflow management
	GetWorkflows() []WorkflowItem
	ExecuteWorkflow(id string) tea.Cmd
	GetWorkflowStatus(id string) WorkflowStatus

	// Tool management
	GetTools() []ToolItem
	ExecuteTool(id string, args map[string]interface{}) tea.Cmd

	// Log streaming simulation
	GetLogs() []LogEntry
	StreamLogs() tea.Cmd

	// Status information
	GetSystemStatus() SystemStatus
	GetMetrics() Metrics
}

// WorkflowItem represents a mock workflow for demonstration
type WorkflowItem struct {
	ID          string                 `json:"id" yaml:"id"`
	Title       string                 `json:"title" yaml:"title"`
	Description string                 `json:"description" yaml:"description"`
	Status      string                 `json:"status" yaml:"status"`
	Tools       []string               `json:"tools" yaml:"tools"`
	Config      map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
}

// FilterValue implements list.Item for Bubbles list component
func (w WorkflowItem) FilterValue() string { return w.Title }

// Title implements list.DefaultItem for Bubbles list component
func (w WorkflowItem) Title() string { return w.Title }

// Description implements list.DefaultItem for Bubbles list component
func (w WorkflowItem) Description() string {
	return w.Description + " • " + w.Status
}

// ToolItem represents a mock tool for demonstration
type ToolItem struct {
	ID          string                 `json:"id" yaml:"id"`
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description" yaml:"description"`
	Category    string                 `json:"category" yaml:"category"`
	Status      string                 `json:"status" yaml:"status"`
	Config      map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
}

// FilterValue implements list.Item for Bubbles list component
func (t ToolItem) FilterValue() string { return t.Name }

// Title implements list.DefaultItem for Bubbles list component
func (t ToolItem) Title() string { return t.Name }

// Description implements list.DefaultItem for Bubbles list component
func (t ToolItem) Description() string {
	return t.Description + " • " + t.Status
}

// LogEntry represents a log entry for streaming simulation
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Source    string    `json:"source"`
	Message   string    `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// WorkflowStatus represents the status of a workflow execution
type WorkflowStatus struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"` // "pending", "running", "completed", "failed"
	Progress   float64   `json:"progress"`
	StartTime  time.Time `json:"start_time"`
	EndTime    *time.Time `json:"end_time,omitempty"`
	Error      string    `json:"error,omitempty"`
	Output     string    `json:"output,omitempty"`
}

// SystemStatus represents overall system status
type SystemStatus struct {
	Status      string        `json:"status"` // "starting", "running", "stopping", "stopped"
	Uptime      time.Duration `json:"uptime"`
	ActiveTasks int           `json:"active_tasks"`
	Completed   int           `json:"completed"`
	Failed      int           `json:"failed"`
	Version     string        `json:"version"`
}

// Metrics represents system performance metrics
type Metrics struct {
	CPU         float64 `json:"cpu"`
	Memory      float64 `json:"memory"`
	ActiveTasks int     `json:"tasks"`
	Throughput  float64 `json:"throughput"`
	ErrorRate   float64 `json:"error_rate"`
}

// Custom Tea messages for simulator events
type WorkflowExecutionMsg struct {
	WorkflowID string `json:"workflow_id"`
	Status     string `json:"status"`
	Progress   float64 `json:"progress"`
	Message    string `json:"message"`
}

type ToolExecutionMsg struct {
	ToolID   string                 `json:"tool_id"`
	Status   string                 `json:"status"`
	Progress float64                `json:"progress"`
	Output   string                 `json:"output"`
	Data     map[string]interface{} `json:"data"`
}

type LogStreamMsg struct {
	Entry LogEntry `json:"entry"`
}

type StatusUpdateMsg struct {
	Status  SystemStatus `json:"status"`
	Metrics Metrics      `json:"metrics"`
}

type ProgressUpdateMsg struct {
	ID       string  `json:"id"`
	Progress float64 `json:"progress"`
	Message  string  `json:"message"`
}