package executor

import (
	"context"
	"sync"
	"time"

	"github.com/charmbracelet/log"
)

// ToolPerformanceProfile defines the performance characteristics of a tool
type ToolPerformanceProfile int

const (
	FastTool   ToolPerformanceProfile = iota // Quick tools like nslookup, ping
	MediumTool                              // Medium tools like naabu, gobuster
	HeavyTool                               // Heavy tools like nmap, sqlmap
)

// NOTE: No hardcoded tool classifications - system learns dynamically from execution times
// All unknown tools start as MediumTool and are reclassified based on actual performance:
// - FastTool: < 5 seconds average execution time  
// - MediumTool: 5-30 seconds average execution time
// - HeavyTool: > 30 seconds average execution time

// ConcurrencyLimits defines limits for each tool type
type ConcurrencyLimits struct {
	FastToolLimit   int
	MediumToolLimit int
	HeavyToolLimit  int
}

// ExecutionRequest represents a tool waiting to be executed
type ExecutionRequest struct {
	ToolName   string
	Profile    ToolPerformanceProfile
	Priority   int
	Context    context.Context
	StartChan  chan struct{} // Signal when execution can start
	CancelFunc context.CancelFunc
}

// ToolPerformanceHistory tracks execution times for dynamic classification
type ToolPerformanceHistory struct {
	TotalExecutions int
	TotalTime       time.Duration
	AverageTime     time.Duration
	LastClassified  ToolPerformanceProfile
	LastUpdate      time.Time
}

// ConcurrencyManager manages dynamic tool execution slots
type ConcurrencyManager struct {
	limits ConcurrencyLimits
	
	// Separate semaphores for each tool type
	fastSem   chan struct{}
	mediumSem chan struct{}
	heavySem  chan struct{}
	
	// Active execution tracking
	activeMutex sync.RWMutex
	activeTools map[string]int // toolName -> active count
	
	// Execution queue
	queueMutex   sync.Mutex
	executionQueue []*ExecutionRequest
	
	// Dynamic tool performance learning
	performanceMutex sync.RWMutex
	performanceHistory map[string]*ToolPerformanceHistory
	
	// Metrics
	metricsMutex sync.RWMutex
	metrics      ConcurrencyMetrics
	
	logger *log.Logger
}

// ConcurrencyMetrics tracks concurrency performance
type ConcurrencyMetrics struct {
	TotalExecuted     int
	QueuedExecutions  int
	AverageWaitTime   time.Duration
	SlotUtilization   map[ToolPerformanceProfile]float64
	PeakConcurrency   map[ToolPerformanceProfile]int
}

// NewConcurrencyManager creates a new dynamic concurrency manager
func NewConcurrencyManager(limits ConcurrencyLimits, logger *log.Logger) *ConcurrencyManager {
	if logger == nil {
		logger = log.New(nil)
		logger.SetLevel(log.ErrorLevel) // Silent by default
	}
	
	return &ConcurrencyManager{
		limits:         limits,
		fastSem:        make(chan struct{}, limits.FastToolLimit),
		mediumSem:      make(chan struct{}, limits.MediumToolLimit),
		heavySem:       make(chan struct{}, limits.HeavyToolLimit),
		activeTools:    make(map[string]int),
		executionQueue: make([]*ExecutionRequest, 0),
		performanceHistory: make(map[string]*ToolPerformanceHistory),
		metrics: ConcurrencyMetrics{
			SlotUtilization: make(map[ToolPerformanceProfile]float64),
			PeakConcurrency: make(map[ToolPerformanceProfile]int),
		},
		logger: logger,
	}
}

// GetToolProfile returns the performance profile for a tool (fully dynamic learning)
func (cm *ConcurrencyManager) GetToolProfile(toolName string) ToolPerformanceProfile {
	cm.performanceMutex.RLock()
	defer cm.performanceMutex.RUnlock()
	
	// Check if we have learned performance data for this tool
	if history, exists := cm.performanceHistory[toolName]; exists {
		// Use learned classification even from first execution
		// This allows immediate adaptation after first run
		return history.LastClassified
	}
	
	// All unknown tools start as MediumTool - completely dynamic, no hardcoded hints
	cm.logger.Debug("Unknown tool, defaulting to medium profile", "tool", toolName)
	return MediumTool
}

