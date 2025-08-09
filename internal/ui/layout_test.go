package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestResponsiveLayouts tests the layout system at different terminal sizes
func TestResponsiveLayouts(t *testing.T) {
	// Test different terminal sizes
	testCases := []struct {
		name     string
		width    int
		height   int
		expected LayoutMode
	}{
		{"Extra Large", 160, 48, LayoutLarge},
		{"Large", 120, 30, LayoutLarge},
		{"Large Minimum", 120, 24, LayoutLarge},
		{"Medium Upper", 119, 24, LayoutMedium},
		{"Medium", 100, 24, LayoutMedium},
		{"Medium Lower", 80, 20, LayoutMedium},
		{"Small Upper", 79, 20, LayoutSmall},
		{"Small", 60, 18, LayoutSmall},
		{"Small Minimum", 40, 15, LayoutSmall},
		{"Too Small Width", 39, 15, LayoutSmall}, // Should still be small, not error
		{"Too Small Height", 50, 9, LayoutSmall}, // Should handle gracefully
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model := NewModel("test.example.com")
			
			// Simulate WindowSizeMsg
			newModel, _ := model.Update(tea.WindowSizeMsg{
				Width:  tc.width,
				Height: tc.height,
			})
			
			m, ok := newModel.(Model)
			if !ok {
				t.Fatal("Failed to convert model to Model type")
			}
			
			// Check that the model is ready after WindowSizeMsg
			if !m.ready {
				t.Error("Model should be ready after WindowSizeMsg")
			}
			
			// Check layout mode
			if m.layoutMode != tc.expected {
				t.Errorf("Expected layout mode %v, got %v", tc.expected, m.layoutMode)
			}
			
			// Check dimensions are set correctly
			if m.width != tc.width {
				t.Errorf("Expected width %d, got %d", tc.width, m.width)
			}
			if m.height != tc.height {
				t.Errorf("Expected height %d, got %d", tc.height, m.height)
			}
			
			// Verify the view renders without panic
			view := m.View()
			if view == "" {
				t.Error("View should not be empty after initialization")
			}
			
			// Check that view respects terminal size constraints
			lines := strings.Split(view, "\n")
			if len(lines) > tc.height {
				t.Errorf("View has %d lines, but terminal height is %d", len(lines), tc.height)
			}
			
			// Check line lengths don't exceed terminal width
			for i, line := range lines {
				// Remove ANSI escape sequences for accurate length measurement
				cleanLine := stripANSIForTest(line)
				if len(cleanLine) > tc.width {
					t.Errorf("Line %d has length %d, but terminal width is %d: %q", 
						i, len(cleanLine), tc.width, cleanLine)
				}
			}
		})
	}
}

// TestLayoutTransitions tests smooth transitions between layout modes
func TestLayoutTransitions(t *testing.T) {
	model := NewModel("test.example.com")
	
	// Start with large layout
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	m1 := newModel.(Model)
	
	if m1.layoutMode != LayoutLarge {
		t.Errorf("Expected LayoutLarge, got %v", m1.layoutMode)
	}
	
	// Transition to medium
	newModel, _ = m1.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m2 := newModel.(Model)
	
	if m2.layoutMode != LayoutMedium {
		t.Errorf("Expected LayoutMedium, got %v", m2.layoutMode)
	}
	
	// Transition to small
	newModel, _ = m2.Update(tea.WindowSizeMsg{Width: 60, Height: 25})
	m3 := newModel.(Model)
	
	if m3.layoutMode != LayoutSmall {
		t.Errorf("Expected LayoutSmall, got %v", m3.layoutMode)
	}
	
	// Verify view still renders correctly after transitions
	view := m3.View()
	if view == "" {
		t.Error("View should not be empty after layout transitions")
	}
}

// TestComponentSizeCalculations tests that components get proper sizes
func TestComponentSizeCalculations(t *testing.T) {
	model := NewModel("test.example.com")
	
	// Test large layout component sizes
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m := newModel.(Model)
	
	// Verify component sizes are reasonable
	wfWidth := m.workflowList.Width()
	wfHeight := m.workflowList.Height()
	if wfWidth <= 0 || wfHeight <= 0 {
		t.Errorf("Workflow list has invalid size: %dx%d", wfWidth, wfHeight)
	}
	
	if m.logViewport.Width <= 0 || m.logViewport.Height <= 0 {
		t.Errorf("Log viewport has invalid size: %dx%d", m.logViewport.Width, m.logViewport.Height)
	}
	
	// Verify components don't exceed available space
	config := m.config
	navWidth := int(float64(m.width) * config.Layout.Panels.Navigation.PreferredWidthRatio)
	
	if wfWidth > navWidth {
		t.Errorf("Workflow list width %d exceeds navigation panel width %d", wfWidth, navWidth)
	}
}

