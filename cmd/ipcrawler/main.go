package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	"github.com/charmbracelet/log"
	"github.com/neur0map/ipcrawler/internal/config"
	"github.com/neur0map/ipcrawler/internal/executor"
	"github.com/neur0map/ipcrawler/internal/output"
)

// isValidHostname performs comprehensive hostname validation according to RFC standards
func isValidHostname(hostname string) bool {
	// Check total length
	if len(hostname) == 0 || len(hostname) > 253 {
		return false
	}

	// Remove trailing dot (FQDN)
	if strings.HasSuffix(hostname, ".") {
		hostname = hostname[:len(hostname)-1]
	}

	// Split into labels
	labels := strings.Split(hostname, ".")
	if len(labels) == 0 {
		return false
	}

	for _, label := range labels {
		if !isValidLabel(label) {
			return false
		}
	}

	return true
}

// isValidLabel validates a single hostname label
func isValidLabel(label string) bool {
	// Label length check (RFC 1035: max 63 characters)
	if len(label) == 0 || len(label) > 63 {
		return false
	}

	// Must not start or end with hyphen
	if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return false
	}

	// Check characters: only letters, digits, and hyphens allowed
	for _, r := range label {
		if !((r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-') {
			return false
		}
	}

	return true
}

// sanitizeTargetForPath converts a target (IP, hostname, CIDR) to a safe directory name

// getProjectDirectory returns the directory where the project files are located
func getProjectDirectory() (string, error) {
	// Try to get executable directory first (for built binaries)
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		// Check if this looks like a project directory by looking for key files
		if _, err := os.Stat(filepath.Join(execDir, "go.mod")); err == nil {
			return execDir, nil
		}
		// If go.mod not found, try parent directory (common when binary is in bin/)
		parentDir := filepath.Dir(execDir)
		if _, err := os.Stat(filepath.Join(parentDir, "go.mod")); err == nil {
			return parentDir, nil
		}
	}

	// Fallback: try current working directory
	if cwd, err := os.Getwd(); err == nil {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return cwd, nil
		}
	}

	// Last resort: use current working directory anyway
	return os.Getwd()
}

// getTerminalSize returns the actual terminal dimensions
func getTerminalSize() (int, int) {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		// Fallback to tput if stty fails
		rowsCmd := exec.Command("tput", "lines")
		colsCmd := exec.Command("tput", "cols")

		rowsOut, err1 := rowsCmd.Output()
		colsOut, err2 := colsCmd.Output()

		if err1 == nil && err2 == nil {
			rows := strings.TrimSpace(string(rowsOut))
			cols := strings.TrimSpace(string(colsOut))

			var height, width int
			fmt.Sscanf(rows, "%d", &height)
			fmt.Sscanf(cols, "%d", &width)

			return width, height
		}

		// Final fallback
		return 80, 24
	}

	var height, width int
	fmt.Sscanf(string(out), "%d %d", &height, &width)
	return width, height
}

// loadWorkflowFromPath loads a workflow from a specific file path
func loadWorkflowFromPath(filePath string) (*executor.Workflow, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow file %s: %v", filePath, err)
	}

	// Define a temporary struct with proper YAML tags for unmarshaling
	type yamlWorkflowStep struct {
		Name               string            `yaml:"name"`
		Tool               string            `yaml:"tool"`
		Description        string            `yaml:"description"`
		Modes              []string          `yaml:"modes"`
		Concurrent         bool              `yaml:"concurrent"`
		CombineResults     bool              `yaml:"combine_results"`
		DependsOn          string            `yaml:"depends_on"`
		StepPriority       string            `yaml:"step_priority"`
		MaxConcurrentTools int               `yaml:"max_concurrent_tools"`
		Variables          map[string]string `yaml:"variables"`
	}

	type yamlWorkflow struct {
		Name                   string             `yaml:"name"`
		Description            string             `yaml:"description"`
		Category               string             `yaml:"category"`
		ParallelWorkflow       bool               `yaml:"parallel_workflow"`
		IndependentExecution   bool               `yaml:"independent_execution"`
		MaxConcurrentWorkflows int                `yaml:"max_concurrent_workflows"`
		WorkflowPriority       string             `yaml:"workflow_priority"`
		Steps                  []yamlWorkflowStep `yaml:"steps"`
	}

	var yamlWf yamlWorkflow
	if err := yaml.Unmarshal(data, &yamlWf); err != nil {
		return nil, fmt.Errorf("failed to parse workflow YAML %s: %v", filePath, err)
	}

	// Convert to executor.Workflow
	workflow := &executor.Workflow{
		Name:                   yamlWf.Name,
		Description:            yamlWf.Description,
		Category:               yamlWf.Category,
		ParallelWorkflow:       yamlWf.ParallelWorkflow,
		IndependentExecution:   yamlWf.IndependentExecution,
		MaxConcurrentWorkflows: yamlWf.MaxConcurrentWorkflows,
		WorkflowPriority:       yamlWf.WorkflowPriority,
		Steps:                  make([]*executor.WorkflowStep, len(yamlWf.Steps)),
	}

	// Convert steps
	for i, yamlStep := range yamlWf.Steps {
		workflow.Steps[i] = &executor.WorkflowStep{
			Name:               yamlStep.Name,
			Tool:               yamlStep.Tool,
			Description:        yamlStep.Description,
			Modes:              yamlStep.Modes,
			Concurrent:         yamlStep.Concurrent,
			CombineResults:     yamlStep.CombineResults,
			DependsOn:          yamlStep.DependsOn,
			StepPriority:       yamlStep.StepPriority,
			MaxConcurrentTools: yamlStep.MaxConcurrentTools,
			Variables:          yamlStep.Variables,
		}
	}

	return workflow, nil
}

