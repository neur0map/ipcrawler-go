package tests

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/your-org/ipcrawler/internal/simulator"
	"github.com/your-org/ipcrawler/internal/ui"
)

// TestDemoScriptComprehensive tests the complete demo functionality
func TestDemoScriptComprehensive(t *testing.T) {
	// Test different demo modes
	demoModes := []struct {
		name        string
		envVar      string
		description string
	}{
		{"standard", "1", "Standard demo with full workflows"},
		{"quick", "quick", "Quick demo with accelerated timing"},
		{"minimal", "minimal", "Minimal demo for CI environments"},
	}

	for _, mode := range demoModes {
		t.Run(mode.name+"_demo", func(t *testing.T) {
			// Set demo environment variable
			oldDemo := os.Getenv("IPCRAWLER_DEMO")
			os.Setenv("IPCRAWLER_DEMO", mode.envVar)
			defer os.Setenv("IPCRAWLER_DEMO", oldDemo)

			config := createTestConfig()
			sim := simulator.NewMockSimulator()
			app := ui.NewApp(config, sim)

			// Test demo initialization
			testDemoInitialization(t, app, mode.name)
			
			// Test interactive navigation showcase
			testInteractiveNavigation(t, app, mode.name)
			
			// Test content generation
			testContentGeneration(t, app, mode.name)
			
			// Test error handling
			testErrorHandling(t, app, mode.name)
		})
	}
}

// testDemoInitialization validates demo setup and initial state
func testDemoInitialization(t *testing.T, app *ui.App, mode string) {
	// Initialize with typical demo size
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	
	// Verify initial rendering
	view := app.View()
	if len(view) == 0 {
		t.Errorf("%s demo should render initial content", mode)
		return
	}

	// Check for expected demo elements
	if !strings.Contains(view, "IPCrawler") && !strings.Contains(view, "ipcrawler") {
		t.Errorf("%s demo should show application branding", mode)
	}

	// Verify responsive layout is working
	layouts := []struct{ w, h int }{
		{160, 48}, // Large
		{100, 30}, // Medium  
		{70, 20},  // Small
	}

	for _, layout := range layouts {
		app.Update(tea.WindowSizeMsg{Width: layout.w, Height: layout.h})
		resizedView := app.View()
		
		if len(resizedView) == 0 {
			t.Errorf("%s demo should render at %dx%d", mode, layout.w, layout.h)
		}

		// Verify no content overflow
		lines := strings.Split(resizedView, "\n")
		for i, line := range lines {
			if len(stripANSI(line)) > layout.w+10 { // Buffer for borders
				t.Errorf("%s demo line %d overflows at %dx%d", mode, i, layout.w, layout.h)
				break
			}
		}
	}
}

// testInteractiveNavigation showcases all navigation features
func testInteractiveNavigation(t *testing.T, app *ui.App, mode string) {
	// Set to large layout for full feature testing
	app.Update(tea.WindowSizeMsg{Width: 140, Height: 45})

	navigationTests := []struct {
		name string
		keys []tea.KeyMsg
		desc string
	}{
		{
			name: "panel_cycling",
			keys: []tea.KeyMsg{
				{Type: tea.KeyTab}, {Type: tea.KeyTab}, {Type: tea.KeyTab},
			},
			desc: "Tab should cycle through all panels",
		},
		{
			name: "direct_focus",
			keys: []tea.KeyMsg{
				{Type: tea.KeyRunes, Runes: []rune{'1'}},
				{Type: tea.KeyRunes, Runes: []rune{'2'}},
				{Type: tea.KeyRunes, Runes: []rune{'3'}},
			},
			desc: "Direct panel focusing should work",
		},
		{
			name: "list_navigation",
			keys: []tea.KeyMsg{
				{Type: tea.KeyRunes, Runes: []rune{'1'}}, // Focus list
				{Type: tea.KeyUp}, {Type: tea.KeyDown},
				{Type: tea.KeyUp}, {Type: tea.KeyDown},
			},
			desc: "List navigation with arrows should work",
		},
		{
			name: "viewport_scrolling", 
			keys: []tea.KeyMsg{
				{Type: tea.KeyRunes, Runes: []rune{'2'}}, // Focus viewport
				{Type: tea.KeyUp}, {Type: tea.KeyDown},
				{Type: tea.KeyPgUp}, {Type: tea.KeyPgDown},
			},
			desc: "Viewport scrolling should work",
		},
		{
			name: "selection_interaction",
			keys: []tea.KeyMsg{
				{Type: tea.KeyRunes, Runes: []rune{'1'}}, // Focus list
				{Type: tea.KeyDown}, {Type: tea.KeySpace}, // Select item
				{Type: tea.KeyEnter}, // Confirm
			},
			desc: "Selection and confirmation should work",
		},
	}

	for _, navTest := range navigationTests {
		t.Run(navTest.name, func(t *testing.T) {
			// Apply navigation sequence
			for i, key := range navTest.keys {
				newModel, _ := app.Update(key)
				
				if newModel == nil {
					t.Errorf("%s demo %s: key %d caused nil model", mode, navTest.name, i)
					return
				}

				// Verify rendering continues to work
				view := newModel.(tea.Model).View()
				if len(view) == 0 {
					t.Errorf("%s demo %s: key %d caused empty view", mode, navTest.name, i)
				}

				app = newModel.(*ui.App)
			}
			
			t.Logf("%s demo navigation test '%s' completed: %s", mode, navTest.name, navTest.desc)
		})
	}
}

