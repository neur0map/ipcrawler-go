package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/neur0map/ipcrawler/internal/config"
	"github.com/neur0map/ipcrawler/internal/output"
	"github.com/neur0map/ipcrawler/internal/tools/naabu"
	"github.com/neur0map/ipcrawler/internal/tools/nmap"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// Workflow represents a complete workflow definition with enhanced parallelism support
type Workflow struct {
	Name        string
	Description string
	Category    string
	Steps       []*WorkflowStep

	// Enhanced workflow-level parallelism controls
	ParallelWorkflow       bool   // Can run simultaneously with other workflows
	IndependentExecution   bool   // Doesn't need to wait for external dependencies
	MaxConcurrentWorkflows int    // Maximum number of workflows that can run in parallel
	WorkflowPriority       string // "low", "medium", "high" - workflow execution priority
}

// WorkflowStep represents a single step in a workflow
type WorkflowStep struct {
	Name           string
	Tool           string
	Description    string
	Modes          []string
	Concurrent     bool
	CombineResults bool
	DependsOn      string
	Variables      map[string]string // Variable mappings for this step

	// Enhanced parallelism controls
	StepPriority       string // "low", "medium", "high" - execution priority
	MaxConcurrentTools int    // Maximum number of tool instances to run simultaneously
}

// WorkflowResult represents the result of executing a workflow step
type WorkflowResult struct {
	StepName     string
	Tool         string
	Modes        []string
	Success      bool
	Results      []*ExecutionResult
	CombinedVars map[string]string
	Duration     time.Duration
	ErrorMessage string
}

// WorkflowExecutor handles execution of multi-step workflows with parallel support
type WorkflowExecutor struct {
	engine    *ToolExecutionEngine
	combiners map[string]interface{} // tool -> result combiner
}

// getPriorityFromString converts string priority to numeric priority for concurrency queue
func getPriorityFromString(priority string) int {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "high":
		return 200 // High priority tools execute first
	case "low":
		return 50 // Low priority tools execute last
	case "medium", "":
		return 100 // Default medium priority
	default:
		return 100 // Fallback to medium for unknown values
	}
}

// WorkflowStatusCallback is a callback function for workflow status updates
type WorkflowStatusCallback func(workflowName, target, status, message string)

// WorkflowOrchestrator manages parallel execution of multiple workflows
type WorkflowOrchestrator struct {
	executor               *WorkflowExecutor
	maxConcurrentWorkflows int
	activeWorkflows        map[string]*WorkflowExecution
	workflowQueue          []*WorkflowQueueItem
	ResourceMonitor        *ResourceMonitor       // Made public for TUI access
	config                 *config.Config         // Configuration reference for priority calculations
	statusCallback         WorkflowStatusCallback // Callback for status updates
	mutex                  sync.RWMutex
	wg                     sync.WaitGroup // WaitGroup to track active workflows

	// Loggers for different output types
	debugLogger *log.Logger
	infoLogger  *log.Logger

	// Output mode for controlling console logging
	outputMode output.OutputMode
}

// WorkflowExecution tracks the execution state of a workflow
type WorkflowExecution struct {
	Workflow       *Workflow
	Target         string
	Status         WorkflowStatus
	StartTime      time.Time
	EndTime        time.Time
	CurrentStep    int
	StepResults    []*WorkflowResult
	Error          error
	TotalSteps     int
	CompletedSteps int
}

// WorkflowQueueItem represents a workflow waiting to be executed
type WorkflowQueueItem struct {
	Workflow     *Workflow
	Target       string
	Priority     int // Calculated priority based on workflow settings
	QueueTime    time.Time
	Dependencies []string // List of workflow names this depends on
}

// WorkflowStatus represents the current state of workflow execution
type WorkflowStatus int

const (
	WorkflowStatusQueued WorkflowStatus = iota
	WorkflowStatusRunning
	WorkflowStatusCompleted
	WorkflowStatusFailed
	WorkflowStatusCancelled
)

// ResourceMonitor tracks system resources for intelligent scheduling
type ResourceMonitor struct {
	maxCPUUsage    float64
	maxMemoryUsage float64
	currentCPU     float64
	currentMemory  float64
	activeTools    int
	maxActiveTools int
	mutex          sync.RWMutex
	debugLogger    *log.Logger
}

// NewWorkflowExecutor creates a new workflow executor
func NewWorkflowExecutor(engine *ToolExecutionEngine) *WorkflowExecutor {
	we := &WorkflowExecutor{
		engine:    engine,
		combiners: make(map[string]interface{}),
	}

	// Register tool-specific result combiners
	we.combiners["naabu"] = &naabu.ResultCombiner{}
	we.combiners["nmap"] = &nmap.ResultCombiner{}

	return we
}

