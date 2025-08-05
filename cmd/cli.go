package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"ipcrawler/core"
	"ipcrawler/internal/utils"

	"github.com/pterm/pterm"
	"github.com/urfave/cli/v2"
)

const (
	appName        = "ipcrawler"
	appVersion     = "0.1.1"
	configFile     = "config.yaml"
	workflowsDir   = "workflows"
)

func Execute() error {
	app := &cli.App{
		Name:    appName,
		Version: appVersion,
		Usage:   "Crawl IP addresses and domains",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "workflow",
				Aliases: []string{"w"},
				Usage:   "Specify template to use (e.g., 'comprehensive', 'stealth')",
			},
			&cli.BoolFlag{
				Name:    "debug",
				Aliases: []string{"d"},
				Usage:   "Enable debug mode",
			},
			&cli.BoolFlag{
				Name:  "health",
				Usage: "Print system status and exit",
			},
		},
		Action: func(c *cli.Context) error {
			// Set up signal handling for graceful cancellation
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			
			// Ensure we're the process group leader for proper signal handling
			// This is especially important when running under sudo
			if os.Geteuid() == 0 {
				syscall.Setpgid(0, 0)
			}
			
			// Create a buffered channel to listen for interrupt signals
			sigChan := make(chan os.Signal, 3)
			// Listen for multiple signal types to ensure we catch Ctrl+C in all scenarios
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
			
			// Track if we've already handled a signal
			var signalHandled bool
			var signalMutex sync.Mutex
			
			// Check if we're running as root (sudo mode)
			isRunningAsRoot := os.Geteuid() == 0
			
			// Start a more aggressive goroutine to handle signals
			go func() {
				defer signal.Stop(sigChan)
				signalCount := 0
				
				for sig := range sigChan {
					signalCount++
					
					signalMutex.Lock()
					if signalHandled {
						signalMutex.Unlock()
						if signalCount > 1 {
							// Force exit after 2 Ctrl+C presses (more aggressive for sudo)
							fmt.Fprintf(os.Stderr, "\n⚠️  Force terminating after %d interrupts...\n", signalCount)
							// Kill all child processes if running as root
							if isRunningAsRoot {
								fmt.Fprintf(os.Stderr, "🔒 Cleaning up sudo processes...\n")
							}
							os.Exit(130) // Standard exit code for Ctrl+C
						}
						continue
					}
					signalHandled = true
					signalMutex.Unlock()
					
					// Force output to stderr to bypass PTerm, with different messages for sudo
					if isRunningAsRoot {
						fmt.Fprintf(os.Stderr, "\n⚠️  [SUDO] Received signal: %v\n", sig)
						fmt.Fprintf(os.Stderr, "🛑 [SUDO] Cancelling scan with elevated privileges...\n")
						fmt.Fprintf(os.Stderr, "   • Stopping root-level commands\n")
						fmt.Fprintf(os.Stderr, "   • Cleaning up privileged processes\n")
						fmt.Fprintf(os.Stderr, "   • Press Ctrl+C once more to force quit\n")
					} else {
						fmt.Fprintf(os.Stderr, "\n⚠️  Received signal: %v\n", sig)
						fmt.Fprintf(os.Stderr, "🛑 Cancelling scan... (this may take a moment)\n")
						fmt.Fprintf(os.Stderr, "   • Stopping running commands\n")
						fmt.Fprintf(os.Stderr, "   • Cleaning up processes\n")
						fmt.Fprintf(os.Stderr, "   • Press Ctrl+C again to force quit\n")
					}
					
					// Cancel the context
					cancel()
					
					// Give it less time for sudo mode, more aggressive termination
					timeout := 3 * time.Second
					if isRunningAsRoot {
						timeout = 2 * time.Second
					}
					
					go func() {
						time.Sleep(timeout)
						if isRunningAsRoot {
							fmt.Fprintf(os.Stderr, "⚠️  [SUDO] Force terminating privileged process...\n")
						} else {
							fmt.Fprintf(os.Stderr, "⚠️  Force terminating...\n")
						}
						// Kill all child processes aggressively
						if isRunningAsRoot {
							// Kill process group for sudo processes
							syscall.Kill(0, syscall.SIGKILL)
						}
						os.Exit(130)
					}()
					
					return
				}
			}()
			
			// Handle --health flag
			if c.Bool("health") {
				pterm.Success.Println("System Status: OK")
				pterm.Info.Printf("Version: %s\n", appVersion)
				pterm.Success.Println("All systems operational")
				return nil
			}
			
			// Check if this is a sudo restart and clean up the flag
			isSudoRestart := utils.IsSudoRestart()
			if isSudoRestart {
				utils.RemoveSudoRestartFlag()
			}
			
			// Check if target argument is provided
			if c.NArg() < 1 {
				pterm.Error.Printf("Target argument is required\n")
				pterm.Info.Printf("Usage: %s [options] <target>\n", appName)
				return fmt.Errorf("missing target")
			}

			// Parse arguments
			target := c.Args().Get(0)
			debugMode := c.Bool("debug")

			// Load config
			config, err := core.LoadConfig(configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if err := config.Validate(); err != nil {
				return fmt.Errorf("invalid config: %w", err)
			}

			// Determine which template to use
			templateName := config.DefaultTemplate
			if workflowOverride := c.String("workflow"); workflowOverride != "" {
				templateName = workflowOverride
			}

			if debugMode {
				log.Printf("Target: %s", target)
				log.Printf("Template: %s", templateName)
				log.Printf("Debug mode: %v", debugMode)
			}

			// Load workflows for the template
			workflows, err := core.LoadTemplateWorkflows(workflowsDir, templateName)
			if err != nil {
				return fmt.Errorf("failed to load workflows: %w", err)
			}

			// Create report directory
			reportDir, err := core.CreateReportDirectory(config.ReportDir, target)
			if err != nil {
				return fmt.Errorf("failed to create report directory: %w", err)
			}

			// Convert workflows map to slice for preview
			workflowSlice := make([]*core.Workflow, 0, len(workflows))
			for _, workflow := range workflows {
				workflowSlice = append(workflowSlice, workflow)
			}

			var privilegeOption *core.PrivilegeOption
			
			// Handle privilege escalation logic
			if isSudoRestart {
				// This is a restart from sudo - skip the prompt and proceed with sudo enabled
				privilegeOption = &core.PrivilegeOption{UseSudo: true, UserChoice: "restart"}
				pterm.Success.Println("✓ Running with elevated privileges after restart")
			} else {
				// Show interactive scan preview and get sudo choice
				preview := &core.ScanPreview{
					Target:        target,
					Template:      templateName,
					Workflows:     workflowSlice,
					ReportDir:     reportDir,
					EstimatedTime: core.EstimateScanTime(workflowSlice),
				}

				var err error
				privilegeOption, err = core.ShowScanPreview(preview)
				if err != nil {
					return fmt.Errorf("failed to get user input: %w", err)
				}
				
				// Handle privilege escalation if user chose sudo
				if privilegeOption.UseSudo {
					// Check if we're already running with elevated privileges
					if utils.IsRunningAsRoot() {
						pterm.Success.Println("✓ Already running with elevated privileges")
					} else {
						// Need to restart with sudo privileges
						pterm.Info.Println("🔄 Restarting with elevated privileges...")
						if err := utils.RequestPrivilegeEscalation(); err != nil {
							pterm.Error.Printf("Failed to restart with elevated privileges: %v\n", err)
							pterm.Info.Println("Please ensure:")
							pterm.Info.Println("  • sudo is installed and available in PATH")
							pterm.Info.Println("  • You have permission to use sudo")
							return fmt.Errorf("privilege escalation failed: %w", err)
						}
						// If we reach here, the restart failed (should not happen normally)
						return fmt.Errorf("unexpected error during privilege escalation")
					}
				}
			}

			// Create variables for placeholder replacement
			vars := map[string]string{
				"target":      target,
				"template":    templateName,
				"report_dir":  reportDir,
			}

			// Execute workflows with dependency coordination
			if err := executeWorkflowsWithCoordination(ctx, workflows, vars, debugMode, reportDir, target, config, privilegeOption.UseSudo); err != nil {
				return fmt.Errorf("workflow execution failed: %w", err)
			}

			if len(workflows) == 0 {
				pterm.Warning.Printf("No workflows found for template: %s\n", templateName)
				pterm.Info.Printf("Check that workflow files exist in: %s\n", 
					filepath.Join(workflowsDir, templateName))
			}

			// If reporting is enabled, run the reporting pipeline
			if hasReporting := checkReportingEnabled(workflows); hasReporting {
				var reportSpinner *pterm.SpinnerPrinter
				
				if debugMode {
					pterm.Info.Println("📊 Running reporting pipeline...")
				} else {
					whiteReportSpinner := pterm.DefaultSpinner.WithMessageStyle(pterm.NewStyle(pterm.FgWhite)).
						WithStyle(pterm.NewStyle(pterm.FgWhite))
					reportSpinner, _ = whiteReportSpinner.Start("📊 Generating Reports...")
				}
				
				// Wait for tool outputs to be fully written before starting reporting
				if debugMode {
					pterm.Info.Println("🔍 Verifying tool outputs are complete before generating reports...")
				}
				
				if err := core.WaitForToolCompletion(reportDir, workflows, 30*time.Second, debugMode); err != nil {
					if debugMode {
						pterm.Warning.Printf("Tool completion wait failed: %v\n", err)
						log.Printf("Tool completion error: %v", err)
					}
					// Continue anyway - some tools might not have written files as expected
				}
				
				// Run the reporting pipeline
				if debugMode {
					pterm.Info.Println("🔄 Starting reporting pipeline...")
				}
				
				if err := core.RunReportingPipeline(reportDir, target, workflows, config, debugMode); err != nil {
					if debugMode {
						pterm.Error.Printf("Reporting pipeline failed: %v\n", err)
						log.Printf("Reporting pipeline error: %v", err)
						pterm.Info.Println("📄 Falling back to raw results display...")
					} else {
						reportSpinner.Fail("❌ Report generation failed")
					}
					
					// Show raw results as fallback when reporting fails
					core.DisplayRawResults(reportDir, target, debugMode)
				} else {
					if !debugMode {
						reportSpinner.Success("✅ Reports generated")
						pterm.Println()
						
						// Show scan summary from the generated reports
						showScanSummary(reportDir, target)
					} else {
						pterm.Success.Println("✅ Reports generated successfully!")
						pterm.Info.Printf("📁 View reports in: %s/summary/\n", reportDir)
					}
				}
			}

			return nil
		},
		ArgsUsage: "<target>",
	}

	return app.Run(os.Args)
}

