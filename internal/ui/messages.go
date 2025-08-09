package ui

import (
	"time"
)

// Messages for Bubble Tea runtime

// TickMsg represents a time tick for animations and updates
type TickMsg struct {
	Time time.Time
}

// WorkflowUpdateMsg represents a workflow status update
type WorkflowUpdateMsg struct {
	ID          string
	Description string
	Status      string
	Progress    float64
	Duration    time.Duration
	Error       error
}

// ToolUpdateMsg represents a tool execution update
type ToolUpdateMsg struct {
	Name       string
	WorkflowID string
	Status     string
	Duration   time.Duration
	Args       []string
	Output     string
	Error      error
}

// LogMsg represents a log entry
type LogMsg struct {
	Timestamp time.Time
	Level     string
	Category  string
	Message   string
}

// ResizeMsg represents a terminal resize event (wraps tea.WindowSizeMsg)
type ResizeMsg struct {
	Width  int
	Height int
}

// QuitMsg represents a quit request
type QuitMsg struct{}

// ErrorMsg represents an error that occurred
type ErrorMsg struct {
	Error error
}

// InitCompleteMsg indicates initialization is complete
type InitCompleteMsg struct{}

// RefreshMsg requests a data refresh
type RefreshMsg struct{}