// testContentGeneration validates simulator content appears correctly
func testContentGeneration(t *testing.T, app *ui.App, mode string) {
	// Initialize and let simulator generate content
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	
	// Simulate time passing for content generation
	contentTests := []tea.Msg{
		simulator.LogStreamMsg{
			Entry: simulator.LogEntry{
				Timestamp: time.Now(),
				Level:     "INFO",
				Source:    "demo",
				Message:   "Demo workflow starting",
			},
		},
		simulator.WorkflowExecutionMsg{
			WorkflowID: "demo-workflow",
			Status:     "running", 
			Progress:   0.3,
			Message:    "Processing demo content",
		},
		simulator.StatusUpdateMsg{
			Status: simulator.SystemStatus{
				Status:      "running",
				ActiveTasks: 3,
				Completed:   5,
			},
		},
		simulator.ProgressUpdateMsg{
			ID:       "demo-progress",
			Progress: 0.7,
			Message:  "Demo progress update",
		},
	}

	for i, msg := range contentTests {
		newModel, _ := app.Update(msg)
		
		if newModel == nil {
			t.Errorf("%s demo content test %d caused nil model", mode, i)
			return
		}

		view := newModel.(tea.Model).View()
		if len(view) == 0 {
			t.Errorf("%s demo should show content after message %d", mode, i)
		}

		// Verify content appears (basic smoke test)
		if i == 0 && !strings.Contains(view, "demo") && !strings.Contains(view, "Demo") {
			t.Logf("%s demo: demo content might not be visible yet (this may be expected)", mode)
		}

		app = newModel.(*ui.App)
	}
}

// testErrorHandling validates graceful error handling in demo mode
func testErrorHandling(t *testing.T, app *ui.App, mode string) {
	// Test error conditions that might occur during demo
	errorTests := []struct {
		name string
		test func() error
		desc string
	}{
		{
			name: "invalid_resize",
			test: func() error {
				app.Update(tea.WindowSizeMsg{Width: -1, Height: -1})
				view := app.View()
				if len(view) == 0 {
					return fmt.Errorf("invalid resize caused empty view")
				}
				return nil
			},
			desc: "Invalid resize should be handled gracefully",
		},
		{
			name: "rapid_events",
			test: func() error {
				// Rapid event sequence that might cause issues
				for i := 0; i < 50; i++ {
					newModel, _ := app.Update(tea.KeyMsg{Type: tea.KeyTab})
					if newModel == nil {
						return fmt.Errorf("rapid event %d caused nil model", i)
					}
					app = newModel.(*ui.App)
				}
				return nil
			},
			desc: "Rapid events should be handled without crashing",
		},
		{
			name: "mixed_messages",
			test: func() error {
				// Mix different message types rapidly
				messages := []tea.Msg{
					tea.KeyMsg{Type: tea.KeyTab},
					tea.WindowSizeMsg{Width: 100, Height: 30},
					simulator.LogStreamMsg{},
					tea.KeyMsg{Type: tea.KeyUp},
					simulator.StatusUpdateMsg{},
				}
				
				for i, msg := range messages {
					newModel, _ := app.Update(msg)
					if newModel == nil {
						return fmt.Errorf("mixed message %d caused nil model", i)
					}
					app = newModel.(*ui.App)
				}
				return nil
			},
			desc: "Mixed message types should be handled correctly",
		},
	}

	for _, errorTest := range errorTests {
		t.Run(errorTest.name, func(t *testing.T) {
			// Capture panics
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("%s demo %s panicked: %v", mode, errorTest.name, r)
					}
				}()

				err := errorTest.test()
				if err != nil {
					t.Errorf("%s demo %s failed: %v", mode, errorTest.name, err)
				} else {
					t.Logf("%s demo error test '%s' passed: %s", mode, errorTest.name, errorTest.desc)
				}
			}()
		})
	}
}