// NewWorkflowOrchestrator creates a new workflow orchestrator for multi-workflow execution
func NewWorkflowOrchestrator(executor *WorkflowExecutor, cfg *config.Config) *WorkflowOrchestrator {
	// Get configuration values with safe defaults
	orchestrationConfig := cfg.Tools.WorkflowOrchestration

	maxConcurrentWorkflows := 3 // Default value
	if orchestrationConfig.MaxConcurrentWorkflows > 0 {
		maxConcurrentWorkflows = orchestrationConfig.MaxConcurrentWorkflows
	}

	maxCPUUsage := 80.0 // Default value
	if orchestrationConfig.ResourceLimits.MaxCPUUsage > 0 {
		maxCPUUsage = orchestrationConfig.ResourceLimits.MaxCPUUsage
	}

	maxMemoryUsage := 80.0 // Default value
	if orchestrationConfig.ResourceLimits.MaxMemoryUsage > 0 {
		maxMemoryUsage = orchestrationConfig.ResourceLimits.MaxMemoryUsage
	}

	maxActiveTools := 15 // Default value
	if orchestrationConfig.ResourceLimits.MaxActiveTools > 0 {
		maxActiveTools = orchestrationConfig.ResourceLimits.MaxActiveTools
	}

	// Setup default loggers (will be overridden when workspace is set)
	debugLogger := log.New(os.Stderr)
	debugLogger.SetLevel(log.DebugLevel)

	infoLogger := log.New(os.Stderr)
	infoLogger.SetLevel(log.InfoLevel)

	return &WorkflowOrchestrator{
		executor:               executor,
		maxConcurrentWorkflows: maxConcurrentWorkflows,
		activeWorkflows:        make(map[string]*WorkflowExecution),
		workflowQueue:          make([]*WorkflowQueueItem, 0),
		config:                 cfg,
		statusCallback:         nil, // Will be set by caller
		debugLogger:            debugLogger,
		infoLogger:             infoLogger,
		ResourceMonitor: &ResourceMonitor{
			maxCPUUsage:    maxCPUUsage,
			maxMemoryUsage: maxMemoryUsage,
			maxActiveTools: maxActiveTools,
			debugLogger:    debugLogger, // Use the same debug logger
		},
	}
}

// SetStatusCallback sets the callback for workflow status updates
func (wo *WorkflowOrchestrator) SetStatusCallback(callback WorkflowStatusCallback) {
	wo.mutex.Lock()
	defer wo.mutex.Unlock()
	wo.statusCallback = callback
}

// SetOutputMode configures the output mode for logging
func (wo *WorkflowOrchestrator) SetOutputMode(mode output.OutputMode) {
	wo.outputMode = mode
}

// SetWorkspaceLoggers sets up loggers that write to workspace log files
func (wo *WorkflowOrchestrator) SetWorkspaceLoggers(workspaceDir string) error {
	debugsDir := filepath.Join(workspaceDir, "logs", "debug")
	infoDir := filepath.Join(workspaceDir, "logs", "info")

	// Create log directories
	if err := os.MkdirAll(debugsDir, 0755); err != nil {
		return fmt.Errorf("failed to create debug log directory: %w", err)
	}
	if err := os.MkdirAll(infoDir, 0755); err != nil {
		return fmt.Errorf("failed to create info log directory: %w", err)
	}

	// Setup debug logger to write to both console and file
	debugFile, err := os.OpenFile(filepath.Join(debugsDir, "workflow.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open debug log file: %w", err)
	}

	// Create MultiWriter based on output mode
	var debugMultiWriter io.Writer
	if wo.outputMode == output.OutputModeVerbose || wo.outputMode == output.OutputModeDebug {
		// In verbose/debug mode, write to both stderr and file
		debugMultiWriter = io.MultiWriter(os.Stderr, debugFile)
	} else {
		// In normal mode, write only to file
		debugMultiWriter = debugFile
	}
	wo.debugLogger = log.New(debugMultiWriter)
	wo.debugLogger.SetReportCaller(false)
	wo.debugLogger.SetReportTimestamp(true)
	wo.debugLogger.SetLevel(log.DebugLevel)

	// Setup info logger to write to both console and file
	infoFile, err := os.OpenFile(filepath.Join(infoDir, "workflow.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open info log file: %w", err)
	}

	// Create MultiWriter based on output mode
	var infoMultiWriter io.Writer
	if wo.outputMode == output.OutputModeVerbose || wo.outputMode == output.OutputModeDebug {
		// In verbose/debug mode, write to both stderr and file
		infoMultiWriter = io.MultiWriter(os.Stderr, infoFile)
	} else {
		// In normal mode, write only to file
		infoMultiWriter = infoFile
	}
	wo.infoLogger = log.New(infoMultiWriter)
	wo.infoLogger.SetReportCaller(false)
	wo.infoLogger.SetReportTimestamp(true)
	wo.infoLogger.SetLevel(log.InfoLevel)

	// Update ResourceMonitor logger
	wo.ResourceMonitor.debugLogger = wo.debugLogger

	return nil
}