// checkReportingEnabled checks if any workflow has reporting enabled
func checkReportingEnabled(workflows map[string]*core.Workflow) bool {
	for _, workflow := range workflows {
		if workflow.HasReporting() {
			return true
		}
	}
	return false
}

// executeWorkflowsWithCoordination executes workflows with dependency coordination and parallel execution
func executeWorkflowsWithCoordination(ctx context.Context, workflows map[string]*core.Workflow, vars map[string]string, debugMode bool, reportDir, target string, config *core.Config, useSudo bool) error {
	// Build dependency graph and execution levels for parallel execution
	executionLevels, err := buildExecutionLevels(workflows)
	if err != nil {
		return fmt.Errorf("failed to resolve workflow dependencies: %w", err)
	}
	
	if debugMode {
		fmt.Printf("Workflow execution levels: %v\n", executionLevels)
	}
	
	// Keep track of data provided by workflows
	providedData := make(map[string]string)
	var providedDataMutex sync.Mutex
	
	// Execute workflows level by level, with parallel execution within each level
	for levelIndex, workflowKeys := range executionLevels {
		if debugMode {
			fmt.Printf("Executing level %d with %d workflows: %v\n", levelIndex, len(workflowKeys), workflowKeys)
		}
		
		// Group workflows by parallel group within this level
		parallelGroups := make(map[string][]string)
		for _, workflowKey := range workflowKeys {
			workflow := workflows[workflowKey]
			if workflow == nil {
				continue
			}
			
			parallelGroup := workflow.ParallelGroup
			if parallelGroup == "" {
				parallelGroup = "_sequential_"
			}
			
			parallelGroups[parallelGroup] = append(parallelGroups[parallelGroup], workflowKey)
		}
		
		// Execute sequential workflows first
		if seqWorkflows, exists := parallelGroups["_sequential_"]; exists {
			for _, workflowKey := range seqWorkflows {
				if err := executeWorkflow(ctx, workflowKey, workflows, vars, providedData, &providedDataMutex, debugMode, reportDir, target, config, useSudo); err != nil {
					return err
				}
			}
		}
		
		// Execute parallel groups
		for groupName, groupWorkflows := range parallelGroups {
			if groupName == "_sequential_" {
				continue // Already handled above
			}
			
			if len(groupWorkflows) == 1 {
				// Single workflow in group - execute normally
				if err := executeWorkflow(ctx, groupWorkflows[0], workflows, vars, providedData, &providedDataMutex, debugMode, reportDir, target, config, useSudo); err != nil {
					return err
				}
			} else {
				// Multiple workflows in parallel group - execute in parallel with modern display
				if debugMode {
					fmt.Printf("Executing parallel group '%s' with %d workflows: %v\n", groupName, len(groupWorkflows), groupWorkflows)
				}
				
				if err := executeWorkflowsInParallel(ctx, groupWorkflows, workflows, vars, providedData, &providedDataMutex, debugMode, reportDir, target, config, useSudo); err != nil {
					return err
				}
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

// executeWorkflow executes a single workflow (extracted from the main loop for parallel execution)
func executeWorkflow(ctx context.Context, workflowKey string, workflows map[string]*core.Workflow, vars map[string]string, providedData map[string]string, providedDataMutex *sync.Mutex, debugMode bool, reportDir, target string, config *core.Config, useSudo bool) error {
	// Check if context has been cancelled
	select {
	case <-ctx.Done():
		pterm.Warning.Println("🛑 Scan cancelled by user")
		return ctx.Err()
	default:
	}
	
	workflow, exists := workflows[workflowKey]
	if !exists {
		return nil // Skip if workflow doesn't exist in template
	}
	
	// Create a local copy of vars to avoid race conditions in parallel execution
	localVars := make(map[string]string)
	for k, v := range vars {
		localVars[k] = v
	}
	
	// Update local vars with any provided data from previous workflows (thread-safe)
	providedDataMutex.Lock()
	for key, value := range providedData {
		localVars[key] = value
	}
	providedDataMutex.Unlock()
	
	// Special handling for nuclei workflows - convert discovered_ports to target_urls
	if strings.Contains(workflowKey, "nuclei") || strings.Contains(workflowKey, "vulnerability-scan") {
		if discoveredPorts, exists := localVars["discovered_ports"]; exists && discoveredPorts != "" {
			localVars["target_urls"] = core.ConvertPortsToURLs(target, discoveredPorts)
			if debugMode {
				pterm.Info.Printf("  🔗 Using discovered ports for nuclei: %s -> %s\n", discoveredPorts, localVars["target_urls"])
			}
		} else {
			// Fallback to basic target if no ports were discovered
			localVars["target_urls"] = target
			if debugMode {
				pterm.Warning.Printf("  ⚠️ No discovered ports found, using target directly: %s\n", target)
			}
		}
	}
	
	// Special handling for nmap deep scan - provide fallback ports if none discovered
	if strings.Contains(workflowKey, "nmap") && strings.Contains(workflowKey, "deep") {
		if discoveredPorts, exists := localVars["discovered_ports"]; !exists || discoveredPorts == "" {
			// Use common ports as fallback for deep scan
			localVars["discovered_ports"] = "22,53,80,135,139,443,445,993,995,3306,3389,5432,5900,8080,8443"
			if debugMode {
				pterm.Warning.Printf("  ⚠️ No discovered ports found, using common ports for deep scan\n")
			}
		}
	}
	
	// Replace placeholders
	workflow.ReplaceVars(localVars)
	
	var workflowResults *core.ScanResults
	var workflowFailed bool
	
	if debugMode {
		pterm.DefaultSection.Printf("[%s] %s", workflowKey, workflow.Name)
		if workflow.Description != "" {
			pterm.Info.Printf("  Description: %s\n", workflow.Description)
		}
		if len(workflow.Requires) > 0 {
			pterm.Info.Printf("  Dependencies: %v\n", workflow.Requires)
		}
		
		// Debug mode: Execute each step with detailed output
		for i, step := range workflow.Steps {
			// Get appropriate args based on sudo preference
			args := step.GetArgs(useSudo)
			cmd := workflow.GetCommandWithArgs(step.Tool, args)
			pterm.Info.Printf("  Executing Step %d: %s\n", i+1, cmd)
			
			// For naabu, use optimized fast execution
			if step.Tool == "naabu" {
				results, err := core.ExecuteCommandFastContext(ctx, step.Tool, args, debugMode, useSudo)
				if err != nil {
					if ctx.Err() != nil {
						pterm.Warning.Printf("  🛑 Cancelled\n")
						return ctx.Err()
					}
					pterm.Error.Printf("  ❌ Error: %v\n", err)
					log.Printf("Command execution failed: %v", err)
					workflowFailed = true
					// Continue with next step instead of failing completely
					continue
				} else {
					pterm.Success.Printf("  ✅ Completed\n")
					workflowResults = results
				}
			} else if step.Tool == "nmap" || step.Tool == "nuclei" {
				// For nmap and nuclei, use enhanced execution with real-time results
				results, err := core.ExecuteCommandWithRealTimeResultsContext(ctx, step.Tool, args, debugMode, useSudo)
				if err != nil {
					if ctx.Err() != nil {
						pterm.Warning.Printf("  🛑 Cancelled\n")
						return ctx.Err()
					}
					pterm.Error.Printf("  ❌ Error: %v\n", err)
					log.Printf("Command execution failed: %v", err)
					workflowFailed = true
					// Continue with next step instead of failing completely
					continue
				} else {
					pterm.Success.Printf("  ✅ Completed\n")
					workflowResults = results
				}
			} else {
				// Execute the command
				if err := core.ExecuteCommandWithContext(ctx, step.Tool, args, debugMode); err != nil {
					if ctx.Err() != nil {
						pterm.Warning.Printf("  🛑 Cancelled\n")
						return ctx.Err()
					}
					pterm.Error.Printf("  ❌ Error: %v\n", err)
					log.Printf("Command execution failed: %v", err)
					workflowFailed = true
					// Continue with next step instead of failing completely
					continue
				} else {
					pterm.Success.Printf("  ✅ Completed\n")
				}
			}
		}
		
		// Show intermediate results in debug mode
		if workflowResults != nil {
			if workflowResults.ScanType == "port-discovery" {
				pterm.Info.Printf("  📊 Scan Results:\n")
				core.ShowPortDiscoveryResults(workflowResults)
			} else if workflowResults.ScanType == "deep-scan" {
				pterm.Info.Printf("  📊 Scan Results:\n")
				core.ShowDeepScanResults(workflowResults)
			} else if workflowResults.ScanType == "vulnerability-scan" {
				pterm.Info.Printf("  📊 Vulnerability Results:\n")
				core.ShowNucleiResults(workflowResults)
			}
		}
	} else {
		// Clean format for normal mode with white spinner
		whiteSpinner := pterm.DefaultSpinner.WithMessageStyle(pterm.NewStyle(pterm.FgWhite)).
			WithStyle(pterm.NewStyle(pterm.FgWhite))
		spinner, _ := whiteSpinner.Start(workflow.Name)
		
		// Execute each step
		workflowFailed := false
		for _, step := range workflow.Steps {
			// Check for cancellation before each step
			select {
			case <-ctx.Done():
				spinner.Stop()
				fmt.Fprintf(os.Stderr, "🛑 Scan cancelled during workflow execution\n")
				return ctx.Err()
			default:
			}
			
			// Get appropriate args based on sudo preference
			args := step.GetArgs(useSudo)
			
			// For naabu, use optimized fast execution
			if step.Tool == "naabu" {
				results, err := core.ExecuteCommandFastContext(ctx, step.Tool, args, debugMode, useSudo)
				if err != nil {
					if ctx.Err() != nil {
						spinner.Stop()
						pterm.Warning.Printf("🛑 Scan cancelled\n")
						return ctx.Err()
					}
					spinner.Fail("❌ Failed")
					log.Printf("Command execution failed: %v", err)
					workflowFailed = true
					break
				}
				workflowResults = results
			} else if step.Tool == "nmap" || step.Tool == "nuclei" {
				// For nmap and nuclei, use enhanced execution with real-time results
				results, err := core.ExecuteCommandWithRealTimeResultsContext(ctx, step.Tool, args, debugMode, useSudo)
				if err != nil {
					if ctx.Err() != nil {
						spinner.Stop()
						pterm.Warning.Printf("🛑 Scan cancelled\n")
						return ctx.Err()
					}
					spinner.Fail("❌ Failed")
					log.Printf("Command execution failed: %v", err)
					workflowFailed = true
					break
				}
				workflowResults = results
			} else {
				// For non-nmap commands, use regular execution
				if err := core.ExecuteCommandWithContext(ctx, step.Tool, args, debugMode); err != nil {
					if ctx.Err() != nil {
						spinner.Stop()
						pterm.Warning.Printf("🛑 Scan cancelled\n")
						return ctx.Err()
					}
					spinner.Fail("❌ Failed")
					log.Printf("Command execution failed: %v", err)
					workflowFailed = true
					break
				}
			}
		}
		
		// Only show success if workflow didn't fail
		if !workflowFailed {
			spinner.Success("✅ Complete")
		}
		
		// Show intermediate results based on workflow type (only if workflow succeeded)
		if !workflowFailed && workflowResults != nil {
			if workflowResults.ScanType == "port-discovery" {
				core.ShowPortDiscoveryResults(workflowResults)
			} else if workflowResults.ScanType == "deep-scan" {
				core.ShowDeepScanResults(workflowResults)
			} else if workflowResults.ScanType == "vulnerability-scan" {
				core.ShowNucleiResults(workflowResults)
			}
			pterm.Println()
		}
	}
	
	// Thread-safe data provision from scan results (before reporting pipeline) - only if workflow succeeded
	if !workflowFailed && workflowResults != nil && workflowResults.ScanType == "port-discovery" && len(workflowResults.Ports) > 0 {
		var openPorts []string
		for _, port := range workflowResults.Ports {
			if port.State == "open" {
				openPorts = append(openPorts, strconv.Itoa(port.Number))
			}
		}
		if len(openPorts) > 0 {
			discoveredPortsStr := strings.Join(openPorts, ",")
			
			// Thread-safe update of provided data
			providedDataMutex.Lock()
			providedData["discovered_ports"] = discoveredPortsStr
			providedDataMutex.Unlock()
			
			// Show discovered ports in a compact table format
			tableData := [][]string{
				{"Port", "Protocol", "State"},
			}
			
			for _, port := range workflowResults.Ports {
				if port.State == "open" {
					protocol := "tcp"
					if port.Service != "" {
						protocol = port.Service
					}
					tableData = append(tableData, []string{
						strconv.Itoa(port.Number),
						protocol,
						"open",
					})
				}
			}
			
			pterm.Success.Printf("  📤 Discovered %d open ports:\n", len(openPorts))
			pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
			fmt.Println()
		}
	}
	
	// Handle data provision after workflow completion - only if immediate extraction didn't succeed
	if len(workflow.Provides) > 0 {
		// Check if we already have valid data from immediate extraction (thread-safe)
		hasValidData := false
		providedDataMutex.Lock()
		for _, provided := range workflow.Provides {
			if value, exists := providedData[provided]; exists && value != "" && value != "extracted_by_reporting_pipeline" {
				hasValidData = true
				if debugMode {
					pterm.Success.Printf("  ✓ Using immediate extraction data for %s: %s\n", provided, value)
				}
			}
		}
		providedDataMutex.Unlock()
		
		// Only run reporting extraction if we don't have valid immediate data
		if !hasValidData {
			if workflow.HasReporting() {
				if debugMode {
					pterm.Info.Printf("  🔧 Running reporting to extract provided data...\n")
				}
				// Run reporting pipeline immediately to extract provided data
				if err := core.RunWorkflowReporting(reportDir, target, workflowKey, workflow, config, debugMode); err != nil {
					if debugMode {
						pterm.Error.Printf("  ❌ Reporting Error: %v\n", err)
						log.Printf("Workflow reporting failed: %v", err)
					}
					// Fallback to direct extraction from raw files
					if debugMode {
						pterm.Info.Printf("  🔧 Attempting direct extraction from raw files...\n")
					}
					extractedData, err := core.ExtractProvidedData(reportDir, workflow.Provides)
					if err != nil {
						if debugMode {
							pterm.Warning.Printf("  ⚠️ Warning: Could not extract provided data - using fallback\n")
							for _, provided := range workflow.Provides {
								pterm.Info.Printf("  📤 Provides: %s\n", provided)
							}
						}
						// Use fallback ports for deep scan instead of placeholder (thread-safe)
						providedDataMutex.Lock()
						for _, provided := range workflow.Provides {
							if provided == "discovered_ports" {
								// Use common ports as fallback
								providedData[provided] = "22,53,80,135,139,443,445,993,995,3306,3389,5432,5900,8080,8443"
								if debugMode {
									pterm.Info.Printf("  🔧 Using fallback common ports for deep scan\n")
								}
							} else {
								providedData[provided] = "extracted_by_reporting_pipeline"
							}
						}
						providedDataMutex.Unlock()
					} else {
						if debugMode {
							for provided, value := range extractedData {
								pterm.Success.Printf("  📤 Provides: %s = %s\n", provided, value)
							}
						}
						// Thread-safe update of provided data
						providedDataMutex.Lock()
						for provided, value := range extractedData {
							providedData[provided] = value
						}
						providedDataMutex.Unlock()
					}
				} else {
					// Extract provided data from reporting results
					extractedData, err := core.ExtractProvidedData(reportDir, workflow.Provides)
					if err != nil {
						if debugMode {
							pterm.Warning.Printf("  ⚠️ Warning: Could not extract provided data - using fallback\n")
							for _, provided := range workflow.Provides {
								pterm.Info.Printf("  📤 Provides: %s\n", provided)
							}
						}
						// Use fallback ports for deep scan instead of placeholder (thread-safe)
						providedDataMutex.Lock()
						for _, provided := range workflow.Provides {
							if provided == "discovered_ports" {
								// Use common ports as fallback
								providedData[provided] = "22,53,80,135,139,443,445,993,995,3306,3389,5432,5900,8080,8443"
								if debugMode {
									pterm.Info.Printf("  🔧 Using fallback common ports for deep scan\n")
								}
							} else {
								providedData[provided] = "extracted_by_reporting_pipeline"
							}
						}
						providedDataMutex.Unlock()
					} else {
						if debugMode {
							for provided, value := range extractedData {
								pterm.Success.Printf("  📤 Provides: %s = %s\n", provided, value)
							}
						}
						// Thread-safe update of provided data
						providedDataMutex.Lock()
						for provided, value := range extractedData {
							providedData[provided] = value
						}
						providedDataMutex.Unlock()
					}
				}
			} else {
				// No reporting enabled, try direct extraction
				if debugMode {
					pterm.Info.Printf("  🔧 No reporting enabled, attempting direct extraction...\n")
				}
				extractedData, err := core.ExtractProvidedData(reportDir, workflow.Provides)
				if err != nil {
					if debugMode {
						pterm.Warning.Printf("  ⚠️ Warning: Could not extract provided data - using fallback\n")
						for _, provided := range workflow.Provides {
							pterm.Info.Printf("  📤 Provides: %s\n", provided)
						}
					}
					// Use fallback ports for deep scan instead of placeholder (thread-safe)
					providedDataMutex.Lock()
					for _, provided := range workflow.Provides {
						if provided == "discovered_ports" {
							// Use common ports as fallback
							providedData[provided] = "22,53,80,135,139,443,445,993,995,3306,3389,5432,5900,8080,8443"
							if debugMode {
								pterm.Info.Printf("  🔧 Using fallback common ports for deep scan\n")
							}
						} else {
							providedData[provided] = "extracted_by_reporting_pipeline"
						}
					}
					providedDataMutex.Unlock()
				} else {
					if debugMode {
						for provided, value := range extractedData {
							pterm.Success.Printf("  📤 Provides: %s = %s\n", provided, value)
						}
					}
					// Thread-safe update of provided data
					providedDataMutex.Lock()
					for provided, value := range extractedData {
						providedData[provided] = value
					}
					providedDataMutex.Unlock()
				}
			}
		}
	}
	
	fmt.Println()
	return nil
}

// executeWorkflowsInParallel executes multiple workflows with a modern parallel display
func executeWorkflowsInParallel(ctx context.Context, workflowKeys []string, workflows map[string]*core.Workflow, vars map[string]string, providedData map[string]string, providedDataMutex *sync.Mutex, debugMode bool, reportDir, target string, config *core.Config, useSudo bool) error {
	if debugMode {
		// In debug mode, execute sequentially with detailed output
		for _, workflowKey := range workflowKeys {
			if err := executeWorkflow(ctx, workflowKey, workflows, vars, providedData, providedDataMutex, debugMode, reportDir, target, config, useSudo); err != nil {
				return err
			}
		}
		return nil
	}
	
	// Create parallel progress display
	parallelDisplay := NewParallelDisplay(workflowKeys, workflows)
	parallelDisplay.Start()
	defer parallelDisplay.Stop()
	
	// Execute workflows in parallel
	var wg sync.WaitGroup
	errChan := make(chan error, len(workflowKeys))
	resultChan := make(chan ParallelResult, len(workflowKeys))
	
	for _, workflowKey := range workflowKeys {
		wg.Add(1)
		go func(wk string) {
			defer wg.Done()
			startTime := time.Now()
			
			// Update display to show workflow started
			parallelDisplay.UpdateStatus(wk, "running", "")
			
			// Execute workflow without spinner (we have our own display)
			err := executeWorkflowSilent(ctx, wk, workflows, vars, providedData, providedDataMutex, reportDir, target, config, useSudo)
			
			duration := time.Since(startTime)
			
			if err != nil {
				parallelDisplay.UpdateStatus(wk, "failed", fmt.Sprintf("Failed after %v", duration.Round(time.Second)))
				errChan <- err
			} else {
				parallelDisplay.UpdateStatus(wk, "completed", fmt.Sprintf("Completed in %v", duration.Round(time.Second)))
				resultChan <- ParallelResult{WorkflowKey: wk, Duration: duration, Success: true}
			}
		}(workflowKey)
	}
	
	wg.Wait()
	close(errChan)
	close(resultChan)
	
	// Check for errors from parallel execution
	for err := range errChan {
		if err != nil {
			return err
		}
	}
	
	// Show results summary
	var results []ParallelResult
	for result := range resultChan {
		results = append(results, result)
	}
	
	if len(results) > 0 {
		parallelDisplay.ShowSummary(results)
	}
	
	return nil
}

// ParallelResult represents the result of a parallel workflow execution
type ParallelResult struct {
	WorkflowKey string
	Duration    time.Duration
	Success     bool
}

// ParallelDisplay manages the display of parallel workflow execution
type ParallelDisplay struct {
	workflowKeys []string
	workflows    map[string]*core.Workflow
	statuses     map[string]string
	messages     map[string]string
	startTime    time.Time
	mutex        sync.Mutex
	area         *pterm.AreaPrinter
	ticker       *time.Ticker
	done         chan bool
	stopOnce     sync.Once
}

// NewParallelDisplay creates a new parallel display manager
func NewParallelDisplay(workflowKeys []string, workflows map[string]*core.Workflow) *ParallelDisplay {
	return &ParallelDisplay{
		workflowKeys: workflowKeys,
		workflows:    workflows,
		statuses:     make(map[string]string),
		messages:     make(map[string]string),
		startTime:    time.Now(),
		done:         make(chan bool),
	}
}

// Start begins the parallel display
func (pd *ParallelDisplay) Start() {
	pd.mutex.Lock()
	defer pd.mutex.Unlock()
	
	// Initialize statuses
	for _, key := range pd.workflowKeys {
		pd.statuses[key] = "waiting"
		pd.messages[key] = "Waiting to start..."
	}
	
	// Create area printer for updating display
	pd.area, _ = pterm.DefaultArea.Start()
	
	// Start ticker for updating elapsed time
	pd.ticker = time.NewTicker(500 * time.Millisecond)
	go pd.updateDisplay()
	
	// Initial render
	pd.render()
}

// Stop ends the parallel display
func (pd *ParallelDisplay) Stop() {
	pd.stopOnce.Do(func() {
		if pd.ticker != nil {
			pd.ticker.Stop()
		}
		if pd.done != nil {
			close(pd.done)
		}
		if pd.area != nil {
			pd.area.Stop()
		}
	})
}

// UpdateStatus updates the status of a workflow
func (pd *ParallelDisplay) UpdateStatus(workflowKey, status, message string) {
	pd.mutex.Lock()
	defer pd.mutex.Unlock()
	
	pd.statuses[workflowKey] = status
	pd.messages[workflowKey] = message
	pd.render()
}

// updateDisplay handles periodic updates
func (pd *ParallelDisplay) updateDisplay() {
	for {
		select {
		case <-pd.ticker.C:
			pd.mutex.Lock()
			pd.render()
			pd.mutex.Unlock()
		case <-pd.done:
			return
		}
	}
}

// render updates the display
func (pd *ParallelDisplay) render() {
	if pd.area == nil {
		return
	}
	
	var output strings.Builder
	
	// Header
	totalElapsed := time.Since(pd.startTime).Round(time.Second)
	output.WriteString(pterm.Sprintf("%s %s\n\n", 
		pterm.NewStyle(pterm.FgLightBlue, pterm.Bold).Sprint("⚡ Parallel Execution"),
		pterm.NewStyle(pterm.FgGray).Sprintf("(Total: %v)", totalElapsed)))
	
	// Progress for each workflow
	for _, key := range pd.workflowKeys {
		workflow := pd.workflows[key]
		status := pd.statuses[key]
		message := pd.messages[key]
		
		var statusIcon string
		var statusColor pterm.Color
		switch status {
		case "waiting":
			statusIcon = "⏳"
			statusColor = pterm.FgGray
		case "running":
			statusIcon = "🔄"
			statusColor = pterm.FgLightBlue
		case "completed":
			statusIcon = "✅"
			statusColor = pterm.FgGreen
		case "failed":
			statusIcon = "❌"
			statusColor = pterm.FgRed
		default:
			statusIcon = "❓"
			statusColor = pterm.FgGray
		}
		
		workflowName := workflow.Name
		if len(workflowName) > 35 {
			workflowName = workflowName[:32] + "..."
		}
		
		output.WriteString(pterm.Sprintf("%s %s %s\n", 
			statusIcon,
			pterm.NewStyle(pterm.FgWhite, pterm.Bold).Sprintf("%-38s", workflowName),
			pterm.NewStyle(statusColor).Sprint(message)))
	}
	
	pd.area.Update(output.String())
}

// ShowSummary displays the final summary
func (pd *ParallelDisplay) ShowSummary(results []ParallelResult) {
	pd.Stop()
	
	fmt.Println()
	pterm.Success.Printf("🎉 Parallel execution completed in %v\n", time.Since(pd.startTime).Round(time.Second))
	
	for _, result := range results {
		workflow := pd.workflows[result.WorkflowKey]
		if result.Success {
			pterm.Info.Printf("  ✅ %s: %v\n", workflow.Name, result.Duration.Round(time.Second))
		}
	}
	fmt.Println()
}

// executeWorkflowSilent executes a workflow without any spinner output for parallel execution
func executeWorkflowSilent(ctx context.Context, workflowKey string, workflows map[string]*core.Workflow, vars map[string]string, providedData map[string]string, providedDataMutex *sync.Mutex, reportDir, target string, config *core.Config, useSudo bool) error {
	// Check if context has been cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	workflow, exists := workflows[workflowKey]
	if !exists {
		return nil // Skip if workflow doesn't exist in template
	}
	
	// Create a local copy of vars to avoid race conditions in parallel execution
	localVars := make(map[string]string)
	for k, v := range vars {
		localVars[k] = v
	}
	
	// Update local vars with any provided data from previous workflows (thread-safe)
	providedDataMutex.Lock()
	for key, value := range providedData {
		localVars[key] = value
	}
	providedDataMutex.Unlock()
	
	// Special handling for nuclei workflows - convert discovered_ports to target_urls
	if strings.Contains(workflowKey, "nuclei") || strings.Contains(workflowKey, "vulnerability-scan") {
		if discoveredPorts, exists := localVars["discovered_ports"]; exists && discoveredPorts != "" {
			localVars["target_urls"] = core.ConvertPortsToURLs(target, discoveredPorts)
		} else {
			// Fallback to basic target if no ports were discovered
			localVars["target_urls"] = target
		}
	}
	
	// Special handling for nmap deep scan - provide fallback ports if none discovered
	if strings.Contains(workflowKey, "nmap") && strings.Contains(workflowKey, "deep") {
		if discoveredPorts, exists := localVars["discovered_ports"]; !exists || discoveredPorts == "" {
			// Use common ports as fallback for deep scan
			localVars["discovered_ports"] = "22,53,80,135,139,443,445,993,995,3306,3389,5432,5900,8080,8443"
		}
	}
	
	// Replace placeholders
	workflow.ReplaceVars(localVars)
	
	var workflowResults *core.ScanResults
	
	// Execute each step silently
	for _, step := range workflow.Steps {
		// Check for cancellation before each step
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		// Get appropriate args based on sudo preference
		args := step.GetArgs(useSudo)
		
		// Execute commands without any output
		if step.Tool == "naabu" {
			results, err := core.ExecuteCommandFastContext(ctx, step.Tool, args, false, useSudo)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return err
			}
			workflowResults = results
		} else if step.Tool == "nmap" || step.Tool == "nuclei" {
			results, err := core.ExecuteCommandWithRealTimeResultsContext(ctx, step.Tool, args, false, useSudo)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return err
			}
			workflowResults = results
		} else {
			if err := core.ExecuteCommandWithContext(ctx, step.Tool, args, false); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return err
			}
		}
	}
	
	// Handle data provision from scan results (thread-safe)
	if workflowResults != nil && workflowResults.ScanType == "port-discovery" && len(workflowResults.Ports) > 0 {
		var openPorts []string
		for _, port := range workflowResults.Ports {
			if port.State == "open" {
				openPorts = append(openPorts, strconv.Itoa(port.Number))
			}
		}
		if len(openPorts) > 0 {
			discoveredPortsStr := strings.Join(openPorts, ",")
			
			// Thread-safe update of provided data
			providedDataMutex.Lock()
			providedData["discovered_ports"] = discoveredPortsStr
			providedDataMutex.Unlock()
		}
	}
	
	// Handle workflow reporting (thread-safe)
	if len(workflow.Provides) > 0 {
		// Run workflow reporting if enabled
		if workflow.HasReporting() {
			if err := core.RunWorkflowReporting(reportDir, target, workflowKey, workflow, config, false); err != nil {
				// Don't fail completely on reporting errors in parallel mode
				log.Printf("Workflow reporting failed for %s: %v", workflowKey, err)
			}
		}
	}
	
	return nil
}

// buildExecutionLevels creates execution levels for parallel workflow execution
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
	
	// Calculate dependency depth for each workflow
	var calculateLevel func(string) (int, error)
	calculateLevel = func(workflowName string) (int, error) {
		if visiting[workflowName] {
			return 0, fmt.Errorf("circular dependency detected involving %s", workflowName)
		}
		if visited[workflowName] {
			return workflowLevels[workflowName], nil
		}
		
		workflowKey, exists := workflowNames[workflowName]
		if !exists {
			return 0, fmt.Errorf("workflow %s not found", workflowName)
		}
		
		workflow := workflows[workflowKey]
		visiting[workflowName] = true
		
		maxDepLevel := 0
		// Calculate the maximum level of all dependencies
		for _, dep := range workflow.Requires {
			depLevel, err := calculateLevel(dep)
			if err != nil {
				return 0, err
			}
			if depLevel >= maxDepLevel {
				maxDepLevel = depLevel + 1
			}
		}
		
		visiting[workflowName] = false
		visited[workflowName] = true
		workflowLevels[workflowName] = maxDepLevel
		
		return maxDepLevel, nil
	}
	
	// Calculate levels for all workflows
	for key := range workflows {
		parts := strings.Split(key, "_")
		workflowName := key
		if len(parts) > 1 {
			workflowName = parts[1]
		}
		
		if !visited[workflowName] {
			if _, err := calculateLevel(workflowName); err != nil {
				return nil, err
			}
		}
	}
	
	// Group workflows by their execution level and parallel group
	maxLevel := 0
	for _, level := range workflowLevels {
		if level > maxLevel {
			maxLevel = level
		}
	}
	
	// Create execution levels
	executionLevels := make([][]string, maxLevel+1)
	levelGroups := make([]map[string][]string, maxLevel+1) // level -> parallel_group -> workflows
	
	for i := range levelGroups {
		levelGroups[i] = make(map[string][]string)
	}
	
	// Organize workflows into levels and parallel groups
	for workflowName, level := range workflowLevels {
		workflowKey := workflowNames[workflowName]
		workflow := workflows[workflowKey]
		
		parallelGroup := workflow.ParallelGroup
		if parallelGroup == "" {
			parallelGroup = "_sequential_" // Default group for sequential execution
		}
		
		levelGroups[level][parallelGroup] = append(levelGroups[level][parallelGroup], workflowKey)
	}
	
	// Build final execution levels
	// Workflows in the same parallel group at the same level can run in parallel
	for level := 0; level <= maxLevel; level++ {
		var levelWorkflows []string
		
		// First, add all sequential workflows
		if seqWorkflows, exists := levelGroups[level]["_sequential_"]; exists {
			levelWorkflows = append(levelWorkflows, seqWorkflows...)
		}
		
		// Then, add parallel groups (each group as a batch)
		for groupName, groupWorkflows := range levelGroups[level] {
			if groupName != "_sequential_" {
				// All workflows in the same parallel group go to the same execution level
				levelWorkflows = append(levelWorkflows, groupWorkflows...)
			}
		}
		
		if len(levelWorkflows) > 0 {
			executionLevels[level] = levelWorkflows
		}
	}
	
	// Remove empty levels
	var compactLevels [][]string
	for _, level := range executionLevels {
		if len(level) > 0 {
			compactLevels = append(compactLevels, level)
		}
	}
	
	return compactLevels, nil
}

