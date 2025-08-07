package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ipcrawler/internal/logging"
	"ipcrawler/internal/reporting"
	"ipcrawler/internal/scanners"
	"ipcrawler/internal/templates"
	"ipcrawler/internal/ui"
	"ipcrawler/internal/utils"

	"github.com/urfave/cli/v2"
)

const appName = "ipcrawler"

func Execute() error {
	app := &cli.App{
		Name:    appName,
		Version: "2.0.0",
		Usage:   "Modern IP and domain reconnaissance tool",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "template",
				Aliases: []string{"t"},
				Value:   "basic",
				Usage:   "Template to use for scanning (basic, custom)",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Output directory (default: auto-generated workspace)",
			},
			&cli.IntFlag{
				Name:    "rate",
				Aliases: []string{"r"},
				Value:   15000,
				Usage:   "Naabu scan rate (packets per second)",
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
				listTemplates()
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

			// Validate target
			if !utils.IsValidTarget(target) {
				return fmt.Errorf("invalid target format: %s", target)
			}

			templateName := c.String("template")
			outputDir := c.String("output")
			naabuRate := c.Int("rate")
			debugMode := c.Bool("debug")
			jsonOutput := c.Bool("json")

			// Show banner in interactive mode
			if !jsonOutput && !debugMode {
				ui.Global.Banners.ShowApplicationBanner("2.0.0", target, templateName)
			}

			// Set up clean cancellation with ctrl+c
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Set up signal handling for graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

			go func() {
				sig := <-sigChan
				ui.Global.Messages.DisableOutput()
				cancel()

				fmt.Fprintf(os.Stderr, "\033[2K\r")
				fmt.Fprintf(os.Stderr, "\n⚠️  Received signal: %v\n", sig)
				fmt.Fprintf(os.Stderr, "🛑 Stopping all operations...\n")
			}()

			// Initialize error logger
			if err := logging.Initialize("."); err != nil && debugMode {
				fmt.Printf("Warning: Failed to initialize error logger: %v\n", err)
			}
			defer func() {
				if logging.GlobalErrorLogger != nil {
					logging.GlobalErrorLogger.Close()
				}
			}()

			// Get template from registry
			template, exists := templates.GlobalRegistry.Get(templateName)
			if !exists {
				return fmt.Errorf("template '%s' not found", templateName)
			}

			// Create workspace
			var workspaceDir string
			if outputDir != "" {
				workspaceDir = outputDir
			} else {
				var err error
				workspaceDir, err = reporting.GlobalWorkspace.CreateWorkspace(target)
				if err != nil {
					return fmt.Errorf("failed to create workspace: %w", err)
				}
			}

			if debugMode {
				fmt.Printf("📁 Workspace: %s\n", workspaceDir)
			}

			// Prepare template options
			opts := templates.TemplateOptions{
				Workspace:     workspaceDir,
				MaxConcurrent: 3,
				Timeout:       300 * time.Second,
				ReportDir:     reporting.GlobalWorkspace.GetReportDir(workspaceDir),
				Debug:         debugMode,
				Silent:        jsonOutput,
				NaabuRate:     naabuRate,
			}

			// Show scan preview
			if !jsonOutput && !debugMode {
				fmt.Printf("🎯 Target: %s\n", target)
				fmt.Printf("📋 Template: %s - %s\n", template.Name(), template.Description())
				fmt.Printf("📁 Output: %s\n", workspaceDir)
				fmt.Printf("⚡ Rate: %d packets/sec\n", naabuRate)
				fmt.Printf("\n🚀 Starting scan...\n\n")
			}

			// Execute template
			result, err := template.Execute(ctx, target, opts)
			if err != nil {
				// Check if it was user cancellation
				if ctx.Err() != nil {
					ui.Global.Messages.ScanCancelled()
					return nil
				}
				return fmt.Errorf("scan execution failed: %w", err)
			}

			// Generate reports
			if err := generateReports(result, workspaceDir, jsonOutput); err != nil && debugMode {
				fmt.Printf("Warning: Failed to generate reports: %v\n", err)
			}

			// Show results
			if !jsonOutput {
				showScanResults(result)
				ui.Global.Messages.ScanCompleted(target)
				ui.Global.Messages.ResultsSaved()
			} else {
				// JSON output
				jsonFile := fmt.Sprintf("%s/scan-results.json", workspaceDir)
				if err := reporting.GlobalReports.GenerateJSONReport(result.ScanResults, jsonFile); err == nil {
					fmt.Printf("%s\n", jsonFile)
				}
			}

			return nil
		},
	}

	return app.Run(os.Args)
}

