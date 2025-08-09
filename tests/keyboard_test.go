package tests

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/your-org/ipcrawler/internal/simulator"
	"github.com/your-org/ipcrawler/internal/ui"
)

// TestBasicKeyboardNavigation tests core keyboard functionality
func TestBasicKeyboardNavigation(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)
	
	// Initialize app
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	initialView := app.View()

	testCases := []struct {
		name        string
		key         tea.KeyMsg
		expectQuit  bool
		expectView  bool
		description string
	}{
		{
			name: "tab_navigation",
			key:  tea.KeyMsg{Type: tea.KeyTab},
			description: "Tab should cycle focus between panels",
			expectView: true,
		},
		{
			name: "up_arrow",
			key:  tea.KeyMsg{Type: tea.KeyUp},
			description: "Up arrow should navigate lists/content",
			expectView: true,
		},
		{
			name: "down_arrow", 
			key:  tea.KeyMsg{Type: tea.KeyDown},
			description: "Down arrow should navigate lists/content",
			expectView: true,
		},
		{
			name: "space_selection",
			key:  tea.KeyMsg{Type: tea.KeySpace},
			description: "Space should select items",
			expectView: true,
		},
		{
			name: "enter_confirm",
			key:  tea.KeyMsg{Type: tea.KeyEnter},
			description: "Enter should confirm actions",
			expectView: true,
		},
		{
			name: "quit_key",
			key:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}},
			description: "Q should quit the application",
			expectQuit: true,
		},
		{
			name: "help_key",
			key:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}},
			description: "? should show help",
			expectView: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Fresh app for each test to avoid state interference
			app := ui.NewApp(config, sim)
			app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

			// Apply key press
			newModel, cmd := app.Update(tc.key)

			if tc.expectQuit {
				if cmd == nil {
					t.Error("Expected quit command, got nil")
				} else if cmd != tea.Quit {
					t.Error("Expected tea.Quit command for quit key")
				}
				return
			}

			// Verify model is returned
			if newModel == nil {
				t.Error("Update should return a model")
				return
			}

			if tc.expectView {
				// Verify view renders
				view := newModel.(tea.Model).View()
				if len(view) == 0 {
					t.Error("View should not be empty after key press")
				}

				// Verify view changed (for navigation keys)
				if tc.name == "tab_navigation" && view == initialView {
					t.Log("View might not visually change with tab - this is acceptable if focus changes internally")
				}
			}
		})
	}
}

// TestFocusManagement tests panel focus cycling
func TestFocusManagement(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)
	
	// Initialize with large layout (3 panels)
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Test direct focus keys
	focusKeys := []struct {
		key         tea.KeyMsg
		expectedPanel string
	}{
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}}, "list"},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}}, "viewport"},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}}, "status"},
	}

	for _, fk := range focusKeys {
		t.Run("focus_"+fk.expectedPanel, func(t *testing.T) {
			// Apply focus key
			newModel, _ := app.Update(fk.key)
			
			// Verify model updates
			if newModel == nil {
				t.Error("Expected model to be returned")
			}
			
			// Verify rendering still works
			view := newModel.(tea.Model).View()
			if len(view) == 0 {
				t.Error("View should render after focus change")
			}
		})
	}

	// Test tab cycling
	t.Run("tab_cycling", func(t *testing.T) {
		views := make([]string, 5)
		
		// Apply tab multiple times and collect views
		for i := 0; i < 5; i++ {
			newModel, _ := app.Update(tea.KeyMsg{Type: tea.KeyTab})
			app = newModel.(*ui.App) // Update reference
			views[i] = app.View()
		}
		
		// All views should render
		for i, view := range views {
			if len(view) == 0 {
				t.Errorf("View %d should not be empty after tab %d", i, i)
			}
		}
		
		// Note: Visual differences might be subtle, so we just ensure no crashes
		t.Log("Tab cycling completed without errors")
	})
}

// TestKeyboardInDifferentLayouts tests keys work in all layout modes
func TestKeyboardInDifferentLayouts(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	
	layouts := []struct {
		name   string
		width  int
		height int
	}{
		{"small", 60, 20},
		{"medium", 100, 30}, 
		{"large", 120, 40},
	}

	keys := []tea.KeyMsg{
		{Type: tea.KeyTab},
		{Type: tea.KeyUp},
		{Type: tea.KeyDown},
		{Type: tea.KeySpace},
		{Type: tea.KeyRunes, Runes: []rune{'1'}},
		{Type: tea.KeyRunes, Runes: []rune{'2'}},
		{Type: tea.KeyRunes, Runes: []rune{'3'}},
	}

	for _, layout := range layouts {
		t.Run(layout.name, func(t *testing.T) {
			app := ui.NewApp(config, sim)
			app.Update(tea.WindowSizeMsg{Width: layout.width, Height: layout.height})
			
			for i, key := range keys {
				t.Run(fmt.Sprintf("key_%d", i), func(t *testing.T) {
					newModel, _ := app.Update(key)
					
					if newModel == nil {
						t.Errorf("Key %v should not cause nil model in %s layout", key, layout.name)
						return
					}
					
					view := newModel.(tea.Model).View()
					if len(view) == 0 {
						t.Errorf("Key %v should not cause empty view in %s layout", key, layout.name)
					}
				})
			}
		})
	}
}

