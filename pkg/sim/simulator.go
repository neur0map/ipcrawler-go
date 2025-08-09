package sim

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/carlosm/ipcrawler/internal/ui/model"
)

// WorkflowSimulator simulates realistic workflow execution for testing
type WorkflowSimulator struct {
	ctx        context.Context
	cancel     context.CancelFunc
	program    *tea.Program
	target     string
	workflows  []SimulatedWorkflow
	tools      []SimulatedTool
	running    bool
	mu         sync.RWMutex
	eventChan  chan tea.Msg
}

// SimulatedWorkflow represents a workflow for simulation
type SimulatedWorkflow struct {
	ID          string
	Name        string
	Description string
	Tools       []SimulatedTool
	Duration    time.Duration
	FailureRate float64 // 0.0 to 1.0
}

// SimulatedTool represents a tool execution for simulation
type SimulatedTool struct {
	Name        string
	MinDuration time.Duration
	MaxDuration time.Duration
	FailureRate float64
	OutputLines []string
}

// NewWorkflowSimulator creates a new workflow simulator
func NewWorkflowSimulator(target string) *WorkflowSimulator {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &WorkflowSimulator{
		ctx:       ctx,
		cancel:    cancel,
		target:    target,
		workflows: createDefaultWorkflows(),
		tools:     createDefaultTools(),
		eventChan: make(chan tea.Msg, 100),
	}
}

// SetProgram sets the Bubble Tea program for sending messages
func (s *WorkflowSimulator) SetProgram(program *tea.Program) {
	s.program = program
}

// Start begins the workflow simulation
func (s *WorkflowSimulator) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("simulator already running")
	}
	s.running = true
	s.mu.Unlock()
	
	// Send initial log message
	s.sendMessage(model.LogMsg{
		Level:     "info",
		Message:   fmt.Sprintf("Starting workflow simulation for target: %s", s.target),
		Timestamp: time.Now(),
		Category:  "simulator",
	})
	
	// Start simulation goroutine
	go s.runSimulation()
	
	return nil
}

// Stop halts the workflow simulation
func (s *WorkflowSimulator) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.running {
		s.cancel()
		s.running = false
		
		s.sendMessage(model.LogMsg{
			Level:     "info",
			Message:   "Workflow simulation stopped",
			Timestamp: time.Now(),
			Category:  "simulator",
		})
	}
}

// IsRunning returns true if the simulator is currently running
func (s *WorkflowSimulator) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// runSimulation executes the simulation loop
func (s *WorkflowSimulator) runSimulation() {
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()
	
	// Simulate discovery phase
	s.simulateDiscovery()
	
	// Wait a bit before starting workflows
	select {
	case <-s.ctx.Done():
		return
	case <-time.After(2 * time.Second):
	}
	
	// Run workflows in parallel
	var wg sync.WaitGroup
	for _, workflow := range s.workflows {
		wg.Add(1)
		go func(wf SimulatedWorkflow) {
			defer wg.Done()
			s.simulateWorkflow(wf)
		}(workflow)
		
		// Stagger workflow starts
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(time.Duration(rand.Intn(3)) * time.Second):
		}
	}
	
	wg.Wait()
	
	// Send completion message
	s.sendMessage(model.LogMsg{
		Level:     "info",
		Message:   "All workflows completed",
		Timestamp: time.Now(),
		Category:  "simulator",
	})
}

// simulateDiscovery simulates the discovery phase
func (s *WorkflowSimulator) simulateDiscovery() {
	discoverySteps := []string{
		"Loading configuration...",
		"Validating target...",
		"Discovering workflows...",
		"Loading tools...",
		"Preparing execution environment...",
	}
	
	for _, step := range discoverySteps {
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		
		s.sendMessage(model.LogMsg{
			Level:     "info",
			Message:   step,
			Timestamp: time.Now(),
			Category:  "system",
		})
		
		// Simulate processing time
		time.Sleep(time.Duration(200+rand.Intn(800)) * time.Millisecond)
	}
}