// discoverAllWorkflows automatically discovers all workflow files in the workflows directory
func discoverAllWorkflows() (map[string]*executor.Workflow, error) {
	workflows := make(map[string]*executor.Workflow)

	err := filepath.WalkDir("workflows", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip descriptions.yaml (metadata only)
		if d.Name() == "descriptions.yaml" {
			return nil
		}

		// Process .yaml files
		if strings.HasSuffix(d.Name(), ".yaml") {
			workflow, err := loadWorkflowFromPath(path)
			if err != nil {
				// Log warning using simple fmt for now (logger will be enhanced separately)
				fmt.Fprintf(os.Stderr, "WARN: Failed to load workflow %s: %v\n", path, err)
				return nil // Continue processing other files
			}

			workflowKey := strings.TrimSuffix(d.Name(), ".yaml")
			workflows[workflowKey] = workflow
		}

		return nil
	})

	return workflows, err
}

// runCLI executes all workflows in CLI mode without TUI
func runCLI(target string, outputMode output.OutputMode) error {
	// Initialize logger for CLI output - suppress if not in verbose/debug mode
	var logger *log.Logger
	if outputMode == output.OutputModeVerbose || outputMode == output.OutputModeDebug {
		logger = log.NewWithOptions(os.Stderr, log.Options{
			ReportCaller:    false,
			ReportTimestamp: true,
			TimeFormat:      time.Kitchen,
			Prefix:          "IPCrawler CLI",
		})
	} else {
		// In normal mode, create a silent logger (sends to /dev/null)
		logger = log.NewWithOptions(io.Discard, log.Options{
			ReportCaller:    false,
			ReportTimestamp: true,
			TimeFormat:      time.Kitchen,
			Prefix:          "IPCrawler CLI",
		})
	}

	logger.Info("=== IPCrawler CLI Mode ===", "target", target)

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %v", err)
	}

	// Validate target
	if target == "" {
		return fmt.Errorf("target cannot be empty")
	}

	// Create workspace directory
	sanitizedTarget := sanitizeTargetForPath(target)
	timestamp := time.Now().Unix()
	workspaceDir := filepath.Join(cfg.Output.WorkspaceBase, fmt.Sprintf("%s_%d", sanitizedTarget, timestamp))

	if err := createWorkspaceStructure(workspaceDir); err != nil {
		return fmt.Errorf("failed to create workspace: %v", err)
	}

	logger.Info("Workspace created", "path", workspaceDir)

	// Set up workspace file logging
	debugLogger, infoLogger, rawLogger, logCleanup, err := setupWorkspaceLogging(workspaceDir)
	if err != nil {
		return fmt.Errorf("failed to setup workspace logging: %v", err)
	}
	defer logCleanup.Close() // Ensure file handles are closed when function exits

	// Make loggers available globally for executors
	setGlobalLoggers(debugLogger, infoLogger, rawLogger)

	// Discover all workflows
	workflows, err := discoverAllWorkflows()
	if err != nil {
		return fmt.Errorf("failed to discover workflows: %v", err)
	}

	if len(workflows) == 0 {
		return fmt.Errorf("no workflows found in workflows directory")
	}

	// Initialize output controller for tree display
	outputController := output.NewOutputController(outputMode)
	globalOutputController = outputController

	// Display workflow tree (always shown regardless of output mode)
	outputController.PrintWorkflowTree("workflows", nil)

	// Log discovered workflows
	workflowNames := make([]string, 0, len(workflows))
	for name, workflow := range workflows {
		workflowNames = append(workflowNames, name)
		logger.Info("Discovered workflow", "name", name, "title", workflow.Name, "description", workflow.Description)
	}

	logger.Info("Starting workflow execution", "count", len(workflows), "workflows", strings.Join(workflowNames, ", "))

	// Initialize execution engine and orchestrator
	executionEngine := executor.NewToolExecutionEngine(cfg, "", outputMode)

	// Set the workspace base directory for consistent path resolution
	executionEngine.SetWorkspaceBase(workspaceDir)

	// Set output mode explicitly (in case it's needed)
	executionEngine.SetOutputMode(outputMode)

	// Set up workspace logging for tool execution engine
	if err := executionEngine.SetWorkspaceLoggers(workspaceDir); err != nil {
		return fmt.Errorf("failed to setup tool execution engine logging: %v", err)
	}

	workflowExecutor := executor.NewWorkflowExecutor(executionEngine)
	workflowOrchestrator := executor.NewWorkflowOrchestrator(workflowExecutor, cfg)

	// Set output mode before setting up loggers
	workflowOrchestrator.SetOutputMode(outputMode)

	// Set up workspace logging for workflow orchestrator
	if err := workflowOrchestrator.SetWorkspaceLoggers(workspaceDir); err != nil {
		return fmt.Errorf("failed to setup workflow orchestrator logging: %v", err)
	}

	// Set up status callback for CLI logging
	workflowOrchestrator.SetStatusCallback(func(workflowName, target, status, message string) {
		logger.Info("Workflow status", "workflow", workflowName, "target", target, "status", status, "message", message)
	})

	// Queue all workflows
	var ctx context.Context
	var cancel context.CancelFunc

	// Set timeout from configuration
	if cfg.Tools.CLIMode.ExecutionTimeoutSeconds > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(cfg.Tools.CLIMode.ExecutionTimeoutSeconds)*time.Second)
		logger.Info("CLI execution timeout set", "seconds", cfg.Tools.CLIMode.ExecutionTimeoutSeconds)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
		logger.Info("CLI execution timeout disabled (unlimited)")
	}
	defer cancel()

	for workflowName, workflow := range workflows {
		logger.Info("Queueing workflow", "name", workflowName, "title", workflow.Name)
		if err := workflowOrchestrator.QueueWorkflow(workflow, target); err != nil {
			logger.Error("Failed to queue workflow", "name", workflowName, "error", err)
			continue
		}
	}

	// Execute queued workflows
	logger.Info("Executing queued workflows...")
	if err := workflowOrchestrator.ExecuteQueuedWorkflows(ctx); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			logger.Warn("Workflow execution timed out", "timeout_seconds", cfg.Tools.CLIMode.ExecutionTimeoutSeconds)
		}
		return fmt.Errorf("failed to execute workflows: %v", err)
	}

	logger.Info("All workflows completed successfully")
	return nil
}

