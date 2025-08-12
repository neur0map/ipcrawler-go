package executor

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/neur0map/ipcrawler/internal/config"
)

// ExecutionContext holds the runtime context for template resolution
type ExecutionContext struct {
	Target     string            // The target being scanned (IP, domain, etc.)
	OutputDir  string            // Base output directory from config
	Workspace  string            // Workspace directory (workspace/target)
	LogsDir    string            // Logs directory path
	ScansDir   string            // Scans directory path
	ReportsDir string            // Reports directory path
	RawDir     string            // Raw output directory path
	OutputFile string            // Specific output filename for this execution
	Timestamp  string            // Execution timestamp
	SessionID  string            // Unique session identifier
	ToolName   string            // Name of the tool being executed
	Mode       string            // Execution mode (aggressive, quick_scan, etc.)
	CustomVars map[string]string // Additional custom variables
}

// TemplateResolver resolves template variables in tool configurations
type TemplateResolver struct {
	config *config.Config
}

// NewTemplateResolver creates a new template resolver with the given configuration
func NewTemplateResolver(cfg *config.Config) *TemplateResolver {
	return &TemplateResolver{
		config: cfg,
	}
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

	// Output file handling
	if ctx.OutputFile == "" {
		// Generate default filename based on tool, target, and timestamp
		timestamp := ctx.Timestamp
		if timestamp == "" {
			timestamp = time.Now().Format("20060102_150405")
		}

		// Sanitize target for filename (replace problematic characters)
		sanitizedTarget := tr.sanitizeForFilename(ctx.Target)
		vars["output_file"] = fmt.Sprintf("%s_%s_%s", ctx.ToolName, sanitizedTarget, timestamp)
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

	// Derived variables
	if outputDir, exists := vars["output_dir"]; exists {
		if outputFile, exists := vars["output_file"]; exists {
			vars["output_path"] = filepath.Join(outputDir, outputFile)
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
		"target",      // The target being scanned
		"output_dir",  // Output directory from config
		"output_file", // Generated or specified output filename
		"output_path", // Full path: output_dir/output_file
		"timestamp",   // Execution timestamp
		"session_id",  // Session identifier
		"tool_name",   // Name of the tool
		"mode",        // Execution mode
		// Custom variables can be added via ExecutionContext.CustomVars
	}
}

// CreateExecutionContext creates a basic execution context with configurable defaults
func (tr *TemplateResolver) CreateExecutionContext(target, toolName, mode string) *ExecutionContext {
	timestamp := time.Now().Format("20060102_150405")
	sessionID := fmt.Sprintf("session_%s", timestamp)

	return &ExecutionContext{
		Target:     target,
		ToolName:   toolName,
		Mode:       mode,
		Timestamp:  timestamp,
		SessionID:  sessionID,
		CustomVars: make(map[string]string),
	}
}
