package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/neur0map/ipcrawler/internal/config"
	"github.com/neur0map/ipcrawler/internal/registry"
)

// ExecutionContext holds the runtime context for template resolution
type ExecutionContext struct {
	Target       string            // The target being scanned (IP, domain, etc.)
	OutputDir    string            // Base output directory from config
	Workspace    string            // Workspace directory (workspace/target)
	LogsDir      string            // Logs directory path
	ScansDir     string            // Scans directory path
	ReportsDir   string            // Reports directory path
	RawDir       string            // Raw output directory path
	OutputFile   string            // Specific output filename for this execution
	Timestamp    string            // Execution timestamp
	SessionID    string            // Unique session identifier
	ToolName     string            // Name of the tool being executed
	Mode         string            // Execution mode (aggressive, quick_scan, etc.)
	WorkflowName string            // Name of the workflow (for unique filenames)
	StepName     string            // Name of the workflow step (for unique filenames)
	CustomVars   map[string]string // Additional custom variables
}

// TemplateResolver resolves template variables in tool configurations
type TemplateResolver struct {
	config         *config.Config
	magicVars      map[string]string
	magicMutex     sync.RWMutex
	registryManager registry.RegistryManager // Optional registry for auto-detection
}

// NewTemplateResolver creates a new template resolver with the given configuration
func NewTemplateResolver(cfg *config.Config) *TemplateResolver {
	return &TemplateResolver{
		config:    cfg,
		magicVars: make(map[string]string),
	}
}

// SetRegistryManager sets the registry manager for auto-detection
func (tr *TemplateResolver) SetRegistryManager(manager registry.RegistryManager) {
	tr.registryManager = manager
}

// ResolveArguments resolves template variables in tool arguments
func (tr *TemplateResolver) ResolveArguments(args []string, ctx *ExecutionContext) ([]string, error) {
	if ctx == nil {
		return nil, fmt.Errorf("execution context cannot be nil")
	}

	// Validate required context fields
	if err := tr.validateContext(ctx); err != nil {
		return nil, fmt.Errorf("invalid execution context: %w", err)
	}

	// Prepare the variable map
	vars := tr.buildVariableMap(ctx)

	// Resolve each argument
	resolved := make([]string, len(args))
	for i, arg := range args {
		resolved[i] = tr.resolveString(arg, vars)
	}

	return resolved, nil
}

// validateContext validates that required context fields are present
func (tr *TemplateResolver) validateContext(ctx *ExecutionContext) error {
	if ctx.Target == "" {
		return fmt.Errorf("target is required")
	}
	if ctx.ToolName == "" {
		return fmt.Errorf("tool name is required")
	}
	return nil
}

