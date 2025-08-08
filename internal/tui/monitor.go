package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Monitor provides TUI monitoring capabilities
type Monitor struct {
	model      tea.Model
	prog       *tea.Program
	cancelFunc context.CancelFunc // Add cancel function for stopping scans
}

// NewMonitor creates a new TUI monitor using tabbed interface
func NewMonitor(target string) *Monitor {
	model := NewTabbedTUIModel(target)
	prog := tea.NewProgram(&model, tea.WithAltScreen())
	
	return &Monitor{
		model: &model,
		prog:  prog,
	}
}

// Start begins the TUI monitoring with cancellation support
func (m *Monitor) Start(ctx context.Context) (context.Context, error) {
	// Create a cancellable context
	cancelCtx, cancel := context.WithCancel(ctx)
	m.cancelFunc = cancel
	
	// Pass the cancel function to the model
	if model, ok := m.model.(*TabbedTUIModel); ok {
		model.SetCancelFunc(cancel)
	}
	
	// Start the TUI in a goroutine
	go func() {
		if _, err := m.prog.Run(); err != nil {
			fmt.Printf("TUI error: %v\n", err)
		}
	}()
	
	return cancelCtx, nil
}

// Stop stops the TUI monitoring
func (m *Monitor) Stop() {
	if m.prog != nil {
		m.prog.Quit()
	}
}

// Send methods to update the TUI with real-time data

// SendWorkflowStart notifies the TUI that a workflow has started
func (m *Monitor) SendWorkflowStart(id, description string) {
	if m.prog != nil {
		m.prog.Send(WorkflowStartMsg{
			ID:          id,
			Description: description,
		})
	}
}

// SendWorkflowComplete notifies the TUI that a workflow has completed
func (m *Monitor) SendWorkflowComplete(id string, duration time.Duration, err error) {
	if m.prog != nil {
		m.prog.Send(WorkflowCompleteMsg{
			ID:       id,
			Duration: duration,
			Error:    err,
		})
	}
}

// SendToolExecution notifies the TUI about tool execution
func (m *Monitor) SendToolExecution(tool, workflow string, duration time.Duration, err error, args []string, output string) {
	if m.prog != nil {
		m.prog.Send(ToolExecutionMsg{
			Tool:     tool,
			Workflow: workflow,
			Duration: duration,
			Error:    err,
			Args:     args,
			Output:   output,
		})
	}
}

// SendLog sends a log entry to the TUI
func (m *Monitor) SendLog(level, category, message string, data map[string]interface{}) {
	if m.prog != nil {
		m.prog.Send(LogMsg{
			Timestamp: time.Now(),
			Level:     level,
			Category:  category,
			Message:   message,
			Data:      data,
		})
	}
}

// Interface compatibility for workflow executor
func (m *Monitor) RecordWorkflowStart(workflowID string, target string) {
	m.SendWorkflowStart(workflowID, fmt.Sprintf("Workflow for %s", target))
	m.SendLog("INFO", "workflow", fmt.Sprintf("Started workflow: %s", workflowID), map[string]interface{}{
		"workflow_id": workflowID,
		"target":      target,
	})
}

func (m *Monitor) RecordWorkflowComplete(workflowID string, target string, duration time.Duration, err error) {
	m.SendWorkflowComplete(workflowID, duration, err)
	
	level := "INFO"
	message := fmt.Sprintf("Completed workflow: %s in %v", workflowID, duration)
	if err != nil {
		level = "ERROR"
		message = fmt.Sprintf("Failed workflow: %s after %v - %v", workflowID, duration, err)
	}
	
	m.SendLog(level, "workflow", message, map[string]interface{}{
		"workflow_id": workflowID,
		"target":      target,
		"duration":    duration.String(),
		"error":       err,
	})
}

func (m *Monitor) RecordStepExecution(workflowID, stepID string, stepType string, duration time.Duration, err error) {
	level := "INFO"
	message := fmt.Sprintf("Step %s/%s completed in %v", workflowID, stepID, duration)
	if err != nil {
		level = "WARNING"
		message = fmt.Sprintf("Step %s/%s failed after %v - %v", workflowID, stepID, duration, err)
	}
	
	m.SendLog(level, "step", message, map[string]interface{}{
		"workflow_id": workflowID,
		"step_id":     stepID,
		"step_type":   stepType,
		"duration":    duration.String(),
		"error":       err,
	})
}

func (m *Monitor) RecordToolExecution(tool string, args []string, duration time.Duration, err error) {
	// Extract workflow from context if possible (simplified for now)
	workflow := "unknown"
	
	m.SendToolExecution(tool, workflow, duration, err, args, "")
}

// Enhanced method with workflow context
func (m *Monitor) RecordToolExecutionWithWorkflow(tool, workflow string, args []string, duration time.Duration, err error) {
	// Generate output summary based on tool type
	outputSummary := m.generateOutputSummary(tool, workflow)
	
	m.SendToolExecution(tool, workflow, duration, err, args, outputSummary)
	
	level := "INFO"
	message := fmt.Sprintf("Tool %s executed in %v", tool, duration)
	if err != nil {
		level = "WARNING"
		message = fmt.Sprintf("Tool %s failed after %v - %v", tool, duration, err)
	}
	
	if outputSummary != "" {
		message += fmt.Sprintf(" - %s", outputSummary)
	}
	
	m.SendLog(level, "tool", message, map[string]interface{}{
		"tool":     tool,
		"workflow": workflow,
		"args":     args,
		"duration": duration.String(),
		"error":    err,
		"output":   outputSummary,
	})
}

// RecordToolStart notifies the TUI that a tool has started
func (m *Monitor) RecordToolStart(tool string, workflow string, args []string) {
	if m.prog != nil {
		m.prog.Send(ToolStartMsg{
			Tool:     tool,
			Workflow: workflow,
			Args:     args,
		})
	}
	
	m.SendLog("INFO", "tool", fmt.Sprintf("Starting %s with args: %v", tool, args), map[string]interface{}{
		"tool":     tool,
		"workflow": workflow,
		"args":     args,
	})
}

// generateOutputSummary creates a meaningful summary of tool output
func (m *Monitor) generateOutputSummary(tool, workflow string) string {
	switch tool {
	case "naabu":
		return "Found open ports"
	case "nmap":
		return "Service fingerprinting complete"
	case "dig":
		return "DNS records resolved"
	case "nslookup":
		return "Name lookup complete"
	default:
		return "Execution complete"
	}
}