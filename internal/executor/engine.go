package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
}

// NewToolExecutionEngine creates a new tool execution engine
func NewToolExecutionEngine(globalConfig *config.Config, toolsPath string) *ToolExecutionEngine {
	return &ToolExecutionEngine{
		configLoader:     NewToolConfigLoader(toolsPath),
		templateResolver: NewTemplateResolver(globalConfig),
		globalConfig:     globalConfig,
		toolsPath:        toolsPath,
	}
}

// ExecuteTool executes a tool with the specified parameters
func (tee *ToolExecutionEngine) ExecuteTool(ctx context.Context, toolName, mode, target string, options *ExecutionOptions) (*ExecutionResult, error) {
	startTime := time.Now()

	result := &ExecutionResult{
		ToolName:  toolName,
		Mode:      mode,
		Target:    target,
		StartTime: startTime,
		Success:   false,
	}

	// Load tool configuration
	toolConfig, err := tee.configLoader.LoadToolConfig(toolName)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to load tool config: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, err
	}

	// Get tool arguments for the specified mode
	argsTemplate, err := toolConfig.GetToolArguments(mode)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to get tool arguments: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, err
	}

	// Create execution context
	execCtx := tee.templateResolver.CreateExecutionContext(target, toolName, mode)

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

	result.CommandLine = append([]string{toolName}, resolvedArgs...)

	// Determine the tool executable path
	toolExecutable, err := tee.findToolExecutable(toolName)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to find tool executable: %v", err)
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
		if tee.globalConfig != nil && tee.globalConfig.Security.Scanning.TimeoutSeconds > 0 {
			options.Timeout = time.Duration(tee.globalConfig.Security.Scanning.TimeoutSeconds) * time.Second
		} else {
			options.Timeout = 30 * time.Minute // Fallback default
		}
	}

	// Create context with timeout
	execContext, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	// Ensure output directory exists
	vars := tee.templateResolver.buildVariableMap(execCtx)
	outputDir := vars["output_dir"]
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to create output directory: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, err
	}

	// Store the expected output path
	if outputPath, exists := vars["output_path"]; exists {
		result.OutputPath = outputPath
	}

	// Execute the tool
	cmd := exec.CommandContext(execContext, toolExecutable, resolvedArgs...)

	// Set working directory
	if options.WorkingDir != "" {
		cmd.Dir = options.WorkingDir
	}

	// Set environment variables
	cmd.Env = os.Environ()
	for key, value := range options.Environment {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Run the command
	err = cmd.Run()

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		} else {
			result.ExitCode = -1
		}
		result.ErrorMessage = fmt.Sprintf("tool execution failed: %v", err)
		return result, err
	}

	result.Success = true
	result.ExitCode = 0

	// Validate output file was created if requested
	if options.ValidateOutput && result.OutputPath != "" {
		if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("expected output file not created: %s", result.OutputPath)
			return result, fmt.Errorf("output validation failed: %s", result.ErrorMessage)
		}
	}

	return result, nil
}

// findToolExecutable locates the executable for a tool
func (tee *ToolExecutionEngine) findToolExecutable(toolName string) (string, error) {
	// Try common executable names and paths
	candidates := []string{
		toolName,                               // Direct name (if in PATH)
		filepath.Join(tee.toolsPath, toolName), // In tools directory
		filepath.Join(tee.toolsPath, "bin", toolName),    // In tools/bin
		filepath.Join(tee.toolsPath, toolName, toolName), // In tools/toolname/toolname
	}

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
	execCtx := tee.templateResolver.CreateExecutionContext(target, toolName, mode)

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