// GetExecutionStatus returns current queue and execution status for monitoring
func (wo *WorkflowOrchestrator) GetExecutionStatus() (queuedCount, activeCount int, queuedNames, activeNames []string) {
	wo.mutex.RLock()
	defer wo.mutex.RUnlock()

	queuedCount = len(wo.workflowQueue)
	activeCount = len(wo.activeWorkflows)

	// Get queued workflow names
	queuedNames = make([]string, 0, queuedCount)
	for _, item := range wo.workflowQueue {
		queuedNames = append(queuedNames, item.Workflow.Name)
	}

	// Get active workflow names
	activeNames = make([]string, 0, activeCount)
	for key := range wo.activeWorkflows {
		activeNames = append(activeNames, key)
	}

	return
}

// QueueWorkflow adds a workflow to the execution queue
func (wo *WorkflowOrchestrator) QueueWorkflow(workflow *Workflow, target string) error {
	wo.mutex.Lock()
	defer wo.mutex.Unlock()

	wo.debugLogger.Printf("Queuing workflow: %s for target: %s", workflow.Name, target)

	// Calculate priority based on workflow settings
	priority := wo.calculatePriority(workflow)
	wo.debugLogger.Printf("Calculated priority: %d for workflow: %s", priority, workflow.Name)

	// Create queue item
	queueItem := &WorkflowQueueItem{
		Workflow:     workflow,
		Target:       target,
		Priority:     priority,
		QueueTime:    time.Now(),
		Dependencies: wo.extractDependencies(workflow),
	}

	// Insert into queue based on priority
	wo.insertByPriority(queueItem)

	wo.debugLogger.Printf("Workflow queued successfully. Total queue size: %d", len(wo.workflowQueue))

	return nil
}

// ExecuteQueuedWorkflows processes the workflow queue with intelligent scheduling
func (wo *WorkflowOrchestrator) ExecuteQueuedWorkflows(ctx context.Context) error {
	wo.mutex.Lock()

	wo.debugLogger.Printf("Starting ExecuteQueuedWorkflows - Queue size: %d, Active workflows: %d, Max concurrent: %d",
		len(wo.workflowQueue), len(wo.activeWorkflows), wo.maxConcurrentWorkflows)

	// Update resource monitor before processing
	if err := wo.ResourceMonitor.UpdateResourceUsageFromSystem(); err != nil {
		wo.debugLogger.Printf("Warning: Failed to update resource usage: %v", err)
	}

	for len(wo.workflowQueue) > 0 && len(wo.activeWorkflows) < wo.maxConcurrentWorkflows {
		wo.debugLogger.Printf("Loop iteration - Queue: %d, Active: %d", len(wo.workflowQueue), len(wo.activeWorkflows))

		// Check if we have enough resources
		if !wo.ResourceMonitor.canStartNewWorkflow() {
			wo.debugLogger.Printf("Breaking due to resource constraints")
			break
		}

		// Find next executable workflow (dependencies satisfied)
		nextIndex := wo.findNextExecutableWorkflow()
		if nextIndex == -1 {
			wo.debugLogger.Printf("No executable workflows found (dependencies not satisfied)")
			break // No workflows can be executed right now
		}

		// Remove from queue and start execution
		queueItem := wo.workflowQueue[nextIndex]
		wo.workflowQueue = append(wo.workflowQueue[:nextIndex], wo.workflowQueue[nextIndex+1:]...)

		wo.debugLogger.Printf("Starting workflow: %s for target: %s", queueItem.Workflow.Name, queueItem.Target)

		// Start workflow execution in a separate goroutine
		wo.wg.Add(1)
		go wo.executeWorkflowAsync(ctx, queueItem)
	}

	wo.debugLogger.Printf("ExecuteQueuedWorkflows completed - Final queue size: %d, Active workflows: %d",
		len(wo.workflowQueue), len(wo.activeWorkflows))

	// Release the mutex before waiting for workflows to complete
	wo.mutex.Unlock()

	// Wait for all started workflows to complete
	wo.debugLogger.Printf("Waiting for all workflows to complete...")
	wo.wg.Wait()
	wo.debugLogger.Printf("All workflows completed!")

	return nil
}