// simulateWorkflow simulates a complete workflow execution
func (s *WorkflowSimulator) simulateWorkflow(workflow SimulatedWorkflow) {
	// Start workflow
	s.sendMessage(model.WorkflowUpdateMsg{
		WorkflowID:  workflow.ID,
		Status:      "running",
		Progress:    0.0,
		Description: workflow.Description,
		Duration:    0,
		StartTime:   time.Now(),
	})
	
	s.sendMessage(model.LogMsg{
		Level:     "info",
		Message:   fmt.Sprintf("Starting workflow: %s", workflow.Name),
		Timestamp: time.Now(),
		Category:  "workflow",
	})
	
	startTime := time.Now()
	totalTools := len(workflow.Tools)
	
	// Execute each tool in the workflow
	for i, tool := range workflow.Tools {
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		
		// Update workflow progress
		progress := float64(i) / float64(totalTools)
		s.sendMessage(model.WorkflowUpdateMsg{
			WorkflowID:  workflow.ID,
			Status:      "running",
			Progress:    progress,
			Description: workflow.Description,
			Duration:    time.Since(startTime),
			StartTime:   startTime,
		})
		
		// Simulate tool execution
		success := s.simulateTool(tool, workflow.ID)
		
		if !success {
			// Tool failed - decide if workflow should fail
			if rand.Float64() < workflow.FailureRate {
				s.sendMessage(model.WorkflowUpdateMsg{
					WorkflowID:  workflow.ID,
					Status:      "failed",
					Progress:    progress,
					Description: workflow.Description,
					Duration:    time.Since(startTime),
					Error:       fmt.Errorf("workflow failed at tool: %s", tool.Name),
					StartTime:   startTime,
				})
				
				s.sendMessage(model.LogMsg{
					Level:     "error",
					Message:   fmt.Sprintf("Workflow %s failed: tool %s execution failed", workflow.Name, tool.Name),
					Timestamp: time.Now(),
					Category:  "workflow",
				})
				return
			}
		}
	}
	
	// Complete workflow successfully
	s.sendMessage(model.WorkflowUpdateMsg{
		WorkflowID:  workflow.ID,
		Status:      "completed",
		Progress:    1.0,
		Description: workflow.Description,
		Duration:    time.Since(startTime),
		StartTime:   startTime,
	})
	
	s.sendMessage(model.LogMsg{
		Level:     "info",
		Message:   fmt.Sprintf("Workflow %s completed successfully in %v", workflow.Name, time.Since(startTime)),
		Timestamp: time.Now(),
		Category:  "workflow",
	})
}

// simulateTool simulates a single tool execution
func (s *WorkflowSimulator) simulateTool(tool SimulatedTool, workflowID string) bool {
	// Calculate random execution duration
	minMs := tool.MinDuration.Milliseconds()
	maxMs := tool.MaxDuration.Milliseconds()
	durationMs := minMs + rand.Int63n(maxMs-minMs+1)
	duration := time.Duration(durationMs) * time.Millisecond
	
	// Start tool execution
	s.sendMessage(model.ToolExecutionMsg{
		ToolName:   tool.Name,
		WorkflowID: workflowID,
		Status:     "running",
		Progress:   0.0,
		Duration:   0,
		Args:       []string{s.target},
	})
	
	s.sendMessage(model.LogMsg{
		Level:     "debug",
		Message:   fmt.Sprintf("Starting tool: %s", tool.Name),
		Timestamp: time.Now(),
		Category:  "tool",
	})
	
	startTime := time.Now()
	
	// Simulate progress updates during execution
	progressTicker := time.NewTicker(duration / 10)
	defer progressTicker.Stop()
	
	progress := 0.0
	outputIndex := 0
	
	for progress < 1.0 {
		select {
		case <-s.ctx.Done():
			return false
		case <-progressTicker.C:
			progress += 0.1
			if progress > 1.0 {
				progress = 1.0
			}
			
			// Send progress update
			s.sendMessage(model.ToolExecutionMsg{
				ToolName:   tool.Name,
				WorkflowID: workflowID,
				Status:     "running",
				Progress:   progress,
				Duration:   time.Since(startTime),
				Args:       []string{s.target},
			})
			
			// Simulate output generation
			if outputIndex < len(tool.OutputLines) && rand.Float64() < 0.7 {
				s.sendMessage(model.LogMsg{
					Level:     "debug",
					Message:   fmt.Sprintf("%s: %s", tool.Name, tool.OutputLines[outputIndex]),
					Timestamp: time.Now(),
					Category:  "tool",
				})
				outputIndex++
			}
		}
	}
	
	// Determine if tool execution succeeds or fails
	success := rand.Float64() >= tool.FailureRate
	
	var status string
	var err error
	var output string
	
	if success {
		status = "completed"
		output = fmt.Sprintf("Tool %s completed successfully", tool.Name)
	} else {
		status = "failed"
		err = fmt.Errorf("tool execution failed")
		output = fmt.Sprintf("Tool %s failed with error", tool.Name)
	}
	
	// Send final tool execution message
	s.sendMessage(model.ToolExecutionMsg{
		ToolName:   tool.Name,
		WorkflowID: workflowID,
		Status:     status,
		Progress:   1.0,
		Duration:   time.Since(startTime),
		Output:     output,
		Error:      err,
		Args:       []string{s.target},
	})
	
	logLevel := "info"
	if !success {
		logLevel = "error"
	}
	
	s.sendMessage(model.LogMsg{
		Level:     logLevel,
		Message:   fmt.Sprintf("Tool %s %s in %v", tool.Name, status, time.Since(startTime)),
		Timestamp: time.Now(),
		Category:  "tool",
	})
	
	return success
}

// sendMessage sends a message to the Bubble Tea program
func (s *WorkflowSimulator) sendMessage(msg tea.Msg) {
	if s.program != nil {
		s.program.Send(msg)
	}
}

