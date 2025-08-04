package agents

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PipelineConfig holds configuration for the processing pipeline
type PipelineConfig struct {
	MaxRetries      int           `yaml:"max_retries"`
	RetryDelay      time.Duration `yaml:"retry_delay"`
	Timeout         time.Duration `yaml:"timeout"`
	ParallelMode    bool          `yaml:"parallel_mode"`
	FailFast        bool          `yaml:"fail_fast"`
	LogLevel        string        `yaml:"log_level"`
	ReportDir       string        `yaml:"report_dir"`
}

// DefaultPipelineConfig returns a default pipeline configuration
func DefaultPipelineConfig() *PipelineConfig {
	return &PipelineConfig{
		MaxRetries:   3,
		RetryDelay:   time.Second * 2,
		Timeout:      time.Minute * 10,
		ParallelMode: false,
		FailFast:     false,
		LogLevel:     "info",
		ReportDir:    "reports",
	}
}

// Pipeline manages the execution of agents in sequence
type Pipeline struct {
	agents    []Agent
	config    *PipelineConfig
	logger    *log.Logger
	reportDir string
	target    string
	startTime time.Time
	mutex     sync.RWMutex
}

// NewPipeline creates a new processing pipeline
func NewPipeline(config *PipelineConfig, logger *log.Logger) *Pipeline {
	if config == nil {
		config = DefaultPipelineConfig()
	}
	if logger == nil {
		logger = log.Default()
	}

	return &Pipeline{
		agents: make([]Agent, 0),
		config: config,
		logger: logger,
	}
}

// AddAgent adds an agent to the pipeline
func (p *Pipeline) AddAgent(agent Agent) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.agents = append(p.agents, agent)
}

// SetTarget sets the target for the pipeline
func (p *Pipeline) SetTarget(target string) {
	p.target = target
}

// SetReportDir sets the report directory for the pipeline
func (p *Pipeline) SetReportDir(reportDir string) {
	p.reportDir = reportDir
}

// SetDebugMode sets debug mode for all agents in the pipeline
func (p *Pipeline) SetDebugMode(debugMode bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	for _, agent := range p.agents {
		// Try to cast to BaseAgent or any agent that supports SetDebugMode
		if debuggableAgent, ok := agent.(interface{ SetDebugMode(bool) }); ok {
			debuggableAgent.SetDebugMode(debugMode)
		}
	}
}

// Execute runs the pipeline with the given initial data
func (p *Pipeline) Execute(target string, initialData interface{}) (*PipelineResult, error) {
	p.startTime = time.Now()
	p.target = target
	
	// Use existing report directory if set, otherwise create new one
	reportDir := p.reportDir
	if reportDir == "" {
		var err error
		reportDir, err = p.CreateReportDirectory(target)
		if err != nil {
			return nil, fmt.Errorf("failed to create report directory: %w", err)
		}
	}
	
	// Create log file for this execution
	logFile, err := p.createLogFile(reportDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()
	
	p.logger.Printf("Starting pipeline execution for target: %s", target)
	
	result := &PipelineResult{
		Target:        target,
		ReportDir:     reportDir,
		StartTime:     p.startTime,
		AgentResults:  make(map[string]*AgentOutput),
		Success:       true,
	}
	
	// Prepare initial input
	input := &AgentInput{
		Data:         initialData,
		Metadata:     make(map[string]string),
		Target:       target,
		ReportDir:    reportDir,
		Timestamp:    p.startTime,
	}
	
	// Execute agents in sequence
	currentInput := input
	for i, agent := range p.agents {
		p.logger.Printf("Executing agent %d/%d: %s", i+1, len(p.agents), agent.Name())
		
		// Validate agent before execution
		if err := agent.Validate(); err != nil {
			p.logger.Printf("Agent validation failed for %s: %v", agent.Name(), err)
			if p.config.FailFast {
				result.Success = false
				result.Error = fmt.Errorf("agent validation failed: %w", err)
				return result, result.Error
			}
			continue
		}
		
		// Execute agent with retry logic
		output, err := p.executeAgentWithRetry(agent, currentInput)
		if err != nil {
			p.logger.Printf("Agent execution failed for %s: %v", agent.Name(), err)
			result.Success = false
			result.Error = err
			
			if p.config.FailFast {
				return result, err
			}
			continue
		}
		
		// Store agent result
		result.AgentResults[agent.Name()] = output
		
		// Check for errors in output
		if output.HasErrors() && p.config.FailFast {
			result.Success = false
			result.Error = fmt.Errorf("agent %s produced errors", agent.Name())
			return result, result.Error
		}
		
		// Prepare input for next agent
		if i < len(p.agents)-1 {
			currentInput = &AgentInput{
				Data:          output.Data,
				Metadata:      output.Metadata,
				PreviousAgent: agent.Name(),
				Target:        target,
				ReportDir:     reportDir,
				Timestamp:     time.Now(),
			}
		}
	}
	
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	
	p.logger.Printf("Pipeline execution completed in %v", result.Duration)
	return result, nil
}

// CreateReportDirectory creates the report directory structure
func (p *Pipeline) CreateReportDirectory(target string) (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	reportPath := filepath.Join(p.config.ReportDir, target, timestamp)
	
	// Create directory structure
	dirs := []string{
		reportPath,
		filepath.Join(reportPath, "raw"),
		filepath.Join(reportPath, "processed"),
		filepath.Join(reportPath, "summary"),
		filepath.Join(reportPath, "logs"),
	}
	
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	
	p.reportDir = reportPath
	p.logger.Printf("Created report directory: %s", reportPath)
	return reportPath, nil
}

// executeAgentWithRetry executes an agent with retry logic
func (p *Pipeline) executeAgentWithRetry(agent Agent, input *AgentInput) (*AgentOutput, error) {
	var lastErr error
	
	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		if attempt > 0 {
			p.logger.Printf("Retrying agent %s (attempt %d/%d)", agent.Name(), attempt+1, p.config.MaxRetries+1)
			time.Sleep(p.config.RetryDelay)
		}
		
		// Create timeout context if needed
		startTime := time.Now()
		output, err := agent.Process(input)
		processingTime := time.Since(startTime)
		
		if err == nil {
			if output != nil {
				output.ProcessingTime = processingTime
			}
			return output, nil
		}
		
		lastErr = err
		p.logger.Printf("Agent %s failed on attempt %d: %v", agent.Name(), attempt+1, err)
	}
	
	return nil, fmt.Errorf("agent %s failed after %d attempts: %w", agent.Name(), p.config.MaxRetries+1, lastErr)
}

// createLogFile creates a log file for the pipeline execution
func (p *Pipeline) createLogFile(reportDir string) (*os.File, error) {
	logPath := filepath.Join(reportDir, "logs", "pipeline.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}
	
	// Create a multi-writer to log to both file and existing logger
	p.logger = log.New(logFile, "", log.LstdFlags)
	return logFile, nil
}

// GetAgents returns the list of agents in the pipeline
func (p *Pipeline) GetAgents() []Agent {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	agents := make([]Agent, len(p.agents))
	copy(agents, p.agents)
	return agents
}

// PipelineResult holds the results of pipeline execution
type PipelineResult struct {
	Target       string                    `json:"target"`
	ReportDir    string                    `json:"report_dir"`
	StartTime    time.Time                 `json:"start_time"`
	EndTime      time.Time                 `json:"end_time"`
	Duration     time.Duration             `json:"duration"`
	Success      bool                      `json:"success"`
	Error        error                     `json:"error,omitempty"`
	AgentResults map[string]*AgentOutput   `json:"agent_results"`
}