// executeWorkflowAsync executes a workflow asynchronously
func (wo *WorkflowOrchestrator) executeWorkflowAsync(ctx context.Context, queueItem *WorkflowQueueItem) {
	wo.debugLogger.Printf("GOROUTINE STARTED: %s for target: %s", queueItem.Workflow.Name, queueItem.Target)

	execution := &WorkflowExecution{
		Workflow:    queueItem.Workflow,
		Target:      queueItem.Target,
		Status:      WorkflowStatusRunning,
		StartTime:   time.Now(),
		TotalSteps:  len(queueItem.Workflow.Steps),
		StepResults: make([]*WorkflowResult, 0),
	}

	wo.debugLogger.Printf("Starting workflow execution: %s for target: %s", queueItem.Workflow.Name, queueItem.Target)

	// Add to active workflows
	wo.debugLogger.Printf("About to acquire mutex for: %s", queueItem.Workflow.Name)
	wo.mutex.Lock()
	wo.debugLogger.Printf("Acquired mutex for: %s", queueItem.Workflow.Name)
	workflowKey := fmt.Sprintf("%s_%s", queueItem.Workflow.Name, queueItem.Target)
	wo.activeWorkflows[workflowKey] = execution
	callback := wo.statusCallback // Capture callback while holding lock
	wo.mutex.Unlock()
	wo.debugLogger.Printf("Released mutex for: %s", queueItem.Workflow.Name)

	// Notify start
	if callback != nil {
		callback(queueItem.Workflow.Name, queueItem.Target, "started", "Workflow execution started")
	}

	// Execute workflow steps IN PARALLEL for true simultaneous execution
	wo.debugLogger.Printf("Workflow has %d steps - executing ALL SIMULTANEOUSLY", len(queueItem.Workflow.Steps))

	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		wo.debugLogger.Printf("Context already cancelled before workflow steps: %v", ctx.Err())
		execution.Error = ctx.Err()
		execution.Status = WorkflowStatusCancelled
		wo.wg.Done()
		return
	default:
		// Continue
	}

	// SMART PARALLEL EXECUTION: Respect dependencies while maximizing parallelism
	stepResults := make([]*WorkflowResult, len(queueItem.Workflow.Steps))
	stepErrors := make([]error, len(queueItem.Workflow.Steps))
	stepCompleted := make([]bool, len(queueItem.Workflow.Steps))
	stepCompletionChans := make([]chan bool, len(queueItem.Workflow.Steps))

	// Initialize completion channels for each step
	for i := range queueItem.Workflow.Steps {
		stepCompletionChans[i] = make(chan bool, 1)
	}

	var stepWg sync.WaitGroup

	// Start all independent steps immediately, dependent steps wait for their dependencies
	for i, step := range queueItem.Workflow.Steps {
		stepWg.Add(1)
		go func(stepIndex int, workflowStep *WorkflowStep) {
			defer stepWg.Done()
			defer func() {
				// Signal completion for dependent steps
				stepCompletionChans[stepIndex] <- true
			}()

			// Wait for dependencies if any
			if workflowStep.DependsOn != "" {
				wo.debugLogger.Printf("Step %d (%s) waiting for dependency: %s", stepIndex+1, workflowStep.Name, workflowStep.DependsOn)

				// Find the dependency step
				depIndex := -1
				for j, depStep := range queueItem.Workflow.Steps {
					if depStep.Name == workflowStep.DependsOn {
						depIndex = j
						break
					}
				}

				if depIndex != -1 {
					// Wait for dependency to complete
					<-stepCompletionChans[depIndex]
					wo.debugLogger.Printf("Dependency satisfied for step %d (%s)", stepIndex+1, workflowStep.Name)
				} else {
					wo.debugLogger.Printf("WARNING: Dependency '%s' not found for step %d (%s)", workflowStep.DependsOn, stepIndex+1, workflowStep.Name)
				}
			} else {
				wo.debugLogger.Printf("STARTING IMMEDIATELY: Step %d: %s (tool: %s, modes: %v) - NO DEPENDENCIES", stepIndex+1, workflowStep.Name, workflowStep.Tool, workflowStep.Modes)
				if callback != nil {
					callback(queueItem.Workflow.Name, queueItem.Target, "step_started",
						fmt.Sprintf("Started step %d/%d: %s", stepIndex+1, len(queueItem.Workflow.Steps), workflowStep.Name))
				}
			}

			wo.debugLogger.Printf("EXECUTING: Step %d: %s", stepIndex+1, workflowStep.Name)

			// Execute step with default options - get validation setting from config
			validateOutput := false // Default fallback
			if wo.config != nil && wo.config.Tools.CLIMode.ValidateOutput {
				validateOutput = wo.config.Tools.CLIMode.ValidateOutput
			}

			options := &ExecutionOptions{
				CaptureOutput:  true,
				ValidateOutput: validateOutput,
			}

			result, err := wo.executor.ExecuteStepWithWorkflow(ctx, workflowStep, queueItem.Target, queueItem.Workflow.Name, options)
			stepResults[stepIndex] = result
			stepErrors[stepIndex] = err
			stepCompleted[stepIndex] = true

			if err != nil {
				wo.debugLogger.Printf("Step FAILED: %s - Error: %v", workflowStep.Name, err)
			} else {
				wo.debugLogger.Printf("Step COMPLETED: %s", workflowStep.Name)
			}

			// Notify step completion immediately when it finishes
			if callback != nil {
				if err != nil {
					callback(queueItem.Workflow.Name, queueItem.Target, "step_failed",
						fmt.Sprintf("Failed step %d/%d: %s - Error: %v", stepIndex+1, len(queueItem.Workflow.Steps), workflowStep.Name, err))
				} else {
					callback(queueItem.Workflow.Name, queueItem.Target, "step_completed",
						fmt.Sprintf("Completed step %d/%d: %s", stepIndex+1, len(queueItem.Workflow.Steps), workflowStep.Name))
				}
			}
		}(i, step)
	}

	// Wait for ALL steps to complete
	wo.debugLogger.Printf("Waiting for all %d steps to complete (with dependencies)...", len(queueItem.Workflow.Steps))
	stepWg.Wait()
	wo.debugLogger.Printf("All steps completed!")

	// Process results and check for failures
	var firstError error
	for i, result := range stepResults {
		if result != nil {
			execution.StepResults = append(execution.StepResults, result)
			if result.Success {
				execution.CompletedSteps++
			}
		}
		if stepErrors[i] != nil && firstError == nil {
			firstError = stepErrors[i]
		}
	}

	// Set overall execution status
	if firstError != nil {
		execution.Error = firstError
		execution.Status = WorkflowStatusFailed
		wo.debugLogger.Printf("Workflow failed with error: %v", firstError)
		if callback != nil {
			callback(queueItem.Workflow.Name, queueItem.Target, "failed", fmt.Sprintf("Workflow failed: %v", firstError))
		}
	}

	// Mark as completed
	execution.EndTime = time.Now()
	if execution.Error == nil {
		execution.Status = WorkflowStatusCompleted
		wo.debugLogger.Printf("Workflow completed successfully: %s", queueItem.Workflow.Name)
		if callback != nil {
			callback(queueItem.Workflow.Name, queueItem.Target, "completed", "Workflow completed successfully")
		}
	}

	// Remove from active workflows
	wo.mutex.Lock()
	delete(wo.activeWorkflows, workflowKey)
	wo.mutex.Unlock()

	// Mark this workflow as done in the WaitGroup
	wo.wg.Done()

	// Note: Removed recursive call to ExecuteQueuedWorkflows to prevent infinite loops
}

