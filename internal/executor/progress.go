package executor

import (
	"fmt"
	"sync"
	"time"

	"github.com/pterm/pterm"
)

// ExecutionTracker manages all tool executions using PTerm MultiPrinter
type ExecutionTracker struct {
	mu         sync.Mutex
	executions map[string]*ExecutionEntry
	multi      *pterm.MultiPrinter
	started    bool
}

// ExecutionEntry represents a single tool execution with its spinner
type ExecutionEntry struct {
	ToolName  string
	Mode      string
	StartTime time.Time
	Spinner   *pterm.SpinnerPrinter
	Key       string
}

// Global execution tracker
var globalTracker = &ExecutionTracker{
	executions: make(map[string]*ExecutionEntry),
	multi:      &pterm.DefaultMultiPrinter,
	started:    false,
}

// SimpleProgress represents a tool's progress (maintains compatibility)
type SimpleProgress struct {
	ToolName  string
	Mode      string
	StartTime time.Time
	key       string
	ticker    *time.Ticker // Keep for compatibility
	done      chan bool    // Keep for compatibility
	mu        sync.Mutex   // Keep for compatibility
}

// NewSimpleProgress creates a new progress indicator using PTerm
func NewSimpleProgress(toolName, mode string) *SimpleProgress {
	key := fmt.Sprintf("%s:%s", toolName, mode)
	
	progress := &SimpleProgress{
		ToolName:  toolName,
		Mode:      mode,
		StartTime: time.Now(),
		key:       key,
		ticker:    time.NewTicker(1 * time.Hour), // Create but don't use
		done:      make(chan bool),               // Create but don't use
	}

	// Register with PTerm tracker
	globalTracker.addExecution(key, toolName, mode)
	
	// Start dummy update loop for compatibility
	go progress.updateLoop()
	
	return progress
}

// addExecution adds a new execution to the PTerm tracker
func (et *ExecutionTracker) addExecution(key, toolName, mode string) {
	et.mu.Lock()
	defer et.mu.Unlock()
	
	// Check for duplicates - prevent same tool/mode from running twice
	if _, exists := et.executions[key]; exists {
		return // Already running
	}
	
	// Start the multi printer if not already started
	if !et.started {
		et.multi.Start()
		et.started = true
	}
	
	// Create a spinner for this execution
	spinner, _ := pterm.DefaultSpinner.
		WithWriter(et.multi.NewWriter()).
		WithText(fmt.Sprintf("%s [%s]", toolName, mode)).
		Start()
	
	// Store the execution entry
	et.executions[key] = &ExecutionEntry{
		ToolName:  toolName,
		Mode:      mode,
		StartTime: time.Now(),
		Spinner:   spinner,
		Key:       key,
	}
}

// updateLoop is kept for compatibility but does nothing
func (sp *SimpleProgress) updateLoop() {
	<-sp.done
}

// Complete marks the tool as completed
func (sp *SimpleProgress) Complete() {
	// Stop the ticker if it exists (for compatibility)
	if sp.ticker != nil {
		sp.ticker.Stop()
	}
	if sp.done != nil {
		select {
		case <-sp.done:
			// Already closed
		default:
			close(sp.done)
		}
	}
	
	globalTracker.completeExecution(sp.key)
}

// Failed marks the tool as failed
func (sp *SimpleProgress) Failed() {
	// Stop the ticker if it exists (for compatibility)
	if sp.ticker != nil {
		sp.ticker.Stop()
	}
	if sp.done != nil {
		select {
		case <-sp.done:
			// Already closed
		default:
			close(sp.done)
		}
	}
	
	globalTracker.failExecution(sp.key)
}

// completeExecution marks an execution as completed
func (et *ExecutionTracker) completeExecution(key string) {
	et.mu.Lock()
	defer et.mu.Unlock()
	
	if entry, exists := et.executions[key]; exists {
		duration := time.Since(entry.StartTime)
		entry.Spinner.Success(fmt.Sprintf("%s [%s] (completed in %s)", 
			entry.ToolName, entry.Mode, formatDuration(duration)))
		
		// Remove from active executions
		delete(et.executions, key)
	}
}

// failExecution marks an execution as failed
func (et *ExecutionTracker) failExecution(key string) {
	et.mu.Lock()
	defer et.mu.Unlock()
	
	if entry, exists := et.executions[key]; exists {
		duration := time.Since(entry.StartTime)
		entry.Spinner.Fail(fmt.Sprintf("%s [%s] (failed after %s)", 
			entry.ToolName, entry.Mode, formatDuration(duration)))
		
		// Remove from active executions
		delete(et.executions, key)
	}
}

// StopAll stops all remaining executions (call at program end)
func StopAll() {
	globalTracker.mu.Lock()
	defer globalTracker.mu.Unlock()
	
	// Stop any remaining spinners
	for _, entry := range globalTracker.executions {
		entry.Spinner.Info(fmt.Sprintf("%s [%s] (interrupted)", entry.ToolName, entry.Mode))
	}
	
	// Stop the multi printer
	if globalTracker.started {
		globalTracker.multi.Stop()
		globalTracker.started = false
	}
	
	// Clear executions
	globalTracker.executions = make(map[string]*ExecutionEntry)
}

// formatDuration formats time duration into human-readable format
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.0fms", float64(d.Nanoseconds())/1e6)
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}

// ClearTracker clears all tracked executions (useful for testing)
func ClearTracker() {
	globalTracker.mu.Lock()
	defer globalTracker.mu.Unlock()
	
	// Stop all active spinners
	for _, entry := range globalTracker.executions {
		entry.Spinner.Stop()
	}
	
	// Reset tracker
	globalTracker.executions = make(map[string]*ExecutionEntry)
	if globalTracker.started {
		globalTracker.multi.Stop()
		globalTracker.started = false
	}
}