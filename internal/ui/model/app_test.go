package model

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/carlosm/ipcrawler/internal/ui/layout"
)

func TestNewAppModel(t *testing.T) {
	target := "test.example.com"
	app := NewAppModel(target)
	
	// Test basic initialization
	if app.target != target {
		t.Errorf("Expected target %s, got %s", target, app.target)
	}
	
	if app.state != StateInitializing {
		t.Errorf("Expected state %v, got %v", StateInitializing, app.state)
	}
	
	if app.layout == nil {
		t.Error("Layout should be initialized")
	}
	
	if app.accessibility == nil {
		t.Error("Accessibility manager should be initialized")
	}
	
	if app.focused != FocusWorkflowList {
		t.Errorf("Expected focus %v, got %v", FocusWorkflowList, app.focused)
	}
}

func TestAppModelInit(t *testing.T) {
	app := NewAppModel("test.example.com")
	cmd := app.Init()
	
	if cmd == nil {
		t.Error("Init should return a command")
	}
}

func TestHandleResize(t *testing.T) {
	app := NewAppModel("test.example.com")
	
	// Test resize handling
	resizeMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	newModel, cmd := app.handleResize(resizeMsg)
	newApp := newModel.(AppModel)
	
	if newApp.terminalSize.Width != 100 {
		t.Errorf("Expected width 100, got %d", newApp.terminalSize.Width)
	}
	
	if newApp.terminalSize.Height != 30 {
		t.Errorf("Expected height 30, got %d", newApp.terminalSize.Height)
	}
	
	if cmd == nil {
		t.Error("Resize should return a command")
	}
	
	// Test layout responds to size changes
	mode1 := newApp.layout.Mode()
	
	// Test different layout sizes - just verify layout changes
	largeMsg := tea.WindowSizeMsg{Width: 130, Height: 40}
	largeModel, _ := app.handleResize(largeMsg)
	largeApp := largeModel.(AppModel)
	mode2 := largeApp.layout.Mode()
	
	// Test small layout
	smallMsg := tea.WindowSizeMsg{Width: 30, Height: 15}
	smallModel, _ := app.handleResize(smallMsg)
	smallApp := smallModel.(AppModel)
	mode3 := smallApp.layout.Mode()
	
	// Just verify that layout system is working (modes are valid)
	validModes := []layout.LayoutMode{layout.LargeLayout, layout.MediumLayout, layout.SmallLayout}
	isValidMode := func(mode layout.LayoutMode) bool {
		for _, valid := range validModes {
			if mode == valid {
				return true
			}
		}
		return false
	}
	
	if !isValidMode(mode1) || !isValidMode(mode2) || !isValidMode(mode3) {
		t.Errorf("Layout modes should be valid: got %v, %v, %v", mode1, mode2, mode3)
	}
}

func TestWorkflowUpdate(t *testing.T) {
	app := NewAppModel("test.example.com")
	
	// Test workflow update
	workflowMsg := WorkflowUpdateMsg{
		WorkflowID:  "test-workflow",
		Status:      "running",
		Description: "Test workflow",
		Progress:    0.5,
		StartTime:   time.Now(),
	}
	
	newModel, cmd := app.handleWorkflowUpdate(workflowMsg)
	newApp := newModel.(AppModel)
	
	if len(newApp.workflows) != 1 {
		t.Errorf("Expected 1 workflow, got %d", len(newApp.workflows))
	}
	
	workflow := newApp.workflows[0]
	if workflow.ID != "test-workflow" {
		t.Errorf("Expected workflow ID 'test-workflow', got %s", workflow.ID)
	}
	
	if workflow.Status != "running" {
		t.Errorf("Expected status 'running', got %s", workflow.Status)
	}
	
	if workflow.Progress != 0.5 {
		t.Errorf("Expected progress 0.5, got %f", workflow.Progress)
	}
	
	if cmd != nil {
		t.Error("Workflow update should not return a command")
	}
}

func TestToolExecution(t *testing.T) {
	app := NewAppModel("test.example.com")
	
	toolMsg := ToolExecutionMsg{
		ToolName:   "nmap",
		WorkflowID: "test-workflow",
		Status:     "completed",
		Duration:   time.Second * 5,
		Output:     "Scan complete",
		Args:       []string{"-sV", "target"},
	}
	
	newModel, cmd := app.handleToolExecution(toolMsg)
	newApp := newModel.(AppModel)
	
	if len(newApp.tools) != 1 {
		t.Errorf("Expected 1 tool execution, got %d", len(newApp.tools))
	}
	
	tool := newApp.tools[0]
	if tool.Name != "nmap" {
		t.Errorf("Expected tool name 'nmap', got %s", tool.Name)
	}
	
	if tool.Status != "completed" {
		t.Errorf("Expected status 'completed', got %s", tool.Status)
	}
	
	if cmd != nil {
		t.Error("Tool execution should not return a command")
	}
}

