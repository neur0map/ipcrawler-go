package tui

import (
	"fmt"
	"runtime"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Navy blue color scheme with modern styling
var (
	primaryColor = lipgloss.Color("#1e3a8a")  // Navy blue
	accentColor  = lipgloss.Color("#3b82f6")  // Bright blue 
	successColor = lipgloss.Color("#059669")  // Green
	warningColor = lipgloss.Color("#d97706")  // Orange
	errorColor   = lipgloss.Color("#dc2626")  // Red
)


// Data structures
type WorkflowStatus struct {
	ID          string
	Description string
	Status      string // "pending", "running", "completed", "failed"
	Progress    float64
	StartTime   time.Time
	Duration    time.Duration
	Error       error
}

type ToolStatus struct {
	Name      string
	Workflow  string
	Status    string
	Duration  time.Duration
	Error     error
	Args      []string
	Output    string
}

type SystemStats struct {
	MemoryUsed     uint64
	MemoryTotal    uint64
	MemoryPercent  float64
	Goroutines     int
	CPUCores       int
	Uptime         time.Duration
}

type LogEntry struct {
	Timestamp time.Time
	Level     string
	Category  string
	Message   string
	Data      map[string]interface{}
}

// Message types for updates
type WorkflowStartMsg struct {
	ID          string
	Description string
}

type WorkflowCompleteMsg struct {
	ID       string
	Duration time.Duration
	Error    error
}

type ToolExecutionMsg struct {
	Tool     string
	Workflow string
	Duration time.Duration
	Error    error
	Args     []string
	Output   string
}

type ToolStartMsg struct {
	Tool     string
	Workflow string
	Args     []string
}

type SystemStatsMsg SystemStats

type LogMsg LogEntry

type TickMsg time.Time

// Helper functions
func getStatusIcon(status string) string {
	switch status {
	case "running":
		return "🚀"
	case "completed":
		return "✅"
	case "failed":
		return "❌"
	case "pending":
		return "⏳"
	default:
		return "❓"
	}
}

func getLevelIcon(level string) string {
	switch level {
	case "ERROR":
		return "🚨"
	case "WARNING":
		return "⚠️"
	case "INFO":
		return "💡"
	case "DEBUG":
		return "🔍"
	default:
		return "📝"
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}

func getSystemStats() SystemStats {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return SystemStats{
		MemoryUsed:     memStats.Alloc,
		MemoryTotal:    memStats.Sys,
		MemoryPercent:  float64(memStats.Alloc) / float64(memStats.Sys) * 100,
		Goroutines:     runtime.NumGoroutine(),
		CPUCores:       runtime.NumCPU(),
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."
	}
	return s[:maxLen-3] + "..."
}