package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/carlosm/ipcrawler/internal/logger"
	"github.com/carlosm/ipcrawler/internal/registry"
	"github.com/carlosm/ipcrawler/internal/services"
	"github.com/carlosm/ipcrawler/internal/template"
)

type Executor struct {
	maxConcurrent int
	semaphore     chan struct{}
	mu            sync.RWMutex // Change to RWMutex for RLock/RUnlock
	running       int
	completed     int
	errors        []error
	db            *services.Database
	monitor       Monitor
	target        string // Add target to executor
}

// Monitor interface for monitoring workflow execution
type Monitor interface {
	RecordWorkflowStart(workflowID string, target string)
	RecordWorkflowComplete(workflowID string, target string, duration time.Duration, err error)
	RecordStepExecution(workflowID, stepID string, stepType string, duration time.Duration, err error)
	RecordToolExecution(tool string, args []string, duration time.Duration, err error)
}

func NewExecutor(maxConcurrent int) *Executor {
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	
	// Load database for configuration
	db, _ := services.LoadDatabase()
	
	return &Executor{
		maxConcurrent: maxConcurrent,
		semaphore:     make(chan struct{}, maxConcurrent),
		db:            db,
	}
}

// SetTarget sets the target for this executor
func (e *Executor) SetTarget(target string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.target = target
}

// SetMonitor sets the monitoring interface
func (e *Executor) SetMonitor(monitor Monitor) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.monitor = monitor
}

func (e *Executor) RunWorkflows(ctx context.Context, workflows []Workflow) error {
	var (
		wg           sync.WaitGroup
		parallelWfs  []Workflow
		sequentialWfs []Workflow
	)
	
	for _, wf := range workflows {
		if wf.Parallel {
			parallelWfs = append(parallelWfs, wf)
		} else {
			sequentialWfs = append(sequentialWfs, wf)
		}
	}
	
	errChan := make(chan error, len(workflows))
	
	for _, wf := range parallelWfs {
		wg.Add(1)
		go func(workflow Workflow) {
			defer wg.Done()
			
			select {
			case e.semaphore <- struct{}{}:
				defer func() { <-e.semaphore }()
				
				e.mu.Lock()
				e.running++
				logger.Printf("Starting parallel workflow: %s (%d/%d running)\n", workflow.ID, e.running, e.maxConcurrent)
				e.mu.Unlock()
				
				if err := e.executeWorkflow(ctx, workflow); err != nil {
					errChan <- fmt.Errorf("workflow %s failed: %w", workflow.ID, err)
				}
				
				e.mu.Lock()
				e.running--
				e.completed++
				logger.Printf("Completed workflow: %s (%d completed)\n", workflow.ID, e.completed)
				e.mu.Unlock()
				
			case <-ctx.Done():
				errChan <- fmt.Errorf("workflow %s cancelled: %w", workflow.ID, ctx.Err())
			}
		}(wf)
	}
	
	wg.Wait()
	
	for i := 0; i < e.maxConcurrent; i++ {
		e.semaphore <- struct{}{}
	}
	
	for _, wf := range sequentialWfs {
		e.mu.Lock()
		logger.Printf("Starting sequential workflow: %s\n", wf.ID)
		e.mu.Unlock()
		
		if err := e.executeWorkflow(ctx, wf); err != nil {
			errChan <- fmt.Errorf("workflow %s failed: %w", wf.ID, err)
		}
		
		e.mu.Lock()
		e.completed++
		logger.Printf("Completed workflow: %s (%d completed)\n", wf.ID, e.completed)
		e.mu.Unlock()
	}
	
	for i := 0; i < e.maxConcurrent; i++ {
		<-e.semaphore
	}
	
	close(errChan)
	
	var allErrors []error
	for err := range errChan {
		allErrors = append(allErrors, err)
	}
	
	if len(allErrors) > 0 {
		return fmt.Errorf("workflow execution had %d errors: %v", len(allErrors), allErrors)
	}
	
	return nil
}