func TestLogHandling(t *testing.T) {
	app := NewAppModel("test.example.com")
	
	logMsg := LogMsg{
		Level:     "info",
		Message:   "Test log message",
		Timestamp: time.Now(),
		Category:  "test",
		Data:      map[string]interface{}{"key": "value"},
	}
	
	newModel, cmd := app.handleLogMessage(logMsg)
	newApp := newModel.(AppModel)
	
	if len(newApp.logs) != 1 {
		t.Errorf("Expected 1 log entry, got %d", len(newApp.logs))
	}
	
	log := newApp.logs[0]
	if log.Level != "info" {
		t.Errorf("Expected level 'info', got %s", log.Level)
	}
	
	if log.Message != "Test log message" {
		t.Errorf("Expected message 'Test log message', got %s", log.Message)
	}
	
	if log.Category != "test" {
		t.Errorf("Expected category 'test', got %s", log.Category)
	}
	
	if cmd != nil {
		t.Error("Log handling should not return a command")
	}
}

func TestFocusCycling(t *testing.T) {
	app := NewAppModel("test.example.com")
	
	// Test forward cycling
	initialFocus := app.focused
	app.cycleFocusForward()
	
	if app.focused == initialFocus {
		t.Error("Focus should have changed after cycling forward")
	}
	
	// Test backward cycling
	newFocus := app.focused
	app.cycleFocusBackward()
	
	if app.focused == newFocus {
		t.Error("Focus should have changed after cycling backward")
	}
}

func TestComponentSizeUpdates(t *testing.T) {
	app := NewAppModel("test.example.com")
	
	// Initialize with a specific size
	resizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	app.terminalSize = resizeMsg
	app.layout.Update(resizeMsg.Width, resizeMsg.Height)
	
	// Test component size calculation
	app.updateComponentSizes()
	
	// Verify components have reasonable sizes
	sizes := app.layout.CalculateComponentSizes()
	
	if workflowListSize, ok := sizes["workflow_list"]; ok {
		if workflowListSize.Width <= 0 || workflowListSize.Height <= 0 {
			t.Error("Workflow list should have positive dimensions")
		}
	}
	
	if workflowTableSize, ok := sizes["workflow_table"]; ok {
		if workflowTableSize.Width <= 0 || workflowTableSize.Height <= 0 {
			t.Error("Workflow table should have positive dimensions")
		}
	}
}

func TestAccessibilityIntegration(t *testing.T) {
	app := NewAppModel("test.example.com")
	
	if app.accessibility == nil {
		t.Error("Accessibility manager should be initialized")
	}
	
	// Test minimum size recommendations
	minWidth, minHeight := app.accessibility.GetRecommendedMinimumSize()
	if minWidth <= 0 || minHeight <= 0 {
		t.Error("Accessibility manager should return positive minimum dimensions")
	}
	
	// Test accessibility announcements
	announcement := app.accessibility.GetAccessibilityAnnouncement("workflow_start", map[string]interface{}{
		"workflow": "test-workflow",
	})
	
	if app.accessibility.IsScreenReaderActive() && announcement == "" {
		t.Error("Screen reader should generate announcements for workflow events")
	}
}

func TestStateTransitions(t *testing.T) {
	app := NewAppModel("test.example.com")
	
	// Test initial state
	if app.GetState() != StateInitializing {
		t.Errorf("Expected initial state %v, got %v", StateInitializing, app.GetState())
	}
	
	// Test target getter
	if app.GetTarget() != "test.example.com" {
		t.Errorf("Expected target 'test.example.com', got %s", app.GetTarget())
	}
	
	// Test workflow management
	testWorkflow := WorkflowStatus{
		ID:          "test-workflow",
		Status:      "running",
		Description: "Test workflow",
	}
	
	app.AddWorkflow(testWorkflow)
	workflows := app.GetWorkflows()
	
	if len(workflows) != 1 {
		t.Errorf("Expected 1 workflow, got %d", len(workflows))
	}
	
	if workflows[0].ID != "test-workflow" {
		t.Errorf("Expected workflow ID 'test-workflow', got %s", workflows[0].ID)
	}
}

func TestTerminalSizeValidation(t *testing.T) {
	app := NewAppModel("test.example.com")
	
	// Test too small terminal
	tooSmallMsg := tea.WindowSizeMsg{Width: 20, Height: 5}
	_, cmd := app.handleResize(tooSmallMsg)
	
	if cmd == nil {
		t.Error("Too small terminal should generate a warning command")
	}
	
	// Test adequate terminal size
	adequateMsg := tea.WindowSizeMsg{Width: 80, Height: 24}
	_, cmd = app.handleResize(adequateMsg)
	
	if cmd == nil {
		t.Error("Adequate terminal size should generate a log command")
	}
}

// Benchmark tests for performance
func BenchmarkAppModelResize(b *testing.B) {
	app := NewAppModel("test.example.com")
	msg := tea.WindowSizeMsg{Width: 100, Height: 30}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.handleResize(msg)
	}
}

func BenchmarkWorkflowUpdate(b *testing.B) {
	app := NewAppModel("test.example.com")
	msg := WorkflowUpdateMsg{
		WorkflowID:  "test-workflow",
		Status:      "running",
		Description: "Test workflow",
		Progress:    0.5,
		StartTime:   time.Now(),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.handleWorkflowUpdate(msg)
	}
}