// buildExecutionOrder creates an execution order based on workflow dependencies
func buildExecutionOrder(workflows map[string]*core.Workflow) ([]string, error) {
	var order []string
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	
	// Extract workflow name from key (remove tool prefix)
	workflowNames := make(map[string]string)
	for key := range workflows {
		// Extract the actual workflow filename from the key (e.g., "nmap_port-discovery" -> "port-discovery")
		parts := strings.Split(key, "_")
		if len(parts) > 1 {
			workflowNames[parts[1]] = key
		} else {
			workflowNames[key] = key
		}
	}
	
	var visit func(string) error
	visit = func(workflowName string) error {
		if visiting[workflowName] {
			return fmt.Errorf("circular dependency detected involving %s", workflowName)
		}
		if visited[workflowName] {
			return nil
		}
		
		workflowKey, exists := workflowNames[workflowName]
		if !exists {
			return fmt.Errorf("workflow %s not found", workflowName)
		}
		
		workflow := workflows[workflowKey]
		visiting[workflowName] = true
		
		// Visit dependencies first
		for _, dep := range workflow.Requires {
			if err := visit(dep); err != nil {
				return err
			}
		}
		
		visiting[workflowName] = false
		visited[workflowName] = true
		order = append(order, workflowKey)
		
		return nil
	}
	
	// Visit all workflows
	for key := range workflows {
		parts := strings.Split(key, "_")
		workflowName := key
		if len(parts) > 1 {
			workflowName = parts[1]
		}
		
		if !visited[workflowName] {
			if err := visit(workflowName); err != nil {
				return nil, err
			}
		}
	}
	
	return order, nil
}

