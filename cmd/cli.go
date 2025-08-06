package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"ipcrawler/internal/core"
	"ipcrawler/internal/utils"
	"ipcrawler/internal/ui"

	"github.com/urfave/cli/v2"
)

const (
	appName        = "ipcrawler"
	configFile     = "configs/config.yaml"
	workflowsDir   = "configs/workflows"
)

// convertWorkflowsToInterface converts workflow map to interface{} map for UI display
func convertWorkflowsToInterface(workflows map[string]*core.Workflow) map[string]interface{} {
	result := make(map[string]interface{})
	for key, workflow := range workflows {
		result[key] = map[string]interface{}{
			"name":        workflow.Name,
			"description": workflow.Description,
			"tool":        getFirstToolFromWorkflow(workflow),
		}
	}
	return result
}

// getFirstToolFromWorkflow extracts the first tool name from a workflow
func getFirstToolFromWorkflow(workflow *core.Workflow) string {
	if len(workflow.Steps) > 0 {
		return workflow.Steps[0].Tool
	}
	return "unknown"
}

func Execute() error {
	// Load config to get version and other settings
	config, err := core.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	app := &cli.App{
		Name:    appName,
		Version: config.Version,
		Usage:   "Crawl IP addresses and domains",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "workflow",
				Aliases: []string{"w"},
				Usage:   "Specify template to use (e.g., 'comprehensive', 'stealth')",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Output directory (default: ./reports)",
			},
			&cli.BoolFlag{
				Name:    "debug",
				Aliases: []string{"d"},
				Usage:   "Enable debug mode with verbose output",
			},
			&cli.BoolFlag{
				Name:    "json",
				Aliases: []string{"j"},
				Usage:   "Output results in JSON format",
			},
			&cli.BoolFlag{
				Name:    "health",
				Aliases: []string{"H"},
				Usage:   "Run health check",
			},
			&cli.BoolFlag{
				Name:    "list-templates",
				Aliases: []string{"l"},
				Usage:   "List available templates",
			},
		},
		Action: func(c *cli.Context) error {
			if c.Bool("health") {
				runHealthCheck()
				return nil
			}

			if c.Bool("list-templates") {
				listTemplates(config)
				return nil
			}

			// Check if target is provided
			if c.NArg() < 1 {
				return fmt.Errorf("no target specified. Use: %s <target>", appName)
			}

			target := c.Args().First()
			if target == "" {
				return fmt.Errorf("empty target specified")
			}

			// Sanitize target to ensure it's a valid hostname/IP
			if !utils.IsValidTarget(target) {
				return fmt.Errorf("invalid target format: %s", target)
			}

			template := c.String("workflow")
			if template == "" {
				template = config.DefaultTemplate
			}

			outputDir := c.String("output")
			if outputDir != "" {
				config.SetReportDir(outputDir)
			}

			debugMode := c.Bool("debug")
			jsonOutput := c.Bool("json")

			// Show banner in interactive mode
			if !jsonOutput && !debugMode {
				ui.Global.Banners.ShowApplicationBanner(config.Version, target, template)
			}

			// Set up clean cancellation with ctrl+c
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Set up signal handling for graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

			go func() {
				sig := <-sigChan
				ui.Global.Messages.DisableOutput()    // Stop any UI updates immediately
				cancel()                               // Cancel all operations
				
				// Clear terminal and show clean shutdown message
				fmt.Fprintf(os.Stderr, "\033[2K\r") // Clear line
				fmt.Fprintf(os.Stderr, "\n⚠️  Received signal: %v\n", sig)
				fmt.Fprintf(os.Stderr, "🛑 Stopping all operations...\n")
			}()

			// Load workflows from template
			if debugMode {
				log.Printf("Loading template: %s", template)
			}
			workflows, err := core.LoadWorkflows(filepath.Join(workflowsDir, template, "scanning"))
			if err != nil {
				return fmt.Errorf("failed to load workflows: %w", err)
			}

			if len(workflows) == 0 {
				return fmt.Errorf("no workflows found in template: %s", template)
			}

			// Show workflow information
			if debugMode {
				ui.Global.Messages.LoadedWorkflows(len(workflows))
				for key, workflow := range workflows {
					ui.Global.Messages.WorkflowInfo(key, workflow.Name)
				}
			}

			// Initialize variables for template
			vars := map[string]string{
				"target": target,
			}

			// Check if this is a sudo restart and clean up the flag
			isSudoRestart := utils.IsSudoRestart()
			if isSudoRestart {
				utils.RemoveSudoRestartFlag()
			}

			// Check which tools need sudo and categorize them
			tools, args := core.ExtractToolsAndArgsFromWorkflows(workflows)
			privilegedTools := []string{}
			normalTools := []string{}
			
			for i, tool := range tools {
				if utils.CheckPrivilegeRequirements([]string{tool}, [][]string{args[i]}) {
					privilegedTools = append(privilegedTools, tool)
				} else {
					normalTools = append(normalTools, tool)
				}
			}

			// Generate report directory path for preview (don't create yet)
			reportDir := core.GenerateReportDirectoryPath(config.ReportDir, target)

			var privilegeOption *ui.PrivilegeOption

			// Only show interactive preview if this is NOT a sudo restart
			if !isSudoRestart {
				// Show interactive scan preview and get user decision
				interactive := ui.NewInteractive()
				scanPreview := &ui.ScanPreview{
					Target:          target,
					Template:        template,
					ReportDir:       reportDir,
					Workflows:       convertWorkflowsToInterface(workflows),
					PrivilegedTools: privilegedTools,
					NormalTools:     normalTools,
				}

				var err error
				privilegeOption, err = interactive.ShowScanPreview(scanPreview)
				if err != nil {
					return fmt.Errorf("failed to get user input: %w", err)
				}

				// Handle privilege escalation if requested
				if privilegeOption.RequestEscalation && !utils.IsRunningAsRoot() {
					return utils.RequestPrivilegeEscalation()
				}
			} else {
				// For sudo restarts, automatically use privileged mode
				privilegeOption = &ui.PrivilegeOption{
					UseSudo:           true,
					RequestEscalation: false, // Already escalated
				}
			}

			// Now create the actual report directory
			reportDir, err = core.CreateReportDirectory(config.ReportDir, target)
			if err != nil {
				return fmt.Errorf("failed to create report directory: %w", err)
			}

			// Add report directory to variables
			vars["report_dir"] = reportDir

			// Execute workflows with coordination
			if err := executeWorkflowsWithCoordination(ctx, workflows, vars, debugMode, reportDir, target, config, privilegeOption.UseSudo); err != nil {
				// Check if it was a user cancellation
				if ctx.Err() != nil {
					// Show the main cancellation message (spinner already handled)
					ui.Global.Messages.ScanCancelled()
					return nil
				}
				return fmt.Errorf("workflow execution failed: %w", err)
			}

			// Success message
			if !jsonOutput {
				ui.Global.Messages.ScanCompleted(target)
				ui.Global.Messages.ResultsSaved()
			}

			return nil
		},
	}

	return app.Run(os.Args)
}