func (e *Executor) executeWorkflow(ctx context.Context, wf Workflow) error {
	// Get target from executor
	e.mu.RLock()
	target := e.target
	e.mu.RUnlock()
	
	if target == "" {
		target = "unknown" // Fallback if target not set
	}
	
	startTime := time.Now()
	
	// Record workflow start
	if e.monitor != nil {
		e.monitor.RecordWorkflowStart(wf.ID, target)
	}
	
	// Analyze the target type
	targetInfo := AnalyzeTarget(target)
	logger.Printf("Executing workflow: %s - %s\n", wf.ID, wf.Description)
	logger.Printf("  Target: %s (Type: %s, Private: %v, Local: %v)\n", 
		target, targetInfo.Type.String(), targetInfo.IsPrivate, targetInfo.IsLocal)
	
	stepResults := make(map[string]StepResult)
	completed := make(map[string]bool)
	inProgress := make(map[string]bool)
	var mu sync.Mutex
	
	// Channel to signal step completion
	stepDone := make(chan string, len(wf.Steps))
	errChan := make(chan error, len(wf.Steps))
	var wg sync.WaitGroup
	
	var executeStep func(step Step)
	executeStep = func(step Step) {
		defer wg.Done()
		
		// Wait for dependencies to complete
		for _, dep := range step.DependsOn {
			for {
				mu.Lock()
				depCompleted := completed[dep]
				mu.Unlock()
				
				if depCompleted {
					break
				}
				
				// Wait a bit before checking again
				time.Sleep(100 * time.Millisecond)
			}
		}
		
		mu.Lock()
		if completed[step.ID] || inProgress[step.ID] {
			mu.Unlock()
			return
		}
		inProgress[step.ID] = true
		mu.Unlock()
		
		// Check if step should be skipped based on target type
		if skip, reason := ShouldSkipStep(step, targetInfo); skip {
			logger.Printf("    Skipping step %s: %s\n", step.ID, reason)
			mu.Lock()
			stepResults[step.ID] = StepResult{
				StepID:  step.ID,
				Output:  step.Output,
				Success: true,
				Data:    nil,
			}
			completed[step.ID] = true
			inProgress[step.ID] = false
			mu.Unlock()
			stepDone <- step.ID
			return
		}
		
		logger.Printf("  Executing step: %s\n", step.ID)
		
		stepStartTime := time.Now()
		result, err := e.runStep(ctx, step, stepResults, target, targetInfo, wf.ID)
		stepDuration := time.Since(stepStartTime)
		
		// Record step execution
		if e.monitor != nil {
			stepType := step.Type
			if stepType == "" && step.Tool != "" {
				stepType = "tool_execution"
			}
			e.monitor.RecordStepExecution(wf.ID, step.ID, stepType, stepDuration, err)
		}
		if err != nil {
			// Check if step is optional or workflow continues on error
			if step.Optional || wf.ContinueOnError {
				logger.Printf("    Warning: Step %s failed (optional/continuing): %v\n", step.ID, err)
				result.Success = false
				result.Error = err
			} else {
				errChan <- fmt.Errorf("step %s failed: %w", step.ID, err)
				return
			}
		}
		
		mu.Lock()
		stepResults[step.ID] = result
		completed[step.ID] = true
		inProgress[step.ID] = false
		mu.Unlock()
		
		stepDone <- step.ID
	}
	
	// Start all steps as goroutines
	for _, step := range wf.Steps {
		wg.Add(1)
		go executeStep(step)
	}
	
	// Wait for all steps to complete
	go func() {
		wg.Wait()
		close(stepDone)
		close(errChan)
	}()
	
	// Monitor for completion or errors
	completedSteps := 0
	totalSteps := len(wf.Steps)
	var workflowErr error
	
	for {
		select {
		case stepID := <-stepDone:
			if stepID != "" {
				completedSteps++
				if completedSteps >= totalSteps {
					goto complete
				}
			}
		case err := <-errChan:
			if err != nil {
				workflowErr = err
				goto complete
			}
		case <-ctx.Done():
			workflowErr = fmt.Errorf("workflow cancelled: %w", ctx.Err())
			goto complete
		}
		
		if completedSteps >= totalSteps {
			break
		}
	}
	
complete:
	// Record workflow completion
	if e.monitor != nil {
		duration := time.Since(startTime)
		e.monitor.RecordWorkflowComplete(wf.ID, target, duration, workflowErr)
	}
	
	return workflowErr
}