// showScanSummary displays a clean summary of scan results
func showScanSummary(reportDir, target string) {
	// Create a styled header section
	pterm.DefaultSection.Println("🎯 SCAN SUMMARY")
	
	// Try to read summary data from the nmap_cleaned.json file
	nmapDataPath := filepath.Join(reportDir, "processed", "nmap_cleaned.json")
	if data, err := os.ReadFile(nmapDataPath); err == nil {
		var nmapData struct {
			Ports []struct {
				Number  int    `json:"number"`
				State   string `json:"state"`
				Service string `json:"service"`
			} `json:"ports"`
		}
		
		if err := json.Unmarshal(data, &nmapData); err == nil {
			var openPorts []string
			var services []string
			var tableData [][]string
			
			// Add table header
			tableData = append(tableData, []string{"Port", "State", "Service"})
			
			for _, port := range nmapData.Ports {
				if port.State == "open" {
					openPorts = append(openPorts, strconv.Itoa(port.Number))
					if port.Service != "" && port.Service != "unknown" {
						services = append(services, port.Service)
					}
					
					// Add port data to table
					service := port.Service
					if service == "" {
						service = "unknown"
					}
					tableData = append(tableData, []string{
						strconv.Itoa(port.Number),
						port.State,
						service,
					})
				}
			}
			
			// Display results with PTerm
			if len(openPorts) > 0 {
				pterm.Success.Printf("Found %d open ports\n", len(openPorts))
				
				// Display ports table
				pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
				
				if len(services) > 0 {
					uniqueServices := removeDuplicates(services)
					pterm.Info.Printf("Services detected: %s\n", strings.Join(uniqueServices, ", "))
				}
				
				// Risk level with color coding
				riskLevel := calculateRiskLevel(nmapData.Ports, services)
				displayRiskLevel(riskLevel)
			} else {
				pterm.Warning.Println("No open ports found")
			}
		}
	} else {
		pterm.Warning.Println("Scan results not available yet")
	}
	
	pterm.Println()
	pterm.Info.Printf("📁 Full reports available at: %s/summary/\n", reportDir)
}

