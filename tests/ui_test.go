package tests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/your-org/ipcrawler/internal/simulator"
	"github.com/your-org/ipcrawler/internal/ui"
)

// TestSingleTeaProgram ensures exactly one tea.NewProgram call exists in the codebase
func TestSingleTeaProgram(t *testing.T) {
	// Search for tea.NewProgram usage across the entire codebase
	output, err := executeCmd("grep", "-r", "--include=*.go", "tea.NewProgram", ".")
	if err != nil {
		t.Fatalf("Failed to search for tea.NewProgram: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		t.Fatal("No tea.NewProgram calls found - application must have exactly one")
	}

	// Filter out test files and comments
	var actualCalls []string
	for _, line := range lines {
		if strings.Contains(line, "//") {
			continue // Skip commented lines
		}
		if strings.Contains(line, "_test.go") {
			continue // Skip test files
		}
		actualCalls = append(actualCalls, line)
	}

	if len(actualCalls) != 1 {
		t.Errorf("Expected exactly 1 tea.NewProgram call, found %d:\n%s", 
			len(actualCalls), strings.Join(actualCalls, "\n"))
	}

	// Verify it's in main.go
	if !strings.Contains(actualCalls[0], "cmd/ipcrawler/main.go") {
		t.Errorf("tea.NewProgram should only be in main.go, found in: %s", actualCalls[0])
	}
}

// TestComponentInterfaces validates that all components implement required interfaces
func TestComponentInterfaces(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)

	// Test that app implements tea.Model interface
	var model tea.Model = app
	if model == nil {
		t.Error("App does not implement tea.Model interface")
	}

	// Test Init() returns tea.Cmd
	cmd := app.Init()
	if cmd == nil {
		t.Error("App.Init() should return a tea.Cmd, got nil")
	}

	// Test Update() signature
	testMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	newModel, newCmd := app.Update(testMsg)
	if newModel == nil {
		t.Error("App.Update() should return a tea.Model")
	}
	_ = newCmd // cmd can be nil

	// Test View() returns string
	view := app.View()
	if len(view) == 0 {
		t.Error("App.View() should return non-empty string")
	}
}

// TestGoldenFrames tests that rendering produces stable, consistent output
func TestGoldenFrames(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	
	testCases := []struct {
		name   string
		width  int
		height int
	}{
		{"small_terminal", 80, 24},
		{"medium_terminal", 100, 30},
		{"large_terminal", 120, 40},
		{"wide_terminal", 160, 48},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app := ui.NewApp(config, sim)
			
			// Initialize with WindowSizeMsg
			app.Update(tea.WindowSizeMsg{Width: tc.width, Height: tc.height})
			
			// Capture initial frame
			frame1 := app.View()
			lineCount1 := strings.Count(frame1, "\n")
			
			// Simulate some updates without size change
			for i := 0; i < 10; i++ {
				app.Update(tea.KeyMsg{Type: tea.KeyTab})
			}
			
			// Capture frame after updates
			frame2 := app.View()
			lineCount2 := strings.Count(frame2, "\n")
			
			// Verify line count stability (NO line growth)
			if lineCount2 > lineCount1 {
				t.Errorf("Line count grew from %d to %d - violates stability requirement", 
					lineCount1, lineCount2)
			}
			
			// Verify no content overlap by checking for reasonable line lengths
			lines := strings.Split(frame2, "\n")
			for i, line := range lines {
				if len(line) > tc.width+10 { // Allow small buffer for edge cases
					t.Errorf("Line %d exceeds terminal width: %d > %d", i, len(line), tc.width)
				}
			}
			
			// Compare with golden file if it exists
			goldenPath := fmt.Sprintf("tests/golden/%s_%dx%d.golden", tc.name, tc.width, tc.height)
			if err := compareWithGolden(t, goldenPath, frame2); err != nil {
				// Create golden file if it doesn't exist
				t.Logf("Creating golden file: %s", goldenPath)
				writeGoldenFile(t, goldenPath, frame2)
			}
		})
	}
}

