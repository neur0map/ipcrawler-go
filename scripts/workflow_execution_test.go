package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/neur0map/ipcrawler/internal/config"
	"github.com/neur0map/ipcrawler/internal/executor"
)

// WorkflowExecutionSimulator simulates real workflow execution with parallel processing
type WorkflowExecutionSimulator struct {
	config               *config.Config
	executionEngine      *executor.ToolExecutionEngine
	workflowExecutor     *executor.WorkflowExecutor
	workflowOrchestrator *executor.WorkflowOrchestrator
	target               string
	outputDir            string
}

// NewWorkflowExecutionSimulator creates a new simulation instance
func NewWorkflowExecutionSimulator(target string) (*WorkflowExecutionSimulator, error) {
	// Load configuration from real config files
	cfg, err := config.LoadConfig("../configs")
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %v", err)
	}

	// Create output directory for this simulation
	outputDir := fmt.Sprintf("../workspace/simulation_%s_%d", target, time.Now().Unix())
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %v", err)
	}

	// Create real execution components
	executionEngine := executor.NewToolExecutionEngine(cfg, "")
	workflowExecutor := executor.NewWorkflowExecutor(executionEngine)
	workflowOrchestrator := executor.NewWorkflowOrchestrator(workflowExecutor, cfg)

	// Set up status callback for real-time monitoring
	workflowOrchestrator.SetStatusCallback(func(workflowName, target, status, message string) {
		timestamp := time.Now().Format("15:04:05.000")
		fmt.Printf("[%s] [%s] %s -> %s: %s\n", timestamp, status, workflowName, target, message)
	})

	return &WorkflowExecutionSimulator{
		config:               cfg,
		executionEngine:      executionEngine,
		workflowExecutor:     workflowExecutor,
		workflowOrchestrator: workflowOrchestrator,
		target:               target,
		outputDir:            outputDir,
	}, nil
}

// CreatePortScanningWorkflow creates the Enhanced Reconnaissance workflow manually
func (s *WorkflowExecutionSimulator) CreatePortScanningWorkflow() *executor.Workflow {
	// Based on workflows/reconnaissance/port-scanning.yaml
	workflow := &executor.Workflow{
		Name:                   "Enhanced Reconnaissance",
		Description:            "Multi-mode parallel port discovery and comprehensive service enumeration",
		Category:               "reconnaissance",
		ParallelWorkflow:       true,
		IndependentExecution:   false,
		MaxConcurrentWorkflows: 2,
		WorkflowPriority:       "medium",
		Steps: []*executor.WorkflowStep{
			{
				Name:        "Multi-Mode Port Discovery",
				Tool:        "naabu",
				Description: "Parallel execution of multiple naabu scan modes for comprehensive coverage",
				Modes:       []string{"fast_scan", "common_ports"},
				Concurrent:  true,
				CombineResults: true,
				StepPriority: "high",
				MaxConcurrentTools: 2,
				Outputs: map[string]interface{}{
					"variables": []map[string]string{
						{"name": "combined_naabu_ports", "source": "combined_ports"},
						{"name": "combined_port_count", "source": "combined_port_count"},
						{"name": "high_coverage_ports", "source": "combined_high_coverage_ports"},
					},
				},
			},
			{
				Name:           "Multi-Mode Service Analysis",
				Tool:           "nmap",
				Description:    "Parallel service analysis with multiple scan techniques",
				Modes:          []string{"pipeline_service_scan"},
				Concurrent:     false,
				CombineResults: true,
				DependsOn:      "Multi-Mode Port Discovery",
				StepPriority:   "medium",
				MaxConcurrentTools: 1,
				Inputs: map[string]interface{}{
					"variables": []map[string]string{
						{"name": "combined_naabu_ports", "target_variable": "ports"},
					},
				},
			},
		},
	}
	
	fmt.Printf("✅ Created Port Scanning workflow: %s\n", workflow.Description)
	return workflow
}