// Helper methods for WorkflowOrchestrator

// calculatePriority determines workflow execution priority
func (wo *WorkflowOrchestrator) calculatePriority(workflow *Workflow) int {
	basePriority := 50 // Default priority

	// Get priority weights from config with safe defaults
	priorityWeights := wo.config.Tools.WorkflowOrchestration.PriorityWeights

	highWeight := 30
	if priorityWeights.High > 0 {
		highWeight = priorityWeights.High
	}

	mediumWeight := 10
	if priorityWeights.Medium != 0 {
		mediumWeight = priorityWeights.Medium
	}

	lowWeight := -10
	if priorityWeights.Low != 0 {
		lowWeight = priorityWeights.Low
	}

	independentBonus := 20
	if priorityWeights.IndependentBonus > 0 {
		independentBonus = priorityWeights.IndependentBonus
	}

	parallelBonus := 5
	if priorityWeights.ParallelBonus > 0 {
		parallelBonus = priorityWeights.ParallelBonus
	}

	switch workflow.WorkflowPriority {
	case "high":
		basePriority += highWeight
	case "medium":
		basePriority += mediumWeight
	case "low":
		basePriority += lowWeight
	}

	// Independent workflows get higher priority
	if workflow.IndependentExecution {
		basePriority += independentBonus
	}

	// Parallel-capable workflows get slight boost
	if workflow.ParallelWorkflow {
		basePriority += parallelBonus
	}

	return basePriority
}