// LearnToolPerformance updates the performance profile based on execution time
func (cm *ConcurrencyManager) LearnToolPerformance(toolName string, executionTime time.Duration) {
	cm.performanceMutex.Lock()
	defer cm.performanceMutex.Unlock()
	
	history, exists := cm.performanceHistory[toolName]
	if !exists {
		history = &ToolPerformanceHistory{
			LastClassified: MediumTool, // Start with medium assumption
			LastUpdate:     time.Now(),
		}
		cm.performanceHistory[toolName] = history
	}
	
	oldProfile := history.LastClassified
	
	// Update statistics
	history.TotalExecutions++
	history.TotalTime += executionTime
	history.AverageTime = history.TotalTime / time.Duration(history.TotalExecutions)
	history.LastUpdate = time.Now()
	
	// Dynamic classification based on execution performance
	// Use weighted average with current execution to be more responsive to recent performance
	currentSeconds := executionTime.Seconds()
	avgSeconds := history.AverageTime.Seconds()
	
	// For early executions (< 5 runs), weight current execution more heavily
	// This allows faster adaptation to tool characteristics
	var effectiveTime float64
	if history.TotalExecutions <= 5 {
		weight := 0.6 // 60% weight to current execution for first few runs
		effectiveTime = (weight * currentSeconds) + ((1 - weight) * avgSeconds)
	} else {
		effectiveTime = avgSeconds // Use pure average for established tools
	}
	
	// Classify based on effective execution time (fully dynamic)
	var newProfile ToolPerformanceProfile
	switch {
	case effectiveTime < 5:
		newProfile = FastTool
	case effectiveTime < 30:
		newProfile = MediumTool
	default:
		newProfile = HeavyTool
	}
	
	// Log classification updates (including first-time classification)
	if newProfile != oldProfile {
		cm.logger.Debug("Tool classification updated", 
			"tool", toolName,
			"old_profile", oldProfile,
			"new_profile", newProfile,
			"current_time", currentSeconds,
			"avg_time", avgSeconds,
			"effective_time", effectiveTime,
			"executions", history.TotalExecutions)
	}
	
	history.LastClassified = newProfile
}

// GetToolPerformanceHistory returns performance data for debugging
func (cm *ConcurrencyManager) GetToolPerformanceHistory() map[string]ToolPerformanceHistory {
	cm.performanceMutex.RLock()
	defer cm.performanceMutex.RUnlock()
	
	result := make(map[string]ToolPerformanceHistory)
	for toolName, history := range cm.performanceHistory {
		result[toolName] = *history // Copy the struct
	}
	return result
}

// RequestExecution requests an execution slot for a tool
func (cm *ConcurrencyManager) RequestExecution(ctx context.Context, toolName string, priority int) (*ExecutionRequest, error) {
	profile := cm.GetToolProfile(toolName)
	
	// Create cancellable context for this request
	requestCtx, cancelFunc := context.WithCancel(ctx)
	
	request := &ExecutionRequest{
		ToolName:   toolName,
		Profile:    profile,
		Priority:   priority,
		Context:    requestCtx,
		StartChan:  make(chan struct{}),
		CancelFunc: cancelFunc,
	}
	
	// Try to acquire slot immediately
	if cm.tryAcquireSlot(request) {
		// Slot acquired, signal immediate start
		close(request.StartChan)
		return request, nil
	}
	
	// No slot available, add to queue
	cm.addToQueue(request)
	cm.logger.Debug("Tool queued", "tool", toolName, "profile", profile, "queue_size", len(cm.executionQueue))
	
	return request, nil
}

