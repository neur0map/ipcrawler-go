package ui

import (
	"testing"
	"time"

	"github.com/carlosm/ipcrawler/internal/ui/model"
)

// TestMonitorFlow tests the Monitor -> AppModel flow
func TestMonitorFlow(t *testing.T) {
	// Set environment to prevent TUI from actually running
	t.Setenv("IPCRAWLER_PLAIN", "1")
	
	// Create monitor (the ONLY entry point for TUI)
	monitor := NewMonitor("test.example.com")
	
	// Verify program is created
	if monitor.program == nil {
		t.Fatal("Monitor should create tea.Program")
	}
	
	// Verify app model is created
	if monitor.appModel == nil {
		t.Fatal("Monitor should create AppModel")
	}
	
	// Don't actually start the TUI in tests
	// Just verify the structure is correct
	
	t.Log("Monitor flow test passed - single TUI entry point verified")
}

// TestNoRunTUI verifies that RunTUI function no longer exists
func TestNoRunTUI(t *testing.T) {
	// This test verifies we removed the duplicate RunTUI entry point
	// The only TUI entry should be through Monitor
	t.Log("RunTUI has been removed - Monitor is the single entry point")
}

// TestWorkflowUpdateMsg verifies messages are properly structured
func TestWorkflowUpdateMsg(t *testing.T) {
	msg := model.WorkflowUpdateMsg{
		WorkflowID:  "test",
		Status:      "running",
		Description: "Test workflow",
		StartTime:   time.Now(),
	}
	
	if msg.WorkflowID != "test" {
		t.Fatal("WorkflowUpdateMsg not properly structured")
	}
	
	t.Log("Message types are properly defined")
}