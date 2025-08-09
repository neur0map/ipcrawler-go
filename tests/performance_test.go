package tests

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/your-org/ipcrawler/internal/simulator"
	"github.com/your-org/ipcrawler/internal/ui"
)

// TestHighVolumeEventStreaming tests responsiveness under 1k events/min
func TestHighVolumeEventStreaming(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)
	
	// Initialize app
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Target: 1000 events per minute = ~16.67 events/second
	eventCount := 1000
	targetDuration := 60 * time.Second
	
	startTime := time.Now()
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Simulate high-volume event streaming
	eventTypes := []tea.Msg{
		simulator.LogStreamMsg{Entry: simulator.LogEntry{
			Timestamp: time.Now(),
			Level:     "INFO",
			Source:    "test",
			Message:   "High volume test message",
		}},
		simulator.StatusUpdateMsg{
			Status: simulator.SystemStatus{Status: "running"},
		},
		simulator.ProgressUpdateMsg{
			ID:       "test-progress",
			Progress: 0.5,
			Message:  "Test progress update",
		},
	}

	for i := 0; i < eventCount; i++ {
		select {
		case <-ctx.Done():
			t.Fatal("Test timed out - performance too slow")
		default:
		}
		
		// Apply event
		eventType := eventTypes[i%len(eventTypes)]
		newModel, _ := app.Update(eventType)
		
		if newModel == nil {
			t.Errorf("Event %d caused nil model", i)
			return
		}
		
		// Periodically verify view renders correctly
		if i%100 == 0 {
			view := newModel.(tea.Model).View()
			if len(view) == 0 {
				t.Errorf("Empty view at event %d", i)
			}
		}
		
		// Update app reference
		app = newModel.(*ui.App)
		
		// Rate limiting to simulate realistic timing
		if i > 0 && i%17 == 0 { // Every ~17 events (close to target rate)
			time.Sleep(time.Millisecond) // Brief pause
		}
	}
	
	duration := time.Since(startTime)
	eventsPerSecond := float64(eventCount) / duration.Seconds()
	
	t.Logf("Processed %d events in %v (%.2f events/sec)", eventCount, duration, eventsPerSecond)
	
	// Verify we can handle at least the target rate
	if eventsPerSecond < 10 { // Lower bound for acceptable performance
		t.Errorf("Performance too slow: %.2f events/sec (minimum 10)", eventsPerSecond)
	}
	
	// Verify final state is still functional
	finalView := app.View()
	if len(finalView) == 0 {
		t.Error("Final view should not be empty after high-volume processing")
	}
}

// TestRapidResizeHandling tests performance during rapid terminal resizes
func TestRapidResizeHandling(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)
	
	// Initialize app
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Simulate rapid resizes with concurrent event processing
	resizeSizes := []struct{ w, h int }{
		{80, 24}, {120, 40}, {100, 30}, {160, 48}, {90, 28}, {140, 35},
		{70, 22}, {130, 42}, {110, 32}, {150, 45}, {85, 26}, {125, 38},
	}

	startTime := time.Now()
	
	// Apply rapid resizes
	for i := 0; i < 100; i++ {
		size := resizeSizes[i%len(resizeSizes)]
		
		// Apply resize
		newModel, _ := app.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		
		if newModel == nil {
			t.Errorf("Resize %d caused nil model", i)
			return
		}
		
		// Verify rendering works
		if i%10 == 0 {
			view := newModel.(tea.Model).View()
			if len(view) == 0 {
				t.Errorf("Empty view after resize %d to %dx%d", i, size.w, size.h)
			}
		}
		
		app = newModel.(*ui.App)
		
		// Simulate some user input during resize
		if i%5 == 0 {
			app.Update(tea.KeyMsg{Type: tea.KeyTab})
		}
	}
	
	duration := time.Since(startTime)
	resizesPerSecond := float64(100) / duration.Seconds()
	
	t.Logf("Handled 100 resizes in %v (%.2f resizes/sec)", duration, resizesPerSecond)
	
	// Verify reasonable performance
	if resizesPerSecond < 50 {
		t.Errorf("Resize handling too slow: %.2f/sec (minimum 50)", resizesPerSecond)
	}
}

