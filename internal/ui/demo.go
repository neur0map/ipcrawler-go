package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
)

// DemoRunner provides a test harness for the new TUI implementation
type DemoRunner struct {
	runner *Runner
	target string
}

// NewDemoRunner creates a demo runner for testing the TUI
func NewDemoRunner(target string) *DemoRunner {
	return &DemoRunner{
		runner: NewRunner(target),
		target: target,
	}
}

// RunDemo starts the demo with simulated workflow events
func (d *DemoRunner) RunDemo() error {
	// Start the TUI in async mode
	ctx := context.Background()
	if err := d.runner.RunAsync(ctx); err != nil {
		return fmt.Errorf("failed to start TUI: %w", err)
	}

	// Wait for TUI to initialize
	time.Sleep(200 * time.Millisecond)

	// Send demo data
	go d.sendDemoEvents()

	// Keep demo running
	log.Info("Demo TUI started", "target", d.target)
	log.Info("Press Ctrl+C to exit")
	
	// Block until context is cancelled or TUI exits
	<-d.runner.Context().Done()
	
	return nil
}

// sendDemoEvents simulates workflow and tool events for testing
func (d *DemoRunner) sendDemoEvents() {
	// Wait a moment for TUI to be ready
	time.Sleep(500 * time.Millisecond)

	// Simulate workflow lifecycle
	d.runner.SendLog("INFO", "demo", "Starting demo simulation...")

	// Start first workflow
	d.runner.SendWorkflowUpdate("scan-ports", "Port scanning workflow", "running", 0.0, 0, nil)
	d.runner.SendLog("INFO", "workflow", "Started port scanning workflow")

	time.Sleep(1 * time.Second)

	// Tool execution events
	d.runner.SendToolUpdate("naabu", "scan-ports", "running", 0, []string{"-p", "80,443,22", d.target}, "", nil)
	d.runner.SendLog("INFO", "tool", "Running naabu port scan")
	
	time.Sleep(2 * time.Second)

	// Update workflow progress
	d.runner.SendWorkflowUpdate("scan-ports", "Port scanning workflow", "running", 0.3, 2*time.Second, nil)
	
	// Complete tool execution
	d.runner.SendToolUpdate("naabu", "scan-ports", "completed", 2*time.Second, []string{"-p", "80,443,22", d.target}, "Found 3 open ports", nil)
	
	time.Sleep(1 * time.Second)

	// Start second workflow
	d.runner.SendWorkflowUpdate("dns-enum", "DNS enumeration workflow", "running", 0.0, 0, nil)
	d.runner.SendLog("INFO", "workflow", "Started DNS enumeration workflow")

	// More tool events
	d.runner.SendToolUpdate("dig", "dns-enum", "running", 0, []string{"@8.8.8.8", d.target, "A"}, "", nil)
	d.runner.SendLog("INFO", "tool", "Running DNS queries")

	time.Sleep(1500 * time.Millisecond)

	// Complete first workflow
	d.runner.SendWorkflowUpdate("scan-ports", "Port scanning workflow", "completed", 1.0, 4*time.Second, nil)
	d.runner.SendLog("INFO", "workflow", "Port scanning completed successfully")

	// Complete DNS tool
	d.runner.SendToolUpdate("dig", "dns-enum", "completed", 1500*time.Millisecond, []string{"@8.8.8.8", d.target, "A"}, "Found 2 A records", nil)

	time.Sleep(1 * time.Second)

	// Start vulnerability workflow  
	d.runner.SendWorkflowUpdate("vuln-scan", "Vulnerability scanning", "running", 0.0, 0, nil)
	d.runner.SendLog("INFO", "workflow", "Started vulnerability scanning")

	// Simulate some failing tools
	d.runner.SendToolUpdate("nmap", "vuln-scan", "running", 0, []string{"-sV", "-sC", d.target}, "", nil)
	
	time.Sleep(2 * time.Second)

	// Complete DNS workflow
	d.runner.SendWorkflowUpdate("dns-enum", "DNS enumeration workflow", "completed", 1.0, 3500*time.Millisecond, nil)
	d.runner.SendLog("INFO", "workflow", "DNS enumeration completed")

	// Fail the nmap tool to test error handling
	d.runner.SendToolUpdate("nmap", "vuln-scan", "failed", 2*time.Second, []string{"-sV", "-sC", d.target}, "", fmt.Errorf("connection timeout"))
	d.runner.SendLog("ERROR", "tool", "Nmap scan failed: connection timeout")

	time.Sleep(1 * time.Second)

	// Fail the vulnerability workflow
	d.runner.SendWorkflowUpdate("vuln-scan", "Vulnerability scanning", "failed", 0.6, 3*time.Second, fmt.Errorf("scan timeout"))
	d.runner.SendLog("ERROR", "workflow", "Vulnerability scan failed: timeout")

	// Continue sending periodic updates to test live updates
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	counter := 1
	for {
		select {
		case <-ticker.C:
			// Send periodic status updates
			d.runner.SendLog("INFO", "demo", fmt.Sprintf("Demo update #%d - TUI running normally", counter))
			counter++

			// Simulate a quick tool execution every 10 seconds
			if counter%2 == 0 {
				toolName := fmt.Sprintf("demo-tool-%d", counter)
				d.runner.SendToolUpdate(toolName, "demo-workflow", "completed", 200*time.Millisecond, []string{"--demo"}, "Demo output", nil)
			}

		case <-d.runner.Context().Done():
			d.runner.SendLog("INFO", "demo", "Demo session ending...")
			return
		}
	}
}

// Quit stops the demo
func (d *DemoRunner) Quit() {
	if d.runner != nil {
		d.runner.Quit()
	}
}

// TestLayoutSizes tests the TUI at different terminal sizes
func TestLayoutSizes() {
	log.Info("Testing TUI layout at different sizes...")
	
	// This would be called by a test framework
	// For now, just log what we would test
	sizes := []struct {
		width, height int
		expectedMode  string
	}{
		{160, 48, "Large (3-panel)"},
		{120, 30, "Large (3-panel)"},
		{100, 24, "Medium (2-panel)"},
		{80, 24, "Medium (2-panel)"},
		{60, 20, "Small (1-panel)"},
		{40, 15, "Small (1-panel)"},
		{30, 10, "Too small (error)"},
	}

	for _, size := range sizes {
		log.Info("Layout test", 
			"width", size.width, 
			"height", size.height, 
			"expected", size.expectedMode)
	}
}