// runHealthCheck performs a system health check
func runHealthCheck() {
	ui.Global.Messages.SystemHealthOK()
	ui.Global.Messages.SystemVersion("0.1.1")
	
	// Check if running as root
	if utils.IsRunningAsRoot() {
		ui.Global.Messages.RunningWithRootPrivileges()
	}
	
	// Check available tools
	tools := []string{"nmap", "naabu"}
	var missingTools []string
	
	for _, tool := range tools {
		if _, err := utils.LookPath(tool); err != nil {
			missingTools = append(missingTools, tool)
		}
	}
	
	if len(missingTools) > 0 {
		ui.Global.Messages.MissingTools(missingTools)
	} else {
		ui.Global.Messages.AllSystemsOperational()
	}
}

// listTemplates lists available workflow templates
func listTemplates(config *core.Config) {
	ui.Global.Messages.AvailableTemplates()
	for _, template := range config.Templates {
		if template == config.DefaultTemplate {
			ui.Global.Messages.DefaultTemplate(template)
		} else {
			ui.Global.Messages.Template(template)
		}
	}
}


// executeWorkflowsWithCoordination executes workflows sequentially based on dependencies
func executeWorkflowsWithCoordination(ctx context.Context, workflows map[string]*core.Workflow, vars map[string]string, debugMode bool, reportDir, target string, config *core.Config, useSudo bool) error {
	// Build dependency graph and execution levels
	executionLevels, err := buildExecutionLevels(workflows)
	if err != nil {
		return fmt.Errorf("failed to resolve workflow dependencies: %w", err)
	}
	
	if debugMode {
		ui.Global.Messages.WorkflowExecutionLevels(executionLevels)
	}
	
	// Keep track of data provided by workflows
	providedData := make(map[string]string)
	var providedDataMutex sync.Mutex
	
	// Execute workflows level by level, sequentially
	for levelIndex, workflowKeys := range executionLevels {
		if debugMode {
			ui.Global.Messages.ExecutingLevel(levelIndex, len(workflowKeys), workflowKeys)
		}
		
		// Execute all workflows in this level sequentially
		for _, workflowKey := range workflowKeys {
			if err := executeWorkflow(ctx, workflowKey, workflows, vars, providedData, &providedDataMutex, debugMode, reportDir, target, config, useSudo); err != nil {
				return err
			}
		}
		
		// Update vars with any newly provided data for next level
		providedDataMutex.Lock()
		for key, value := range providedData {
			vars[key] = value
		}
		providedDataMutex.Unlock()
	}
	
	return nil
}

