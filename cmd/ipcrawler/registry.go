package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/neur0map/ipcrawler/internal/registry"
	"github.com/neur0map/ipcrawler/internal/registry/scanners"
)

// Registry CLI functions - called when registry command line flags are detected

func runRegistryCommand(args []string) error {
	if len(args) < 2 {
		printRegistryUsage()
		return nil
	}

	command := args[1]
	commandArgs := args[2:]

	switch command {
	case "list":
		return runRegistryList(commandArgs)
	case "search":
		return runRegistrySearch(commandArgs)
	case "show":
		return runRegistryShow(commandArgs)
	case "stats":
		return runRegistryStats(commandArgs)
	case "validate":
		return runRegistryValidate(commandArgs)
	case "scan":
		return runRegistryScan(commandArgs)
	case "export":
		return runRegistryExport(commandArgs)
	default:
		fmt.Printf("Unknown registry command: %s\n\n", command)
		printRegistryUsage()
		return fmt.Errorf("unknown command: %s", command)
	}
}

func printRegistryUsage() {
	fmt.Println("IPCrawler Variable Registry Operations")
	fmt.Println("=====================================")
	fmt.Println()
	fmt.Println("Usage: ipcrawler registry <command> [options]")
	fmt.Println()
	fmt.Println("Available Commands:")
	fmt.Println("  list      List variables in the registry")
	fmt.Println("  search    Search for variables by name, description, or tags")
	fmt.Println("  show      Show detailed information about a variable")
	fmt.Println("  stats     Show registry statistics and summary")
	fmt.Println("  validate  Validate registry for issues and inconsistencies")
	fmt.Println("  scan      Scan project files for variables and auto-register them")
	fmt.Println("  export    Export registry database in specified format")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  ipcrawler registry list")
	fmt.Println("  ipcrawler registry search \"port\"")
	fmt.Println("  ipcrawler registry show \"{{target}}\"")
	fmt.Println("  ipcrawler registry stats")
	fmt.Println("  ipcrawler registry scan")
}

func runRegistryList(args []string) error {
	// Parse flags
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	var (
		varType    = fs.String("type", "", "Filter by variable type")
		tool       = fs.String("tool", "", "Filter by tool name")
		category   = fs.String("category", "", "Filter by category")
		verbose    = fs.Bool("verbose", false, "Show detailed information")
		help       = fs.Bool("help", false, "Show help")
	)

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *help {
		fmt.Println("List variables in the registry")
		fmt.Println("Usage: ipcrawler registry list [options]")
		fmt.Println("Options:")
		fs.PrintDefaults()
		return nil
	}

	manager, err := getRegistryManager()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	variables := manager.ListVariables()

	// Apply filters
	if *varType != "" {
		variables = filterByType(variables, registry.VariableType(*varType))
	}
	if *tool != "" {
		variables = filterByTool(variables, *tool)
	}
	if *category != "" {
		variables = filterByCategory(variables, registry.VariableCategory(*category))
	}

	if len(variables) == 0 {
		fmt.Println("No variables found matching the criteria.")
		return nil
	}

	if *verbose {
		// Detailed output
		for _, variable := range variables {
			printVariableDetailed(variable)
			fmt.Println(strings.Repeat("-", 80))
		}
	} else {
		// Table output
		printVariableTable(variables)
	}

	return nil
}

func runRegistrySearch(args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: ipcrawler registry search <query>")
		return fmt.Errorf("search query required")
	}

	manager, err := getRegistryManager()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	query := args[0]
	variables := manager.SearchVariables(query)

	if len(variables) == 0 {
		fmt.Printf("No variables found matching '%s'.\n", query)
		return nil
	}

	fmt.Printf("Found %d variables matching '%s':\n\n", len(variables), query)
	printVariableTable(variables)

	return nil
}

func runRegistryShow(args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: ipcrawler registry show <variable_name>")
		return fmt.Errorf("variable name required")
	}

	manager, err := getRegistryManager()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	varName := args[0]
	
	// Ensure variable name has proper format
	if !strings.HasPrefix(varName, "{{") || !strings.HasSuffix(varName, "}}") {
		varName = fmt.Sprintf("{{%s}}", varName)
	}

	variable, err := manager.GetVariable(varName)
	if err != nil {
		return fmt.Errorf("variable not found: %w", err)
	}

	printVariableDetailed(variable)
	return nil
}

