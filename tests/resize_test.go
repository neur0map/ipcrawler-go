package tests

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/your-org/ipcrawler/internal/simulator"
	"github.com/your-org/ipcrawler/internal/ui"
)

// TestWindowResizeStability tests that the TUI handles window resizes gracefully
func TestWindowResizeStability(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)

	// Test resize scenarios that commonly cause issues
	resizeSequences := [][]struct{ w, h int }{
		// Gradual resize
		{{80, 24}, {90, 24}, {100, 30}, {110, 35}, {120, 40}},
		// Extreme resize
		{{160, 48}, {60, 15}, {160, 48}, {60, 15}},
		// Layout mode switching
		{{60, 20}, {80, 24}, {120, 40}, {80, 24}, {60, 20}},
		// Rapid size changes
		{{80, 24}, {81, 24}, {80, 24}, {81, 24}, {80, 24}},
	}

	for seqIdx, sequence := range resizeSequences {
		t.Run(fmt.Sprintf("sequence_%d", seqIdx), func(t *testing.T) {
			// Fresh app for each sequence
			app := ui.NewApp(config, sim)
			
			var lastLineCount int
			for stepIdx, size := range sequence {
				// Apply resize
				app.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
				
				// Render and validate
				view := app.View()
				currentLineCount := strings.Count(view, "\n")
				
				// Validate basic invariants
				if len(view) == 0 {
					t.Errorf("Step %d: Empty view after resize to %dx%d", stepIdx, size.w, size.h)
					continue
				}
				
				// Check for line count stability (no growth)
				if stepIdx > 0 && currentLineCount > lastLineCount+5 { // Allow small buffer
					t.Errorf("Step %d: Line count grew significantly: %d -> %d", 
						stepIdx, lastLineCount, currentLineCount)
				}
				
				// Check for content overflow
				lines := strings.Split(view, "\n")
				for lineIdx, line := range lines {
					cleanLine := stripANSI(line)
					if len(cleanLine) > size.w+10 { // Allow small buffer for borders
						t.Errorf("Step %d: Line %d too long: %d > %d", 
							stepIdx, lineIdx, len(cleanLine), size.w+10)
						break
					}
				}
				
				lastLineCount = currentLineCount
			}
		})
	}
}

// TestLayoutModeTransitions tests transitions between layout modes
func TestLayoutModeTransitions(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)

	// Define layout transitions
	transitions := []struct {
		name     string
		from     struct{ w, h int }
		to       struct{ w, h int }
		expected string
	}{
		{"small_to_medium", struct{ w, h int }{60, 20}, struct{ w, h int }{80, 24}, "stable"},
		{"medium_to_large", struct{ w, h int }{80, 24}, struct{ w, h int }{120, 40}, "stable"},
		{"large_to_small", struct{ w, h int }{120, 40}, struct{ w, h int }{60, 20}, "stable"},
		{"small_to_large", struct{ w, h int }{60, 20}, struct{ w, h int }{120, 40}, "stable"},
	}

	for _, transition := range transitions {
		t.Run(transition.name, func(t *testing.T) {
			// Start with initial size
			app.Update(tea.WindowSizeMsg{Width: transition.from.w, Height: transition.from.h})
			viewBefore := app.View()
			linesBefore := strings.Split(viewBefore, "\n")
			
			// Transition to new size
			app.Update(tea.WindowSizeMsg{Width: transition.to.w, Height: transition.to.h})
			viewAfter := app.View()
			linesAfter := strings.Split(viewAfter, "\n")
			
			// Validate transition was handled properly
			if len(viewAfter) == 0 {
				t.Error("View should not be empty after layout transition")
			}
			
			// Check that content adapts reasonably
			if len(linesAfter) == 0 {
				t.Error("No lines rendered after transition")
			}
			
			// Verify no content corruption (no raw control characters)
			for i, line := range linesAfter {
				if strings.Contains(line, "\x00") || strings.Contains(line, "\r") {
					t.Errorf("Line %d contains control characters after transition: %q", i, line)
				}
			}
		})
	}
}