// tryAcquireSlot attempts to immediately acquire an execution slot
func (cm *ConcurrencyManager) tryAcquireSlot(request *ExecutionRequest) bool {
	var sem chan struct{}
	
	switch request.Profile {
	case FastTool:
		sem = cm.fastSem
	case MediumTool:
		sem = cm.mediumSem
	case HeavyTool:
		sem = cm.heavySem
	}
	
	select {
	case sem <- struct{}{}:
		// Slot acquired
		cm.trackToolStart(request.ToolName, request.Profile)
		return true
	default:
		// No slot available
		return false
	}
}

// addToQueue adds a request to the execution queue with priority ordering
func (cm *ConcurrencyManager) addToQueue(request *ExecutionRequest) {
	cm.queueMutex.Lock()
	defer cm.queueMutex.Unlock()
	
	// Insert request in priority order (higher priority first)
	inserted := false
	for i, queuedRequest := range cm.executionQueue {
		if request.Priority > queuedRequest.Priority {
			// Insert at position i
			cm.executionQueue = append(cm.executionQueue[:i], 
				append([]*ExecutionRequest{request}, cm.executionQueue[i:]...)...)
			inserted = true
			break
		}
	}
	
	if !inserted {
		// Append to end
		cm.executionQueue = append(cm.executionQueue, request)
	}
	
	cm.metricsMutex.Lock()
	cm.metrics.QueuedExecutions++
	cm.metricsMutex.Unlock()
}

// ReleaseExecution releases an execution slot and processes queue
func (cm *ConcurrencyManager) ReleaseExecution(request *ExecutionRequest) {
	var sem chan struct{}
	
	switch request.Profile {
	case FastTool:
		sem = cm.fastSem
	case MediumTool:
		sem = cm.mediumSem
	case HeavyTool:
		sem = cm.heavySem
	}
	
	// Release the semaphore slot
	<-sem
	
	// Update tracking
	cm.trackToolEnd(request.ToolName, request.Profile)
	
	// Process queue for newly available slot
	cm.processQueue(request.Profile)
	
	cm.logger.Debug("Execution slot released", "tool", request.ToolName, "profile", request.Profile)
}

// processQueue checks if any queued tools can now be executed - prioritizes by priority, not profile
func (cm *ConcurrencyManager) processQueue(releasedProfile ToolPerformanceProfile) {
	cm.queueMutex.Lock()
	defer cm.queueMutex.Unlock()
	
	// Look for highest priority tools that can use ANY available slot (not just the released type)
	for i, request := range cm.executionQueue {
		// Check if request context is still valid
		if request.Context.Err() != nil {
			// Remove cancelled request
			cm.executionQueue = append(cm.executionQueue[:i], cm.executionQueue[i+1:]...)
			continue
		}
		
		// Try to acquire slot for this request (regardless of profile - priority wins)
		if cm.tryAcquireSlot(request) {
			// Remove from queue and signal start
			cm.executionQueue = append(cm.executionQueue[:i], cm.executionQueue[i+1:]...)
			close(request.StartChan)
			cm.logger.Debug("Queued tool starting", "tool", request.ToolName, "priority", request.Priority, "waited_slots", i+1)
			return
		}
	}
}

// trackToolStart updates metrics when a tool starts
func (cm *ConcurrencyManager) trackToolStart(toolName string, profile ToolPerformanceProfile) {
	cm.activeMutex.Lock()
	defer cm.activeMutex.Unlock()
	
	cm.activeTools[toolName]++
	
	cm.metricsMutex.Lock()
	defer cm.metricsMutex.Unlock()
	
	cm.metrics.TotalExecuted++
	
	// Update peak concurrency
	activeCount := cm.getActiveCountByProfile(profile)
	if activeCount > cm.metrics.PeakConcurrency[profile] {
		cm.metrics.PeakConcurrency[profile] = activeCount
	}
}

// trackToolEnd updates metrics when a tool ends
func (cm *ConcurrencyManager) trackToolEnd(toolName string, profile ToolPerformanceProfile) {
	cm.activeMutex.Lock()
	defer cm.activeMutex.Unlock()
	
	cm.activeTools[toolName]--
	if cm.activeTools[toolName] <= 0 {
		delete(cm.activeTools, toolName)
	}
}