// runHealthCheck performs a system health check
func runHealthCheck() {
	ui.Global.Messages.SystemHealthOK()
	ui.Global.Messages.SystemVersion("2.0.0")

	// Check if running as root
	if utils.IsRunningAsRoot() {
		ui.Global.Messages.RunningWithRootPrivileges()
	}

	// Check registered scanners
	scannerNames := scanners.GlobalRegistry.List()
	fmt.Printf("📋 Registered scanners: %d\n", len(scannerNames))
	for _, name := range scannerNames {
		fmt.Printf("  ✅ %s\n", name)
	}

	// Check registered templates
	templateNames := templates.GlobalRegistry.List()
	fmt.Printf("📄 Registered templates: %d\n", len(templateNames))
	for _, name := range templateNames {
		fmt.Printf("  ✅ %s\n", name)
	}

	ui.Global.Messages.AllSystemsOperational()
}

// listTemplates lists available templates
func listTemplates() {
	ui.Global.Messages.AvailableTemplates()
	
	templateNames := templates.GlobalRegistry.List()
	for _, name := range templateNames {
		template, _ := templates.GlobalRegistry.Get(name)
		if name == "basic" {
			ui.Global.Messages.DefaultTemplate(fmt.Sprintf("%s - %s", name, template.Description()))
		} else {
			ui.Global.Messages.Template(fmt.Sprintf("%s - %s", name, template.Description()))
		}
	}
}

// generateReports creates comprehensive scan reports
func generateReports(result *templates.TemplateResult, workspaceDir string, jsonMode bool) error {
	if jsonMode {
		// Only generate JSON report in JSON mode
		jsonFile := fmt.Sprintf("%s/scan-results.json", workspaceDir)
		return reporting.GlobalReports.GenerateJSONReport(result.ScanResults, jsonFile)
	}

	// Generate all report types
	reportDir := reporting.GlobalWorkspace.GetReportDir(workspaceDir)
	
	// Summary report
	summaryFile := fmt.Sprintf("%s/scan-summary.txt", reportDir)
	if err := reporting.GlobalReports.GenerateSummary(result.ScanResults, summaryFile); err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}
	
	// Detailed report
	detailsFile := fmt.Sprintf("%s/scan-details.txt", reportDir)
	if err := reporting.GlobalReports.GenerateDetailedReport(result.ScanResults, detailsFile); err != nil {
		return fmt.Errorf("failed to generate detailed report: %w", err)
	}
	
	// JSON report
	jsonFile := fmt.Sprintf("%s/scan-results.json", reportDir)
	if err := reporting.GlobalReports.GenerateJSONReport(result.ScanResults, jsonFile); err != nil {
		return fmt.Errorf("failed to generate JSON report: %w", err)
	}

	// Combine ports from naabu instances
	portsFile := fmt.Sprintf("%s/all-ports.txt", reportDir)
	portFiles := []string{
		fmt.Sprintf("%s/ports-1.txt", reportDir),
		fmt.Sprintf("%s/ports-2.txt", reportDir),
	}
	if err := reporting.GlobalPorts.CombinePorts(portsFile, portFiles...); err != nil && len(result.ScanResults) > 1 {
		return fmt.Errorf("failed to combine ports: %w", err)
	}
	
	return nil
}

// showScanResults displays scan results summary
func showScanResults(result *templates.TemplateResult) {
	fmt.Printf("\n📊 Scan Results Summary\n")
	fmt.Printf("=======================\n")
	fmt.Printf("🎯 Target: %s\n", result.Target)
	fmt.Printf("⏱️  Duration: %v\n", result.Duration.Round(time.Millisecond))
	fmt.Printf("✅ Success: %t\n", result.Success)
	fmt.Printf("📋 Total Ports: %d\n", result.Summary.TotalPorts)
	fmt.Printf("🔓 Open Ports: %d\n", result.Summary.OpenPorts)
	fmt.Printf("🔍 Services: %d\n", result.Summary.Services)
	fmt.Printf("🌐 DNS Resolved: %t\n", result.Summary.DNSResolved)
	
	if len(result.Errors) > 0 {
		fmt.Printf("⚠️  Errors: %d\n", len(result.Errors))
	}
	
	fmt.Printf("\n📈 Scanner Performance:\n")
	for scanner, stat := range result.Summary.ScannerStats {
		status := "✅"
		if !stat.Success {
			status = "❌"
		}
		fmt.Printf("  %s %-10s: %v (%d results)\n", 
			status, scanner, stat.Duration.Round(time.Millisecond), stat.Results)
	}
	fmt.Printf("\n")
}