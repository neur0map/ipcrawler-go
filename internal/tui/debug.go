package tui

import (
	"fmt"
	"os"
	"time"
)

// DebugModel creates a debug output of the TUI model state
func (m *TabbedTUIModel) DebugModel() {
	if os.Getenv("TUI_DEBUG") != "1" {
		return
	}
	
	fmt.Printf("\n=== TUI Model Debug at %s ===\n", time.Now().Format("15:04:05"))
	fmt.Printf("Target: %s\n", m.target)
	fmt.Printf("Active Tab: %s\n", tabNames[m.activeTab])
	fmt.Printf("Terminal Size: %dx%d\n", m.width, m.height)
	
	fmt.Printf("\nActive Tools (%d):\n", len(m.activeTools))
	for i, tool := range m.activeTools {
		elapsed := time.Since(tool.StartTime)
		fmt.Printf("  %d. %s (%s) - Running for %s\n", 
			i+1, tool.Name, tool.Workflow, formatDuration(elapsed))
	}
	
	fmt.Printf("\nRecent Completions (%d):\n", len(m.recentCompList))
	for i, completion := range m.recentCompList {
		status := "✅"
		if completion.Status == "failed" {
			status = "❌"
		}
		fmt.Printf("  %d. %s %s (%s) - %s in %s\n", 
			i+1, status, completion.Tool, completion.Workflow, 
			completion.Output, formatDuration(completion.Duration))
	}
	
	fmt.Printf("\nWorkflows (%d):\n", len(m.workflows))
	for i, wf := range m.workflows {
		duration := formatDuration(wf.Duration)
		if wf.Status == "running" {
			duration = formatDuration(time.Since(wf.StartTime))
		}
		fmt.Printf("  %d. %s %s - %s (%s)\n", 
			i+1, getStatusIcon(wf.Status), wf.ID, wf.Status, duration)
	}
	
	fmt.Printf("\nSystem Metrics:\n")
	ramMB := float64(m.systemMetrics.RAMUsed) / 1024 / 1024
	fmt.Printf("  RAM: %.1f MB (%.1f%%)\n", ramMB, m.systemMetrics.RAMPercent)
	fmt.Printf("  CPU: %.1f%%\n", m.systemMetrics.CPUPercent)
	fmt.Printf("  PID: %d\n", m.systemMetrics.ProcessPID)
	fmt.Printf("  Last Update: %s\n", m.systemMetrics.UpdateTime.Format("15:04:05"))
	
	fmt.Printf("\nLogs (%d):\n", len(m.logs))
	start := 0
	if len(m.logs) > 5 {
		start = len(m.logs) - 5
	}
	for i, log := range m.logs[start:] {
		fmt.Printf("  %d. [%s] %s %s: %s\n", 
			i+1, log.Timestamp.Format("15:04:05"), 
			getLevelIcon(log.Level), log.Category, log.Message)
	}
	
	fmt.Printf("===============================\n\n")
}