// getActiveCountByProfile returns the number of active tools for a profile
func (cm *ConcurrencyManager) getActiveCountByProfile(profile ToolPerformanceProfile) int {
	count := 0
	for toolName, activeCount := range cm.activeTools {
		if cm.GetToolProfile(toolName) == profile {
			count += activeCount
		}
	}
	return count
}

// GetStatus returns current concurrency status
func (cm *ConcurrencyManager) GetStatus() map[string]interface{} {
	cm.activeMutex.RLock()
	defer cm.activeMutex.RUnlock()
	
	cm.queueMutex.Lock()
	defer cm.queueMutex.Unlock()
	
	// Calculate slot utilization
	fastActive := cm.getActiveCountByProfile(FastTool)
	mediumActive := cm.getActiveCountByProfile(MediumTool)
	heavyActive := cm.getActiveCountByProfile(HeavyTool)
	
	status := map[string]interface{}{
		"slots": map[string]interface{}{
			"fast": map[string]interface{}{
				"active":    fastActive,
				"available": cm.limits.FastToolLimit - fastActive,
				"total":     cm.limits.FastToolLimit,
				"usage":     float64(fastActive) / float64(cm.limits.FastToolLimit),
			},
			"medium": map[string]interface{}{
				"active":    mediumActive,
				"available": cm.limits.MediumToolLimit - mediumActive,
				"total":     cm.limits.MediumToolLimit,
				"usage":     float64(mediumActive) / float64(cm.limits.MediumToolLimit),
			},
			"heavy": map[string]interface{}{
				"active":    heavyActive,
				"available": cm.limits.HeavyToolLimit - heavyActive,
				"total":     cm.limits.HeavyToolLimit,
				"usage":     float64(heavyActive) / float64(cm.limits.HeavyToolLimit),
			},
		},
		"queue": map[string]interface{}{
			"size":  len(cm.executionQueue),
			"tools": cm.getQueuedToolNames(),
		},
		"active_tools": cm.copyActiveTools(),
	}
	
	return status
}

// getQueuedToolNames returns names of tools in queue
func (cm *ConcurrencyManager) getQueuedToolNames() []string {
	names := make([]string, len(cm.executionQueue))
	for i, request := range cm.executionQueue {
		names[i] = request.ToolName
	}
	return names
}

// copyActiveTools returns a copy of active tools map
func (cm *ConcurrencyManager) copyActiveTools() map[string]int {
	copy := make(map[string]int)
	for toolName, count := range cm.activeTools {
		copy[toolName] = count
	}
	return copy
}

// GetMetrics returns performance metrics
func (cm *ConcurrencyManager) GetMetrics() ConcurrencyMetrics {
	cm.metricsMutex.RLock()
	defer cm.metricsMutex.RUnlock()
	
	// Create a copy to avoid data races
	metrics := cm.metrics
	metrics.SlotUtilization = make(map[ToolPerformanceProfile]float64)
	metrics.PeakConcurrency = make(map[ToolPerformanceProfile]int)
	
	for profile, peak := range cm.metrics.PeakConcurrency {
		metrics.PeakConcurrency[profile] = peak
	}
	
	for profile, util := range cm.metrics.SlotUtilization {
		metrics.SlotUtilization[profile] = util
	}
	
	return metrics
}

// WaitForExecution waits for an execution slot to become available
func (request *ExecutionRequest) WaitForExecution() error {
	select {
	case <-request.StartChan:
		return nil // Execution can start
	case <-request.Context.Done():
		return request.Context.Err() // Request cancelled
	}
}

// Cancel cancels a pending execution request
func (request *ExecutionRequest) Cancel() {
	request.CancelFunc()
}

// SetLogLevel updates the logger's level for controlling message visibility
func (cm *ConcurrencyManager) SetLogLevel(level log.Level) {
	if cm.logger != nil {
		cm.logger.SetLevel(level)
	}
}