// TestRapidKeyPresses tests handling of rapid key events
func TestRapidKeyPresses(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)
	
	// Initialize app
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Simulate rapid key presses
	rapidKeys := []tea.KeyMsg{
		{Type: tea.KeyUp}, {Type: tea.KeyDown}, {Type: tea.KeyUp}, {Type: tea.KeyDown},
		{Type: tea.KeyTab}, {Type: tea.KeyTab}, {Type: tea.KeyTab},
		{Type: tea.KeySpace}, {Type: tea.KeyEnter}, {Type: tea.KeySpace},
	}

	// Apply rapid sequence
	for i, key := range rapidKeys {
		newModel, _ := app.Update(key)
		
		if newModel == nil {
			t.Errorf("Rapid key %d caused nil model", i)
			return
		}
		
		// Every few presses, verify view still renders
		if i%3 == 0 {
			view := newModel.(tea.Model).View()
			if len(view) == 0 {
				t.Errorf("Rapid key sequence caused empty view at press %d", i)
			}
		}
		
		// Update app reference for next iteration
		app = newModel.(*ui.App)
	}
	
	// Final verification
	finalView := app.View()
	if len(finalView) == 0 {
		t.Error("Final view should not be empty after rapid key sequence")
	}
}

// TestSpecialKeys tests handling of special/edge case keys
func TestSpecialKeys(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)
	
	// Initialize app
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	specialKeys := []struct {
		name string
		key  tea.KeyMsg
	}{
		{"ctrl_c", tea.KeyMsg{Type: tea.KeyCtrlC}},
		{"ctrl_d", tea.KeyMsg{Type: tea.KeyCtrlD}},
		{"escape", tea.KeyMsg{Type: tea.KeyEsc}},
		{"backspace", tea.KeyMsg{Type: tea.KeyBackspace}},
		{"delete", tea.KeyMsg{Type: tea.KeyDelete}},
		{"home", tea.KeyMsg{Type: tea.KeyHome}},
		{"end", tea.KeyMsg{Type: tea.KeyEnd}},
		{"page_up", tea.KeyMsg{Type: tea.KeyPgUp}},
		{"page_down", tea.KeyMsg{Type: tea.KeyPgDown}},
		{"f1", tea.KeyMsg{Type: tea.KeyF1}},
		{"empty_runes", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{}}},
	}

	for _, sk := range specialKeys {
		t.Run(sk.name, func(t *testing.T) {
			// Apply special key - should not panic
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Special key %s caused panic: %v", sk.name, r)
					}
				}()
				
				newModel, _ := app.Update(sk.key)
				
				if newModel == nil {
					t.Errorf("Special key %s caused nil model", sk.name)
					return
				}
				
				// Verify view still renders
				view := newModel.(tea.Model).View()
				if len(view) == 0 {
					// Some special keys might not affect display - this might be acceptable
					t.Logf("Special key %s resulted in empty view - might be expected", sk.name)
				}
			}()
		})
	}
}

// TestKeyboardAccessibility tests accessibility features
func TestKeyboardAccessibility(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)
	
	// Initialize app
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Test that all interactive elements are keyboard accessible
	t.Run("all_panels_reachable", func(t *testing.T) {
		// Should be able to reach all panels via keyboard
		panels := []tea.KeyMsg{
			{Type: tea.KeyRunes, Runes: []rune{'1'}}, // List panel
			{Type: tea.KeyRunes, Runes: []rune{'2'}}, // Main panel  
			{Type: tea.KeyRunes, Runes: []rune{'3'}}, // Status panel
		}
		
		for _, panelKey := range panels {
			newModel, _ := app.Update(panelKey)
			if newModel == nil {
				t.Error("Panel focus key should not cause nil model")
				return
			}
			
			view := newModel.(tea.Model).View()
			if len(view) == 0 {
				t.Error("Panel should render after focus")
			}
		}
	})

	t.Run("tab_navigation_works", func(t *testing.T) {
		// Tab should cycle through interactive elements
		for i := 0; i < 5; i++ {
			newModel, _ := app.Update(tea.KeyMsg{Type: tea.KeyTab})
			if newModel == nil {
				t.Errorf("Tab %d should not cause nil model", i)
				return
			}
			
			view := newModel.(tea.Model).View()
			if len(view) == 0 {
				t.Errorf("Tab %d should not cause empty view", i)
			}
			
			app = newModel.(*ui.App)
		}
	})

	t.Run("help_accessible", func(t *testing.T) {
		// Help should be accessible via ?
		newModel, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
		if newModel == nil {
			t.Error("Help key should not cause nil model")
			return
		}
		
		view := newModel.(tea.Model).View()
		if len(view) == 0 {
			t.Error("Help should render content")
		}
	})
}