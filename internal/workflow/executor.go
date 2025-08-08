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

	"github.com/carlosm/ipcrawler/internal/registry"
	"github.com/carlosm/ipcrawler/internal/services"
	"github.com/carlosm/ipcrawler/internal/template"
)

type Executor struct {
	maxConcurrent int
	semaphore     chan struct{}
	mu            sync.Mutex
	running       int
	completed     int
	errors        []error
	db            *services.Database
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

func (e *Executor) RunWorkflows(ctx context.Context, workflows []Workflow, target string) error {
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
				msg := e.db.GetStatusMessage("starting_parallel", map[string]string{
					"workflow_id": workflow.ID,
					"running": fmt.Sprintf("%d", e.running),
					"max_concurrent": fmt.Sprintf("%d", e.maxConcurrent),
				})
				fmt.Println(msg)
				e.mu.Unlock()
				
				if err := e.executeWorkflow(ctx, workflow, target); err != nil {
					errChan <- fmt.Errorf("workflow %s failed: %w", workflow.ID, err)
				}
				
				e.mu.Lock()
				e.running--
				e.completed++
				msg = e.db.GetStatusMessage("completed", map[string]string{
					"workflow_id": workflow.ID,
					"completed": fmt.Sprintf("%d", e.completed),
				})
				fmt.Println(msg)
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
		seqMsg := e.db.GetStatusMessage("starting_sequential", map[string]string{
			"workflow_id": wf.ID,
		})
		fmt.Println(seqMsg)
		e.mu.Unlock()
		
		if err := e.executeWorkflow(ctx, wf, target); err != nil {
			errChan <- fmt.Errorf("workflow %s failed: %w", wf.ID, err)
		}
		
		e.mu.Lock()
		e.completed++
		compMsg := e.db.GetStatusMessage("completed", map[string]string{
			"workflow_id": wf.ID,
			"completed": fmt.Sprintf("%d", e.completed),
		})
		fmt.Println(compMsg)
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

func (e *Executor) executeWorkflow(ctx context.Context, wf Workflow, target string) error {
	fmt.Printf("Executing workflow: %s - %s\n", wf.ID, wf.Description)
	
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
		
		stepMsg := e.db.GetStatusMessage("step_executing", map[string]string{
			"step_id": step.ID,
		})
		fmt.Println(stepMsg)
		
		result, err := e.runStep(ctx, step, stepResults, target)
		if err != nil {
			errChan <- fmt.Errorf("step %s failed: %w", step.ID, err)
			return
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
	
	for {
		select {
		case stepID := <-stepDone:
			if stepID != "" {
				completedSteps++
				if completedSteps >= totalSteps {
					return nil
				}
			}
		case err := <-errChan:
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return fmt.Errorf("workflow cancelled: %w", ctx.Err())
		}
		
		if completedSteps >= totalSteps {
			break
		}
	}
	
	return nil
}

func (e *Executor) runStep(ctx context.Context, step Step, results map[string]StepResult, target string) (StepResult, error) {
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
		fmt.Printf("    Converting JSON to host list: %s -> %s\n", step.Inputs[0], step.Output)
		
		// Read the JSON input file
		inputFile := template.ApplyTemplate(step.Inputs[0], templateData)
		data, err := os.ReadFile(inputFile)
		if err != nil {
			result.Error = fmt.Errorf("reading input file: %w", err)
			return result, result.Error
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
		
		// Extract unique IP addresses
		uniqueHosts := make(map[string]bool)
		for _, item := range jsonData {
			if ip, ok := item["ip"].(string); ok && ip != "" {
				uniqueHosts[ip] = true
			}
			if host, ok := item["host"].(string); ok && host != "" {
				uniqueHosts[host] = true
			}
		}
		
		// Write to plain text file (one host per line)
		var hostList strings.Builder
		for host := range uniqueHosts {
			hostList.WriteString(host + "\n")
		}
		
		if err := os.WriteFile(step.Output, []byte(hostList.String()), 0644); err != nil {
			result.Error = fmt.Errorf("writing host list file: %w", err)
			return result, result.Error
		}
		
		result.Success = true
		result.Data = uniqueHosts
		
	} else if step.Type == "merge_files" {
		mergeMsg := e.db.GetStatusMessage("merge_operation", map[string]string{
			"inputs": fmt.Sprintf("%v", step.Inputs),
			"output": step.Output,
		})
		fmt.Println(mergeMsg)
		
		var allData []interface{}
		for _, inputFile := range step.Inputs {
			inputFile = template.ApplyTemplate(inputFile, templateData)
			
			if _, err := os.Stat(inputFile); os.IsNotExist(err) {
				fmt.Printf("    Warning: Input file does not exist: %s\n", inputFile)
				continue
			}
			
			data, err := os.ReadFile(inputFile)
			if err != nil {
				fmt.Printf("    Warning: Failed to read %s: %v\n", inputFile, err)
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
		toolMsg := e.db.GetStatusMessage("tool_running", map[string]string{
			"tool": step.Tool,
			"output": step.Output,
		})
		fmt.Println(toolMsg)
		
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
		
		toolResult, err := tool.Execute(ctx, args, target)
		if err != nil {
			result.Error = fmt.Errorf("executing tool %s: %w", step.Tool, err)
			return result, result.Error
		}
		
		if toolResult.Error != nil {
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