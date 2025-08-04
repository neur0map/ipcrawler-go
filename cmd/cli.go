package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ipcrawler/core"

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
			// Handle --health flag
			if c.Bool("health") {
				pterm.Success.Println("System Status: OK")
				pterm.Info.Printf("Version: %s\n", appVersion)
				pterm.Success.Println("All systems operational")
				return nil
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

			// Show interactive scan preview and get sudo choice
			preview := &core.ScanPreview{
				Target:        target,
				Template:      templateName,
				Workflows:     workflowSlice,
				ReportDir:     reportDir,
				EstimatedTime: core.EstimateScanTime(workflowSlice),
			}

			privilegeOption, err := core.ShowScanPreview(preview)
			if err != nil {
				return fmt.Errorf("failed to get user input: %w", err)
			}

			// Create variables for placeholder replacement
			vars := map[string]string{
				"target":      target,
				"template":    templateName,
				"report_dir":  reportDir,
			}

			// Execute workflows with dependency coordination
			if err := executeWorkflowsWithCoordination(workflows, vars, debugMode, reportDir, target, config, privilegeOption.UseSudo); err != nil {
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
func executeWorkflowsWithCoordination(workflows map[string]*core.Workflow, vars map[string]string, debugMode bool, reportDir, target string, config *core.Config, useSudo bool) error {
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
		workflow, exists := workflows[workflowKey]
		if !exists {
			continue // Skip if workflow doesn't exist in template
		}
		
		// Update vars with any provided data from previous workflows
		for key, value := range providedData {
			vars[key] = value
		}
		
		// Special handling for nuclei workflows - convert discovered_ports to target_urls
		if strings.Contains(workflowKey, "nuclei") {
			if discoveredPorts, exists := providedData["discovered_ports"]; exists {
				vars["target_urls"] = core.ConvertPortsToURLs(target, discoveredPorts)
			} else {
				vars["target_urls"] = target
			}
		}
		
		// Replace placeholders
		workflow.ReplaceVars(vars)
		
		var workflowResults *core.ScanResults
		
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
				
				// For nmap, nuclei, and naabu commands, use enhanced execution to get results
				if step.Tool == "nmap" || step.Tool == "nuclei" || step.Tool == "naabu" {
					results, err := core.ExecuteCommandWithRealTimeResults(step.Tool, args, debugMode)
					if err != nil {
						pterm.Error.Printf("  ❌ Error: %v\n", err)
						log.Printf("Command execution failed: %v", err)
					} else {
						pterm.Success.Printf("  ✅ Completed\n")
						workflowResults = results
					}
				} else {
					// Execute the command
					if err := core.ExecuteCommand(step.Tool, args, debugMode); err != nil {
						pterm.Error.Printf("  ❌ Error: %v\n", err)
						log.Printf("Command execution failed: %v", err)
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
			for _, step := range workflow.Steps {
				// Get appropriate args based on sudo preference
				args := step.GetArgs(useSudo)
				
				// For nmap, nuclei, and naabu commands, use enhanced execution to get results
				if step.Tool == "nmap" || step.Tool == "nuclei" || step.Tool == "naabu" {
					results, err := core.ExecuteCommandWithRealTimeResults(step.Tool, args, debugMode)
					if err != nil {
						spinner.Fail("❌ Failed")
						log.Printf("Command execution failed: %v", err)
						goto nextWorkflow
					}
					workflowResults = results
				} else {
					// For non-nmap commands, use regular execution
					if err := core.ExecuteCommand(step.Tool, args, debugMode); err != nil {
						spinner.Fail("❌ Failed")
						log.Printf("Command execution failed: %v", err)
						goto nextWorkflow
					}
				}
			}
			
			spinner.Success("✅ Complete")
			
			// Show intermediate results based on workflow type
			if workflowResults != nil {
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
		
		nextWorkflow:
		
		// Immediate data provision from scan results (before reporting pipeline)
		if workflowResults != nil && workflowResults.ScanType == "port-discovery" && len(workflowResults.Ports) > 0 {
			var openPorts []string
			for _, port := range workflowResults.Ports {
				if port.State == "open" {
					openPorts = append(openPorts, strconv.Itoa(port.Number))
				}
			}
			if len(openPorts) > 0 {
				providedData["discovered_ports"] = strings.Join(openPorts, ",")
				// Always show this info to help with debugging
				pterm.Success.Printf("  📤 Discovered ports: %s\n", providedData["discovered_ports"])
			}
		}
		
		// Handle data provision after workflow completion
		if len(workflow.Provides) > 0 && workflow.HasReporting() {
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
						pterm.Warning.Printf("  ⚠️ Warning: Could not extract provided data - using placeholder\n")
						for _, provided := range workflow.Provides {
							pterm.Info.Printf("  📤 Provides: %s\n", provided)
						}
					}
					for _, provided := range workflow.Provides {
						providedData[provided] = "extracted_by_reporting_pipeline"
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
						pterm.Warning.Printf("  ⚠️ Warning: Could not extract provided data - using placeholder\n")
						for _, provided := range workflow.Provides {
							pterm.Info.Printf("  📤 Provides: %s\n", provided)
						}
					}
					for _, provided := range workflow.Provides {
						providedData[provided] = "extracted_by_reporting_pipeline"
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
		} else if len(workflow.Provides) > 0 {
			// No reporting enabled, try direct extraction
			if debugMode {
				pterm.Info.Printf("  🔧 No reporting enabled, attempting direct extraction...\n")
			}
			extractedData, err := core.ExtractProvidedData(reportDir, workflow.Provides)
			if err != nil {
				if debugMode {
					pterm.Warning.Printf("  ⚠️ Warning: Could not extract provided data - using placeholder\n")
					for _, provided := range workflow.Provides {
						pterm.Info.Printf("  📤 Provides: %s\n", provided)
					}
				}
				for _, provided := range workflow.Provides {
					providedData[provided] = "extracted_by_reporting_pipeline"
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