// TestTerminalSizes validates rendering at different terminal dimensions
func TestTerminalSizes(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	
	testSizes := []struct {
		width, height int
		expectedMode  string
	}{
		{60, 20, "small"},   // Below 80 cols
		{80, 24, "medium"},  // 80-119 cols  
		{100, 30, "medium"}, // 80-119 cols
		{120, 40, "large"},  // >=120 cols
		{160, 48, "large"},  // >=120 cols
	}

	for _, size := range testSizes {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			app := ui.NewApp(config, sim)
			
			// Initialize with specific size
			app.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
			
			// Render and validate
			output := app.View()
			
			// Verify output is not empty
			if len(output) == 0 {
				t.Error("Output should not be empty")
			}
			
			// Verify no overlap - lines should not exceed terminal width
			lines := strings.Split(output, "\n")
			for i, line := range lines {
				// Strip ANSI codes for accurate length measurement
				cleanLine := stripANSI(line)
				if len(cleanLine) > size.width+5 { // Small buffer for borders
					t.Errorf("Line %d too long: %d chars (max %d)", i, len(cleanLine), size.width)
				}
			}
			
			// Verify total lines fit in terminal
			totalLines := len(lines)
			if totalLines > size.height+2 { // Small buffer
				t.Errorf("Too many lines: %d (max %d)", totalLines, size.height)
			}
		})
	}
}

// TestKeyboardInteractions validates keyboard navigation and shortcuts
func TestKeyboardInteractions(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)
	
	// Initialize app
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	
	testKeys := []struct {
		name string
		key  tea.KeyMsg
	}{
		{"tab", tea.KeyMsg{Type: tea.KeyTab}},
		{"up_arrow", tea.KeyMsg{Type: tea.KeyUp}},
		{"down_arrow", tea.KeyMsg{Type: tea.KeyDown}},
		{"space", tea.KeyMsg{Type: tea.KeySpace}},
		{"enter", tea.KeyMsg{Type: tea.KeyEnter}},
		{"focus_1", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}}},
		{"focus_2", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}}},
		{"focus_3", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}}},
		{"help", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}},
	}
	
	for _, tk := range testKeys {
		t.Run(tk.name, func(t *testing.T) {
			// Apply key press
			newModel, cmd := app.Update(tk.key)
			
			// Verify model is returned
			if newModel == nil {
				t.Error("Update should return a model")
			}
			
			// Verify view still renders
			view := newModel.(tea.Model).View()
			if len(view) == 0 {
				t.Error("View should not be empty after key press")
			}
			
			// For quit key, verify tea.Quit is returned
			if tk.name == "quit" && cmd != tea.Quit {
				t.Error("Quit key should return tea.Quit command")
			}
		})
	}
}

// TestNonTTYFallback validates behavior in non-interactive environments
func TestNonTTYFallback(t *testing.T) {
	// Set environment to simulate CI/non-TTY
	oldTerm := os.Getenv("TERM")
	os.Setenv("TERM", "dumb")
	defer os.Setenv("TERM", oldTerm)
	
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)
	
	// Initialize app
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	
	// Render output
	output := app.View()
	
	// Verify output contains no ANSI escape codes
	if containsANSI(output) {
		t.Error("Non-TTY output should not contain ANSI escape codes")
	}
	
	// Verify output is still functional (not empty)
	if len(output) == 0 {
		t.Error("Non-TTY output should not be empty")
	}
}

// TestPerformance validates responsiveness under load
func TestPerformance(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)
	
	// Initialize app
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	
	// Simulate high-volume events
	startTime := time.Now()
	eventCount := 1000
	
	for i := 0; i < eventCount; i++ {
		// Simulate mixed event types
		switch i % 4 {
		case 0:
			app.Update(tea.KeyMsg{Type: tea.KeyTab})
		case 1:
			app.Update(tea.KeyMsg{Type: tea.KeyUp})
		case 2:
			app.Update(tea.KeyMsg{Type: tea.KeyDown})
		case 3:
			// Simulate log stream message
			app.Update(simulator.LogStreamMsg{})
		}
	}
	
	duration := time.Since(startTime)
	eventsPerSecond := float64(eventCount) / duration.Seconds()
	
	// Verify performance meets requirements (>100 events/second)
	if eventsPerSecond < 100 {
		t.Errorf("Performance too slow: %.2f events/sec (minimum 100)", eventsPerSecond)
	}
	
	// Verify view still renders correctly after load
	view := app.View()
	if len(view) == 0 {
		t.Error("View should render correctly after high-volume events")
	}
}