// buildVariableMap creates a map of all available template variables
func (tr *TemplateResolver) buildVariableMap(ctx *ExecutionContext) map[string]string {
	vars := make(map[string]string)

	// Target-related variables
	vars["target"] = ctx.Target

	// Workspace and output directory variables
	if ctx.Workspace != "" {
		vars["workspace"] = ctx.Workspace
		vars["output_dir"] = ctx.Workspace // For backward compatibility
	} else if ctx.OutputDir != "" {
		vars["output_dir"] = ctx.OutputDir
		vars["workspace"] = ctx.OutputDir // Use output_dir as workspace if not set
	} else {
		// Use raw output directory from config as default
		vars["output_dir"] = tr.config.Output.Raw.Directory
		vars["workspace"] = tr.config.Output.Raw.Directory
	}

	// Add specific directory paths if provided
	if ctx.LogsDir != "" {
		vars["logs_dir"] = ctx.LogsDir
	}
	if ctx.ScansDir != "" {
		vars["scans_dir"] = ctx.ScansDir
	}
	if ctx.ReportsDir != "" {
		vars["reports_dir"] = ctx.ReportsDir
	}
	if ctx.RawDir != "" {
		vars["raw_dir"] = ctx.RawDir
	}

	// Output file handling based on scan_output_mode
	if ctx.OutputFile == "" {
		// Generate default filename based on tool, target, and scan output mode
		timestamp := ctx.Timestamp
		if timestamp == "" {
			timestamp = time.Now().Format("20060102_150405")
		}

		// Sanitize target for filename (replace problematic characters)
		sanitizedTarget := tr.sanitizeForFilename(ctx.Target)
		
		// Handle different output modes
		outputMode := tr.config.Output.ScanOutputMode
		
		// Create unique identifier from workflow and step names
		workflowID := ""
		if ctx.WorkflowName != "" {
			workflowID = "_" + strings.ReplaceAll(strings.ToLower(ctx.WorkflowName), " ", "-")
		}
		if ctx.StepName != "" {
			workflowID += "_" + strings.ReplaceAll(strings.ToLower(ctx.StepName), " ", "-")
		}
		
		switch outputMode {
		case "overwrite":
			// No timestamp - same filename always overwrites
			vars["output_file"] = fmt.Sprintf("%s_%s%s", ctx.ToolName, sanitizedTarget, workflowID)
		case "timestamp":
			// Include timestamp for unique files
			vars["output_file"] = fmt.Sprintf("%s_%s%s_%s", ctx.ToolName, sanitizedTarget, workflowID, timestamp)
		case "both":
			// Default to timestamped, but we'll also create a latest link
			vars["output_file"] = fmt.Sprintf("%s_%s%s_%s", ctx.ToolName, sanitizedTarget, workflowID, timestamp)
			vars["output_file_latest"] = fmt.Sprintf("%s_%s%s_latest", ctx.ToolName, sanitizedTarget, workflowID)
		default:
			// Fallback to timestamp mode
			vars["output_file"] = fmt.Sprintf("%s_%s%s_%s", ctx.ToolName, sanitizedTarget, workflowID, timestamp)
		}
	} else {
		vars["output_file"] = ctx.OutputFile
	}

	// Session and execution metadata
	if ctx.SessionID != "" {
		vars["session_id"] = ctx.SessionID
	}
	if ctx.Timestamp != "" {
		vars["timestamp"] = ctx.Timestamp
	}
	if ctx.Mode != "" {
		vars["mode"] = ctx.Mode
	}

	// Tool-specific variables
	vars["tool_name"] = ctx.ToolName

	// Additional custom variables
	for key, value := range ctx.CustomVars {
		vars[key] = value
	}

	// Magic variables from completed tools
	tr.magicMutex.RLock()
	for key, value := range tr.magicVars {
		vars[key] = value
	}
	tr.magicMutex.RUnlock()

	// Derived variables - handle scans_dir specifically for better path resolution
	if scansDir, exists := vars["scans_dir"]; exists {
		if outputFile, exists := vars["output_file"]; exists {
			vars["output_path"] = filepath.Join(scansDir, outputFile)
		}
		if outputFileLatest, exists := vars["output_file_latest"]; exists {
			vars["output_path_latest"] = filepath.Join(scansDir, outputFileLatest)
		}
	} else if outputDir, exists := vars["output_dir"]; exists {
		// Fallback to output_dir if scans_dir not available
		if outputFile, exists := vars["output_file"]; exists {
			vars["output_path"] = filepath.Join(outputDir, outputFile)
		}
		if outputFileLatest, exists := vars["output_file_latest"]; exists {
			vars["output_path_latest"] = filepath.Join(outputDir, outputFileLatest)
		}
	}

	return vars
}

// resolveString resolves template variables in a single string
func (tr *TemplateResolver) resolveString(input string, vars map[string]string) string {
	result := input

	// Replace all {{variable}} patterns
	for varName, varValue := range vars {
		placeholder := fmt.Sprintf("{{%s}}", varName)
		result = strings.ReplaceAll(result, placeholder, varValue)
	}

	return result
}