func (e *Executor) runStep(ctx context.Context, step Step, results map[string]StepResult, target string, targetInfo *TargetInfo, workflowID string) (StepResult, error) {
	result := StepResult{
		StepID:    step.ID,
		Output:    step.Output,
		Success:   false,
		Timestamp: time.Now(),
	}
	
	if err := os.MkdirAll(filepath.Dir(step.Output), 0755); err != nil {
		result.Error = fmt.Errorf("creating output directory: %w", err)
		return result, result.Error
	}
	
	templateData := make(map[string]string)
	for id, res := range results {
		templateData[id+"_output"] = res.Output
	}
	
	if step.Type == "json_to_hostlist" {
		logger.Printf("    Converting JSON to host list: %s -> %s\n", step.Inputs[0], step.Output)
		
		// Read the JSON input file
		inputFile := template.ApplyTemplate(step.Inputs[0], templateData)
		data, err := os.ReadFile(inputFile)
		if err != nil {
			// If file doesn't exist, create empty hostlist
			if os.IsNotExist(err) {
				logger.Printf("    Warning: Input file does not exist, creating empty hostlist: %s\n", inputFile)
				if err := os.WriteFile(step.Output, []byte(""), 0644); err != nil {
					result.Error = fmt.Errorf("writing empty host list file: %w", err)
					return result, result.Error
				}
				// Also create an empty ports file for nmap
				portsFile := strings.Replace(step.Output, "hostlist.txt", "ports.txt", 1)
				os.WriteFile(portsFile, []byte(""), 0644)
				result.Success = true
				result.Data = make(map[string]bool)
				return result, nil
			}
			result.Error = fmt.Errorf("reading input file: %w", err)
			return result, result.Error
		}
		
		// Handle empty file
		if len(strings.TrimSpace(string(data))) == 0 {
			logger.Printf("    Warning: Input file is empty, creating empty hostlist\n")
			if err := os.WriteFile(step.Output, []byte(""), 0644); err != nil {
				result.Error = fmt.Errorf("writing empty host list file: %w", err)
				return result, result.Error
			}
			// Also create an empty ports file for nmap
			portsFile := strings.Replace(step.Output, "hostlist.txt", "ports.txt", 1)
			os.WriteFile(portsFile, []byte(""), 0644)
			result.Success = true
			result.Data = make(map[string]bool)
			return result, nil
		}
		
		// Parse JSON data
		var jsonData []map[string]interface{}
		if err := json.Unmarshal(data, &jsonData); err != nil {
			// Try JSON Lines format
			content := strings.TrimSpace(string(data))
			lines := strings.Split(content, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				
				var lineData map[string]interface{}
				if err := json.Unmarshal([]byte(line), &lineData); err == nil {
					jsonData = append(jsonData, lineData)
				}
			}
		}
		
		// Extract unique IP addresses and collect ports
		uniqueHosts := make(map[string]bool)
		uniquePorts := make(map[int]bool)
		for _, item := range jsonData {
			// Get IP address
			if ip, ok := item["ip"].(string); ok && ip != "" {
				uniqueHosts[ip] = true
			}
			// Also check for host field
			if host, ok := item["host"].(string); ok && host != "" {
				uniqueHosts[host] = true
			}
			// Collect ports
			if port, ok := item["port"].(float64); ok {
				uniquePorts[int(port)] = true
			} else if port, ok := item["port"].(int); ok {
				uniquePorts[port] = true
			}
		}
		
		// Write hosts to file (one host per line)
		var hostList strings.Builder
		for host := range uniqueHosts {
			hostList.WriteString(host + "\n")
		}
		
		// Write ports to separate file for nmap argument
		var portsList []string
		for port := range uniquePorts {
			portsList = append(portsList, fmt.Sprintf("%d", port))
		}
		portsFile := strings.Replace(step.Output, "hostlist.txt", "ports.txt", 1)
		portsContent := strings.Join(portsList, ",")
		if err := os.WriteFile(portsFile, []byte(portsContent), 0644); err != nil {
			logger.Printf("    Warning: Failed to write ports file: %v\n", err)
		} else if len(portsList) > 0 {
			logger.Printf("    Found %d unique ports: %s\n", len(portsList), portsContent)
		}
		
		
		// Handle case where no hosts were found
		if len(uniqueHosts) == 0 {
			logger.Printf("    Warning: No hosts found in JSON data, creating empty hostlist\n")
		}
		
		if err := os.WriteFile(step.Output, []byte(hostList.String()), 0644); err != nil {
			result.Error = fmt.Errorf("writing host list file: %w", err)
			return result, result.Error
		}
		
		result.Success = true
		result.Data = map[string]interface{}{
			"hosts": uniqueHosts,
			"ports": uniquePorts,
		}
		
	} else if step.Type == "merge_files" {
		logger.Printf("    Merging files: %v -> %s\n", step.Inputs, step.Output)
		
		var allData []interface{}
		for _, inputFile := range step.Inputs {
			inputFile = template.ApplyTemplate(inputFile, templateData)
			
			if _, err := os.Stat(inputFile); os.IsNotExist(err) {
				logger.Printf("    Warning: Input file does not exist: %s\n", inputFile)
				continue
			}
			
			data, err := os.ReadFile(inputFile)
			if err != nil {
				logger.Printf("    Warning: Failed to read %s: %v\n", inputFile, err)
				continue
			}
			
			// First try to parse as standard JSON
			var jsonData interface{}
			if err := json.Unmarshal(data, &jsonData); err != nil {
				// If standard JSON fails, try JSON Lines format (NDJSON)
				content := strings.TrimSpace(string(data))
				if content != "" {
					lines := strings.Split(content, "\n")
					for _, line := range lines {
						line = strings.TrimSpace(line)
						if line == "" {
							continue
						}
						
						var lineData interface{}
						if err := json.Unmarshal([]byte(line), &lineData); err == nil {
							allData = append(allData, lineData)
						} else {
							fmt.Printf("    Warning: Failed to parse JSON line from %s: %v\n", inputFile, err)
						}
					}
				} else {
					fmt.Printf("    Warning: Failed to parse JSON from %s: %v\n", inputFile, err)
					continue
				}
			} else {
				// Successfully parsed as standard JSON
				if arr, ok := jsonData.([]interface{}); ok {
					allData = append(allData, arr...)
				} else {
					allData = append(allData, jsonData)
				}
			}
		}
		
		mergedJSON, err := json.MarshalIndent(allData, "", "  ")
		if err != nil {
			result.Error = fmt.Errorf("marshaling merged data: %w", err)
			return result, result.Error
		}
		
		if err := os.WriteFile(step.Output, mergedJSON, 0644); err != nil {
			result.Error = fmt.Errorf("writing merged file: %w", err)
			return result, result.Error
		}
		
		result.Success = true
		result.Data = allData
		
	} else if step.Tool != "" {
		logger.Printf("    Running tool: %s -> %s\n", step.Tool, step.Output)
		
		tool, err := registry.Get(step.Tool)
		if err != nil {
			result.Error = fmt.Errorf("getting tool %s: %w", step.Tool, err)
			return result, result.Error
		}
		
		args := make([]string, 0)
		
		if step.UseFlags != "" {
			toolConfig, err := registry.GetConfig(step.Tool)
			if err == nil {
				if flags, ok := toolConfig.Args.Flags[step.UseFlags]; ok {
					args = append(args, flags...)
				}
			}
		}
		
		args = append(args, step.OverrideArgs...)
		
		// Record tool start for TUI monitoring
		if e.monitor != nil {
			if enhancedMonitor, ok := e.monitor.(interface {
				RecordToolStart(tool, workflow string, args []string)
			}); ok {
				enhancedMonitor.RecordToolStart(step.Tool, workflowID, args)
			}
		}
		
		toolStartTime := time.Now()
		toolResult, err := tool.Execute(ctx, args, target)
		toolDuration := time.Since(toolStartTime)
		
		// Record tool execution with workflow context
		if e.monitor != nil {
			// Use enhanced monitoring interface if available
			if enhancedMonitor, ok := e.monitor.(interface {
				RecordToolExecutionWithWorkflow(tool, workflow string, args []string, duration time.Duration, err error)
			}); ok {
				enhancedMonitor.RecordToolExecutionWithWorkflow(step.Tool, workflowID, args, toolDuration, err)
			} else {
				e.monitor.RecordToolExecution(step.Tool, args, toolDuration, err)
			}
		}
		if err != nil {
			result.Error = fmt.Errorf("executing tool %s: %w", step.Tool, err)
			return result, result.Error
		}
		
		if toolResult.Error != nil {
			// Handle specific nmap errors when hostlist is empty
			if step.Tool == "nmap" && strings.Contains(toolResult.Error.Error(), "exit status 1") {
				// Check if the input hostlist is empty
				if step.UseFlags == "fingerprint" && len(step.OverrideArgs) >= 2 {
					if step.OverrideArgs[0] == "-iL" {
						hostlistPath := step.OverrideArgs[1]
						// Apply template if needed
						if strings.Contains(hostlistPath, "{{") {
							hostlistPath = template.ApplyTemplate(hostlistPath, map[string]string{"target": target})
						}
						if data, err := os.ReadFile(hostlistPath); err == nil {
							if len(strings.TrimSpace(string(data))) == 0 {
								logger.Printf("    Warning: Skipping nmap fingerprint - no hosts to scan\n")
								result.Success = true
								result.Data = nil
								// Write empty XML output
								emptyXML := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE nmaprun>
<nmaprun scanner="nmap" args="" start="0" version="7.94">
<verbose level="0"/>
<debug level="0"/>
<runstats><finished time="0" timestr=""/><hosts up="0" down="0" total="0"/></runstats>
</nmaprun>`
								if err := os.WriteFile(step.Output, []byte(emptyXML), 0644); err != nil {
									result.Error = fmt.Errorf("writing output file: %w", err)
									return result, result.Error
								}
								return result, nil
							}
						}
					}
				}
			}
			result.Error = fmt.Errorf("tool %s failed: %w", step.Tool, toolResult.Error)
			return result, result.Error
		}
		
		if err := os.WriteFile(step.Output, []byte(toolResult.Output), 0644); err != nil {
			result.Error = fmt.Errorf("writing output file: %w", err)
			return result, result.Error
		}
		
		result.Success = true
		result.Data = toolResult.Data
	}
	
	return result, nil
}

type StepResult struct {
	StepID    string
	Output    string
	Success   bool
	Error     error
	Timestamp time.Time
	Data      interface{}
}

func findStep(steps []Step, id string) *Step {
	for i := range steps {
		if steps[i].ID == id {
			return &steps[i]
		}
	}
	return nil
}