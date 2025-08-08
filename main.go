package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/carlosm/ipcrawler/internal/config"
	"github.com/carlosm/ipcrawler/internal/logger"
	"github.com/carlosm/ipcrawler/internal/registry"
	"github.com/carlosm/ipcrawler/internal/report"
	"github.com/carlosm/ipcrawler/internal/tui"
	"github.com/carlosm/ipcrawler/internal/workflow"
	"github.com/spf13/cobra"
)

var (
	cfgFile    string
	workflowID string
	parallel   int
	noTUI      bool
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "ipcrawler [target]",
		Short: "IPCrawler - Config-driven wrapper for external CLI tools",
		Long: `IPCrawler is a scalable, config-driven orchestration tool that wraps
external CLI tools like naabu, nmap, and others without hardcoding tool names.`,
		Args: cobra.ExactArgs(1),
		RunE: runIPCrawler,
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: global.yaml)")
	rootCmd.PersistentFlags().StringVarP(&workflowID, "workflow", "w", "", "specific workflow ID to run")
	rootCmd.PersistentFlags().IntVarP(&parallel, "parallel", "p", 0, "max concurrent workflows (default from config)")
	rootCmd.PersistentFlags().BoolVar(&noTUI, "no-tui", false, "disable TUI monitoring and use plain output")

	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "List available tools and workflows",
		RunE:  runList,
	}

	rootCmd.AddCommand(listCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runIPCrawler(cmd *cobra.Command, args []string) error {
	target := args[0]
	
	// Always show startup message to console first
	fmt.Printf("IPCrawler starting for target: %s\n\n", target)

	cfg, err := config.LoadGlobalConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if parallel > 0 {
		cfg.MaxConcurrentWorkflows = parallel
	}

	fmt.Printf("Configuration:\n")
	fmt.Printf("  Auto-discover workflow folders: true\n")
	fmt.Printf("  Max concurrent workflows: %d\n", cfg.MaxConcurrentWorkflows)
	fmt.Printf("  Output directory: %s\n", cfg.DefaultOutputDir)
	fmt.Printf("  Report directory: %s\n\n", cfg.DefaultReportDir)

	if err := createDirectories(cfg, target); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	fmt.Println("Loading workflows...")
	workflows, err := workflow.LoadWorkflowsAutoDiscover(target)
	if err != nil {
		return fmt.Errorf("loading workflows: %w", err)
	}

	if len(workflows) == 0 {
		return fmt.Errorf("no workflows found in auto-discovered folders")
	}

	var selectedWorkflows []workflow.Workflow
	if workflowID != "" {
		// Support comma-separated workflow IDs
		requestedIDs := strings.Split(strings.TrimSpace(workflowID), ",")
		for i := range requestedIDs {
			requestedIDs[i] = strings.TrimSpace(requestedIDs[i])
		}
		
		for _, reqID := range requestedIDs {
			found := false
			for _, wf := range workflows {
				if wf.ID == reqID {
					selectedWorkflows = append(selectedWorkflows, wf)
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("workflow not found: %s", reqID)
			}
		}
	} else {
		// Auto-run all discovered workflows when no -w flag specified
		selectedWorkflows = workflows
	}

	// Extract tools needed by selected workflows
	requiredTools := extractRequiredTools(selectedWorkflows)

	fmt.Println("Loading tools...")
	if err := registry.LoadSpecificTools(requiredTools); err != nil {
		return fmt.Errorf("loading tools: %w", err)
	}

	tools := registry.ListTools()
	if len(tools) > 0 {
		fmt.Printf("Available tools: %v\n\n", tools)
	} else {
		fmt.Println("Warning: No tools found in tools/ directory\n")
	}

	fmt.Printf("Found %d workflow(s) to execute:\n", len(selectedWorkflows))
	for _, wf := range selectedWorkflows {
		mode := "sequential"
		if wf.Parallel {
			mode = "parallel"
		}
		fmt.Printf("  - %s: %s [%s]\n", wf.ID, wf.Description, mode)
	}
	fmt.Println()

	fmt.Println("Starting workflow execution...")
	fmt.Println(strings.Repeat("━", 51))
	
	executor := workflow.NewExecutor(cfg.MaxConcurrentWorkflows)
	executor.SetTarget(target) // Set the target on the executor
	ctx := context.Background()
	
	// Initialize TUI monitoring if not disabled
	var monitor *tui.Monitor
	if !noTUI {
		monitor = tui.NewMonitor(target)
		cancelCtx, err := monitor.Start(ctx)
		if err != nil {
			fmt.Printf("Warning: Failed to start TUI monitoring: %v\n", err)
			fmt.Println("Continuing with plain output...")
			logger.SetConsoleLogger() // Use console logger if TUI fails
		} else {
			// Use the cancellable context for workflow execution
			ctx = cancelCtx
			// Set the monitor on the executor
			executor.SetMonitor(monitor)
			// Set logger to use TUI
			logger.SetTUILogger(monitor)
			fmt.Println("🎯 TUI monitoring started - enjoy the beautiful interface!")
			time.Sleep(time.Millisecond * 500) // Brief pause to show message
		}
		
		defer func() {
			if monitor != nil {
				monitor.Stop()
			}
		}()
	} else {
		// Explicitly use console logger when TUI is disabled
		logger.SetConsoleLogger()
	}
	
	if err := executor.RunWorkflows(ctx, selectedWorkflows); err != nil {
		return fmt.Errorf("executing workflows: %w", err)
	}

	fmt.Println(strings.Repeat("━", 51))
	fmt.Printf("\n🎉 All workflows completed successfully!\n")
	
	fmt.Println("Generating reports...")
	reportGen := report.NewReportGenerator(target, cfg.DefaultOutputDir, cfg.DefaultReportDir)
	if err := reportGen.GenerateReports(); err != nil {
		fmt.Printf("Warning: Failed to generate reports: %v\n", err)
	}
	
	fmt.Printf("Results saved to: %s/%s/\n", cfg.DefaultOutputDir, target)
	fmt.Printf("Reports available in: %s/%s/\n", cfg.DefaultReportDir, target)

	return nil
}

func runList(cmd *cobra.Command, args []string) error {
	fmt.Println("Loading tools...")
	if err := registry.LoadAllTools(); err != nil {
		return fmt.Errorf("loading tools: %w", err)
	}

	tools := registry.ListTools()
	fmt.Println("\nAvailable tools:")
	if len(tools) == 0 {
		fmt.Println("  (none found)")
	} else {
		for _, tool := range tools {
			fmt.Printf("  - %s\n", tool)
		}
	}

	fmt.Println("\nLoading workflows...")
	workflows, err := workflow.LoadWorkflowsAutoDiscover("example.com")
	if err != nil {
		return fmt.Errorf("loading workflows: %w", err)
	}

	fmt.Println("\nAvailable workflows:")
	if len(workflows) == 0 {
		fmt.Println("  (none found)")
	} else {
		for _, wf := range workflows {
			mode := "sequential"
			if wf.Parallel {
				mode = "parallel"
			}
			fmt.Printf("  - %s: %s [%s]\n", wf.ID, wf.Description, mode)
			for _, step := range wf.Steps {
				deps := ""
				if len(step.DependsOn) > 0 {
					deps = fmt.Sprintf(" (depends on: %v)", step.DependsOn)
				}
				if step.Tool != "" {
					fmt.Printf("      %s: tool=%s%s\n", step.ID, step.Tool, deps)
				} else if step.Type != "" {
					fmt.Printf("      %s: type=%s%s\n", step.ID, step.Type, deps)
				}
			}
		}
	}

	return nil
}

func createDirectories(cfg *config.GlobalConfig, target string) error {
	dirs := []string{
		filepath.Join(cfg.DefaultOutputDir, target),
		filepath.Join(cfg.DefaultReportDir, target),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// extractRequiredTools extracts the unique tool names needed by the selected workflows
func extractRequiredTools(workflows []workflow.Workflow) []string {
	toolSet := make(map[string]bool)
	
	for _, wf := range workflows {
		for _, step := range wf.Steps {
			if step.Tool != "" {
				toolSet[step.Tool] = true
			}
		}
	}
	
	tools := make([]string, 0, len(toolSet))
	for tool := range toolSet {
		tools = append(tools, tool)
	}
	
	return tools
}