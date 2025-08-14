package executor

import (
	"fmt"
	"strings"
	"time"

	"github.com/neur0map/ipcrawler/internal/registry"
)

// ToolOutputParser defines the interface for tool-specific output parsing
// Each tool implements this interface in its own isolated package
type ToolOutputParser interface {
	ParseOutput(outputPath string) map[string]string
	GetToolName() string
}

// MagicVariableManager handles automatic creation of magic variables
// from tool outputs. This is generic code with NO tool-specific logic.
type MagicVariableManager struct {
	parsers         map[string]ToolOutputParser
	registryManager registry.RegistryManager // Optional registry for auto-detection
}

// NewMagicVariableManager creates a new magic variable manager
func NewMagicVariableManager() *MagicVariableManager {
	return &MagicVariableManager{
		parsers: make(map[string]ToolOutputParser),
	}
}

// SetRegistryManager sets the registry manager for auto-detection
func (mvm *MagicVariableManager) SetRegistryManager(manager registry.RegistryManager) {
	mvm.registryManager = manager
}

// RegisterParser registers a tool-specific output parser
func (mvm *MagicVariableManager) RegisterParser(parser ToolOutputParser) {
	toolName := strings.ToLower(parser.GetToolName())
	mvm.parsers[toolName] = parser
}

// ProcessToolOutput processes the output files from a completed tool
// and creates magic variables. This is completely generic.
func (mvm *MagicVariableManager) ProcessToolOutput(toolName string, outputFiles []string) map[string]string {
	toolName = strings.ToLower(toolName)
	
	parser, exists := mvm.parsers[toolName]
	if !exists {
		// No parser registered = no magic variables created
		return make(map[string]string)
	}

	magicVariables := make(map[string]string)

	// Process each output file
	for _, outputFile := range outputFiles {
		if outputFile == "" {
			continue
		}

		// Let the tool-specific parser extract data
		toolVars := parser.ParseOutput(outputFile)

		// Create magic variables with tool prefix
		for key, value := range toolVars {
			magicVarName := fmt.Sprintf("%s_%s", toolName, key)
			magicVariables[magicVarName] = value
			
			// Auto-register with registry if available
			if mvm.registryManager != nil {
				context := registry.DetectionContext{
					FilePath:   outputFile,
					LineNumber: 0,
					Context:    fmt.Sprintf("Magic variable from %s parser: %s", toolName, key),
					Source:     registry.ToolParserSource,
					Tool:       toolName,
					Timestamp:  time.Now(),
				}
				
				// Register the variable (ignore errors to avoid disrupting execution)
				mvm.registryManager.AutoRegisterVariable(fmt.Sprintf("{{%s}}", magicVarName), context)
			}
		}
	}

	return magicVariables
}

// GetAvailableParsers returns a list of tools that have output parsers registered
func (mvm *MagicVariableManager) GetAvailableParsers() []string {
	var tools []string
	for toolName := range mvm.parsers {
		tools = append(tools, toolName)
	}
	return tools
}

// HasParser checks if a parser is registered for the given tool
func (mvm *MagicVariableManager) HasParser(toolName string) bool {
	_, exists := mvm.parsers[strings.ToLower(toolName)]
	return exists
}