// CreateDNSDiscoveryWorkflow creates the DNS Discovery workflow manually
func (s *WorkflowExecutionSimulator) CreateDNSDiscoveryWorkflow() *executor.Workflow {
	// Based on workflows/reconnaissance/dns-enumeration.yaml
	workflow := &executor.Workflow{
		Name:                   "DNS Discovery",
		Description:            "Comprehensive DNS information gathering and reconnaissance",
		Category:               "reconnaissance",
		ParallelWorkflow:       true,
		IndependentExecution:   true,
		MaxConcurrentWorkflows: 3,
		WorkflowPriority:       "medium",
		Steps: []*executor.WorkflowStep{
			{
				Name:               "DNS Information Gathering",
				Tool:               "nslookup",
				Description:        "Basic DNS information lookup and record enumeration",
				Modes:              []string{"default_lookup"},
				Concurrent:         false,
				CombineResults:     false,
				StepPriority:       "medium",
				MaxConcurrentTools: 1,
			},
		},
	}
	
	fmt.Printf("✅ Created DNS Discovery workflow: %s\n", workflow.Description)
	return workflow
}

// SimulateToolExecution creates realistic tool execution simulation
func (s *WorkflowExecutionSimulator) SimulateToolExecution(toolName, mode, target string) (*executor.ExecutionResult, error) {
	startTime := time.Now()
	
	// Create realistic output based on tool type
	var output string
	var exitCode int
	
	// Simulate execution time based on tool complexity
	var executionTime time.Duration
	
	switch toolName {
	case "naabu":
		executionTime = time.Millisecond * time.Duration(500 + (len(mode) * 100)) // 500-1500ms
		switch mode {
		case "fast_scan":
			output = fmt.Sprintf(`{"host":"%s","ports":[{"port":22,"protocol":"tcp","service":"ssh"},{"port":80,"protocol":"tcp","service":"http"},{"port":443,"protocol":"tcp","service":"https"}],"timestamp":"%s"}`, 
				target, time.Now().Format(time.RFC3339))
		case "common_ports":
			output = fmt.Sprintf(`{"host":"%s","ports":[{"port":22,"protocol":"tcp","service":"ssh"},{"port":80,"protocol":"tcp","service":"http"},{"port":443,"protocol":"tcp","service":"https"},{"port":8080,"protocol":"tcp","service":"http-proxy"}],"timestamp":"%s"}`, 
				target, time.Now().Format(time.RFC3339))
		}
		exitCode = 0
		
	case "nmap":
		executionTime = time.Millisecond * time.Duration(1000 + (len(target) * 50)) // 1-3s
		output = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<nmaprun scanner="nmap" version="7.80">
<host>
<address addr="%s" addrtype="ipv4"/>
<ports>
<port protocol="tcp" portid="22"><state state="open"/><service name="ssh"/></port>
<port protocol="tcp" portid="80"><state state="open"/><service name="http"/></port>
<port protocol="tcp" portid="443"><state state="open"/><service name="https"/></port>
</ports>
</host>
</nmaprun>`, target)
		exitCode = 0
		
	case "nslookup":
		executionTime = time.Millisecond * time.Duration(200 + (len(target) * 20)) // 200-800ms
		output = fmt.Sprintf(`Server:		8.8.8.8
Address:	8.8.8.8#53

Non-authoritative answer:
Name:	%s
Address: 93.184.216.34
Name:	%s
Address: 2606:2800:220:1:248:1893:25c8:1946`, target, target)
		exitCode = 0
	}
	
	// Simulate realistic execution delay
	time.Sleep(executionTime)
	
	endTime := time.Now()
	duration := endTime.Sub(startTime)
	
	fmt.Printf("⚡ %s[%s] completed in %v (simulated)\n", toolName, mode, duration)
	
	// Create execution result
	result := &executor.ExecutionResult{
		Tool:         toolName,
		Mode:         mode,
		Target:       target,
		Output:       output,
		StartTime:    startTime,
		EndTime:      endTime,
		Duration:     duration,
		ExitCode:     exitCode,
		Success:      exitCode == 0,
	}
	
	// Write output to real files (simulated)
	outputFile := fmt.Sprintf("%s/%s_%s_%d", s.outputDir, toolName, mode, time.Now().Unix())
	switch toolName {
	case "naabu":
		outputFile += ".json"
	case "nmap":
		outputFile += ".xml"
	case "nslookup":
		outputFile += ".txt"
	}
	
	if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
		fmt.Printf("⚠️  Warning: Failed to write output file: %v\n", err)
	} else {
		fmt.Printf("📄 Output written to: %s\n", outputFile)
	}
	
	return result, nil
}

// ExecuteWorkflowsInParallel executes multiple workflows concurrently
func (s *WorkflowExecutionSimulator) ExecuteWorkflowsInParallel() error {
	fmt.Printf("\n🚀 Starting parallel execution of 2 workflows...\n")
	fmt.Printf("🎯 Target: %s\n", s.target)
	fmt.Printf("📊 Config limits: Max concurrent workflows: %d, Max tools per step: %d\n", 
		s.config.Tools.WorkflowOrchestration.MaxConcurrentWorkflows,
		s.config.Tools.WorkflowOrchestration.MaxConcurrentToolsPerStep)
	
	var wg sync.WaitGroup
	ctx := context.Background()
	
	// Create workflows manually based on YAML configurations
	portScanWorkflow := s.CreatePortScanningWorkflow()
	dnsWorkflow := s.CreateDNSDiscoveryWorkflow()
	
	workflows := []*executor.Workflow{portScanWorkflow, dnsWorkflow}
	
	// Queue workflows for execution
	for _, workflow := range workflows {
		if err := s.workflowOrchestrator.QueueWorkflow(workflow, s.target); err != nil {
			fmt.Printf("❌ Failed to queue workflow %s: %v\n", workflow.Name, err)
			continue
		}
		
		fmt.Printf("📋 Queued workflow: %s\n", workflow.Name)
	}
	
	// Monitor execution status
	go s.monitorExecution(ctx)
	
	// Execute all queued workflows
	fmt.Printf("\n▶️  Executing queued workflows...\n")
	if err := s.workflowOrchestrator.ExecuteQueuedWorkflows(ctx); err != nil {
		return fmt.Errorf("failed to execute workflows: %v", err)
	}
	
	// Wait for completion (simulate real execution monitoring)
	fmt.Printf("⏳ Waiting for workflow completion...\n")
	for i := 0; i < 30; i++ { // Wait up to 30 seconds
		queuedCount, activeCount, queuedNames, activeNames := s.workflowOrchestrator.GetExecutionStatus()
		
		if queuedCount == 0 && activeCount == 0 {
			fmt.Printf("✅ All workflows completed!\n")
			break
		}
		
		if activeCount > 0 {
			fmt.Printf("🔄 Active workflows (%d): %v\n", activeCount, activeNames)
		}
		if queuedCount > 0 {
			fmt.Printf("📋 Queued workflows (%d): %v\n", queuedCount, queuedNames)
		}
		
		time.Sleep(1 * time.Second)
	}
	
	wg.Wait()
	return nil
}

// monitorExecution provides real-time monitoring of workflow execution
func (s *WorkflowExecutionSimulator) monitorExecution(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Update resource monitor
			if err := s.workflowOrchestrator.ResourceMonitor.UpdateResourceUsageFromSystem(); err == nil {
				// Only log if there's activity
				queuedCount, activeCount, _, _ := s.workflowOrchestrator.GetExecutionStatus()
				if queuedCount > 0 || activeCount > 0 {
					fmt.Printf("📊 System resources updated | Queue: %d | Active: %d\n", queuedCount, activeCount)
				}
			}
		}
	}
}

func main() {
	fmt.Println("🎯 IPCrawler Workflow Execution Simulation")
	fmt.Println("==========================================")
	
	// Define test target
	target := "example.com"
	
	// Create simulator
	simulator, err := NewWorkflowExecutionSimulator(target)
	if err != nil {
		log.Fatal("Failed to create simulator:", err)
	}
	
	fmt.Printf("✅ Simulation initialized\n")
	fmt.Printf("📁 Output directory: %s\n", simulator.outputDir)
	
	// Execute workflows in parallel
	startTime := time.Now()
	if err := simulator.ExecuteWorkflowsInParallel(); err != nil {
		log.Fatal("Execution failed:", err)
	}
	
	totalDuration := time.Since(startTime)
	fmt.Printf("\n🎉 Simulation completed in %v\n", totalDuration)
	fmt.Printf("📂 Results saved to: %s\n", simulator.outputDir)
	
	// Show execution summary
	fmt.Println("\n📈 Execution Summary:")
	fmt.Println("====================")
	fmt.Printf("• Target: %s\n", target)
	fmt.Printf("• Workflows executed: %d\n", 2)
	fmt.Printf("• Total execution time: %v\n", totalDuration)
	fmt.Printf("• Parallelism: Both workflows ran concurrently\n")
	fmt.Printf("• Tool parallelism: Tools within workflows executed per config settings\n")
	fmt.Println("\nWorkflow details:")
	fmt.Println("• Port Scanning: naabu (fast_scan + common_ports in parallel) → nmap (pipeline_service_scan)")
	fmt.Println("• DNS Discovery: nslookup (default_lookup)")
	fmt.Println("\n🔍 Check the output directory for generated scan results!")
}