package ui

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
)

// Runner manages the TUI lifecycle and provides the interface for the main application
type Runner struct {
	program *tea.Program
	model   *Model
}

// NewRunner creates a new TUI runner
func NewRunner(target string) *Runner {
	model := NewModel(target)
	
	// Create the Bubble Tea program with appropriate options
	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),       // Use alternate screen buffer
		tea.WithMouseCellMotion(), // Enable mouse support
	)
	
	return &Runner{
		program: program,
		model:   &model,
	}
}

// Run starts the TUI and blocks until it exits
func (r *Runner) Run() error {
	// Set up logging to file since stdout is used by TUI
	if os.Getenv("DEBUG") != "" {
		logFile, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			return fmt.Errorf("failed to setup debug logging: %w", err)
		}
		defer logFile.Close()
		
		log.Info("TUI starting", "target", r.model.target)
	}
	
	// Run the program
	_, err := r.program.Run()
	return err
}

// RunAsync starts the TUI in a goroutine and returns immediately
func (r *Runner) RunAsync(ctx context.Context) error {
	go func() {
		if err := r.Run(); err != nil {
			log.Error("TUI error", "error", err)
		}
	}()
	
	// Wait a moment for the TUI to initialize
	time.Sleep(100 * time.Millisecond)
	
	return nil
}

// Send sends a message to the TUI
func (r *Runner) Send(msg tea.Msg) {
	if r.program != nil {
		r.program.Send(msg)
	}
}

// SendWorkflowUpdate sends a workflow update to the TUI
func (r *Runner) SendWorkflowUpdate(id, description, status string, progress float64, duration time.Duration, err error) {
	r.Send(WorkflowUpdateMsg{
		ID:          id,
		Description: description,
		Status:      status,
		Progress:    progress,
		Duration:    duration,
		Error:       err,
	})
}

// SendToolUpdate sends a tool execution update to the TUI
func (r *Runner) SendToolUpdate(name, workflowID, status string, duration time.Duration, args []string, output string, err error) {
	r.Send(ToolUpdateMsg{
		Name:       name,
		WorkflowID: workflowID,
		Status:     status,
		Duration:   duration,
		Args:       args,
		Output:     output,
		Error:      err,
	})
}

// SendLog sends a log message to the TUI
func (r *Runner) SendLog(level, category, message string) {
	r.Send(LogMsg{
		Timestamp: time.Now(),
		Level:     level,
		Category:  category,
		Message:   message,
	})
}

// Quit requests the TUI to quit
func (r *Runner) Quit() {
	if r.program != nil {
		r.program.Quit()
	}
}

// Context returns the TUI's context for cancellation
func (r *Runner) Context() context.Context {
	if r.model != nil {
		return r.model.Context()
	}
	return context.Background()
}

// Interface compatibility with existing Monitor
// These methods provide compatibility with the existing codebase

// Start starts the TUI (compatibility method)
func (r *Runner) Start(ctx context.Context) (context.Context, error) {
	return r.Context(), r.RunAsync(ctx)
}

// Stop stops the TUI (compatibility method)
func (r *Runner) Stop() {
	r.Quit()
}

// RecordWorkflowStart records a workflow start (compatibility method)
func (r *Runner) RecordWorkflowStart(workflowID, target string) {
	r.SendWorkflowUpdate(workflowID, fmt.Sprintf("Workflow for %s", target), "running", 0, 0, nil)
	r.SendLog("INFO", "workflow", fmt.Sprintf("Started workflow: %s", workflowID))
}

// RecordWorkflowComplete records a workflow completion (compatibility method)
func (r *Runner) RecordWorkflowComplete(workflowID, target string, duration time.Duration, err error) {
	status := "completed"
	level := "INFO"
	message := fmt.Sprintf("Completed workflow: %s in %v", workflowID, duration)
	
	if err != nil {
		status = "failed"
		level = "ERROR"
		message = fmt.Sprintf("Failed workflow: %s after %v - %v", workflowID, duration, err)
	}
	
	r.SendWorkflowUpdate(workflowID, "", status, 1.0, duration, err)
	r.SendLog(level, "workflow", message)
}

// RecordToolExecution records a tool execution (compatibility method)
func (r *Runner) RecordToolExecution(tool string, args []string, duration time.Duration, err error) {
	r.RecordToolExecutionWithWorkflow(tool, "unknown", args, duration, err)
}

// RecordToolExecutionWithWorkflow records a tool execution with workflow context (compatibility method)
func (r *Runner) RecordToolExecutionWithWorkflow(tool, workflow string, args []string, duration time.Duration, err error) {
	status := "completed"
	level := "INFO"
	message := fmt.Sprintf("Tool %s executed in %v", tool, duration)
	
	if err != nil {
		status = "failed"
		level = "ERROR"
		message = fmt.Sprintf("Tool %s failed after %v - %v", tool, duration, err)
	}
	
	r.SendToolUpdate(tool, workflow, status, duration, args, "", err)
	r.SendLog(level, "tool", message)
}

// RecordStepExecution records a workflow step execution (compatibility method)
func (r *Runner) RecordStepExecution(workflowID, stepID, stepType string, duration time.Duration, err error) {
	level := "INFO"
	message := fmt.Sprintf("Step %s/%s completed in %v", workflowID, stepID, duration)
	
	if err != nil {
		level = "ERROR"
		message = fmt.Sprintf("Step %s/%s failed after %v - %v", workflowID, stepID, duration, err)
	}
	
	r.SendLog(level, "step", message)
}

// WaitForReady waits for the TUI to be ready (compatibility method)
func (r *Runner) WaitForReady() {
	// Simple wait since the new TUI starts much faster
	time.Sleep(200 * time.Millisecond)
}