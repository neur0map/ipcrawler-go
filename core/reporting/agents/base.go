package agents

import (
	"fmt"
	"log"
	"time"
)

// Agent defines the interface that all processing agents must implement
type Agent interface {
	Name() string
	Process(input *AgentInput) (*AgentOutput, error)
	Validate() error
}

// AgentInput represents data passed between agents in the pipeline
type AgentInput struct {
	Data         interface{}            `json:"data"`
	Metadata     map[string]string      `json:"metadata"`
	PreviousAgent string                `json:"previous_agent"`
	Timestamp    time.Time              `json:"timestamp"`
	Target       string                 `json:"target"`
	ReportDir    string                 `json:"report_dir"`
}

// AgentOutput represents the result of agent processing
type AgentOutput struct {
	Data         interface{}            `json:"data"`
	Metadata     map[string]string      `json:"metadata"`
	NextAgent    string                 `json:"next_agent"`
	Errors       []error                `json:"errors,omitempty"`
	Warnings     []string               `json:"warnings,omitempty"`
	Success      bool                   `json:"success"`
	ProcessingTime time.Duration        `json:"processing_time"`
}

// BaseAgent provides common functionality for all agents
type BaseAgent struct {
	name      string
	logger    *log.Logger
	debugMode bool
}

// NewBaseAgent creates a new base agent with logging
func NewBaseAgent(name string, logger *log.Logger) *BaseAgent {
	if logger == nil {
		logger = log.Default()
	}
	return &BaseAgent{
		name:      name,
		logger:    logger,
		debugMode: false, // Default to false, will be set by pipeline
	}
}

// SetDebugMode sets debug mode for the agent
func (b *BaseAgent) SetDebugMode(debug bool) {
	b.debugMode = debug
}

// Name returns the agent name
func (b *BaseAgent) Name() string {
	return b.name
}

// LogInfo logs an info message (only in debug mode)
func (b *BaseAgent) LogInfo(format string, args ...interface{}) {
	if b.debugMode {
		b.logger.Printf("[%s] INFO: "+format, append([]interface{}{b.name}, args...)...)
	}
}

// LogWarning logs a warning message (only in debug mode)
func (b *BaseAgent) LogWarning(format string, args ...interface{}) {
	if b.debugMode {
		b.logger.Printf("[%s] WARNING: "+format, append([]interface{}{b.name}, args...)...)
	}
}

// LogError logs an error message (only in debug mode)
func (b *BaseAgent) LogError(format string, args ...interface{}) {
	if b.debugMode {
		b.logger.Printf("[%s] ERROR: "+format, append([]interface{}{b.name}, args...)...)
	}
}

// ValidateInput performs basic validation on agent input
func (b *BaseAgent) ValidateInput(input *AgentInput) error {
	if input == nil {
		return fmt.Errorf("agent input cannot be nil")
	}
	if input.Target == "" {
		return fmt.Errorf("target is required")
	}
	if input.ReportDir == "" {
		return fmt.Errorf("report directory is required")
	}
	return nil
}

// CreateOutput creates a standardized agent output
func (b *BaseAgent) CreateOutput(data interface{}, metadata map[string]string, success bool) *AgentOutput {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	
	metadata["processed_by"] = b.name
	metadata["processed_at"] = time.Now().Format(time.RFC3339)
	
	return &AgentOutput{
		Data:     data,
		Metadata: metadata,
		Success:  success,
		Errors:   make([]error, 0),
		Warnings: make([]string, 0),
	}
}

// AddError adds an error to the output
func (output *AgentOutput) AddError(err error) {
	if output.Errors == nil {
		output.Errors = make([]error, 0)
	}
	output.Errors = append(output.Errors, err)
	output.Success = false
}

// AddWarning adds a warning to the output
func (output *AgentOutput) AddWarning(warning string) {
	if output.Warnings == nil {
		output.Warnings = make([]string, 0)
	}
	output.Warnings = append(output.Warnings, warning)
}

// HasErrors returns true if the output contains errors
func (output *AgentOutput) HasErrors() bool {
	return len(output.Errors) > 0
}

// HasWarnings returns true if the output contains warnings
func (output *AgentOutput) HasWarnings() bool {
	return len(output.Warnings) > 0
}