// TestResizeUnderLoad tests resize handling while processing events
func TestResizeUnderLoad(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)

	// Initialize with base size
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Simulate concurrent event processing and resizing
	done := make(chan bool)
	var resizeErrors []string

	// Start resize simulation in background
	go func() {
		defer func() { done <- true }()
		
		sizes := []struct{ w, h int }{
			{80, 24}, {120, 40}, {100, 30}, {160, 48}, {90, 28},
		}
		
		for i := 0; i < 20; i++ {
			size := sizes[i%len(sizes)]
			app.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
			
			// Verify view renders without panic
			view := app.View()
			if len(view) == 0 {
				resizeErrors = append(resizeErrors, fmt.Sprintf("Empty view at iteration %d", i))
			}
			
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Simulate concurrent user events
	go func() {
		defer func() { done <- true }()
		
		keys := []tea.KeyMsg{
			{Type: tea.KeyTab},
			{Type: tea.KeyUp},
			{Type: tea.KeyDown},
			{Type: tea.KeySpace},
		}
		
		for i := 0; i < 50; i++ {
			key := keys[i%len(keys)]
			app.Update(key)
			time.Sleep(5 * time.Millisecond)
		}
	}()

	// Wait for both goroutines with timeout
	completedCount := 0
	timeout := time.After(5 * time.Second)
	
	for completedCount < 2 {
		select {
		case <-done:
			completedCount++
		case <-timeout:
			t.Fatal("Test timed out - possible deadlock or infinite loop")
		}
	}

	// Check for any resize errors
	if len(resizeErrors) > 0 {
		t.Errorf("Resize errors occurred: %v", resizeErrors)
	}

	// Final validation
	finalView := app.View()
	if len(finalView) == 0 {
		t.Error("Final view should not be empty")
	}
}

// TestMinimumSizes tests behavior at minimum viable terminal sizes
func TestMinimumSizes(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	
	// Test extremely small sizes that might cause edge cases
	minSizes := []struct {
		w, h int
		name string
	}{
		{20, 10, "tiny"},
		{40, 15, "narrow"},
		{60, 12, "short"},
		{1, 1, "minimal"},     // Edge case
		{0, 0, "zero"},        // Edge case
		{-1, -1, "negative"},  // Edge case
	}

	for _, size := range minSizes {
		t.Run(size.name, func(t *testing.T) {
			app := ui.NewApp(config, sim)
			
			// This should not panic
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Panic occurred with size %dx%d: %v", size.w, size.h, r)
					}
				}()
				
				app.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
				view := app.View()
				
				// For positive sizes, we should get some output
				if size.w > 0 && size.h > 0 && len(view) == 0 {
					t.Errorf("Expected some output for size %dx%d", size.w, size.h)
				}
			}()
		})
	}
}

// TestResizeMessagePropagation tests that resize messages reach all components
func TestResizeMessagePropagation(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)

	// Apply various sizes and verify components respond
	testSizes := []struct{ w, h int }{
		{80, 24},   // Medium layout
		{120, 40},  // Large layout
		{60, 20},   // Small layout
	}

	for _, size := range testSizes {
		t.Run(fmt.Sprintf("%dx%d", size.w, size.h), func(t *testing.T) {
			// Apply resize
			app.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
			
			// Render and check that all components are present
			view := app.View()
			
			// Basic content validation - all layouts should have some recognizable content
			if len(view) < 50 { // Arbitrary minimum
				t.Errorf("View too short for size %dx%d: %d chars", size.w, size.h, len(view))
			}
			
			// Verify no component is completely missing (basic smoke test)
			// This is layout-dependent but there should be some structured content
			lines := strings.Split(view, "\n")
			if len(lines) < 3 {
				t.Errorf("Too few lines rendered for size %dx%d: %d", size.w, size.h, len(lines))
			}
		})
	}
}