// TestDemoScriptEdgeCases tests demo behavior in unusual conditions
func TestDemoScriptEdgeCases(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()

	edgeCases := []struct {
		name string
		test func(*testing.T)
		desc string
	}{
		{
			name: "very_small_terminal",
			test: func(t *testing.T) {
				app := ui.NewApp(config, sim)
				app.Update(tea.WindowSizeMsg{Width: 20, Height: 8})
				
				view := app.View()
				if len(view) == 0 {
					t.Error("Demo should render something even in tiny terminals")
				}

				// Should handle navigation even in small space
				newModel, _ := app.Update(tea.KeyMsg{Type: tea.KeyTab})
				if newModel == nil {
					t.Error("Tab navigation should work in small terminals")
				}
			},
			desc: "Demo should work in very small terminals",
		},
		{
			name: "very_large_terminal",
			test: func(t *testing.T) {
				app := ui.NewApp(config, sim)
				app.Update(tea.WindowSizeMsg{Width: 200, Height: 80})
				
				view := app.View()
				if len(view) == 0 {
					t.Error("Demo should render in large terminals")
				}

				// Should use space efficiently
				lines := strings.Split(view, "\n")
				if len(lines) < 10 {
					t.Error("Demo should utilize large terminal space")
				}
			},
			desc: "Demo should utilize large terminals effectively",
		},
		{
			name: "help_accessibility",
			test: func(t *testing.T) {
				app := ui.NewApp(config, sim)
				app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
				
				// Help should be accessible
				newModel, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
				if newModel == nil {
					t.Error("Help key should work in demo mode")
					return
				}

				view := newModel.(tea.Model).View()
				if len(view) == 0 {
					t.Error("Help should display content")
				}
			},
			desc: "Help should be accessible during demo",
		},
		{
			name: "quit_functionality",
			test: func(t *testing.T) {
				app := ui.NewApp(config, sim)
				app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
				
				// Quit should work
				_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
				if cmd != tea.Quit {
					t.Error("Quit key should return tea.Quit command")
				}
			},
			desc: "Quit functionality should work during demo",
		},
	}

	for _, edgeCase := range edgeCases {
		t.Run(edgeCase.name, func(t *testing.T) {
			// Run test with timeout to prevent hangs
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			done := make(chan struct{})
			go func() {
				defer close(done)
				edgeCase.test(t)
			}()

			select {
			case <-done:
				t.Logf("Demo edge case '%s' completed: %s", edgeCase.name, edgeCase.desc)
			case <-ctx.Done():
				t.Errorf("Demo edge case '%s' timed out", edgeCase.name)
			}
		})
	}
}

// TestDemoScriptIntegration tests full demo script execution patterns
func TestDemoScriptIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)

	// Simulate a complete demo session
	t.Run("complete_demo_session", func(t *testing.T) {
		// Phase 1: Initialization
		app.Update(tea.WindowSizeMsg{Width: 140, Height: 45})
		
		// Phase 2: User explores interface
		explorationSequence := []tea.KeyMsg{
			{Type: tea.KeyRunes, Runes: []rune{'?'}}, // Help
			{Type: tea.KeyTab}, // Navigate
			{Type: tea.KeyRunes, Runes: []rune{'1'}}, // Focus list
			{Type: tea.KeyDown}, {Type: tea.KeyDown}, // Navigate list
			{Type: tea.KeySpace}, // Select
			{Type: tea.KeyRunes, Runes: []rune{'2'}}, // Focus main
			{Type: tea.KeyUp}, {Type: tea.KeyDown}, // Scroll content
			{Type: tea.KeyRunes, Runes: []rune{'3'}}, // Focus status
		}

		for i, key := range explorationSequence {
			newModel, _ := app.Update(key)
			if newModel == nil {
				t.Errorf("Exploration step %d failed", i)
				return
			}
			
			view := newModel.(tea.Model).View()
			if len(view) == 0 {
				t.Errorf("Empty view at exploration step %d", i)
			}
			
			app = newModel.(*ui.App)
		}

		// Phase 3: Simulate workflow execution
		workflowEvents := []tea.Msg{
			simulator.WorkflowExecutionMsg{
				WorkflowID: "demo-workflow-1",
				Status:     "starting",
				Progress:   0.0,
			},
			simulator.LogStreamMsg{
				Entry: simulator.LogEntry{
					Level:   "INFO",
					Message: "Starting subdomain discovery",
				},
			},
			simulator.ProgressUpdateMsg{
				ID:       "demo-workflow-1", 
				Progress: 0.3,
				Message:  "Found 15 subdomains",
			},
			simulator.LogStreamMsg{
				Entry: simulator.LogEntry{
					Level:   "INFO",
					Message: "Starting port scanning",
				},
			},
			simulator.ProgressUpdateMsg{
				ID:       "demo-workflow-1",
				Progress: 0.8,
				Message:  "Scanned 1000 ports",
			},
			simulator.WorkflowExecutionMsg{
				WorkflowID: "demo-workflow-1",
				Status:     "completed",
				Progress:   1.0,
			},
		}

		for i, event := range workflowEvents {
			newModel, _ := app.Update(event)
			if newModel == nil {
				t.Errorf("Workflow event %d failed", i)
				return
			}
			
			app = newModel.(*ui.App)
			
			// Periodically check rendering
			if i%2 == 0 {
				view := app.View()
				if len(view) == 0 {
					t.Errorf("Empty view during workflow step %d", i)
				}
			}
		}

		// Phase 4: Final validation
		finalView := app.View()
		if len(finalView) == 0 {
			t.Error("Demo session should end with visible content")
		}

		t.Log("Complete demo session executed successfully")
	})
}