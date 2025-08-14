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
)

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
	
	// Concurrency control
	concurrentSem    chan struct{}
	parallelSem      chan struct{}
	runningMutex     sync.RWMutex
	runningTools     map[string]int // toolName -> count
	
	// Execution tracking for magic variables
	completedTools   map[string]*ExecutionResult
	completedMutex   sync.RWMutex
	
	// Loggers for different output types
	debugLogger *log.Logger
	infoLogger  *log.Logger
}

// NewToolExecutionEngine creates a new tool execution engine  
func NewToolExecutionEngine(globalConfig *config.Config, toolsPath string) *ToolExecutionEngine {
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
		
		// Initialize concurrency control
		concurrentSem:    make(chan struct{}, maxConcurrent),
		parallelSem:      make(chan struct{}, maxParallel),
		runningTools:     make(map[string]int),
		
		// Initialize execution tracking
		completedTools:   make(map[string]*ExecutionResult),
	}
}

// SetWorkspaceBase sets the base workspace directory for this execution session
func (tee *ToolExecutionEngine) SetWorkspaceBase(workspaceDir string) {
	tee.workspaceBase = workspaceDir
}

// SetWorkspaceLoggers sets up loggers that write to workspace log files
func (tee *ToolExecutionEngine) SetWorkspaceLoggers(workspaceDir string) error {
	debugsDir := filepath.Join(workspaceDir, "logs", "debug")
	infoDir := filepath.Join(workspaceDir, "logs", "info")
	
	// Create log directories
	if err := os.MkdirAll(debugsDir, 0755); err != nil {
		return fmt.Errorf("failed to create debug log directory: %v", err)
	}
	if err := os.MkdirAll(infoDir, 0755); err != nil {
		return fmt.Errorf("failed to create info log directory: %v", err)
	}
	
	// Setup debug logger to write to both console and file
	debugFile, err := os.OpenFile(filepath.Join(debugsDir, "tools.log"), 
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open debug log file: %v", err)
	}
	
	// Create MultiWriter to write to both stderr and file
	debugMultiWriter := io.MultiWriter(os.Stderr, debugFile)
	tee.debugLogger = log.New(debugMultiWriter)
	tee.debugLogger.SetReportCaller(false)
	tee.debugLogger.SetReportTimestamp(true)
	tee.debugLogger.SetLevel(log.DebugLevel)
	
	// Setup info logger to write to both console and file  
	infoFile, err := os.OpenFile(filepath.Join(infoDir, "tools.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open info log file: %v", err)
	}
	
	// Create MultiWriter to write to both stderr and file
	infoMultiWriter := io.MultiWriter(os.Stderr, infoFile)
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

	// Acquire concurrent execution semaphore
	select {
	case tee.concurrentSem <- struct{}{}:
		defer func() { <-tee.concurrentSem }()
	case <-ctx.Done():
		result.ErrorMessage = "execution cancelled while waiting for concurrent slot"
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, ctx.Err()
	}

	// Check if we can run this tool in parallel (based on tool-specific limits)
	tee.runningMutex.Lock()
	currentRunning := tee.runningTools[toolName]
	
	// If this tool is already running and we're at the parallel limit, acquire parallel semaphore
	needsParallelSem := currentRunning >= 1
	
	if needsParallelSem {
		tee.runningMutex.Unlock()
		select {
		case tee.parallelSem <- struct{}{}:
			defer func() { <-tee.parallelSem }()
		case <-ctx.Done():
			result.ErrorMessage = "execution cancelled while waiting for parallel slot"
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			return result, ctx.Err()
		}
		tee.runningMutex.Lock()
	}
	
	// Track this execution
	tee.runningTools[toolName]++
	tee.runningMutex.Unlock()
	
	// Ensure we decrement the counter when done
	defer func() {
		tee.runningMutex.Lock()
		tee.runningTools[toolName]--
		if tee.runningTools[toolName] <= 0 {
			delete(tee.runningTools, toolName)
		}
		tee.runningMutex.Unlock()
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

	// Ensure all workspace directories exist
	dirsToCreate := []string{
		execCtx.Workspace,
		execCtx.ScansDir,
		execCtx.LogsDir,
		execCtx.ReportsDir,
		execCtx.RawDir,
	}
	
	for _, dir := range dirsToCreate {
		if dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				result.ErrorMessage = fmt.Sprintf("failed to create directory %s: %v", dir, err)
				result.EndTime = time.Now()
				result.Duration = result.EndTime.Sub(result.StartTime)
				return result, err
			}
		}
	}

	// Build variable map for template resolution
	vars := tee.templateResolver.buildVariableMap(execCtx)

	// Store the expected output path
	if outputPath, exists := vars["output_path"]; exists {
		result.OutputPath = outputPath
		
		// For certain tools, we need to append the file extension that they use
		// This is needed because the template resolves to base path but tools add extensions
		if toolName == "naabu" && !strings.HasSuffix(outputPath, ".json") {
			result.OutputPath = outputPath + ".json"
		} else if toolName == "nmap" && !strings.HasSuffix(outputPath, ".xml") {
			result.OutputPath = outputPath + ".xml"
		}
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

		// Capture output if requested
		if options.CaptureOutput {
			// Use MultiWriter to show output in real-time AND capture it
			stdoutMultiWriter := io.MultiWriter(os.Stdout, &stdoutBuf)
			stderrMultiWriter := io.MultiWriter(os.Stderr, &stderrBuf)
			execCmd.Stdout = stdoutMultiWriter
			execCmd.Stderr = stderrMultiWriter
		}

		// Run the command
		tee.debugLogger.Debug("Running command", "attempt", attempt+1, "max_attempts", retryAttempts+1)
		tee.writeDebugLog("Running command (attempt %d/%d)...", attempt+1, retryAttempts+1)
		lastErr = execCmd.Run()
		tee.debugLogger.Debug("Command completed", "error", lastErr)
		tee.writeDebugLog("Command completed with error: %v", lastErr)

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
			break
		}

		// Handle error
		if exitError, ok := lastErr.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		} else {
			result.ExitCode = -1
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
			fmt.Printf("Warning: Failed to process magic variables for %s: %v\n", toolName, err)
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
	tee.runningMutex.RLock()
	defer tee.runningMutex.RUnlock()
	
	status := map[string]interface{}{
		"concurrent_slots_available": cap(tee.concurrentSem) - len(tee.concurrentSem),
		"concurrent_slots_total":     cap(tee.concurrentSem),
		"parallel_slots_available":   cap(tee.parallelSem) - len(tee.parallelSem),
		"parallel_slots_total":       cap(tee.parallelSem),
		"running_tools":              make(map[string]int),
	}
	
	// Copy running tools map
	runningTools := make(map[string]int)
	for tool, count := range tee.runningTools {
		runningTools[tool] = count
	}
	status["running_tools"] = runningTools
	
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