// TestMinimumSizeHandling tests behavior at minimum terminal sizes
func TestMinimumSizeHandling(t *testing.T) {
	model := NewModel("test.example.com")
	
	// Test with very small terminal
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 30, Height: 8})
	m := newModel.(Model)
	
	// Should still render something (error message)
	view := m.View()
	if view == "" {
		t.Error("View should not be empty even at small sizes")
	}
	
	// Should contain error message about size
	if !strings.Contains(view, "too small") && !strings.Contains(view, "Terminal") {
		t.Error("Should display size error message for very small terminals")
	}
}

// TestConfigurationLoading tests that configuration affects layout calculations
func TestConfigurationLoading(t *testing.T) {
	model := NewModel("test.example.com")
	
	// Verify configuration is loaded
	if model.config == nil {
		t.Error("Configuration should be loaded")
	}
	
	// Verify layout breakpoints from config are used
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: model.config.Layout.Breakpoints.Large, Height: 30})
	m := newModel.(Model)
	
	if m.layoutMode != LayoutLarge {
		t.Error("Should use large layout at configured breakpoint")
	}
	
	// Test medium breakpoint
	newModel, _ = m.Update(tea.WindowSizeMsg{Width: model.config.Layout.Breakpoints.Medium, Height: 30})
	m = newModel.(Model)
	
	if m.layoutMode != LayoutMedium {
		t.Error("Should use medium layout at configured breakpoint")
	}
}

// stripANSIForTest removes ANSI escape sequences for accurate length measurement
func stripANSIForTest(s string) string {
	// Simple ANSI escape sequence removal for testing
	// This is a basic implementation - could use a proper library for production
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

// BenchmarkLayoutRendering benchmarks layout rendering performance
func BenchmarkLayoutRendering(b *testing.B) {
	model := NewModel("test.example.com")
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m := newModel.(Model)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

// TestLayoutModeString provides string representation for layout modes (for debugging)
func TestLayoutModeString(t *testing.T) {
	modes := []LayoutMode{LayoutLarge, LayoutMedium, LayoutSmall}
	expected := []string{"Large", "Medium", "Small"}
	
	for i, mode := range modes {
		got := layoutModeString(mode)
		if got != expected[i] {
			t.Errorf("Expected %s, got %s for mode %v", expected[i], got, mode)
		}
	}
}

// layoutModeString returns string representation of layout mode
func layoutModeString(mode LayoutMode) string {
	switch mode {
	case LayoutLarge:
		return "Large"
	case LayoutMedium:
		return "Medium"
	case LayoutSmall:
		return "Small"
	default:
		return "Unknown"
	}
}

// TestWorkflowDataHandling tests workflow update integration
func TestWorkflowDataHandling(t *testing.T) {
	model := NewModel("test.example.com")
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	
	// Add some workflow data
	workflowMsg := WorkflowUpdateMsg{
		ID:          "test-workflow",
		Description: "Test workflow description",
		Status:      "running",
		Progress:    0.5,
	}
	
	newModel, _ = newModel.Update(workflowMsg)
	m := newModel.(Model)
	
	// Verify workflow was added
	if len(m.workflows) != 1 {
		t.Errorf("Expected 1 workflow, got %d", len(m.workflows))
	}
	
	if m.workflows[0].ID != "test-workflow" {
		t.Errorf("Expected workflow ID 'test-workflow', got '%s'", m.workflows[0].ID)
	}
	
	// Verify list was updated
	if m.workflowList.Items() == nil || len(m.workflowList.Items()) != 1 {
		t.Error("Workflow list should contain 1 item")
	}
}

// Example demonstrates how to use the TUI in different scenarios
func ExampleModel_responsiveLayout() {
	model := NewModel("example.com")
	
	// Simulate different terminal sizes
	sizes := []struct{ width, height int }{
		{160, 40}, // Large layout
		{100, 30}, // Medium layout
		{60, 20},  // Small layout
	}
	
	for _, size := range sizes {
		newModel, _ := model.Update(tea.WindowSizeMsg{
			Width:  size.width,
			Height: size.height,
		})
		
		m := newModel.(Model)
		fmt.Printf("Terminal %dx%d -> Layout: %s\n", 
			size.width, size.height, layoutModeString(m.layoutMode))
		model = m // Update for next iteration
	}
	
	// Output:
	// Terminal 160x40 -> Layout: Large
	// Terminal 100x30 -> Layout: Medium  
	// Terminal 60x20 -> Layout: Small
}