// executeWorkflow executes a single workflow
func executeWorkflow(ctx context.Context, workflowKey string, workflows map[string]*core.Workflow, vars map[string]string, providedData map[string]string, providedDataMutex *sync.Mutex, debugMode bool, reportDir, target string, config *core.Config, useSudo bool) error {
	// Check if context has been cancelled
	select {
	case <-ctx.Done():
		// Context cancelled before starting, don't show messages
		return ctx.Err()
	default:
	}
	
	workflow, exists := workflows[workflowKey]
	if !exists {
		return nil // Skip if workflow doesn't exist in template
	}
	
	// Skip the workflow spinner since tool-specific progress will be shown
	if debugMode {
		ui.Global.Messages.StartingWorkflow(workflow.Name, workflow.Description)
	}
	
	// Create a local copy of vars
	localVars := make(map[string]string)
	for k, v := range vars {
		localVars[k] = v
	}
	
	// Update with any provided data from previous workflows
	providedDataMutex.Lock()
	for key, value := range providedData {
		localVars[key] = value
	}
	providedDataMutex.Unlock()
	
	// Special handling for nmap deep scan - provide fallback ports if none discovered
	if core.IsNmapDeepScan(workflow) {
		if discoveredPorts, exists := localVars["discovered_ports"]; !exists || discoveredPorts == "" {
			// Use common ports as fallback for deep scan
			localVars["discovered_ports"] = "22,53,80,135,139,443,445,993,995,3306,3389,5432,5900,8080,8443"
			if debugMode {
				ui.Global.Messages.NoDiscoveredPorts()
			}
		} else {
			if debugMode {
				ui.Global.Messages.UsingDiscoveredPorts(discoveredPorts)
			}
		}
	}
	
	// Replace placeholders in workflow
	workflow.ReplaceVars(localVars)
	
	var workflowResults *core.ScanResults
	var workflowError error
	
	// Execute each step
	for _, step := range workflow.Steps {
		// Check for cancellation before each step
		select {
		case <-ctx.Done():
			if !debugMode {
				ui.Global.Spinners.Fail("Cancelled by user")
			}
			return ctx.Err()
		default:
		}
		
		// Get appropriate args based on sudo preference
		args := step.GetArgs(useSudo)
		
		if debugMode {
			ui.Global.Messages.ExecutingCommand(step.Tool, args)
		}
		
		// Execute commands with specific handling for different tools
		if step.Tool == "naabu" {
			// Use fast execution for naabu with context
			results, err := core.ExecuteCommandFastContext(ctx, step.Tool, args, debugMode, useSudo)
			if err != nil {
				workflowError = err
				if ctx.Err() != nil {
					if !debugMode {
						ui.Global.Spinners.Fail("Cancelled by user")
					}
					return ctx.Err()
				}
				break
			}
			workflowResults = results
			
			if debugMode && results != nil {
				core.ShowNaabuResults(results)
			}
		} else if step.Tool == "nmap" {
			// For nmap, use enhanced execution with real-time results
			// Show real-time results in debug mode
			results, err := core.ExecuteCommandWithRealTimeResultsContext(ctx, step.Tool, args, debugMode, useSudo)
			if err != nil {
				workflowError = err
				if ctx.Err() != nil {
					if !debugMode {
						ui.Global.Spinners.Fail("Cancelled by user")
					}
					return ctx.Err()
				}
				break
			}
			workflowResults = results
			
			if debugMode && results != nil {
				core.ShowNmapResults(results)
			}
		} else {
			// Standard execution for other tools
			if err := core.ExecuteCommandWithContext(ctx, step.Tool, args, debugMode); err != nil {
				workflowError = err
				if ctx.Err() != nil {
					if !debugMode {
						ui.Global.Spinners.Fail("Cancelled by user")
					}
					return ctx.Err()
				}
				break
			}
		}
	}
	
	// Handle workflow errors (success is already handled by tool-specific completion)
	if !debugMode && workflowError != nil {
		ui.Global.Spinners.Fail(fmt.Sprintf("%s failed: %v", workflow.Name, workflowError))
	}
	
	if workflowError != nil {
		return fmt.Errorf("workflow %s failed: %w", workflow.Name, workflowError)
	}
	
	// Handle results and data provision
	if workflowResults != nil {
		// Display vulnerability results if this was a vulnerability scan
		if workflowResults.ScanType == "vulnerability-scan" {
			if !debugMode && len(workflowResults.Vulnerabilities) > 0 {
				core.ShowVulnerabilityResults(workflowResults)
			}
		}
		
		// If this workflow provides data, extract it
		if workflowResults.ScanType == "port-discovery" && len(workflowResults.Ports) > 0 {
			// Extract open ports for next workflow
			var openPorts []string
			for _, port := range workflowResults.Ports {
				if port.State == "open" {
					openPorts = append(openPorts, strconv.Itoa(port.Number))
				}
			}
			if len(openPorts) > 0 {
				discoveredPortsStr := strings.Join(openPorts, ",")
				providedDataMutex.Lock()
				providedData["discovered_ports"] = discoveredPortsStr
				providedDataMutex.Unlock()
				
				if debugMode {
					ui.Global.Messages.DiscoveredPorts(len(openPorts), discoveredPortsStr)
				}
			}
		}
	}
	
	// Handle workflow-specific data provision from file outputs
	if len(workflow.Provides) > 0 {
		// Extract data from output files
		extractedData, err := core.ExtractProvidedData(reportDir, workflow.Provides)
		if err == nil && len(extractedData) > 0 {
			providedDataMutex.Lock()
			for key, value := range extractedData {
				providedData[key] = value
				if debugMode {
					ui.Global.Messages.ProvidedData(key, value)
				}
			}
			providedDataMutex.Unlock()
		}
	}
	
	fmt.Println()
	return nil
}

