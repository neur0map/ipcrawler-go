package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/neur0map/ipcrawler/internal/config"
	"github.com/neur0map/ipcrawler/internal/output"
)

// ANSI color codes for terminal output
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

// ToolError represents a tool execution error with context
type ToolError struct {
	ToolName  string        `json:"tool_name"`
	Mode      string        `json:"mode"`
	Target    string        `json:"target"`
	Command   []string      `json:"command"`
	ExitCode  int           `json:"exit_code"`
	Stderr    string        `json:"stderr"`
	Stdout    string        `json:"stdout"`
	ErrorMsg  string        `json:"error_message"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
}

// ErrorHandler manages tool error reporting and logging
type ErrorHandler struct {
	workspaceDir string
	outputMode   output.OutputMode
	errorLogger  *log.Logger
	mutex        sync.Mutex
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(workspaceDir string, outputMode output.OutputMode) *ErrorHandler {
	return &ErrorHandler{
		workspaceDir: workspaceDir,
		outputMode:   outputMode,
	}
}

// SetupErrorLogging initializes error logging to workspace
func (eh *ErrorHandler) SetupErrorLogging() error {
	if eh.workspaceDir == "" {
		return nil // No workspace set yet
	}

	eh.mutex.Lock()
	defer eh.mutex.Unlock()

	// Create error log directory
	errorDir := filepath.Join(eh.workspaceDir, "logs", "errors")
	if err := os.MkdirAll(errorDir, 0755); err != nil {
		return fmt.Errorf("failed to create error log directory: %w", err)
	}

	// Open error log file
	errorLogPath := filepath.Join(errorDir, "error.log")
	errorFile, err := os.OpenFile(errorLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open error log file: %w", err)
	}

	// Create error logger
	eh.errorLogger = log.New(errorFile)
	eh.errorLogger.SetReportCaller(false)
	eh.errorLogger.SetReportTimestamp(true)
	eh.errorLogger.SetLevel(log.ErrorLevel)

	return nil
}

// HandleToolError processes and reports tool execution errors
func (eh *ErrorHandler) HandleToolError(toolErr *ToolError) {
	eh.mutex.Lock()
	defer eh.mutex.Unlock()

	// Log to error file if available
	if eh.errorLogger != nil {
		eh.errorLogger.Error("Tool execution failed",
			"tool", toolErr.ToolName,
			"mode", toolErr.Mode,
			"target", toolErr.Target,
			"exit_code", toolErr.ExitCode,
			"duration", toolErr.Duration,
			"error", toolErr.ErrorMsg,
			"stderr", toolErr.Stderr)
	}

	// Display to user based on output mode
	switch eh.outputMode {
	case output.OutputModeNormal:
		// Short error message for normal mode
		eh.displayShortError(toolErr)
	case output.OutputModeVerbose, output.OutputModeDebug:
		// Detailed error message for verbose/debug mode
		eh.displayDetailedError(toolErr)
	}
}

// displayShortError shows a brief error message for normal mode
func (eh *ErrorHandler) displayShortError(toolErr *ToolError) {
	fmt.Printf("\n%s⚠️  %s [%s] failed%s\n", colorYellow, toolErr.ToolName, toolErr.Mode, colorReset)
}

// displayDetailedError shows comprehensive error information for verbose/debug mode
func (eh *ErrorHandler) displayDetailedError(toolErr *ToolError) {
	fmt.Printf("\n%s════════════════════════════════════════════════════════════════════════════════%s\n", colorRed, colorReset)
	fmt.Printf("%s%s⚠️  ERROR: %s [%s] failed%s%s\n", colorBold, colorRed, toolErr.ToolName, toolErr.Mode, colorReset, colorReset)
	fmt.Printf("%s════════════════════════════════════════════════════════════════════════════════%s\n", colorRed, colorReset)

	fmt.Printf("%sTarget:%s %s\n", colorCyan, colorReset, toolErr.Target)
	fmt.Printf("%sCommand:%s %s\n", colorCyan, colorReset, strings.Join(toolErr.Command, " "))
	fmt.Printf("%sExit Code:%s %d\n", colorCyan, colorReset, toolErr.ExitCode)
	fmt.Printf("%sDuration:%s %v\n", colorCyan, colorReset, toolErr.Duration)

	if toolErr.ErrorMsg != "" {
		fmt.Printf("%sError:%s %s\n", colorCyan, colorReset, toolErr.ErrorMsg)
	}

	if toolErr.Stderr != "" {
		fmt.Printf("%sStderr:%s\n%s\n", colorCyan, colorReset, toolErr.Stderr)
	}

	if toolErr.Stdout != "" && len(toolErr.Stdout) < 500 {
		fmt.Printf("%sStdout:%s\n%s\n", colorCyan, colorReset, toolErr.Stdout)
	}

	fmt.Printf("%s────────────────────────────────────────────────────────────────────────────────%s\n", colorGray, colorReset)
}

// ExecutionResult represents the result of a tool execution
type ExecutionResult struct {
	ToolName     string        `json:"tool_name"`
	Mode         string        `json:"mode"`
	Target       string        `json:"target"`
	StartTime    time.Time     `json:"start_time"`
	EndTime      time.Time     `json:"end_time"`
	Duration     time.Duration `json:"duration"`
	ExitCode     int           `json:"exit_code"`
	Success      bool          `json:"success"`
	OutputPath   string        `json:"output_path"`
	ErrorMessage string        `json:"error_message,omitempty"`
	CommandLine  []string      `json:"command_line"`
	Stdout       string        `json:"stdout,omitempty"`
	Stderr       string        `json:"stderr,omitempty"`
}

// ExecutionOptions contains options for tool execution
type ExecutionOptions struct {
	Timeout        time.Duration     // Maximum execution time
	WorkingDir     string            // Working directory for execution
	Environment    map[string]string // Additional environment variables
	CaptureOutput  bool              // Whether to capture stdout/stderr
	ValidateOutput bool              // Whether to validate output file was created
	Priority       int               // Execution priority for concurrency queue (higher = more priority)
}

// ToolExecutionEngine orchestrates tool execution with template resolution
type ToolExecutionEngine struct {
	configLoader     *ToolConfigLoader
	templateResolver *TemplateResolver
	globalConfig     *config.Config
	toolsPath        string
	validator        *SecurityValidator
	magicVarManager  *MagicVariableManager
	workspaceBase    string // Base workspace directory for this execution session

	// Dynamic concurrency control
	concurrencyManager *ConcurrencyManager

	// Legacy concurrency control (deprecated but kept for compatibility)
	concurrentSem chan struct{}
	parallelSem   chan struct{}
	runningMutex  sync.RWMutex
	runningTools  map[string]int // toolName -> count

	// Execution tracking for magic variables
	completedTools map[string]*ExecutionResult
	completedMutex sync.RWMutex

	// Loggers for different output types
	debugLogger *log.Logger
	infoLogger  *log.Logger

	// Output controller for console display
	outputController *output.OutputController

	// Error handling
	errorHandler *ErrorHandler
}

// NewToolExecutionEngine creates a new tool execution engine
func NewToolExecutionEngine(globalConfig *config.Config, toolsPath string, outputMode output.OutputMode) *ToolExecutionEngine {
	// If toolsPath is empty, use the configured tools path or default to allowing system PATH
	if toolsPath == "" && globalConfig != nil {
		toolsPath = globalConfig.Tools.Execution.ToolsPath
	}
	// Get concurrency limits from config or use defaults
	maxConcurrent := 3
	maxParallel := 2

	if globalConfig != nil && globalConfig.Tools.ToolExecution.MaxConcurrentExecutions > 0 {
		maxConcurrent = globalConfig.Tools.ToolExecution.MaxConcurrentExecutions
	}

	if globalConfig != nil && globalConfig.Tools.ToolExecution.MaxParallelExecutions > 0 {
		maxParallel = globalConfig.Tools.ToolExecution.MaxParallelExecutions
	}

	// Create dynamic concurrency limits based on total concurrent limit
	// Fast tools get more slots, heavy tools get fewer
	fastLimit := maxConcurrent * 2  // 2x multiplier for fast tools
	mediumLimit := maxConcurrent    // 1x multiplier for medium tools
	heavyLimit := maxConcurrent / 2 // 0.5x multiplier for heavy tools
	if heavyLimit < 1 {
		heavyLimit = 1 // Always allow at least 1 heavy tool
	}

	// Config loader always uses "./tools" for config files
	configToolsPath := "./tools"

	// Initialize magic variable manager and register parsers
	magicVarManager := NewMagicVariableManager()
	RegisterAllParsers(magicVarManager)

	// Setup default loggers (will be overridden when workspace is set)
	debugLogger := log.New(os.Stderr)
	debugLogger.SetLevel(log.DebugLevel)

	infoLogger := log.New(os.Stderr)
	infoLogger.SetLevel(log.InfoLevel)

	// Create error handler
	errorHandler := NewErrorHandler("", outputMode)

	// Create dynamic concurrency manager
	concurrencyLimits := ConcurrencyLimits{
		FastToolLimit:   fastLimit,
		MediumToolLimit: mediumLimit,
		HeavyToolLimit:  heavyLimit,
	}
	concurrencyManager := NewConcurrencyManager(concurrencyLimits, debugLogger)

	return &ToolExecutionEngine{
		configLoader:     NewToolConfigLoader(configToolsPath),
		templateResolver: NewTemplateResolver(globalConfig),
		globalConfig:     globalConfig,
		toolsPath:        toolsPath, // This can be empty for system PATH
		validator:        NewSecurityValidator(globalConfig),
		magicVarManager:  magicVarManager,
		workspaceBase:    "", // Will be set by SetWorkspaceBase if needed
		debugLogger:      debugLogger,
		infoLogger:       infoLogger,
		outputController: output.NewOutputController(outputMode),

		// Dynamic concurrency control
		concurrencyManager: concurrencyManager,

		// Error handling
		errorHandler: errorHandler,

		// Legacy concurrency control (kept for compatibility)
		concurrentSem: make(chan struct{}, maxConcurrent),
		parallelSem:   make(chan struct{}, maxParallel),
		runningTools:  make(map[string]int),

		// Initialize execution tracking
		completedTools: make(map[string]*ExecutionResult),
	}
}

// SetWorkspaceBase sets the base workspace directory for this execution session
func (tee *ToolExecutionEngine) SetWorkspaceBase(workspaceDir string) {
	tee.workspaceBase = workspaceDir

	// Setup error logging for this workspace
	if tee.errorHandler != nil {
		tee.errorHandler.workspaceDir = workspaceDir
		if err := tee.errorHandler.SetupErrorLogging(); err != nil {
			// Log setup error but don't fail
			if tee.debugLogger != nil {
				tee.debugLogger.Error("Failed to setup error logging", "error", err)
			}
		}
	}
}

// SetOutputMode configures the output mode for logging
func (tee *ToolExecutionEngine) SetOutputMode(mode output.OutputMode) {
	// Update the output controller if it exists
	if tee.outputController != nil {
		tee.outputController = output.NewOutputController(mode)
	}

	// Update error handler output mode
	if tee.errorHandler != nil {
		tee.errorHandler.outputMode = mode
	}

	// Update concurrency manager logger level based on output mode
	if tee.concurrencyManager != nil {
		switch mode {
		case output.OutputModeDebug:
			// Debug mode: show debug messages
			tee.concurrencyManager.SetLogLevel(log.DebugLevel)
		case output.OutputModeVerbose:
			// Verbose mode: show info and above
			tee.concurrencyManager.SetLogLevel(log.InfoLevel)
		default:
			// Normal mode: suppress debug messages
			tee.concurrencyManager.SetLogLevel(log.WarnLevel)
		}
	}
}

// SetWorkspaceLoggers sets up loggers that write to workspace log files
func (tee *ToolExecutionEngine) SetWorkspaceLoggers(workspaceDir string) error {
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
	debugFile, err := os.OpenFile(filepath.Join(debugsDir, "tools.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open debug log file: %w", err)
	}

	// Create MultiWriter based on output mode (check if outputController exists to get mode)
	var debugMultiWriter io.Writer
	if tee.outputController != nil && (tee.outputController.ShouldShowLogs()) {
		// In verbose/debug mode, write to both stderr and file
		debugMultiWriter = io.MultiWriter(os.Stderr, debugFile)
	} else {
		// In normal mode, write only to file
		debugMultiWriter = debugFile
	}
	tee.debugLogger = log.New(debugMultiWriter)
	tee.debugLogger.SetReportCaller(false)
	tee.debugLogger.SetReportTimestamp(true)
	tee.debugLogger.SetLevel(log.DebugLevel)

	// Setup info logger to write to both console and file
	infoFile, err := os.OpenFile(filepath.Join(infoDir, "tools.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open info log file: %w", err)
	}

	// Create MultiWriter based on output mode
	var infoMultiWriter io.Writer
	if tee.outputController != nil && (tee.outputController.ShouldShowLogs()) {
		// In verbose/debug mode, write to both stderr and file
		infoMultiWriter = io.MultiWriter(os.Stderr, infoFile)
	} else {
		// In normal mode, write only to file
		infoMultiWriter = infoFile
	}
	tee.infoLogger = log.New(infoMultiWriter)
	tee.infoLogger.SetReportCaller(false)
	tee.infoLogger.SetReportTimestamp(true)
	tee.infoLogger.SetLevel(log.InfoLevel)

	return nil
}

// writeRawOutput writes tool output to the raw output log file
func (tee *ToolExecutionEngine) writeRawOutput(toolName, mode, outputType, content string) {
	if tee.workspaceBase == "" {
		return // No workspace set
	}

	rawLogPath := filepath.Join(tee.workspaceBase, "raw", "tool_output.log")

	// Create raw directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(rawLogPath), 0755); err != nil {
		if tee.debugLogger != nil {
			tee.debugLogger.Error("Failed to create raw log directory", "error", err)
		}
		return
	}

	// Open log file in append mode
	file, err := os.OpenFile(rawLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		if tee.debugLogger != nil {
			tee.debugLogger.Error("Failed to open raw log file", "error", err)
		}
		return
	}
	defer file.Close()

	// Write timestamped entry
	timestamp := time.Now().Format(time.RFC3339)
	header := fmt.Sprintf("\n[%s] === %s: %s %s ===\n", timestamp, outputType, toolName, mode)
	footer := fmt.Sprintf("=== END %s ===\n", outputType)

	file.WriteString(header)
	file.WriteString(content)
	file.WriteString(footer)
}

// writeDebugLog writes debug messages to the debug log file
func (tee *ToolExecutionEngine) writeDebugLog(message string, args ...interface{}) {
	if tee.workspaceBase == "" {
		return // No workspace set
	}

	debugLogPath := filepath.Join(tee.workspaceBase, "logs", "debug", "execution.log")

	// Create debug directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(debugLogPath), 0755); err != nil {
		return // Silent failure to avoid infinite loops
	}

	// Open log file in append mode
	file, err := os.OpenFile(debugLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return // Silent failure
	}
	defer file.Close()

	// Write timestamped entry
	timestamp := time.Now().Format(time.RFC3339)
	var logMessage string
	if len(args) > 0 {
		logMessage = fmt.Sprintf(message, args...)
	} else {
		logMessage = message
	}

	file.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, logMessage))
}

// ExecuteTool executes a tool with the specified parameters (legacy interface)
func (tee *ToolExecutionEngine) ExecuteTool(ctx context.Context, toolName, mode, target string, options *ExecutionOptions) (*ExecutionResult, error) {
	return tee.ExecuteToolWithContext(ctx, toolName, mode, target, "", "", options)
}

// ExecuteToolWithContext executes a tool with workflow context for unique filename generation
func (tee *ToolExecutionEngine) ExecuteToolWithContext(ctx context.Context, toolName, mode, target, workflowName, stepName string, options *ExecutionOptions) (*ExecutionResult, error) {
	startTime := time.Now()

	tee.debugLogger.Debug("Starting tool execution", "tool", toolName, "mode", mode, "target", target)
	tee.writeDebugLog("Starting tool execution: %s mode=%s target=%s", toolName, mode, target)

	result := &ExecutionResult{
		ToolName:  toolName,
		Mode:      mode,
		Target:    target,
		StartTime: startTime,
		Success:   false,
	}

	// Determine priority from options or use default
	priority := 100 // Default medium priority
	if options != nil && options.Priority > 0 {
		priority = options.Priority
	}

	// Debug: Log the priority being used (only in debug mode)
	if tee.debugLogger.GetLevel() <= log.DebugLevel {
		tee.debugLogger.Debug("Requesting execution slot", "tool", toolName, "mode", mode, "priority", priority)
	}

	// Request execution slot from dynamic concurrency manager
	executionRequest, err := tee.concurrencyManager.RequestExecution(ctx, toolName, priority)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to request execution slot: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, err
	}

	// Wait for execution slot to become available
	if err := executionRequest.WaitForExecution(); err != nil {
		result.ErrorMessage = "execution cancelled while waiting for slot"
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, err
	}

	// Ensure we release the execution slot when done
	defer func() {
		tee.concurrencyManager.ReleaseExecution(executionRequest)
	}()

	// Load tool configuration
	tee.debugLogger.Debug("Loading config for tool", "tool", toolName)
	tee.writeDebugLog("Loading config for tool: %s", toolName)
	toolConfig, err := tee.configLoader.LoadToolConfig(toolName)
	if err != nil {
		tee.debugLogger.Error("Failed to load tool config", "tool", toolName, "error", err)
		result.ErrorMessage = fmt.Sprintf("failed to load tool config: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, err
	}
	tee.debugLogger.Debug("Tool config loaded successfully", "tool", toolName)
	tee.writeDebugLog("Tool config loaded successfully")

	// Get tool arguments for the specified mode
	argsTemplate, err := toolConfig.GetToolArguments(mode)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to get tool arguments: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, err
	}

	// Create execution context
	execCtx := tee.templateResolver.CreateExecutionContextWithWorkflow(target, toolName, mode, workflowName, stepName)

	// Generate workspace paths - use workspaceBase if set, otherwise generate from target
	var workspaceDir string
	if tee.workspaceBase != "" {
		// Use the pre-created workspace directory from CLI
		workspaceDir = tee.workspaceBase
		tee.debugLogger.Debug("Using preset workspace", "workspace", workspaceDir)
	} else {
		// Generate workspace path from target (fallback for TUI mode)
		sanitizedTarget := sanitizeForFilename(target)
		workspaceDir = filepath.Join("./workspace", sanitizedTarget)
		tee.debugLogger.Debug("Generated workspace", "workspace", workspaceDir)
	}

	execCtx.Workspace = workspaceDir
	execCtx.OutputDir = workspaceDir
	execCtx.ScansDir = filepath.Join(workspaceDir, "scans")
	execCtx.LogsDir = filepath.Join(workspaceDir, "logs")
	execCtx.ReportsDir = filepath.Join(workspaceDir, "reports")
	execCtx.RawDir = filepath.Join(workspaceDir, "raw")

	// Set custom output file if tool config specifies one
	if toolConfig.File != "" {
		execCtx.OutputFile = toolConfig.File
	}

	// Resolve template variables in arguments
	resolvedArgs, err := tee.templateResolver.ResolveArguments(argsTemplate, execCtx)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to resolve template variables: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, err
	}

	// Validate arguments against security policies
	if err := tee.validator.ValidateArguments(resolvedArgs); err != nil {
		result.ErrorMessage = fmt.Sprintf("argument validation failed: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, err
	}

	result.CommandLine = append([]string{toolName}, resolvedArgs...)

	// Determine the tool executable path
	toolExecutable, err := tee.findToolExecutable(toolName)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to find tool executable: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, err
	}

	// Validate executable path against security policies
	if err := tee.validator.ValidateExecutable(toolExecutable); err != nil {
		result.ErrorMessage = fmt.Sprintf("executable validation failed: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, err
	}

	// Set up execution options
	if options == nil {
		options = &ExecutionOptions{}
	}
	if options.Timeout == 0 {
		// Use timeout from global config if available, otherwise use a reasonable default
		if tee.globalConfig != nil && tee.globalConfig.Tools.DefaultTimeout > 0 {
			options.Timeout = time.Duration(tee.globalConfig.Tools.DefaultTimeout) * time.Second
		} else {
			options.Timeout = 30 * time.Minute // Fallback default
		}
	}

	// Create context with timeout
	execContext, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	// Only create directories that don't already exist (performance optimization)
	dirsToCreate := []string{
		execCtx.Workspace,
		execCtx.ScansDir,
		execCtx.LogsDir,
		execCtx.ReportsDir,
		execCtx.RawDir,
	}

	for _, dir := range dirsToCreate {
		if dir != "" {
			// Check if directory already exists before creating (CLI mode pre-creates these)
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				if err := os.MkdirAll(dir, 0755); err != nil {
					result.ErrorMessage = fmt.Sprintf("failed to create directory %s: %v", dir, err)
					result.EndTime = time.Now()
					result.Duration = result.EndTime.Sub(result.StartTime)
					return result, err
				}
			}
		}
	}

	// Build variable map for template resolution
	vars := tee.templateResolver.buildVariableMap(execCtx)

	// Store the expected output path (remove hardcoded tool-specific extensions)
	if outputPath, exists := vars["output_path"]; exists {
		result.OutputPath = outputPath
	}

	// Prepare output buffers
	var stdoutBuf, stderrBuf bytes.Buffer

	// Execute with retry logic
	retryAttempts := 1
	if tee.globalConfig != nil && tee.globalConfig.Tools.RetryAttempts > 0 {
		retryAttempts = tee.globalConfig.Tools.RetryAttempts
	}

	var lastErr error
	for attempt := 0; attempt <= retryAttempts; attempt++ {
		// Reset buffers for each attempt
		if options.CaptureOutput {
			stdoutBuf.Reset()
			stderrBuf.Reset()
		}

		// Create a new command for each attempt
		tee.debugLogger.Debug("Executing command", "executable", toolExecutable, "args", resolvedArgs)
		tee.writeDebugLog("Executing command: %s %v", toolExecutable, resolvedArgs)
		execCmd := exec.CommandContext(execContext, toolExecutable, resolvedArgs...)

		// Set working directory
		if options.WorkingDir != "" {
			execCmd.Dir = options.WorkingDir
		}

		// Set environment variables
		execCmd.Env = os.Environ()
		for key, value := range options.Environment {
			execCmd.Env = append(execCmd.Env, fmt.Sprintf("%s=%s", key, value))
		}

		// Set up output capture using temporary files instead of pipes to avoid deadlocks
		var stdoutFile, stderrFile *os.File
		if options.CaptureOutput {
			// Create temporary files for stdout and stderr
			stdoutFile, _ = os.CreateTemp("", "ipcrawler-stdout-*")
			stderrFile, _ = os.CreateTemp("", "ipcrawler-stderr-*")
			execCmd.Stdout = stdoutFile
			execCmd.Stderr = stderrFile
		} else {
			// If not capturing, just connect directly to console
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr
		}

		// Start the command
		tee.debugLogger.Debug("Starting command", "attempt", attempt+1, "max_attempts", retryAttempts+1)
		tee.writeDebugLog("Starting command (attempt %d/%d)...", attempt+1, retryAttempts+1)

		if err := execCmd.Start(); err != nil {
			lastErr = err
			tee.debugLogger.Debug("Failed to start command", "error", lastErr)
			continue
		}

		// SIMPLIFIED EXECUTION using temporary files
		if options.CaptureOutput {
			var progress *SimpleProgress

			// Start progress tracking if needed
			if toolConfig.ShowSeparator {
				progress = NewSimpleProgress(toolName, mode)
			}

			// Wait for command to complete with timeout
			done := make(chan error, 1)
			go func() {
				done <- execCmd.Wait()
			}()

			// Set tool-specific timeout
			timeout := 5 * time.Second
			if toolName == "nmap" {
				timeout = 15 * time.Second // nmap service detection needs more time
			}

			select {
			case lastErr = <-done:
				// Command completed normally
			case <-time.After(timeout):
				// Command timeout - kill it and continue
				execCmd.Process.Kill()
				lastErr = fmt.Errorf("command timeout after %v: %s %v", timeout, toolExecutable, resolvedArgs)
				<-done // Wait for the goroutine to finish

				tee.debugLogger.Debug("Command timed out - will check for valid output after reading files", "timeout", timeout)
			}

			// Close files and read their contents
			if stdoutFile != nil {
				stdoutFile.Close()
				if data, err := os.ReadFile(stdoutFile.Name()); err == nil {
					stdoutBuf.Write(data)
				}
				os.Remove(stdoutFile.Name()) // Clean up temp file
			}

			if stderrFile != nil {
				stderrFile.Close()
				if data, err := os.ReadFile(stderrFile.Name()); err == nil {
					stderrBuf.Write(data)
				}
				os.Remove(stderrFile.Name()) // Clean up temp file
			}

			// Complete the progress tracking
			if progress != nil {
				progress.Complete()

				// Only show raw output in verbose mode
				if tee.outputController.ShouldShowRaw() {
					if stdoutBuf.Len() > 0 || stderrBuf.Len() > 0 {
						if toolConfig.ShowSeparator {
							tee.outputController.PrintCompleteToolOutput(toolName, mode, stdoutBuf.String(), stderrBuf.String(), lastErr != nil)
						} else {
							// Just print raw output without separators for tools that don't want them
							if stdoutBuf.Len() > 0 {
								fmt.Print(stdoutBuf.String())
							}
							if stderrBuf.Len() > 0 {
								fmt.Fprintf(os.Stderr, "\033[31m%s\033[0m", stderrBuf.String())
							}
						}
					}
				}
			} else if stdoutBuf.Len() > 0 || stderrBuf.Len() > 0 {
				// Tool completed without showing progress (no separator config)
				if tee.outputController.ShouldShowRaw() {
					if stdoutBuf.Len() > 0 {
						fmt.Print(stdoutBuf.String())
					}
					if stderrBuf.Len() > 0 {
						fmt.Fprintf(os.Stderr, "\033[31m%s\033[0m", stderrBuf.String())
					}
				}
			}
		} else {
			// Just wait for command if not capturing
			lastErr = execCmd.Wait()
		}

		tee.debugLogger.Debug("Command completed", "error", lastErr)
		tee.writeDebugLog("Command completed with error: %v", lastErr)

		// Check for timeout errors and validate if tool produced valid output
		if lastErr != nil && strings.Contains(lastErr.Error(), "timeout") {
			toolProducedValidOutput := false

			// Check if output file was created successfully
			if result.OutputPath != "" {
				outputPaths := []string{result.OutputPath, result.OutputPath + ".json", result.OutputPath + ".xml"}

				for _, path := range outputPaths {
					if _, err := os.Stat(path); err == nil {
						// For nmap XML files, verify they contain scan data
						if strings.HasSuffix(path, ".xml") && toolName == "nmap" {
							if data, err := os.ReadFile(path); err == nil {
								content := string(data)
								// Check for nmap XML structure with scan initiation
								if strings.Contains(content, "<nmaprun") && strings.Contains(content, "scan initiated") {
									toolProducedValidOutput = true
									tee.debugLogger.Debug("Command timed out but valid nmap XML created, treating as success", "output_path", path)
									break
								}
							}
						} else {
							toolProducedValidOutput = true
							tee.debugLogger.Debug("Command timed out but output file created, treating as success", "output_path", path)
							break
						}
					}
				}
			}

			// Also check if stdout contains valid JSON output (for tools like naabu)
			if !toolProducedValidOutput && stdoutBuf.Len() > 0 {
				stdout := stdoutBuf.String()
				// Check for JSON output patterns (naabu produces JSON lines)
				if strings.Contains(stdout, `"host":`) && strings.Contains(stdout, `"port":`) && strings.Contains(stdout, `"protocol":`) {
					toolProducedValidOutput = true
					tee.debugLogger.Debug("Command timed out but produced valid JSON output, treating as success", "stdout_length", len(stdout))
				}
			}

			// If tool produced valid output, mark as successful
			if toolProducedValidOutput {
				lastErr = nil
				tee.debugLogger.Debug("Tool timeout overridden due to valid output production")
			} else {
				tee.debugLogger.Debug("Command timed out with no valid output detected")
			}
		}

		// Handle tool errors if execution failed
		if lastErr != nil {
			toolErr := &ToolError{
				ToolName:  toolName,
				Mode:      mode,
				Target:    target,
				Command:   append([]string{toolExecutable}, resolvedArgs...),
				ExitCode:  -1, // Will be updated below if possible
				Stderr:    stderrBuf.String(),
				Stdout:    stdoutBuf.String(),
				ErrorMsg:  lastErr.Error(),
				Timestamp: time.Now(),
				Duration:  time.Since(startTime),
			}

			// Extract exit code if available
			if exitErr, ok := lastErr.(*exec.ExitError); ok {
				toolErr.ExitCode = exitErr.ExitCode()
			}

			// Report the error
			if tee.errorHandler != nil {
				tee.errorHandler.HandleToolError(toolErr)
			}
		}

		// Store captured output in result
		if options.CaptureOutput {
			result.Stdout = stdoutBuf.String()
			result.Stderr = stderrBuf.String()

			// Write captured output to raw output files (real-time display already handled above)
			if result.Stdout != "" {
				tee.writeRawOutput(toolName, mode, "STDOUT", result.Stdout)
			}
			if result.Stderr != "" {
				tee.writeRawOutput(toolName, mode, "STDERR", result.Stderr)
			}
		}

		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)

		if lastErr == nil {
			// Success
			result.Success = true
			result.ExitCode = 0
			// Tool end marker is now handled in PrintCompleteToolOutput
			break
		}

		// Handle error
		if exitError, ok := lastErr.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		} else {
			result.ExitCode = -1
		}

		// Don't retry timeout errors - they'll just timeout again (unless they were validated as successful)
		if lastErr != nil && strings.Contains(lastErr.Error(), "timeout") {
			result.ErrorMessage = fmt.Sprintf("tool execution timed out: %v", lastErr)
			return result, lastErr
		}

		// If this was the last attempt, set final error
		if attempt == retryAttempts {
			result.ErrorMessage = fmt.Sprintf("tool execution failed after %d attempts: %v", attempt+1, lastErr)
			return result, lastErr
		}

		// Wait before retrying (exponential backoff)
		if attempt < retryAttempts {
			waitTime := time.Duration(attempt+1) * time.Second
			select {
			case <-time.After(waitTime):
				// Continue to retry
			case <-execContext.Done():
				result.ErrorMessage = "execution cancelled during retry wait"
				return result, execContext.Err()
			}
		}
	}

	// Save captured stdout to file if tool succeeded and has output but no file was created
	if result.Success && options.CaptureOutput && result.Stdout != "" && result.OutputPath != "" {
		if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
			// Tool didn't create output file, so save captured stdout
			tee.debugLogger.Debug("Saving captured stdout", "path", result.OutputPath)
			if err := os.WriteFile(result.OutputPath, []byte(result.Stdout), 0644); err != nil {
				tee.debugLogger.Error("Failed to save stdout", "error", err)
			} else {
				tee.debugLogger.Debug("Successfully saved stdout", "bytes", len(result.Stdout))
			}
		}
	}

	// Validate output file was created if requested
	if options.ValidateOutput && result.OutputPath != "" {
		if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("expected output file not created: %s", result.OutputPath)
			return result, fmt.Errorf("output validation failed: %s", result.ErrorMessage)
		}
	}

	// Store completed tool result for magic variable processing
	tee.completedMutex.Lock()
	tee.completedTools[toolName] = result
	tee.completedMutex.Unlock()

	// Auto-process magic variables if tool succeeded
	if result.Success && result.OutputPath != "" {
		if err := tee.processToolOutputForMagicVariables(toolName, []string{result.OutputPath}); err != nil {
			// Log warning but don't fail the execution
			tee.outputController.PrintWarning("Failed to process magic variables for %s: %v", toolName, err)
		}
	}

	return result, nil
}

// ExecuteWithDependencies executes a tool with dependency handling and magic variables
func (tee *ToolExecutionEngine) ExecuteWithDependencies(ctx context.Context, toolName, mode, target, dependsOn string, options *ExecutionOptions) (*ExecutionResult, error) {
	// Process dependencies and create magic variables
	if dependsOn != "" {
		if err := tee.processDependencies(dependsOn); err != nil {
			return nil, fmt.Errorf("dependency processing failed: %w", err)
		}
	}

	// Execute the tool normally (magic variables are automatically available)
	return tee.ExecuteTool(ctx, toolName, mode, target, options)
}

// processDependencies processes completed tool outputs and creates magic variables
func (tee *ToolExecutionEngine) processDependencies(dependsOn string) error {
	// Get the completed tool result
	tee.completedMutex.RLock()
	completedResult, exists := tee.completedTools[dependsOn]
	tee.completedMutex.RUnlock()

	if !exists {
		return fmt.Errorf("dependency tool '%s' has not completed yet", dependsOn)
	}

	if !completedResult.Success {
		return fmt.Errorf("dependency tool '%s' failed: %s", dependsOn, completedResult.ErrorMessage)
	}

	// Find output files from the completed tool
	outputFiles := []string{}
	if completedResult.OutputPath != "" {
		outputFiles = append(outputFiles, completedResult.OutputPath)
	}

	// Process magic variables using the generic system
	magicVars := tee.magicVarManager.ProcessToolOutput(dependsOn, outputFiles)

	// Add magic variables to the template resolver
	for varName, varValue := range magicVars {
		tee.templateResolver.AddVariable(varName, varValue)
	}

	return nil
}

// processToolOutputForMagicVariables processes tool output and creates magic variables automatically
func (tee *ToolExecutionEngine) processToolOutputForMagicVariables(toolName string, outputFiles []string) error {
	// Process magic variables using the generic system
	magicVars := tee.magicVarManager.ProcessToolOutput(toolName, outputFiles)

	// Add magic variables to the template resolver
	for varName, varValue := range magicVars {
		tee.templateResolver.AddVariable(varName, varValue)
	}

	return nil
}

// ExecuteToolWithWorkflowVariables executes a tool with workflow-defined variable mapping
func (tee *ToolExecutionEngine) ExecuteToolWithWorkflowVariables(ctx context.Context, toolName, mode, target string, workflowVars map[string]string, options *ExecutionOptions) (*ExecutionResult, error) {
	// Add workflow variables to template resolver before execution
	for varName, varValue := range workflowVars {
		tee.templateResolver.AddVariable(varName, varValue)
	}

	// Execute tool normally with enhanced variable context
	return tee.ExecuteTool(ctx, toolName, mode, target, options)
}

// GetMagicVariables returns the current magic variables (useful for debugging)
func (tee *ToolExecutionEngine) GetMagicVariables() map[string]string {
	return tee.templateResolver.GetAllVariables()
}

// GetTemplateResolver returns the template resolver for workflow variable mapping
func (tee *ToolExecutionEngine) GetTemplateResolver() *TemplateResolver {
	return tee.templateResolver
}

// findToolExecutable locates the executable for a tool
func (tee *ToolExecutionEngine) findToolExecutable(toolName string) (string, error) {
	var candidates []string

	// If toolsPath is set, try tools directory first (security priority)
	if tee.toolsPath != "" {
		candidates = append(candidates,
			filepath.Join(tee.toolsPath, toolName, toolName), // In tools/toolname/toolname
			filepath.Join(tee.toolsPath, "bin", toolName),    // In tools/bin
			filepath.Join(tee.toolsPath, toolName),           // In tools directory
		)
	}

	// Always try system PATH as fallback
	candidates = append(candidates, toolName)

	// Add common executable extensions on Windows
	if strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") {
		var windowsCandidates []string
		for _, candidate := range candidates {
			windowsCandidates = append(windowsCandidates, candidate+".exe")
			windowsCandidates = append(windowsCandidates, candidate+".bat")
		}
		candidates = append(candidates, windowsCandidates...)
	}

	// Check each candidate
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}

		// Also check if file exists directly
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			// Check if file is executable (Unix-like systems)
			if info.Mode()&0111 != 0 {
				return candidate, nil
			}
		}
	}

	return "", fmt.Errorf("executable for tool '%s' not found in any expected location", toolName)
}

// GetAvailableTools returns a list of available tools
func (tee *ToolExecutionEngine) GetAvailableTools() ([]string, error) {
	return tee.configLoader.GetAvailableTools()
}

// GetToolConfig returns the configuration for a specific tool
func (tee *ToolExecutionEngine) GetToolConfig(toolName string) (*ToolConfig, error) {
	return tee.configLoader.LoadToolConfig(toolName)
}

// ValidateToolConfiguration validates that a tool is properly configured and executable
func (tee *ToolExecutionEngine) ValidateToolConfiguration(toolName string) error {
	// Load tool config
	toolConfig, err := tee.configLoader.LoadToolConfig(toolName)
	if err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Check that tool has at least one execution mode
	if len(toolConfig.Args) == 0 {
		return fmt.Errorf("tool '%s' has no execution modes defined", toolName)
	}

	// Check that executable exists
	_, err = tee.findToolExecutable(toolName)
	if err != nil {
		return fmt.Errorf("executable validation failed: %w", err)
	}

	return nil
}

// PreviewCommand generates the command that would be executed without actually running it
func (tee *ToolExecutionEngine) PreviewCommand(toolName, mode, target string) ([]string, error) {
	return tee.PreviewCommandWithContext(toolName, mode, target, "", "")
}

// PreviewCommandWithContext generates the command with workflow context
func (tee *ToolExecutionEngine) PreviewCommandWithContext(toolName, mode, target, workflowName, stepName string) ([]string, error) {
	// Load tool configuration
	toolConfig, err := tee.configLoader.LoadToolConfig(toolName)
	if err != nil {
		return nil, fmt.Errorf("failed to load tool config: %w", err)
	}

	// Get tool arguments for the specified mode
	argsTemplate, err := toolConfig.GetToolArguments(mode)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool arguments: %w", err)
	}

	// Create execution context
	execCtx := tee.templateResolver.CreateExecutionContextWithWorkflow(target, toolName, mode, workflowName, stepName)

	// Set custom output file if tool config specifies one
	if toolConfig.File != "" {
		execCtx.OutputFile = toolConfig.File
	}

	// Resolve template variables in arguments
	resolvedArgs, err := tee.templateResolver.ResolveArguments(argsTemplate, execCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve template variables: %w", err)
	}

	// Find tool executable
	toolExecutable, err := tee.findToolExecutable(toolName)
	if err != nil {
		return nil, fmt.Errorf("failed to find tool executable: %w", err)
	}

	return append([]string{toolExecutable}, resolvedArgs...), nil
}

// GetExecutionStatus returns information about current executions
func (tee *ToolExecutionEngine) GetExecutionStatus() map[string]interface{} {
	// Get dynamic concurrency status
	dynamicStatus := tee.concurrencyManager.GetStatus()

	// Add legacy status for compatibility
	tee.runningMutex.RLock()
	defer tee.runningMutex.RUnlock()

	legacyStatus := map[string]interface{}{
		"concurrent_slots_available": cap(tee.concurrentSem) - len(tee.concurrentSem),
		"concurrent_slots_total":     cap(tee.concurrentSem),
		"parallel_slots_available":   cap(tee.parallelSem) - len(tee.parallelSem),
		"parallel_slots_total":       cap(tee.parallelSem),
		"running_tools_legacy":       make(map[string]int),
	}

	// Copy legacy running tools map
	runningTools := make(map[string]int)
	for tool, count := range tee.runningTools {
		runningTools[tool] = count
	}
	legacyStatus["running_tools_legacy"] = runningTools

	// Merge dynamic and legacy status
	status := dynamicStatus
	status["legacy"] = legacyStatus

	return status
}

// sanitizeForFilename removes or replaces characters that are problematic in filenames
func sanitizeForFilename(input string) string {
	replacements := map[string]string{
		"/":  "_",
		"\\": "_",
		":":  "_",
		"*":  "_",
		"?":  "_",
		"\"": "_",
		"<":  "_",
		">":  "_",
		"|":  "_",
		" ":  "_",
		".":  "_",
	}

	result := input
	for old, new := range replacements {
		result = strings.ReplaceAll(result, old, new)
	}

	// Limit length to reasonable filename size
	if len(result) > 50 {
		result = result[:50]
	}

	return result
}
