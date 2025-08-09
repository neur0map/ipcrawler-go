package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/carlosm/ipcrawler/internal/ui"
)

func main() {
	// Test different terminal sizes
	testSizes := []struct {
		width, height int
		name          string
	}{
		{160, 48, "Extra Large"},
		{120, 30, "Large"},
		{100, 24, "Medium"},
		{80, 20, "Medium Min"},
		{60, 18, "Small"},
		{40, 15, "Small Min"},
	}

	// Allow custom size from command line
	if len(os.Args) >= 3 {
		if width, err1 := strconv.Atoi(os.Args[1]); err1 == nil {
			if height, err2 := strconv.Atoi(os.Args[2]); err2 == nil {
				testSizes = []struct{ width, height int; name string }{
					{width, height, fmt.Sprintf("Custom %dx%d", width, height)},
				}
			}
		}
	}

	fmt.Println("IPCrawler TUI Layout Verification")
	fmt.Println("=================================")

	for _, size := range testSizes {
		fmt.Printf("\nTesting %s (%dx%d):\n", size.name, size.width, size.height)
		
		// Create model
		model := ui.NewModel("test.example.com")
		
		// Simulate WindowSizeMsg
		newModel, _ := model.Update(tea.WindowSizeMsg{
			Width:  size.width,
			Height: size.height,
		})
		
		m := newModel.(ui.Model)
		
		// Generate view
		view := m.View()
		
		// Analyze view
		lines := strings.Split(view, "\n")
		maxLineLength := 0
		violatingLines := 0
		
		for i, line := range lines {
			// Simple length check (ignoring ANSI for now)
			cleanLine := stripBasicANSI(line)
			if len(cleanLine) > size.width {
				if violatingLines < 3 { // Only show first few violations
					fmt.Printf("  Line %d exceeds width (%d > %d): %q\n", 
						i, len(cleanLine), size.width, truncateString(cleanLine, 50))
				}
				violatingLines++
			}
			if len(cleanLine) > maxLineLength {
				maxLineLength = len(cleanLine)
			}
		}
		
		// Height check
		if len(lines) > size.height {
			fmt.Printf("  Height violation: %d lines > %d terminal height\n", len(lines), size.height)
		}
		
		// Summary
		if violatingLines == 0 && len(lines) <= size.height {
			fmt.Printf("  ✓ Layout fits within bounds (max width: %d, height: %d)\n", 
				maxLineLength, len(lines))
		} else {
			fmt.Printf("  ✗ Layout violations: %d width violations, height: %d/%d\n", 
				violatingLines, len(lines), size.height)
		}
	}

	fmt.Println("\nVerification complete.")
}

// Simple ANSI escape sequence removal
func stripBasicANSI(s string) string {
	var result strings.Builder
	inEscape := false
	
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	
	return result.String()
}

// Truncate string for display
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}