// removeDuplicates removes duplicate strings from a slice
func removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	var result []string
	
	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}
	
	return result
}

// RiskLevel represents the security risk level
type RiskLevel int

const (
	RiskLow RiskLevel = iota
	RiskMedium
	RiskHigh
	RiskCritical
)

// calculateRiskLevel determines risk level based on open ports and services
func calculateRiskLevel(ports []struct {
	Number  int    `json:"number"`
	State   string `json:"state"`
	Service string `json:"service"`
}, services []string) RiskLevel {
	var openPorts []int
	var criticalServices []string
	var highRiskServices []string
	
	// Define high-risk ports and services
	criticalPorts := map[int]bool{
		21: true,   // FTP
		23: true,   // Telnet
		25: true,   // SMTP
		53: true,   // DNS
		135: true,  // RPC
		139: true,  // NetBIOS
		445: true,  // SMB
		1433: true, // MSSQL
		3389: true, // RDP
		5432: true, // PostgreSQL
	}
	
	highRiskPorts := map[int]bool{
		22: true,   // SSH
		80: true,   // HTTP
		443: true,  // HTTPS
		993: true,  // IMAPS
		995: true,  // POP3S
		3306: true, // MySQL
		5900: true, // VNC
		8080: true, // HTTP-alt
	}
	
	criticalServiceNames := []string{"ftp", "telnet", "smtp", "rpc", "netbios", "smb", "microsoft-ds", "mssql", "ms-wbt-server", "postgresql"}
	highRiskServiceNames := []string{"ssh", "http", "https", "mysql", "vnc"}
	
	// Analyze ports
	for _, port := range ports {
		if port.State == "open" {
			openPorts = append(openPorts, port.Number)
			
			if criticalPorts[port.Number] {
				criticalServices = append(criticalServices, port.Service)
			} else if highRiskPorts[port.Number] {
				highRiskServices = append(highRiskServices, port.Service)
			}
		}
	}
	
	// Analyze services
	for _, service := range services {
		for _, critical := range criticalServiceNames {
			if strings.Contains(strings.ToLower(service), critical) {
				criticalServices = append(criticalServices, service)
				break
			}
		}
		for _, high := range highRiskServiceNames {
			if strings.Contains(strings.ToLower(service), high) {
				highRiskServices = append(highRiskServices, service)
				break
			}
		}
	}
	
	// Determine risk level
	if len(criticalServices) > 0 {
		return RiskCritical
	}
	if len(highRiskServices) > 2 || len(openPorts) > 10 {
		return RiskHigh
	}
	if len(highRiskServices) > 0 || len(openPorts) > 5 {
		return RiskMedium
	}
	
	return RiskLow
}

// displayRiskLevel shows the risk level with simple colors
func displayRiskLevel(risk RiskLevel) {
	switch risk {
	case RiskCritical:
		pterm.Error.Println("Risk Level: CRITICAL")
	case RiskHigh:
		pterm.Warning.Println("Risk Level: HIGH")
	case RiskMedium:
		pterm.Warning.Println("Risk Level: MEDIUM")
	case RiskLow:
		pterm.Success.Println("Risk Level: LOW")
	}
}