// createDefaultWorkflows creates sample workflows for simulation
func createDefaultWorkflows() []SimulatedWorkflow {
	return []SimulatedWorkflow{
		{
			ID:          "dns_discovery",
			Name:        "DNS Discovery",
			Description: "DNS enumeration and subdomain discovery",
			Tools:       []SimulatedTool{createDNSTool(), createSubfinderTool()},
			Duration:    15 * time.Second,
			FailureRate: 0.1,
		},
		{
			ID:          "port_scan",
			Name:        "Port Scanning",
			Description: "Network port scanning and service detection",
			Tools:       []SimulatedTool{createNaabuTool(), createNmapTool()},
			Duration:    30 * time.Second,
			FailureRate: 0.05,
		},
		{
			ID:          "vhost_discovery",
			Name:        "Virtual Host Discovery",
			Description: "Virtual host enumeration and analysis",
			Tools:       []SimulatedTool{createVHostTool()},
			Duration:    20 * time.Second,
			FailureRate: 0.15,
		},
	}
}

// createDefaultTools creates sample tools for simulation
func createDefaultTools() []SimulatedTool {
	return []SimulatedTool{
		createDNSTool(),
		createSubfinderTool(),
		createNaabuTool(),
		createNmapTool(),
		createVHostTool(),
	}
}

func createDNSTool() SimulatedTool {
	return SimulatedTool{
		Name:        "dig",
		MinDuration: 1 * time.Second,
		MaxDuration: 5 * time.Second,
		FailureRate: 0.05,
		OutputLines: []string{
			"Querying DNS servers...",
			"Found A record: 1.2.3.4",
			"Found MX record: mail.example.com",
			"Found NS record: ns1.example.com",
			"DNS lookup completed",
		},
	}
}

func createSubfinderTool() SimulatedTool {
	return SimulatedTool{
		Name:        "subfinder",
		MinDuration: 5 * time.Second,
		MaxDuration: 15 * time.Second,
		FailureRate: 0.1,
		OutputLines: []string{
			"Starting subdomain discovery...",
			"Found subdomain: www.example.com",
			"Found subdomain: api.example.com",
			"Found subdomain: mail.example.com",
			"Found subdomain: cdn.example.com",
			"Subdomain discovery completed",
		},
	}
}

func createNaabuTool() SimulatedTool {
	return SimulatedTool{
		Name:        "naabu",
		MinDuration: 10 * time.Second,
		MaxDuration: 30 * time.Second,
		FailureRate: 0.08,
		OutputLines: []string{
			"Starting port scan...",
			"Open port found: 80/tcp",
			"Open port found: 443/tcp",
			"Open port found: 22/tcp",
			"Port scan completed",
		},
	}
}

func createNmapTool() SimulatedTool {
	return SimulatedTool{
		Name:        "nmap",
		MinDuration: 15 * time.Second,
		MaxDuration: 45 * time.Second,
		FailureRate: 0.12,
		OutputLines: []string{
			"Starting service detection...",
			"Service on 80/tcp: HTTP",
			"Service on 443/tcp: HTTPS",
			"Service on 22/tcp: SSH",
			"Service detection completed",
		},
	}
}

func createVHostTool() SimulatedTool {
	return SimulatedTool{
		Name:        "vhost-discovery",
		MinDuration: 8 * time.Second,
		MaxDuration: 25 * time.Second,
		FailureRate: 0.15,
		OutputLines: []string{
			"Starting virtual host discovery...",
			"Testing virtual hosts...",
			"Found vhost: admin.example.com",
			"Found vhost: staging.example.com",
			"Virtual host discovery completed",
		},
	}
}

// CreateQuickDemo creates a quick demonstration with fast timings
func CreateQuickDemo(target string) *WorkflowSimulator {
	sim := NewWorkflowSimulator(target)
	
	// Override with faster workflows for demo
	sim.workflows = []SimulatedWorkflow{
		{
			ID:          "quick_scan",
			Name:        "Quick Scan",
			Description: "Fast demonstration scan",
			Tools: []SimulatedTool{
				{
					Name:        "ping",
					MinDuration: 500 * time.Millisecond,
					MaxDuration: 1 * time.Second,
					FailureRate: 0.0,
					OutputLines: []string{"Pinging target...", "Target is reachable"},
				},
				{
					Name:        "dns-check",
					MinDuration: 1 * time.Second,
					MaxDuration: 2 * time.Second,
					FailureRate: 0.05,
					OutputLines: []string{"Checking DNS...", "DNS resolved successfully"},
				},
			},
			Duration:    5 * time.Second,
			FailureRate: 0.0,
		},
		{
			ID:          "demo_scan",
			Name:        "Demo Scan",
			Description: "Demonstration workflow with simulated failures",
			Tools: []SimulatedTool{
				{
					Name:        "port-scan",
					MinDuration: 2 * time.Second,
					MaxDuration: 4 * time.Second,
					FailureRate: 0.3, // Higher failure rate for demo
					OutputLines: []string{"Scanning ports...", "Found open ports"},
				},
			},
			Duration:    8 * time.Second,
			FailureRate: 0.2,
		},
	}
	
	return sim
}