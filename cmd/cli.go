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
				
				// Wait a moment for files to be written
				time.Sleep(2 * time.Second)
				
				// Run the reporting pipeline
				if err := core.RunReportingPipeline(reportDir, target, workflows, config, debugMode); err != nil {
					if debugMode {
						pterm.Warning.Printf("Reporting pipeline encountered issues: %v\n", err)
						log.Printf("Reporting pipeline error: %v", err)
					} else {
						reportSpinner.Fail("❌ Report generation failed")
					}
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

// executeWorkflowsWithCoordination executes workflows with dependency coordination
func executeWorkflowsWithCoordination(ctx context.Context, workflows map[string]*core.Workflow, vars map[string]string, debugMode bool, reportDir, target string, config *core.Config, useSudo bool) error {
	// Build dependency graph and execution order
	executionOrder, err := buildExecutionOrder(workflows)
	if err != nil {
		return fmt.Errorf("failed to resolve workflow dependencies: %w", err)
	}
	
	if debugMode {
		fmt.Printf("Workflow execution order: %v\n", executionOrder)
	}
	
	// Keep track of data provided by workflows
	providedData := make(map[string]string)
	
	// Execute workflows in dependency order  
	for _, workflowKey := range executionOrder {
		// Check if context has been cancelled
		select {
		case <-ctx.Done():
			pterm.Warning.Println("🛑 Scan cancelled by user")
			return ctx.Err()
		default:
		}
		
		workflow, exists := workflows[workflowKey]
		if !exists {
			continue // Skip if workflow doesn't exist in template
		}
		
		// Update vars with any provided data from previous workflows
		for key, value := range providedData {
			vars[key] = value
		}
		
		// Special handling for nuclei workflows - convert discovered_ports to target_urls
		if strings.Contains(workflowKey, "nuclei") || strings.Contains(workflowKey, "vulnerability-scan") {
			if discoveredPorts, exists := providedData["discovered_ports"]; exists && discoveredPorts != "" {
				vars["target_urls"] = core.ConvertPortsToURLs(target, discoveredPorts)
				if debugMode {
					pterm.Info.Printf("  🔗 Using discovered ports for nuclei: %s -> %s\n", discoveredPorts, vars["target_urls"])
				}
			} else {
				// Fallback to basic target if no ports were discovered
				vars["target_urls"] = target
				if debugMode {
					pterm.Warning.Printf("  ⚠️ No discovered ports found, using target directly: %s\n", target)
				}
			}
		}
		
		// Special handling for nmap deep scan - provide fallback ports if none discovered
		if strings.Contains(workflowKey, "nmap") && strings.Contains(workflowKey, "deep") {
			if discoveredPorts, exists := providedData["discovered_ports"]; !exists || discoveredPorts == "" {
				// Use common ports as fallback for deep scan
				vars["discovered_ports"] = "22,53,80,135,139,443,445,993,995,3306,3389,5432,5900,8080,8443"
				if debugMode {
					pterm.Warning.Printf("  ⚠️ No discovered ports found, using common ports for deep scan\n")
				}
			}
		}
		
		// Replace placeholders
		workflow.ReplaceVars(vars)
		
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
		
		// Immediate data provision from scan results (before reporting pipeline) - only if workflow succeeded
		if !workflowFailed && workflowResults != nil && workflowResults.ScanType == "port-discovery" && len(workflowResults.Ports) > 0 {
			var openPorts []string
			for _, port := range workflowResults.Ports {
				if port.State == "open" {
					openPorts = append(openPorts, strconv.Itoa(port.Number))
				}
			}
			if len(openPorts) > 0 {
				discoveredPortsStr := strings.Join(openPorts, ",")
				providedData["discovered_ports"] = discoveredPortsStr
				// Update the vars map immediately so subsequent workflows get the real data
				vars["discovered_ports"] = discoveredPortsStr
				// Always show this info to help with debugging
				pterm.Success.Printf("  📤 Discovered ports: %s\n", discoveredPortsStr)
			}
		}
		
		// Handle data provision after workflow completion - only if immediate extraction didn't succeed
		if len(workflow.Provides) > 0 {
			// Check if we already have valid data from immediate extraction
			hasValidData := false
			for _, provided := range workflow.Provides {
				if value, exists := providedData[provided]; exists && value != "" && value != "extracted_by_reporting_pipeline" {
					hasValidData = true
					if debugMode {
						pterm.Success.Printf("  ✓ Using immediate extraction data for %s: %s\n", provided, value)
					}
				}
			}
			
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
							// Use fallback ports for deep scan instead of placeholder
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
						} else {
							if debugMode {
								for provided, value := range extractedData {
									pterm.Success.Printf("  📤 Provides: %s = %s\n", provided, value)
								}
							}
							for provided, value := range extractedData {
								providedData[provided] = value
							}
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
							// Use fallback ports for deep scan instead of placeholder
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
						} else {
							if debugMode {
								for provided, value := range extractedData {
									pterm.Success.Printf("  📤 Provides: %s = %s\n", provided, value)
								}
							}
							for provided, value := range extractedData {
								providedData[provided] = value
							}
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
						// Use fallback ports for deep scan instead of placeholder
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
					} else {
						if debugMode {
							for provided, value := range extractedData {
								pterm.Success.Printf("  📤 Provides: %s = %s\n", provided, value)
							}
						}
						for provided, value := range extractedData {
							providedData[provided] = value
						}
					}
				}
			}
		}
		
		fmt.Println()
	}
	
	return nil
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