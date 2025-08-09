package ui

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/carlosm/ipcrawler/internal/ui/model"
	"github.com/carlosm/ipcrawler/pkg/sim"
)

// Monitor provides TUI monitoring capabilities with compatibility for the workflow executor
type Monitor struct {
	program    *tea.Program
	appModel   *model.AppModel
	cancelFunc context.CancelFunc
	readyChan  chan bool
}

// NewMonitor creates a new TUI monitor compatible with the workflow executor interface
func NewMonitor(target string) *Monitor {
	appModel := model.NewAppModel(target)
	
	// Create program with options based on terminal capabilities
	var program *tea.Program
	
	// Create program with options for non-interactive mode
	program = tea.NewProgram(
		appModel,
		tea.WithOutput(os.Stderr),
		tea.WithInput(nil), // No input needed for monitoring
		tea.WithoutSignalHandler(), // We handle our own cancellation
	)
	
	return &Monitor{
		program:   program,
		appModel:  &appModel,
		readyChan: make(chan bool, 1),
	}
}

// Start begins the TUI monitoring with cancellation support
func (m *Monitor) Start(ctx context.Context) (context.Context, error) {
	// Create a cancellable context
	cancelCtx, cancel := context.WithCancel(ctx)
	m.cancelFunc = cancel
	
	// Set the cancel function on the model
	m.appModel.SetCancelFunc(cancel)
	
	// Check if demo mode is enabled
	if shouldRunDemo() {
		m.startWithDemo()
	}
	
	// Start the TUI in a goroutine with better error handling
	go func() {
		if _, err := m.program.Run(); err != nil {
			fmt.Printf("TUI error: %v\n", err)
		}
	}()
	
	// Give the TUI time to initialize
	time.Sleep(1 * time.Second)
	
	// Signal that TUI is ready (after delay)
	select {
	case m.readyChan <- true:
	default:
	}
	
	
	return cancelCtx, nil
}

// WaitForReady waits for the TUI to be ready to receive messages
func (m *Monitor) WaitForReady() {
	select {
	case <-m.readyChan:
		// TUI is ready
	case <-time.After(5 * time.Second):
		// Timeout, proceed anyway
	}
}

// Stop stops the TUI monitoring
func (m *Monitor) Stop() {
	if m.program != nil {
		m.program.Quit()
	}
}

// SendWorkflowStart notifies the TUI that a workflow has started
func (m *Monitor) SendWorkflowStart(id, description string) {
	if m.program != nil {
		m.program.Send(model.WorkflowUpdateMsg{
			WorkflowID:  id,
			Status:      "running",
			Description: description,
			StartTime:   time.Now(),
		})
	}
}

// SendWorkflowComplete notifies the TUI that a workflow has completed
func (m *Monitor) SendWorkflowComplete(id string, duration time.Duration, err error) {
	status := "completed"
	if err != nil {
		status = "failed"
	}
	
	if m.program != nil {
		m.program.Send(model.WorkflowUpdateMsg{
			WorkflowID: id,
			Status:     status,
			Duration:   duration,
			Error:      err,
		})
	}
}

// SendToolExecution notifies the TUI about tool execution
func (m *Monitor) SendToolExecution(tool, workflow string, duration time.Duration, err error, args []string, output string) {
	status := "completed"
	if err != nil {
		status = "failed"
	}
	
	if m.program != nil {
		m.program.Send(model.ToolExecutionMsg{
			ToolName:   tool,
			WorkflowID: workflow,
			Status:     status,
			Duration:   duration,
			Output:     output,
			Error:      err,
			Args:       args,
		})
	}
}

// SendLog sends a log entry to the TUI
func (m *Monitor) SendLog(level, category, message string, data map[string]interface{}) {
	if m.program != nil {
		m.program.Send(model.LogMsg{
			Timestamp: time.Now(),
			Level:     level,
			Category:  category,
			Message:   message,
			Data:      data,
		})
	}
}

// Interface compatibility methods for workflow executor

// RecordWorkflowStart records the start of a workflow
func (m *Monitor) RecordWorkflowStart(workflowID string, target string) {
	m.SendWorkflowStart(workflowID, fmt.Sprintf("Workflow for %s", target))
	m.SendLog("INFO", "workflow", fmt.Sprintf("Started workflow: %s", workflowID), map[string]interface{}{
		"workflow_id": workflowID,
		"target":      target,
	})
}

// RecordWorkflowComplete records the completion of a workflow
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

// RecordStepExecution records the execution of a workflow step
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

// RecordToolExecution records tool execution
func (m *Monitor) RecordToolExecution(tool string, args []string, duration time.Duration, err error) {
	// Extract workflow from context if possible (simplified for now)
	workflow := "unknown"
	
	m.SendToolExecution(tool, workflow, duration, err, args, "")
}

// RecordToolExecutionWithWorkflow records tool execution with workflow context
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
	if m.program != nil {
		m.program.Send(model.ToolStartMsg{
			ToolName:   tool,
			WorkflowID: workflow,
			Args:       args,
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

// startWithDemo starts the TUI with workflow simulator for demo mode
func (m *Monitor) startWithDemo() {
	demoType := os.Getenv("IPCRAWLER_DEMO")
	
	var simulator *sim.WorkflowSimulator
	if demoType == "quick" {
		simulator = sim.CreateQuickDemo(m.appModel.GetTarget())
	} else {
		simulator = sim.NewWorkflowSimulator(m.appModel.GetTarget())
	}
	
	simulator.SetProgram(m.program)
	
	// Start simulator after a short delay
	go func() {
		time.Sleep(2 * time.Second)
		simulator.Start()
	}()
}