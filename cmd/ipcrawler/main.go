package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/carlosm/ipcrawler/internal/config"
	"github.com/carlosm/ipcrawler/internal/registry"
	"github.com/carlosm/ipcrawler/internal/workflow"
	"github.com/spf13/cobra"
)

var (
	cfgFile    string
	workflowID string
	parallel   int
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

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: .ipcrawlerrc.yaml)")
	rootCmd.PersistentFlags().StringVarP(&workflowID, "workflow", "w", "", "specific workflow ID(s) to run (comma-separated: basic,dns)")
	rootCmd.PersistentFlags().IntVarP(&parallel, "parallel", "p", 0, "max concurrent workflows (default from config)")

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

	fmt.Println("Loading tools...")
	if err := registry.LoadAllTools(); err != nil {
		return fmt.Errorf("loading tools: %w", err)
	}

	tools := registry.ListTools()
	if len(tools) > 0 {
		fmt.Printf("Available tools: %v\n\n", tools)
	} else {
		fmt.Println("Warning: No tools found in tools/ directory\n")
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
	fmt.Println("=" + "="*50)
	
	executor := workflow.NewExecutor(cfg.MaxConcurrentWorkflows)
	ctx := context.Background()
	
	if err := executor.RunWorkflows(ctx, selectedWorkflows); err != nil {
		return fmt.Errorf("executing workflows: %w", err)
	}

	fmt.Println("="*50)
	fmt.Println("\nAll workflows completed successfully!")
	fmt.Printf("Results saved to: %s/%s/\n", cfg.DefaultOutputDir, target)
	fmt.Printf("Reports available in: %s/%s/\n", cfg.DefaultReportDir, target)

	return nil
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadGlobalConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

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