// sanitizeForFilename removes or replaces characters that are problematic in filenames
func (tr *TemplateResolver) sanitizeForFilename(input string) string {
	// Replace common problematic characters
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

// GetAvailableVariables returns a list of all available template variables for documentation
func (tr *TemplateResolver) GetAvailableVariables() []string {
	return []string{
		"target",             // The target being scanned
		"output_dir",         // Output directory from config
		"output_file",        // Generated or specified output filename
		"output_file_latest", // Latest version filename (when scan_output_mode is "both")
		"output_path",        // Full path: output_dir/output_file
		"output_path_latest", // Full path to latest version (when available)
		"timestamp",          // Execution timestamp
		"session_id",         // Session identifier
		"tool_name",          // Name of the tool
		"mode",               // Execution mode
		// Custom variables can be added via ExecutionContext.CustomVars
	}
}

// CreateExecutionContext creates a basic execution context with configurable defaults
func (tr *TemplateResolver) CreateExecutionContext(target, toolName, mode string) *ExecutionContext {
	return tr.CreateExecutionContextWithWorkflow(target, toolName, mode, "", "")
}

func (tr *TemplateResolver) CreateExecutionContextWithWorkflow(target, toolName, mode, workflowName, stepName string) *ExecutionContext {
	timestamp := time.Now().Format("20060102_150405")
	sessionID := fmt.Sprintf("session_%s", timestamp)

	return &ExecutionContext{
		Target:       target,
		ToolName:     toolName,
		Mode:         mode,
		WorkflowName: workflowName,
		StepName:     stepName,
		Timestamp:    timestamp,
		SessionID:    sessionID,
		CustomVars:   make(map[string]string),
	}
}

// CreateLatestSymlink creates a symlink to the latest scan result
func (tr *TemplateResolver) CreateLatestSymlink(timestampedPath, latestPath string) error {
	// Only create symlinks if the config option is enabled
	if !tr.config.Output.CreateLatestLinks {
		return nil
	}

	// Only proceed if scan_output_mode is "both"
	if tr.config.Output.ScanOutputMode != "both" {
		return nil
	}

	// Remove existing symlink if it exists
	if _, err := os.Lstat(latestPath); err == nil {
		if err := os.Remove(latestPath); err != nil {
			return fmt.Errorf("failed to remove existing latest symlink: %w", err)
		}
	}

	// Create new symlink pointing to the timestamped file
	if err := os.Symlink(filepath.Base(timestampedPath), latestPath); err != nil {
		return fmt.Errorf("failed to create latest symlink: %w", err)
	}

	return nil
}

// AddVariable adds a magic variable for template resolution
func (tr *TemplateResolver) AddVariable(name, value string) {
	tr.magicMutex.Lock()
	defer tr.magicMutex.Unlock()
	tr.magicVars[name] = value
	
	// Auto-register with registry if available
	if tr.registryManager != nil {
		context := registry.DetectionContext{
			FilePath:   "runtime",
			LineNumber: 0,
			Context:    fmt.Sprintf("Variable added at runtime: %s = %s", name, value),
			Source:     registry.ExecutionContextSource,
			Tool:       "",
			Timestamp:  time.Now(),
		}
		
		// Attempt to auto-register (ignore errors to avoid disrupting execution)
		tr.registryManager.AutoRegisterVariable(name, context)
	}
}

// GetAllVariables returns all current variables (regular + magic)
func (tr *TemplateResolver) GetAllVariables() map[string]string {
	tr.magicMutex.RLock()
	defer tr.magicMutex.RUnlock()
	
	// Create a copy to avoid race conditions
	result := make(map[string]string)
	for k, v := range tr.magicVars {
		result[k] = v
	}
	return result
}

// ClearMagicVariables clears all magic variables (useful for testing)
func (tr *TemplateResolver) ClearMagicVariables() {
	tr.magicMutex.Lock()
	defer tr.magicMutex.Unlock()
	tr.magicVars = make(map[string]string)
}

// MapWorkflowVariable maps a workflow variable from source to target name
// This allows workflows to define how tool outputs map to tool inputs
func (tr *TemplateResolver) MapWorkflowVariable(sourceVar, targetVar string) {
	tr.magicMutex.RLock()
	sourceValue, exists := tr.magicVars[sourceVar]
	tr.magicMutex.RUnlock()
	
	if exists {
		tr.AddVariable(targetVar, sourceValue)
		
		// Track workflow variable mapping in registry
		if tr.registryManager != nil {
			context := registry.DetectionContext{
				FilePath:   "workflow",
				LineNumber: 0,
				Context:    fmt.Sprintf("Workflow mapping: %s -> %s", sourceVar, targetVar),
				Source:     registry.WorkflowFileSource,
				Tool:       "",
				Timestamp:  time.Now(),
			}
			
			// Register both the mapping and the target variable
			tr.registryManager.AutoRegisterVariable(targetVar, context)
		}
	}
}