// TestMemoryUsageBounds tests that memory doesn't grow unbounded
func TestMemoryUsageBounds(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)
	
	// Initialize app
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Force garbage collection and get baseline
	runtime.GC()
	runtime.GC() // Second call to ensure cleanup
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)
	baselineHeap := m1.HeapInuse

	// Simulate long-running session
	sessionDuration := 30 * time.Second
	endTime := time.Now().Add(sessionDuration)
	eventCount := 0

	for time.Now().Before(endTime) {
		// Mix of different event types
		switch eventCount % 5 {
		case 0:
			app.Update(simulator.LogStreamMsg{Entry: simulator.LogEntry{
				Timestamp: time.Now(),
				Level:     "INFO", 
				Message:   "Long session test message",
			}})
		case 1:
			app.Update(tea.KeyMsg{Type: tea.KeyTab})
		case 2:
			app.Update(tea.WindowSizeMsg{Width: 120 + (eventCount%20), Height: 40 + (eventCount%10)})
		case 3:
			app.Update(simulator.StatusUpdateMsg{})
		case 4:
			app.View() // Trigger rendering
		}
		eventCount++
		
		// Brief pause to prevent overwhelming
		if eventCount%100 == 0 {
			time.Sleep(time.Millisecond)
		}
	}

	t.Logf("Processed %d events over %v", eventCount, sessionDuration)

	// Force garbage collection and measure final memory
	runtime.GC()
	runtime.GC()
	runtime.ReadMemStats(&m2)
	finalHeap := m2.HeapInuse

	memoryGrowth := int64(finalHeap) - int64(baselineHeap)
	memoryGrowthMB := float64(memoryGrowth) / (1024 * 1024)

	t.Logf("Memory growth: %.2f MB (baseline: %d, final: %d)", memoryGrowthMB, baselineHeap, finalHeap)

	// Allow reasonable memory growth but catch leaks
	maxGrowthMB := 50.0
	if memoryGrowthMB > maxGrowthMB {
		t.Errorf("Memory growth too high: %.2f MB (max allowed: %.2f MB)", memoryGrowthMB, maxGrowthMB)
	}

	// Verify app is still functional
	finalView := app.View()
	if len(finalView) == 0 {
		t.Error("App should still be functional after extended session")
	}
}

// TestViewportPerformance tests viewport scrolling with high-volume content
func TestViewportPerformance(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)
	
	// Initialize with large viewport
	app.Update(tea.WindowSizeMsg{Width: 160, Height: 48})

	// Generate many log entries
	logCount := 10000
	startTime := time.Now()

	for i := 0; i < logCount; i++ {
		logMsg := simulator.LogStreamMsg{
			Entry: simulator.LogEntry{
				Timestamp: time.Now(),
				Level:     "INFO",
				Source:    "performance-test",
				Message:   fmt.Sprintf("Log entry %d for viewport performance test", i),
			},
		}
		
		newModel, _ := app.Update(logMsg)
		if newModel == nil {
			t.Errorf("Log update %d caused nil model", i)
			return
		}
		
		app = newModel.(*ui.App)
		
		// Periodically test scrolling
		if i%1000 == 0 {
			// Test viewport navigation
			app.Update(tea.KeyMsg{Type: tea.KeyUp})
			app.Update(tea.KeyMsg{Type: tea.KeyDown})
			app.Update(tea.KeyMsg{Type: tea.KeyPgUp})
			app.Update(tea.KeyMsg{Type: tea.KeyPgDown})
			
			// Verify rendering still works
			view := app.View()
			if len(view) == 0 {
				t.Errorf("Empty view after %d log entries", i)
			}
		}
	}

	duration := time.Since(startTime)
	logsPerSecond := float64(logCount) / duration.Seconds()
	
	t.Logf("Processed %d log entries in %v (%.2f logs/sec)", logCount, duration, logsPerSecond)
	
	// Verify reasonable performance
	if logsPerSecond < 100 {
		t.Errorf("Viewport performance too slow: %.2f logs/sec (minimum 100)", logsPerSecond)
	}
}