func runRegistryStats(args []string) error {
	manager, err := getRegistryManager()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	stats := manager.GetStatistics()
	printRegistryStats(stats)
	return nil
}

func runRegistryValidate(args []string) error {
	manager, err := getRegistryManager()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	issues := manager.ValidateRegistry()

	if len(issues) == 0 {
		fmt.Println("✅ Registry validation passed. No issues found.")
		return nil
	}

	fmt.Printf("❌ Registry validation found %d issues:\n\n", len(issues))
	for i, issue := range issues {
		fmt.Printf("%d. %s\n", i+1, issue)
	}

	return nil
}

func runRegistryScan(args []string) error {
	manager, err := getRegistryManager()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// Determine scan directory
	scanDir := "."
	if len(args) > 0 {
		scanDir = args[0]
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(scanDir)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	fmt.Printf("Scanning project for variables: %s\n", absPath)

	// Create auto-detector
	detector := scanners.NewAutoDetector(manager)

	// Get initial variable count
	initialStats := manager.GetStatistics()
	initialCount := initialStats.TotalVariables

	// Scan the project
	if err := detector.ScanProjectForVariables(absPath); err != nil {
		return fmt.Errorf("failed to scan project: %w", err)
	}

	// Get final variable count
	finalStats := manager.GetStatistics()
	finalCount := finalStats.TotalVariables

	newVariables := finalCount - initialCount
	if newVariables > 0 {
		fmt.Printf("✅ Scan completed. Found and registered %d new variables.\n", newVariables)
		fmt.Printf("Total variables in registry: %d\n", finalCount)
	} else {
		fmt.Println("✅ Scan completed. No new variables found.")
	}

	return nil
}

func runRegistryExport(args []string) error {
	// Parse flags
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	var (
		output = fs.String("output", "", "Output file (default: stdout)")
		format = fs.String("format", "json", "Export format")
		help   = fs.Bool("help", false, "Show help")
	)

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *help {
		fmt.Println("Export registry database")
		fmt.Println("Usage: ipcrawler registry export [options]")
		fmt.Println("Options:")
		fs.PrintDefaults()
		return nil
	}

	manager, err := getRegistryManager()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	data, err := manager.ExportDatabase(*format)
	if err != nil {
		return fmt.Errorf("failed to export registry: %w", err)
	}

	if *output != "" {
		// Write to file
		if err := os.WriteFile(*output, data, 0644); err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
		fmt.Printf("Registry exported to: %s\n", *output)
	} else {
		// Write to stdout
		fmt.Print(string(data))
	}

	return nil
}

// Helper functions

func getRegistryManager() (registry.RegistryManager, error) {
	// Use the same path as the main application
	dbPath := filepath.Join("internal", "registry", "database", "variables.json")
	return registry.NewRegistryManager(dbPath)
}

func filterByType(variables []*registry.VariableRecord, varType registry.VariableType) []*registry.VariableRecord {
	var filtered []*registry.VariableRecord
	for _, v := range variables {
		if v.Type == varType {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

func filterByTool(variables []*registry.VariableRecord, toolName string) []*registry.VariableRecord {
	var filtered []*registry.VariableRecord
	for _, v := range variables {
		if strings.EqualFold(v.ToolName, toolName) {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

func filterByCategory(variables []*registry.VariableRecord, category registry.VariableCategory) []*registry.VariableRecord {
	var filtered []*registry.VariableRecord
	for _, v := range variables {
		if v.Category == category {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

func printVariableTable(variables []*registry.VariableRecord) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tTYPE\tCATEGORY\tTOOL\tUSAGE\tDESCRIPTION")
	fmt.Fprintln(w, "----\t----\t--------\t----\t-----\t-----------")

	for _, variable := range variables {
		description := variable.Description
		if len(description) > 50 {
			description = description[:47] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n",
			variable.Name,
			variable.Type,
			variable.Category,
			variable.ToolName,
			variable.UsageCount,
			description,
		)
	}

	w.Flush()
}

func printVariableDetailed(variable *registry.VariableRecord) {
	fmt.Printf("Variable: %s\n", variable.Name)
	fmt.Printf("Type: %s\n", variable.Type)
	fmt.Printf("Category: %s\n", variable.Category)
	fmt.Printf("Description: %s\n", variable.Description)
	fmt.Printf("Data Type: %s\n", variable.DataType)
	fmt.Printf("Source: %s\n", variable.Source)
	
	if variable.ToolName != "" {
		fmt.Printf("Tool: %s\n", variable.ToolName)
	}
	
	fmt.Printf("Usage Count: %d\n", variable.UsageCount)
	fmt.Printf("Auto-detected: %v\n", variable.AutoDetected)
	fmt.Printf("First Detected: %s\n", variable.FirstDetected.Format("2006-01-02 15:04:05"))
	fmt.Printf("Last Seen: %s\n", variable.LastSeen.Format("2006-01-02 15:04:05"))

	if len(variable.ExampleValues) > 0 {
		fmt.Printf("Example Values: %s\n", strings.Join(variable.ExampleValues, ", "))
	}

	if variable.DefaultValue != "" {
		fmt.Printf("Default Value: %s\n", variable.DefaultValue)
	}

	if len(variable.Tags) > 0 {
		fmt.Printf("Tags: %s\n", strings.Join(variable.Tags, ", "))
	}

	if len(variable.RequiredBy) > 0 {
		fmt.Printf("Required By: %s\n", strings.Join(variable.RequiredBy, ", "))
	}

	if len(variable.DependsOn) > 0 {
		fmt.Printf("Depends On: %s\n", strings.Join(variable.DependsOn, ", "))
	}

	if variable.Deprecated {
		fmt.Printf("⚠️  Deprecated: true\n")
		if variable.ReplacedBy != "" {
			fmt.Printf("Replaced By: %s\n", variable.ReplacedBy)
		}
	}

	if len(variable.UsedIn) > 0 {
		fmt.Printf("\nUsed In:\n")
		for _, usage := range variable.UsedIn {
			fmt.Printf("  • %s: %s", usage.Type, usage.Path)
			if usage.Line > 0 {
				fmt.Printf(":%d", usage.Line)
			}
			if usage.Context != "" {
				fmt.Printf(" (%s)", usage.Context)
			}
			fmt.Println()
		}
	}

	if variable.Notes != "" {
		fmt.Printf("\nNotes: %s\n", variable.Notes)
	}
}

func printRegistryStats(stats registry.RegistryStatistics) {
	fmt.Printf("Registry Statistics\n")
	fmt.Printf("==================\n\n")

	fmt.Printf("Total Variables: %d\n", stats.TotalVariables)
	fmt.Printf("Auto-detected: %d\n", stats.AutoDetectedCount)
	fmt.Printf("Manually Added: %d\n", stats.ManuallyAddedCount)
	fmt.Printf("Deprecated: %d\n", stats.DeprecatedCount)

	fmt.Printf("\nVariables by Type:\n")
	for varType, count := range stats.VariablesByType {
		fmt.Printf("  %s: %d\n", varType, count)
	}

	fmt.Printf("\nVariables by Category:\n")
	for category, count := range stats.VariablesByCategory {
		fmt.Printf("  %s: %d\n", category, count)
	}

	fmt.Printf("\nVariables by Source:\n")
	for source, count := range stats.VariablesBySource {
		fmt.Printf("  %s: %d\n", source, count)
	}

	if len(stats.MostUsedVariables) > 0 {
		fmt.Printf("\nMost Used Variables:\n")
		for i, usage := range stats.MostUsedVariables {
			fmt.Printf("  %d. %s (%d uses)\n", i+1, usage.Name, usage.UsageCount)
		}
	}

	if len(stats.UnusedVariables) > 0 {
		fmt.Printf("\nUnused Variables (%d):\n", len(stats.UnusedVariables))
		for _, name := range stats.UnusedVariables {
			fmt.Printf("  • %s\n", name)
		}
	}
}