// extractDependencies identifies workflow dependencies
func (wo *WorkflowOrchestrator) extractDependencies(workflow *Workflow) []string {
	dependencies := make([]string, 0)

	// If not independent, it may have external dependencies
	if !workflow.IndependentExecution {
		// For now, assume workflows with the same target might depend on each other
		// This can be enhanced with explicit dependency declarations
	}

	return dependencies
}

// insertByPriority inserts a workflow into the queue based on priority
func (wo *WorkflowOrchestrator) insertByPriority(queueItem *WorkflowQueueItem) {
	// Find insertion point based on priority
	insertIndex := len(wo.workflowQueue)
	for i, item := range wo.workflowQueue {
		if queueItem.Priority > item.Priority {
			insertIndex = i
			break
		}
	}

	// Insert at the calculated position
	wo.workflowQueue = append(wo.workflowQueue[:insertIndex],
		append([]*WorkflowQueueItem{queueItem}, wo.workflowQueue[insertIndex:]...)...)
}

// findNextExecutableWorkflow finds the next workflow that can be executed
func (wo *WorkflowOrchestrator) findNextExecutableWorkflow() int {
	for i, queueItem := range wo.workflowQueue {
		// Check if dependencies are satisfied
		if wo.areDependenciesSatisfied(queueItem.Dependencies) {
			return i
		}
	}
	return -1
}

// areDependenciesSatisfied checks if all dependencies for a workflow are met
func (wo *WorkflowOrchestrator) areDependenciesSatisfied(dependencies []string) bool {
	for _, dep := range dependencies {
		// Check if dependency workflow is completed
		if execution, exists := wo.activeWorkflows[dep]; exists {
			if execution.Status != WorkflowStatusCompleted {
				return false
			}
		}
	}
	return true
}

// GetActiveWorkflows returns information about currently running workflows
func (wo *WorkflowOrchestrator) GetActiveWorkflows() map[string]*WorkflowExecution {
	wo.mutex.RLock()
	defer wo.mutex.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[string]*WorkflowExecution)
	for k, v := range wo.activeWorkflows {
		result[k] = v
	}
	return result
}

// GetQueueStatus returns the current workflow queue status
func (wo *WorkflowOrchestrator) GetQueueStatus() []*WorkflowQueueItem {
	wo.mutex.RLock()
	defer wo.mutex.RUnlock()

	// Return a copy
	result := make([]*WorkflowQueueItem, len(wo.workflowQueue))
	copy(result, wo.workflowQueue)
	return result
}

// ResourceMonitor helper methods

// canStartNewWorkflow checks if system resources allow starting a new workflow
func (rm *ResourceMonitor) canStartNewWorkflow() bool {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	// Debug: Always log resource check attempts
	if rm.debugLogger != nil {
		rm.debugLogger.Debug("Checking workflow start permissions",
			"cpu_percent", rm.currentCPU, "cpu_max", rm.maxCPUUsage,
			"memory_percent", rm.currentMemory, "memory_max", rm.maxMemoryUsage,
			"active_tools", rm.activeTools, "max_tools", rm.maxActiveTools)
	}

	// Check CPU and memory limits
	if rm.currentCPU > rm.maxCPUUsage {
		if rm.debugLogger != nil {
			rm.debugLogger.Debug("BLOCKED: CPU usage too high", "current", rm.currentCPU, "max", rm.maxCPUUsage)
		}
		return false
	}

	if rm.currentMemory > rm.maxMemoryUsage {
		if rm.debugLogger != nil {
			rm.debugLogger.Debug("BLOCKED: Memory usage too high", "current", rm.currentMemory, "max", rm.maxMemoryUsage)
		}
		return false
	}

	// Check active tools limit
	if rm.activeTools >= rm.maxActiveTools {
		if rm.debugLogger != nil {
			rm.debugLogger.Debug("BLOCKED: Too many active tools", "current", rm.activeTools, "max", rm.maxActiveTools)
		}
		return false
	}

	if rm.debugLogger != nil {
		rm.debugLogger.Debug("ALLOWED: All resource checks passed")
	}
	return true
}