// buildExecutionLevels creates execution levels based on dependencies
func buildExecutionLevels(workflows map[string]*core.Workflow) ([][]string, error) {
	// First, build a dependency graph to determine execution levels
	workflowNames := make(map[string]string) // workflow name -> workflow key
	workflowLevels := make(map[string]int)   // workflow name -> execution level
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	
	// Extract workflow name from key (remove tool prefix)
	for key := range workflows {
		parts := strings.Split(key, "_")
		if len(parts) > 1 {
			workflowNames[parts[1]] = key
		} else {
			workflowNames[key] = key
		}
	}
	
	// Calculate execution level for each workflow using DFS
	var calculateLevel func(workflowKey string) (int, error)
	calculateLevel = func(workflowKey string) (int, error) {
		workflow := workflows[workflowKey]
		
		// Check for circular dependencies
		if visiting[workflowKey] {
			return 0, fmt.Errorf("circular dependency detected involving %s", workflow.Name)
		}
		
		// If already calculated, return the level
		if visited[workflowKey] {
			return workflowLevels[workflowKey], nil
		}
		
		visiting[workflowKey] = true
		maxDepLevel := -1
		
		// Check dependencies
		for _, dep := range workflow.Requires {
			// Find the workflow key for this dependency
			depKey, exists := workflowNames[dep]
			if !exists {
				// Try to find by scanning all workflows
				for key, w := range workflows {
					if w.Name == dep || strings.Contains(key, dep) {
						depKey = key
						workflowNames[dep] = key
						break
					}
				}
				if depKey == "" {
					return 0, fmt.Errorf("dependency '%s' not found for workflow '%s'", dep, workflow.Name)
				}
			}
			
			level, err := calculateLevel(depKey)
			if err != nil {
				return 0, err
			}
			if level > maxDepLevel {
				maxDepLevel = level
			}
		}
		
		// This workflow's level is one more than its highest dependency
		workflowLevels[workflowKey] = maxDepLevel + 1
		visiting[workflowKey] = false
		visited[workflowKey] = true
		
		return workflowLevels[workflowKey], nil
	}
	
	// Calculate levels for all workflows
	for key := range workflows {
		if _, err := calculateLevel(key); err != nil {
			return nil, err
		}
	}
	
	// Group workflows by level
	maxLevel := 0
	for _, level := range workflowLevels {
		if level > maxLevel {
			maxLevel = level
		}
	}
	
	executionLevels := make([][]string, maxLevel+1)
	for key, level := range workflowLevels {
		executionLevels[level] = append(executionLevels[level], key)
	}
	
	return executionLevels, nil
}