// TestConcurrentOperations tests handling of concurrent resize and events
func TestConcurrentOperations(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)
	
	// Initialize app
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Use channels to coordinate concurrent operations
	done := make(chan struct{})
	errors := make(chan error, 10)
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Goroutine 1: Continuous resizing
	go func() {
		defer func() { done <- struct{}{} }()
		
		sizes := []struct{ w, h int }{
			{80, 24}, {120, 40}, {100, 30}, {140, 35},
		}
		
		for i := 0; i < 100; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}
			
			size := sizes[i%len(sizes)]
			newModel, _ := app.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
			
			if newModel == nil {
				errors <- fmt.Errorf("resize %d caused nil model", i)
				return
			}
			
			app = newModel.(*ui.App)
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Goroutine 2: Continuous user input
	go func() {
		defer func() { done <- struct{}{} }()
		
		keys := []tea.KeyMsg{
			{Type: tea.KeyTab}, {Type: tea.KeyUp}, {Type: tea.KeyDown}, {Type: tea.KeySpace},
		}
		
		for i := 0; i < 200; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}
			
			key := keys[i%len(keys)]
			newModel, _ := app.Update(key)
			
			if newModel == nil {
				errors <- fmt.Errorf("key %d caused nil model", i)
				return
			}
			
			app = newModel.(*ui.App)
			time.Sleep(5 * time.Millisecond)
		}
	}()

	// Goroutine 3: Continuous log streaming
	go func() {
		defer func() { done <- struct{}{} }()
		
		for i := 0; i < 500; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}
			
			logMsg := simulator.LogStreamMsg{
				Entry: simulator.LogEntry{
					Timestamp: time.Now(),
					Message:   fmt.Sprintf("Concurrent test log %d", i),
				},
			}
			
			newModel, _ := app.Update(logMsg)
			if newModel == nil {
				errors <- fmt.Errorf("log %d caused nil model", i)
				return
			}
			
			app = newModel.(*ui.App)
			time.Sleep(2 * time.Millisecond)
		}
	}()

	// Wait for all goroutines to complete
	completedCount := 0
	for completedCount < 3 {
		select {
		case <-done:
			completedCount++
		case err := <-errors:
			t.Errorf("Concurrent operation error: %v", err)
		case <-ctx.Done():
			t.Fatal("Test timed out - possible deadlock")
		}
	}

	// Verify final state
	finalView := app.View()
	if len(finalView) == 0 {
		t.Error("Final view should not be empty after concurrent operations")
	}
	
	t.Log("Concurrent operations completed successfully")
}

// TestRenderingPerformance tests pure rendering performance
func TestRenderingPerformance(t *testing.T) {
	config := createTestConfig()
	sim := simulator.NewMockSimulator()
	app := ui.NewApp(config, sim)
	
	// Initialize app with populated state
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	
	// Add some content first
	for i := 0; i < 100; i++ {
		app.Update(simulator.LogStreamMsg{
			Entry: simulator.LogEntry{
				Message: fmt.Sprintf("Content line %d", i),
			},
		})
	}

	// Measure pure rendering performance
	renderCount := 1000
	startTime := time.Now()
	
	for i := 0; i < renderCount; i++ {
		view := app.View()
		if len(view) == 0 {
			t.Errorf("Render %d produced empty view", i)
		}
	}
	
	duration := time.Since(startTime)
	rendersPerSecond := float64(renderCount) / duration.Seconds()
	
	t.Logf("Rendered %d frames in %v (%.2f fps)", renderCount, duration, rendersPerSecond)
	
	// Should be able to render at reasonable frame rates
	minFPS := 30.0
	if rendersPerSecond < minFPS {
		t.Errorf("Rendering too slow: %.2f fps (minimum %.2f)", rendersPerSecond, minFPS)
	}
}