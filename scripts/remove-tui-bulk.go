package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
)

// remove-tui-bulk.go - Automated TUI removal script
// This script removes all TUI-related code from the IPCrawler codebase

func main() {
	fmt.Println("=== IPCrawler TUI Removal Script ===")
	
	// Confirm operation
	fmt.Print("This will remove ALL TUI code from the codebase. Continue? (yes/no): ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	if strings.ToLower(scanner.Text()) != "yes" {
		fmt.Println("Operation cancelled.")
		return
	}
	
	// Step 1: Remove TUI directories
	fmt.Println("Step 1: Removing TUI directories...")
	if err := removeTUIDirectories(); err != nil {
		log.Fatalf("Failed to remove TUI directories: %v", err)
	}
	
	// Step 2: Remove TUI scripts
	fmt.Println("Step 2: Removing TUI scripts...")
	if err := removeTUIScripts(); err != nil {
		log.Fatalf("Failed to remove TUI scripts: %v", err)
	}
	
	// Step 3: Clean main.go
	fmt.Println("Step 3: Cleaning main.go...")
	if err := cleanMainGo(); err != nil {
		log.Fatalf("Failed to clean main.go: %v", err)
	}
	
	// Step 4: Update go.mod
	fmt.Println("Step 4: Updating go.mod...")
	if err := updateGoMod(); err != nil {
		log.Fatalf("Failed to update go.mod: %v", err)
	}
	
	// Step 5: Update Makefile
	fmt.Println("Step 5: Updating Makefile...")
	if err := updateMakefile(); err != nil {
		log.Fatalf("Failed to update Makefile: %v", err)
	}
	
	// Step 6: Update configs
	fmt.Println("Step 6: Updating configuration files...")
	if err := updateConfigs(); err != nil {
		log.Fatalf("Failed to update configs: %v", err)
	}
	
	fmt.Println("✅ TUI removal completed successfully!")
	fmt.Println("Next steps:")
	fmt.Println("  1. Run 'go mod tidy' to clean dependencies")
	fmt.Println("  2. Run 'make build' to verify compilation")
	fmt.Println("  3. Test with './bin/ipcrawler <target>'")
}

func removeTUIDirectories() error {
	dirsToRemove := []string{
		"internal/tui",
	}
	
	for _, dir := range dirsToRemove {
		if _, err := os.Stat(dir); err == nil {
			fmt.Printf("  Removing directory: %s\n", dir)
			if err := os.RemoveAll(dir); err != nil {
				return fmt.Errorf("failed to remove %s: %v", dir, err)
			}
		} else {
			fmt.Printf("  Directory not found (skipping): %s\n", dir)
		}
	}
	return nil
}

func removeTUIScripts() error {
	scriptsToRemove := []string{
		"scripts/tui-launch-window.sh",
	}
	
	for _, script := range scriptsToRemove {
		if _, err := os.Stat(script); err == nil {
			fmt.Printf("  Removing script: %s\n", script)
			if err := os.Remove(script); err != nil {
				return fmt.Errorf("failed to remove %s: %v", script, err)
			}
		} else {
			fmt.Printf("  Script not found (skipping): %s\n", script)
		}
	}
	return nil
}