// updateResourceUsage updates current resource usage metrics
func (rm *ResourceMonitor) updateResourceUsage(cpuUsage, memory float64, activeTools int) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	rm.currentCPU = cpuUsage
	rm.currentMemory = memory
	rm.activeTools = activeTools
}

// UpdateResourceUsageFromSystem automatically updates resource usage using system metrics
func (rm *ResourceMonitor) UpdateResourceUsageFromSystem() error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	// Get CPU usage
	cpuPercents, err := cpu.Percent(0, false)
	if err == nil && len(cpuPercents) > 0 {
		rm.currentCPU = cpuPercents[0]
	}

	// Get memory usage
	memInfo, err := mem.VirtualMemory()
	if err == nil {
		rm.currentMemory = memInfo.UsedPercent
	}

	// Active tools count needs to be updated separately by the orchestrator
	return nil
}

// ExecuteStep executes a single workflow step with parallel mode support
func (we *WorkflowExecutor) ExecuteStep(ctx context.Context, step *WorkflowStep, target string, options *ExecutionOptions) (*WorkflowResult, error) {
	return we.ExecuteStepWithWorkflow(ctx, step, target, "", options)
}

// ExecuteStepWithWorkflow executes a single workflow step with workflow context for unique filenames
func (we *WorkflowExecutor) ExecuteStepWithWorkflow(ctx context.Context, step *WorkflowStep, target, workflowName string, options *ExecutionOptions) (*WorkflowResult, error) {
	startTime := time.Now()

	result := &WorkflowResult{
		StepName:     step.Name,
		Tool:         step.Tool,
		Modes:        step.Modes,
		Success:      false,
		Results:      []*ExecutionResult{},
		CombinedVars: make(map[string]string),
	}

	// Create a copy of options to modify without affecting the original
	var stepOptions *ExecutionOptions
	if options != nil {
		// Copy existing options
		stepOptions = &ExecutionOptions{
			Timeout:        options.Timeout,
			WorkingDir:     options.WorkingDir,
			Environment:    options.Environment,
			CaptureOutput:  options.CaptureOutput,
			ValidateOutput: options.ValidateOutput,
			Priority:       options.Priority,
		}
	} else {
		stepOptions = &ExecutionOptions{
			CaptureOutput: true,
		}
	}

	// Override priority based on step's priority setting
	if step.StepPriority != "" {
		stepOptions.Priority = getPriorityFromString(step.StepPriority)
	} else if stepOptions.Priority == 0 {
		stepOptions.Priority = 100 // Default medium priority
	}

	// Apply variable mappings for this step
	if step.Variables != nil {
		for sourceVar, targetVar := range step.Variables {
			we.engine.GetTemplateResolver().MapWorkflowVariable(sourceVar, targetVar)
		}
	}

	if step.Concurrent && len(step.Modes) > 1 {
		// Execute all modes in parallel
		results, err := we.executeModesParallelWithWorkflow(ctx, step, target, workflowName, stepOptions)
		if err != nil {
			result.ErrorMessage = err.Error()
			result.Duration = time.Since(startTime)
			return result, err
		}
		result.Results = results
	} else {
		// Execute modes sequentially
		for _, mode := range step.Modes {
			execResult, err := we.engine.ExecuteToolWithContext(ctx, step.Tool, mode, target, workflowName, step.Name, stepOptions)
			if err != nil {
				result.ErrorMessage = fmt.Sprintf("mode %s failed: %v", mode, err)
				result.Duration = time.Since(startTime)
				return result, err
			}
			result.Results = append(result.Results, execResult)
		}
	}

	// Combine results if requested and tool has a combiner (even for single results to create magic variables)
	if step.CombineResults && len(result.Results) >= 1 {
		combinedVars, err := we.combineToolResults(step.Tool, result.Results)
		if err != nil {
			result.ErrorMessage = fmt.Sprintf("result combining failed: %v", err)
		} else {
			result.CombinedVars = combinedVars

			// Add combined variables to template resolver
			for varName, varValue := range combinedVars {
				we.engine.GetTemplateResolver().AddVariable(varName, varValue)
			}
		}
	}

	// Check if all executions succeeded
	allSucceeded := true
	for _, execResult := range result.Results {
		if !execResult.Success {
			allSucceeded = false
			break
		}
	}

	result.Success = allSucceeded
	result.Duration = time.Since(startTime)
	return result, nil
}