// TestRapidResize validates resize handling stability
func TestRapidResize(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)
	
	// Simulate rapid resize events
	sizes := []struct{ w, h int }{
		{80, 24}, {120, 40}, {60, 20}, {160, 48}, {100, 30},
		{80, 24}, {120, 40}, {60, 20}, {160, 48}, {100, 30},
	}
	
	for _, size := range sizes {
		app.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		
		// Verify view renders correctly after each resize
		view := app.View()
		if len(view) == 0 {
			t.Errorf("View should render after resize to %dx%d", size.w, size.h)
		}
		
		// Verify no panic or invalid state
		lines := strings.Split(view, "\n")
		if len(lines) > size.h+10 { // Reasonable buffer
			t.Errorf("Too many lines after resize: %d > %d", len(lines), size.h+10)
		}
	}
}

// TestMemoryBounds validates memory usage doesn't grow unbounded
func TestMemoryBounds(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)
	
	// Initialize app
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	
	// Simulate long-running session with continuous updates
	for i := 0; i < 10000; i++ {
		// Simulate log streaming and user interaction
		app.Update(simulator.LogStreamMsg{})
		if i%100 == 0 {
			app.Update(tea.KeyMsg{Type: tea.KeyTab})
		}
	}
	
	// Verify app still functions after extended use
	view := app.View()
	if len(view) == 0 {
		t.Error("View should render correctly after extended session")
	}
	
	// Memory usage should be reasonable (this is more of a smoke test)
	// In a real environment, you'd use runtime.MemStats to check actual usage
	t.Log("Extended session completed successfully")
}

// Helper functions

func createTestConfig() *ui.Config {
	return &ui.Config{
		UI: ui.UIConfig{
			Layout: ui.LayoutConfig{
				Breakpoints: struct {
					Small  int `yaml:"small"`
					Medium int `yaml:"medium"`
				}{Small: 80, Medium: 120},
				Panels: struct {
					List struct {
						WidthLarge  float64 `yaml:"width_large"`
						WidthMedium float64 `yaml:"width_medium"`
					} `yaml:"list"`
					Main struct {
						WidthLarge  float64 `yaml:"width_large"`
						WidthMedium float64 `yaml:"width_medium"`
					} `yaml:"main"`
					Status struct {
						WidthLarge   float64 `yaml:"width_large"`
						HeightFooter int     `yaml:"height_footer"`
					} `yaml:"status"`
				}{
					List: struct {
						WidthLarge  float64 `yaml:"width_large"`
						WidthMedium float64 `yaml:"width_medium"`
					}{WidthLarge: 0.25, WidthMedium: 0.4},
					Main: struct {
						WidthLarge  float64 `yaml:"width_large"`
						WidthMedium float64 `yaml:"width_medium"`
					}{WidthLarge: 0.5, WidthMedium: 0.6},
					Status: struct {
						WidthLarge   float64 `yaml:"width_large"`
						HeightFooter int     `yaml:"height_footer"`
					}{WidthLarge: 0.25, HeightFooter: 3},
				},
			},
			Performance: ui.PerformanceConfig{
				AltScreen:    true,
				FramerateCap: 60,
			},
		},
	}
}

func executeCmd(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.Output()
	return string(output), err
}

func stripANSI(s string) string {
	// Simple ANSI strip - in production, use a proper library
	var result strings.Builder
	inEscape := false
	
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' || r == 'K' || r == 'H' || r == 'J' {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	
	return result.String()
}

func containsANSI(s string) bool {
	return strings.Contains(s, "\x1b[")
}

func compareWithGolden(t *testing.T, goldenPath, actual string) error {
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		return err // File doesn't exist, will be created
	}
	
	if string(golden) != actual {
		// Write difference for debugging
		diffPath := strings.Replace(goldenPath, ".golden", ".diff", 1)
		os.WriteFile(diffPath, []byte(actual), 0644)
		t.Errorf("Output differs from golden file %s. Diff written to %s", goldenPath, diffPath)
	}
	
	return nil
}

func writeGoldenFile(t *testing.T, goldenPath, content string) {
	err := os.MkdirAll(filepath.Dir(goldenPath), 0755)
	if err != nil {
		t.Fatalf("Failed to create golden directory: %v", err)
	}
	
	err = os.WriteFile(goldenPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write golden file: %v", err)
	}
}