func cleanMainGo() error {
	mainFile := "cmd/ipcrawler/main.go"
	
	// Read current main.go
	content, err := os.ReadFile(mainFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %v", mainFile, err)
	}
	
	lines := strings.Split(string(content), "\n")
	var newLines []string
	
	// Track what we're removing
	skipMode := false
	skipFunction := false
	braceCount := 0
	inImports := false
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Skip TUI imports
		if strings.Contains(line, "import (") {
			inImports = true
		}
		if inImports && strings.Contains(line, ")") {
			inImports = false
		}
		
		// Remove specific imports
		if inImports && (strings.Contains(line, "bubbletea") || 
			strings.Contains(line, "lipgloss") || 
			strings.Contains(line, "bubbles") ||
			strings.Contains(line, "go-isatty")) {
			fmt.Printf("  Removing import: %s\n", strings.TrimSpace(line))
			continue
		}
		
		// Remove TUI-related type definitions and variables
		if strings.Contains(line, "lipgloss.Style") ||
			strings.Contains(line, "tea.Model") ||
			strings.Contains(line, "spinner.Model") ||
			strings.Contains(line, "list.Model") ||
			strings.Contains(line, "viewport.Model") {
			fmt.Printf("  Removing type definition: %s\n", strings.TrimSpace(line))
			continue
		}
		
		// Skip entire TUI functions
		if strings.Contains(line, "func runTUI") ||
			strings.Contains(line, "func (m *model)") ||
			strings.Contains(line, "func newModel") ||
			strings.Contains(line, "func (m model) Init") ||
			strings.Contains(line, "func (m model) Update") ||
			strings.Contains(line, "func (m model) View") ||
			strings.Contains(line, "func setupTheme") ||
			strings.Contains(line, "func (m *model) getThemeColor") {
			skipFunction = true
			braceCount = 0
			fmt.Printf("  Removing function: %s\n", strings.TrimSpace(line))
		}
		
		if skipFunction {
			// Count braces to know when function ends
			braceCount += strings.Count(line, "{")
			braceCount -= strings.Count(line, "}")
			
			if braceCount <= 0 && (strings.Contains(line, "}") || trimmed == "}") {
				skipFunction = false
			}
			continue
		}
		
		// Remove model struct definition
		if strings.Contains(line, "type model struct") {
			skipMode = true
			braceCount = 0
			fmt.Printf("  Removing struct: model\n")
		}
		
		if skipMode {
			braceCount += strings.Count(line, "{")
			braceCount -= strings.Count(line, "}")
			
			if braceCount <= 0 && (strings.Contains(line, "}") || trimmed == "}") {
				skipMode = false
			}
			continue
		}
		
		// Remove lines with TUI-specific content
		if strings.Contains(line, "tea.NewProgram") ||
			strings.Contains(line, "lipgloss.") ||
			strings.Contains(line, "bubbles/") ||
			strings.Contains(line, "isatty.IsTerminal") ||
			strings.Contains(line, "runTUI()") {
			fmt.Printf("  Removing TUI line: %s\n", strings.TrimSpace(line))
			continue
		}
		
		// Keep the line
		newLines = append(newLines, line)
	}
	
	// Write the cleaned content
	newContent := strings.Join(newLines, "\n")
	
	// Create backup
	backupFile := mainFile + ".backup"
	if err := os.WriteFile(backupFile, content, 0644); err != nil {
		return fmt.Errorf("failed to create backup: %v", err)
	}
	
	// Write cleaned version
	if err := os.WriteFile(mainFile, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write cleaned file: %v", err)
	}
	
	fmt.Printf("  ✅ Cleaned %s (backup: %s)\n", mainFile, backupFile)
	return nil
}

func updateGoMod() error {
	goModFile := "go.mod"
	
	content, err := os.ReadFile(goModFile)
	if err != nil {
		return fmt.Errorf("failed to read go.mod: %v", err)
	}
	
	lines := strings.Split(string(content), "\n")
	var newLines []string
	
	// Remove TUI dependencies
	tuiDeps := []string{
		"github.com/charmbracelet/bubbles",
		"github.com/charmbracelet/bubbletea", 
		"github.com/charmbracelet/lipgloss",
		"github.com/mattn/go-isatty",
	}
	
	for _, line := range lines {
		skipLine := false
		for _, dep := range tuiDeps {
			if strings.Contains(line, dep) {
				fmt.Printf("  Removing dependency: %s\n", strings.TrimSpace(line))
				skipLine = true
				break
			}
		}
		if !skipLine {
			newLines = append(newLines, line)
		}
	}
	
	newContent := strings.Join(newLines, "\n")
	
	// Create backup
	backupFile := goModFile + ".backup"
	if err := os.WriteFile(backupFile, content, 0644); err != nil {
		return fmt.Errorf("failed to create backup: %v", err)
	}
	
	// Write updated version
	if err := os.WriteFile(goModFile, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write updated go.mod: %v", err)
	}
	
	fmt.Printf("  ✅ Updated %s (backup: %s)\n", goModFile, backupFile)
	return nil
}

func updateMakefile() error {
	makeFile := "Makefile"
	
	content, err := os.ReadFile(makeFile)
	if err != nil {
		return fmt.Errorf("failed to read Makefile: %v", err)
	}
	
	lines := strings.Split(string(content), "\n")
	var newLines []string
	
	for _, line := range lines {
		// Remove TUI-related targets
		if strings.Contains(line, "tui-launch-window.sh") ||
			strings.Contains(line, "tea.NewProgram") {
			fmt.Printf("  Removing Makefile line: %s\n", strings.TrimSpace(line))
			continue
		}
		
		// Update run target to use CLI directly
		if strings.Contains(line, "./scripts/tui-launch-window.sh") {
			newLine := "\t@./bin/ipcrawler $(TARGET)"
			fmt.Printf("  Updating run target: %s -> %s\n", strings.TrimSpace(line), strings.TrimSpace(newLine))
			newLines = append(newLines, newLine)
			continue
		}
		
		newLines = append(newLines, line)
	}
	
	newContent := strings.Join(newLines, "\n")
	
	// Create backup
	backupFile := makeFile + ".backup"
	if err := os.WriteFile(backupFile, content, 0644); err != nil {
		return fmt.Errorf("failed to create backup: %v", err)
	}
	
	// Write updated version
	if err := os.WriteFile(makeFile, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write updated Makefile: %v", err)
	}
	
	fmt.Printf("  ✅ Updated %s (backup: %s)\n", makeFile, backupFile)
	return nil
}

func updateConfigs() error {
	// For now, we'll keep ui.yaml but could strip it down later
	// The configuration system can handle missing UI settings gracefully
	fmt.Println("  ✅ Configuration files preserved (ui.yaml kept for potential CLI settings)")
	return nil
}