// Helper functions for CLI mode
func sanitizeTargetForPath(target string) string {
	// Replace special characters for safe directory names
	sanitized := strings.ReplaceAll(target, ".", "_")
	sanitized = strings.ReplaceAll(sanitized, ":", "_")
	sanitized = strings.ReplaceAll(sanitized, "/", "_")
	sanitized = strings.ReplaceAll(sanitized, "\\", "_")
	return sanitized
}

func createWorkspaceStructure(workspaceDir string) error {
	// Create base workspace directory
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return err
	}

	// Create subdirectories
	subdirs := []string{"logs/info", "logs/debug", "logs/error", "logs/warning", "raw", "scans", "reports"}
	for _, subdir := range subdirs {
		if err := os.MkdirAll(filepath.Join(workspaceDir, subdir), 0755); err != nil {
			return err
		}
	}

	return nil
}

// LoggerCleanup holds file handles that need to be closed
type LoggerCleanup struct {
	files []*os.File
}

// Close closes all log files
func (lc *LoggerCleanup) Close() error {
	var lastErr error
	for _, file := range lc.files {
		if err := file.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// setupWorkspaceLogging creates file loggers for the workspace
func setupWorkspaceLogging(workspaceDir string) (*log.Logger, *log.Logger, *log.Logger, *LoggerCleanup, error) {
	cleanup := &LoggerCleanup{}

	// Create debug logger
	debugFile, err := os.OpenFile(filepath.Join(workspaceDir, "logs/debug/execution.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create debug log file: %v", err)
	}
	cleanup.files = append(cleanup.files, debugFile)

	debugLogger := log.NewWithOptions(debugFile, log.Options{
		ReportCaller:    false,
		ReportTimestamp: true,
		TimeFormat:      time.RFC3339,
		Prefix:          "DEBUG",
	})

	// Create info logger
	infoFile, err := os.OpenFile(filepath.Join(workspaceDir, "logs/info/workflow.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		cleanup.Close() // Clean up already opened files
		return nil, nil, nil, nil, fmt.Errorf("failed to create info log file: %v", err)
	}
	cleanup.files = append(cleanup.files, infoFile)

	infoLogger := log.NewWithOptions(infoFile, log.Options{
		ReportCaller:    false,
		ReportTimestamp: true,
		TimeFormat:      time.RFC3339,
		Prefix:          "INFO",
	})

	// Create raw output logger
	rawFile, err := os.OpenFile(filepath.Join(workspaceDir, "raw/tool_output.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		cleanup.Close() // Clean up already opened files
		return nil, nil, nil, nil, fmt.Errorf("failed to create raw output file: %v", err)
	}
	cleanup.files = append(cleanup.files, rawFile)

	rawLogger := log.NewWithOptions(rawFile, log.Options{
		ReportCaller:    false,
		ReportTimestamp: true,
		TimeFormat:      time.RFC3339,
		Prefix:          "RAW",
	})

	return debugLogger, infoLogger, rawLogger, cleanup, nil
}

// Global loggers for executor modules
var (
	globalDebugLogger *log.Logger
	globalInfoLogger  *log.Logger
	globalRawLogger   *log.Logger
)

// setGlobalLoggers makes the workspace loggers available to executor modules
func setGlobalLoggers(debugLogger, infoLogger, rawLogger *log.Logger) {
	globalDebugLogger = debugLogger
	globalInfoLogger = infoLogger
	globalRawLogger = rawLogger
}

// Global output controller for use across the application
var globalOutputController *output.OutputController

// logDebug writes debug messages to both console and file
func logDebug(msg string, args ...interface{}) {
	// Use output controller if available, otherwise fallback to direct printing
	if globalOutputController != nil {
		globalOutputController.PrintLog("DEBUG", msg, args...)
	} else {
		// Fallback for when output controller is not yet set
		if len(args) > 0 {
			fmt.Printf("[DEBUG] "+msg+"\n", args...)
		} else {
			fmt.Printf("[DEBUG] %s\n", msg)
		}
	}

	// Also write to file if available
	if globalDebugLogger != nil {
		if len(args) > 0 {
			globalDebugLogger.Debugf(msg, args...)
		} else {
			globalDebugLogger.Debug(msg)
		}
	}
}

// logRaw writes raw tool output to both console and file
func logRaw(toolName, mode, output string) {
	// Use output controller if available, otherwise fallback to direct printing
	if globalOutputController != nil {
		globalOutputController.PrintRawSection(toolName, mode, output)
	} else {
		// Fallback for when output controller is not yet set
		fmt.Printf("\n=== RAW OUTPUT: %s %s ===\n", toolName, mode)
		fmt.Print(output)
		fmt.Printf("=== END OUTPUT ===\n\n")
	}

	// Also write to file if available
	if globalRawLogger != nil {
		globalRawLogger.Infof("=== %s %s ===\n%s", toolName, mode, output)
	}
}

func main() {
	// Define flags
	var (
		verbose = pflag.BoolP("verbose", "v", false, "Show both logs and raw tool output")
		debug   = pflag.BoolP("debug", "d", false, "Show only logs, no raw tool output")
		help    = pflag.BoolP("help", "h", false, "Show this help message")
	)

	// Parse flags
	pflag.Parse()

	// Show help if requested
	if *help {
		fmt.Fprintf(os.Stderr, "Usage: %s [FLAGS] <target>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "       %s registry <command>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nFlags:\n")
		pflag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nOutput Modes:\n")
		fmt.Fprintf(os.Stderr, "  Normal (default): Only raw tool output\n")
		fmt.Fprintf(os.Stderr, "  -v, --verbose:    Both logs and raw tool output\n")
		fmt.Fprintf(os.Stderr, "  -d, --debug:      Only logs, no raw tool output\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s google.com\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -v 192.168.1.1\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --debug example.com\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s registry list\n", os.Args[0])
		os.Exit(0)
	}

	// Get remaining arguments after flag parsing
	args := pflag.Args()

	// Check for registry command
	if len(args) > 0 && args[0] == "registry" {
		if err := runRegistryCommand(args); err != nil {
			fmt.Fprintf(os.Stderr, "Registry command failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Require target argument
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: target argument is required\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [FLAGS] <target>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Use --help for more information\n")
		os.Exit(1)
	}

	// Determine output mode
	var outputMode output.OutputMode
	if *debug && *verbose {
		fmt.Fprintf(os.Stderr, "Error: cannot use both --debug and --verbose flags together\n")
		os.Exit(1)
	} else if *debug {
		outputMode = output.OutputModeDebug
	} else if *verbose {
		outputMode = output.OutputModeVerbose
	} else {
		outputMode = output.OutputModeNormal
	}

	// Set global output controller before running CLI
	globalOutputController = output.NewOutputController(outputMode)

	// Run CLI with target and output mode
	target := args[0]
	if err := runCLI(target, outputMode); err != nil {
		fmt.Fprintf(os.Stderr, "CLI execution failed: %v\n", err)
		os.Exit(1)
	}
}

// isRunningAsRoot checks if the current process is running with root privileges
func isRunningAsRoot() bool {
	// Check if UID is 0 (root)
	if os.Geteuid() == 0 {
		return true
	}
	return false
}

// isRunningWithSudo checks if the process was started with sudo
func isRunningWithSudo() bool {
	// Check SUDO_UID environment variable (set by sudo)
	if os.Getenv("SUDO_UID") != "" {
		return true
	}

	// Check if we're root but SUDO_USER is set
	if isRunningAsRoot() && os.Getenv("SUDO_USER") != "" {
		return true
	}

	return false
}

// isRootlessEnvironment detects if we're in a rootless environment like containers/HTB
func isRootlessEnvironment() bool {
	// Check if we're running as root but in a container-like environment
	if !isRunningAsRoot() {
		return false
	}

	// Check for container indicators
	containerIndicators := []string{
		"/.dockerenv",        // Docker
		"/run/.containerenv", // Podman
		"/proc/1/cgroup",     // Check if we can read cgroup (container sign)
	}

	for _, indicator := range containerIndicators {
		if _, err := os.Stat(indicator); err == nil {
			return true
		}
	}

	// Check if we're in a limited root environment
	// HTB machines often have root but with limited capabilities
	if isRunningAsRoot() {
		// Check if we can access typical root-only files
		restrictedPaths := []string{
			"/etc/shadow",
			"/root/.ssh",
		}

		accessCount := 0
		for _, path := range restrictedPaths {
			if _, err := os.Stat(path); err == nil {
				accessCount++
			}
		}

		// If we're root but can't access typical root files, likely rootless
		if accessCount == 0 {
			return true
		}
	}

	return false
}

// getPrivilegeStatus returns a description of current privilege level
func getPrivilegeStatus() (bool, string) {
	if isRunningAsRoot() {
		if isRunningWithSudo() {
			return true, "Running with sudo privileges"
		} else if isRootlessEnvironment() {
			return true, "Running in rootless environment (container/sandbox)"
		} else {
			return true, "Running as root user"
		}
	}

	// Check if user might have capabilities without being root
	currentUser, err := user.Current()
	if err == nil && currentUser.Username != "" {
		// Check if user is in privileged groups
		groups := []string{"wheel", "admin", "sudo", "root"}
		for _, group := range groups {
			if checkUserInGroup(currentUser.Username, group) {
				return false, fmt.Sprintf("Running as %s (member of %s group)", currentUser.Username, group)
			}
		}
		return false, fmt.Sprintf("Running as %s (unprivileged)", currentUser.Username)
	}

	return false, "Running as unprivileged user"
}

// checkUserInGroup checks if a user is in a specific group (Unix-like systems)
func checkUserInGroup(username, groupname string) bool {
	if runtime.GOOS == "windows" {
		return false // Skip group checking on Windows
	}

	cmd := exec.Command("id", "-Gn", username)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	groups := strings.Fields(string(output))
	for _, group := range groups {
		if group == groupname {
			return true
		}
	}
	return false
}
