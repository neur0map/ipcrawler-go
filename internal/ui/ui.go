package ui

// UI provides a centralized interface for all user interface operations
type UI struct {
	Messages    *Messages
	Spinners    *Spinner
	Tables      *Tables
	Banners     *Banners
	Interactive *Interactive
	Progress    *ProgressDisplay
}

// New creates a new UI instance with all components initialized
func New() *UI {
	return &UI{
		Messages:    NewMessages(),
		Spinners:    NewSpinner(),
		Tables:      NewTables(),
		Banners:     NewBanners(),
		Interactive: NewInteractive(),
		Progress:    nil, // Will be created per-scan
	}
}

// CreateProgressDisplay creates a new progress display for a scan
func (ui *UI) CreateProgressDisplay(target, toolName string) *ProgressDisplay {
	ui.Progress = NewProgressDisplay(target, toolName)
	return ui.Progress
}

// CreateStreamingProcessor creates a new streaming output processor
func (ui *UI) CreateStreamingProcessor(toolName, target string) *StreamingOutputProcessor {
	return NewStreamingOutputProcessor(toolName, target)
}

// Global UI instance - this allows for easy access throughout the application
var Global *UI

func init() {
	Global = New()
}