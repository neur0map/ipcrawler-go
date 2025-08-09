package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/carlosm/ipcrawler/internal/ui"
)

func main() {
	fmt.Println("Simple Layout Test")
	fmt.Println("==================")
	
	// Test terminal size
	width := 80
	height := 24
	
	fmt.Printf("Testing %dx%d layout:\n\n", width, height)
	
	// Create model
	model := ui.NewModel("test.example.com")
	
	// Simulate WindowSizeMsg
	newModel, _ := model.Update(tea.WindowSizeMsg{
		Width:  width,
		Height: height,
	})
	
	m := newModel.(ui.Model)
	
	// Test basic Lipgloss layout
	fmt.Println("Testing basic Lipgloss constraints:")
	
	// Create constrained panels
	leftPanel := lipgloss.NewStyle().
		Width(20).
		Height(10).
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Render("Left Panel\nContent")
	
	rightPanel := lipgloss.NewStyle().
		Width(30).
		Height(10).
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Render("Right Panel\nContent")
	
	joined := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	
	lines := strings.Split(joined, "\n")
	fmt.Printf("Joined layout has %d lines\n", len(lines))
	for i, line := range lines {
		fmt.Printf("Line %d (len=%d): %q\n", i, len(line), line)
	}
	
	fmt.Println("\nNow testing actual TUI layout:")
	
	// Generate TUI view
	view := m.View()
	lines = strings.Split(view, "\n")
	
	maxWidth := 0
	violations := 0
	
	for i, line := range lines {
		if len(line) > width {
			if violations < 5 { // Show first few violations
				fmt.Printf("Line %d exceeds width: %d > %d\n", i, len(line), width)
			}
			violations++
		}
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}
	
	fmt.Printf("Total violations: %d\n", violations)
	fmt.Printf("Max line width: %d (should be <= %d)\n", maxWidth, width)
	fmt.Printf("Total lines: %d (should be <= %d)\n", len(lines), height)
}