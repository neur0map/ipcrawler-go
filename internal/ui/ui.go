package ui

import (
	"os"
	"time"

	"github.com/carlosm/ipcrawler/internal/term"
)

// NOTE: The main TUI entry point is through Monitor (monitor.go)
// which is used by main.go. This file only contains utility functions.

// shouldRunDemo checks if demo mode is enabled
func shouldRunDemo() bool {
	demo := os.Getenv("IPCRAWLER_DEMO")
	return demo != ""
}

// runPlainOutput runs in plain text mode for non-TTY environments
func runPlainOutput(target string, workflows interface{}) error {
	options := term.GetFallbackOptions()
	
	// Print header
	if options.UseColor {
		println("\033[1mIPCrawler - Target:", target, "\033[0m")
	} else {
		println("IPCrawler - Target:", target)
	}
	
	// Simulate some basic output
	println("Starting workflows...")
	
	// If demo mode, run a quick simulation
	if shouldRunDemo() {
		return runPlainDemo(target, options)
	}
	
	// Otherwise, run actual workflows (placeholder)
	return runActualWorkflows(target, workflows, options)
}

// runPlainDemo runs a demonstration in plain text mode
func runPlainDemo(target string, options term.FallbackOptions) error {
	workflows := []struct {
		name     string
		duration time.Duration
	}{
		{"DNS Discovery", 3 * time.Second},
		{"Port Scanning", 5 * time.Second},
		{"VHost Discovery", 4 * time.Second},
	}
	
	for _, wf := range workflows {
		status := term.FormatStatus("running", options)
		println(status, "Starting", wf.name, "...")
		
		// Simulate progress
		for i := 0; i <= 10; i++ {
			progress := float64(i) / 10.0
			progressBar := term.FormatProgress(progress, 20, options)
			print("\r", wf.name, progressBar)
			time.Sleep(wf.duration / 10)
		}
		
		status = term.FormatStatus("completed", options)
		println("\n" + status + " " + wf.name + " completed")
	}
	
	println("\nAll workflows completed successfully!")
	return nil
}

// runActualWorkflows runs real workflows in plain text mode
func runActualWorkflows(target string, workflows interface{}, options term.FallbackOptions) error {
	// Placeholder for actual workflow execution
	println("Running actual workflows...")
	println("(Integration with existing workflow system would go here)")
	
	// For now, just simulate successful completion
	status := term.FormatStatus("completed", options)
	println(status, "All workflows completed")
	
	return nil
}

// GetTerminalInfo returns information about the current terminal
func GetTerminalInfo() term.TTYInfo {
	return term.GetTTYInfo()
}

// IsInteractiveMode returns true if the application should run in interactive mode
func IsInteractiveMode() bool {
	return term.ShouldUseTUI()
}

// FormatForTerminal formats text appropriately for the current terminal
func FormatForTerminal(text string) string {
	options := term.GetFallbackOptions()
	
	if !options.UseColor {
		// Strip any existing ANSI codes
		return stripANSI(text)
	}
	
	return text
}

// stripANSI removes ANSI escape sequences from text
func stripANSI(text string) string {
	// Simple ANSI stripping (basic implementation)
	result := ""
	inEscape := false
	
	for _, char := range text {
		if char == '\033' { // Start of ANSI escape
			inEscape = true
			continue
		}
		
		if inEscape {
			if char == 'm' { // End of color escape
				inEscape = false
			}
			continue
		}
		
		result += string(char)
	}
	
	return result
}