// executeModesParallel executes multiple modes in parallel using goroutines
func (we *WorkflowExecutor) executeModesParallel(ctx context.Context, step *WorkflowStep, target string, options *ExecutionOptions) ([]*ExecutionResult, error) {
	return we.executeModesParallelWithWorkflow(ctx, step, target, "", options)
}

// executeModesParallelWithWorkflow executes multiple modes in parallel with workflow context
func (we *WorkflowExecutor) executeModesParallelWithWorkflow(ctx context.Context, step *WorkflowStep, target, workflowName string, options *ExecutionOptions) ([]*ExecutionResult, error) {
	var wg sync.WaitGroup
	results := make([]*ExecutionResult, len(step.Modes))
	errors := make([]error, len(step.Modes))

	// Enforce MaxConcurrentTools limit - this prevents any step from consuming all semaphore slots
	maxConcurrent := len(step.Modes) // Default: run all modes in parallel
	if step.MaxConcurrentTools > 0 && step.MaxConcurrentTools < len(step.Modes) {
		maxConcurrent = step.MaxConcurrentTools
	}

	// Create semaphore to limit concurrent executions within this step
	semaphore := make(chan struct{}, maxConcurrent)

	// Execute each mode in a separate goroutine with concurrency control
	for i, mode := range step.Modes {
		wg.Add(1)
		go func(index int, modeName string) {
			defer wg.Done()

			// Acquire semaphore slot
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Execute this mode
			execResult, err := we.engine.ExecuteToolWithContext(ctx, step.Tool, modeName, target, workflowName, step.Name, options)
			results[index] = execResult
			errors[index] = err
		}(i, mode)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Check for errors
	var failedModes []string
	var validResults []*ExecutionResult

	for i, err := range errors {
		if err != nil {
			failedModes = append(failedModes, step.Modes[i])
		} else if results[i] != nil {
			validResults = append(validResults, results[i])
		}
	}

	if len(failedModes) > 0 {
		return validResults, fmt.Errorf("failed modes: %v", failedModes)
	}

	return validResults, nil
}

// combineToolResults combines multiple execution results using tool-specific combiner
func (we *WorkflowExecutor) combineToolResults(toolName string, results []*ExecutionResult) (map[string]string, error) {
	combiner, exists := we.combiners[toolName]
	if !exists {
		return nil, fmt.Errorf("no result combiner registered for tool: %s", toolName)
	}

	// Extract output paths from results
	var outputPaths []string
	for _, result := range results {
		if result.Success && result.OutputPath != "" {
			outputPaths = append(outputPaths, result.OutputPath)
		}
	}

	if len(outputPaths) == 0 {
		return nil, fmt.Errorf("no successful results to combine")
	}

	// Use tool-specific combiner
	switch c := combiner.(type) {
	case *naabu.ResultCombiner:
		return c.CombineResults(outputPaths), nil
	case *nmap.ResultCombiner:
		return c.CombineResults(outputPaths), nil
	default:
		return nil, fmt.Errorf("unsupported combiner type for tool: %s", toolName)
	}
}

// GetRegisteredCombiners returns list of tools that have result combiners
func (we *WorkflowExecutor) GetRegisteredCombiners() []string {
	var tools []string
	for tool := range we.combiners {
		tools = append(tools, tool)
	}
	return tools
}

// ExecuteWorkflow executes a complete workflow with dependency management
func (we *WorkflowExecutor) ExecuteWorkflow(ctx context.Context, steps []*WorkflowStep, target string, options *ExecutionOptions) ([]*WorkflowResult, error) {
	return we.ExecuteWorkflowWithName(ctx, steps, target, "", options)
}

// ExecuteWorkflowWithName executes a complete workflow with workflow context for unique filenames
func (we *WorkflowExecutor) ExecuteWorkflowWithName(ctx context.Context, steps []*WorkflowStep, target, workflowName string, options *ExecutionOptions) ([]*WorkflowResult, error) {
	var results []*WorkflowResult
	completed := make(map[string]bool)

	for _, step := range steps {
		// Check dependencies
		if step.DependsOn != "" && !completed[step.DependsOn] {
			return results, fmt.Errorf("dependency '%s' not completed for step '%s'", step.DependsOn, step.Name)
		}

		// Execute step
		stepResult, err := we.ExecuteStepWithWorkflow(ctx, step, target, workflowName, options)
		if err != nil {
			return results, fmt.Errorf("step '%s' failed: %w", step.Name, err)
		}

		results = append(results, stepResult)
		completed[step.Name] = stepResult.Success

		if !stepResult.Success {
			return results, fmt.Errorf("step '%s' failed", step.Name)
		}
	}

	return results, nil
}
