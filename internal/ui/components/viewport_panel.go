package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/your-org/ipcrawler/internal/simulator"
	"github.com/your-org/ipcrawler/internal/ui/styles"
)

// Config interface to avoid import cycle
type ViewportConfig struct {
	HighPerformance bool
	AutoScroll      bool
	LineNumbers     bool
}

// ViewportPanel manages the center panel showing logs and output
type ViewportPanel struct {
	config *ViewportConfig
	theme  *styles.Theme

	// Bubbles viewport component
	viewport viewport.Model

	// State
	width  int
	height int
	ready  bool

	// Log content
	logs        []string
	maxLogLines int
	autoScroll  bool
}

// NewViewportPanel creates a new viewport panel component
func NewViewportPanel(config *ViewportConfig, theme *styles.Theme) *ViewportPanel {
	// Initialize viewport with high-performance mode if enabled
	vp := viewport.New(0, 0)
	vp.HighPerformanceRendering = config.HighPerformance

	// Configure viewport key bindings
	vp.KeyMap = viewport.KeyMap{
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+f"),
			key.WithHelp("pgdn", "page down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+b"),
			key.WithHelp("pgup", "page up"),
		),
		HalfPageUp: key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("ctrl+u", "½ page up"),
		),
		HalfPageDown: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d", "½ page down"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
	}

	return &ViewportPanel{
		config:      config,
		theme:       theme,
		viewport:    vp,
		logs:        []string{},
		maxLogLines: 1000, // Configurable max lines
		autoScroll:  config.AutoScroll,
	}
}

// Init initializes the viewport panel component
func (vp *ViewportPanel) Init() tea.Cmd {
	// Initialize with welcome message
	vp.addLogEntry("System", "info", "IPCrawler TUI initialized successfully")
	vp.addLogEntry("System", "info", "Press 'w' to view workflows, 't' for tools")
	vp.addLogEntry("System", "info", "Use arrow keys to navigate, Enter to select")
	
	return nil
}

// Update handles messages and updates the viewport panel
func (vp *ViewportPanel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle viewport-specific key bindings
		switch msg.String() {
		case "g":
			vp.viewport.GotoTop()
			return vp, nil
		case "G":
			vp.viewport.GotoBottom()
			return vp, nil
		case "home":
			vp.viewport.GotoTop()
			return vp, nil
		case "end":
			vp.viewport.GotoBottom()
			return vp, nil
		}

	case simulator.LogStreamMsg:
		// Add new log entry
		vp.addLogEntry(msg.Entry.Source, msg.Entry.Level, msg.Entry.Message)
		
		// Auto-scroll to bottom if enabled and not manually scrolled
		if vp.autoScroll && vp.viewport.AtBottom() {
			vp.viewport.GotoBottom()
		}

	case simulator.WorkflowExecutionMsg:
		// Handle workflow execution updates
		vp.addLogEntry("Workflow", "info", fmt.Sprintf("Workflow %s: %s (%.1f%%)", 
			msg.WorkflowID, msg.Status, msg.Progress*100))

	case simulator.ToolExecutionMsg:
		// Handle tool execution updates
		vp.addLogEntry("Tool", "info", fmt.Sprintf("Tool %s: %s (%.1f%%)", 
			msg.ToolID, msg.Status, msg.Progress*100))

	case simulator.StatusUpdateMsg:
		// Handle system status updates
		status := msg.Status
		vp.addLogEntry("System", "info", fmt.Sprintf("Status: %s, Tasks: %d/%d", 
			status.Status, status.ActiveTasks, status.Completed))
	}

	// Update the underlying viewport
	var cmd tea.Cmd
	vp.viewport, cmd = vp.viewport.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return vp, tea.Batch(cmds...)
}

// View renders the viewport panel
func (vp *ViewportPanel) View() string {
	if !vp.ready {
		return vp.theme.TitleStyle.Render("Loading viewport...")
	}

	// Render the viewport with header
	header := vp.renderHeader()
	content := vp.viewport.View()
	
	// Apply viewport styling
	styledContent := vp.theme.ViewportStyle.Render(content)
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		styledContent,
	)
}

// SetSize updates the panel dimensions
func (vp *ViewportPanel) SetSize(width, height int) tea.Cmd {
	vp.width = width
	vp.height = height
	vp.ready = true

	// Update viewport size (accounting for header and borders)
	viewportHeight := height - 4 // Reserve space for header and borders
	vp.viewport.Width = width - 4
	vp.viewport.Height = viewportHeight

	// Update content to fit new size
	vp.updateContent()

	return nil
}

// addLogEntry adds a new log entry to the viewport with proper wrapping
func (vp *ViewportPanel) addLogEntry(source, level, message string) {
	timestamp := time.Now().Format("15:04:05")
	
	// CRITICAL: Wrap text to viewport width to prevent truncation
	viewportWidth := vp.viewport.Width
	if viewportWidth <= 0 {
		viewportWidth = 80 // Fallback width
	}
	
	// Wrap long messages to fit viewport width
	wrappedMessage := vp.wrapText(message, viewportWidth-25) // Reserve space for timestamp/level
	
	// Format log entry with theme colors
	logLine := vp.theme.RenderLogEntry(timestamp, level, source, wrappedMessage)
	
	// Add to log storage
	vp.logs = append(vp.logs, logLine)
	
	// Maintain max lines limit
	if len(vp.logs) > vp.maxLogLines {
		vp.logs = vp.logs[len(vp.logs)-vp.maxLogLines:]
	}
	
	// Update viewport content
	vp.updateContent()
}

// updateContent refreshes the viewport content
func (vp *ViewportPanel) updateContent() {
	if len(vp.logs) == 0 {
		vp.viewport.SetContent("No logs available yet...")
		return
	}
	
	// Join all log lines
	content := strings.Join(vp.logs, "\n")
	vp.viewport.SetContent(content)
}

// renderHeader renders the viewport header with info
func (vp *ViewportPanel) renderHeader() string {
	title := "System Logs & Output"
	
	// Show current viewport position info
	positionInfo := ""
	if vp.ready {
		if vp.viewport.AtTop() {
			positionInfo = "Top"
		} else if vp.viewport.AtBottom() {
			positionInfo = "Bottom"
		} else {
			positionInfo = fmt.Sprintf("%.0f%%", vp.viewport.ScrollPercent()*100)
		}
	}
	
	logCount := fmt.Sprintf("Lines: %d", len(vp.logs))
	
	// Style the header components
	titleStyle := vp.theme.TitleStyle
	infoStyle := lipgloss.NewStyle().
		Foreground(vp.theme.Colors.Secondary).
		Align(lipgloss.Right)
	
	// Create header with title on left, info on right
	headerWidth := vp.width - 6 // Account for padding
	if headerWidth < 0 {
		headerWidth = 40 // Fallback minimum width
	}
	
	titleRendered := titleStyle.Render(title)
	infoRendered := infoStyle.Render(fmt.Sprintf("%s | %s", logCount, positionInfo))
	
	// Calculate spacing to right-align info
	titleLen := lipgloss.Width(titleRendered)
	infoLen := lipgloss.Width(infoRendered)
	spacing := headerWidth - titleLen - infoLen
	if spacing < 1 {
		spacing = 1
	}
	
	spacer := strings.Repeat(" ", spacing)
	
	return titleRendered + spacer + infoRendered
}

// ClearLogs clears all log entries
func (vp *ViewportPanel) ClearLogs() {
	vp.logs = []string{}
	vp.updateContent()
	vp.viewport.GotoTop()
}

// GetLogCount returns the current number of log entries
func (vp *ViewportPanel) GetLogCount() int {
	return len(vp.logs)
}

// IsAtBottom returns true if the viewport is scrolled to the bottom
func (vp *ViewportPanel) IsAtBottom() bool {
	return vp.viewport.AtBottom()
}

// ScrollToBottom scrolls the viewport to the bottom
func (vp *ViewportPanel) ScrollToBottom() {
	vp.viewport.GotoBottom()
}

// SetAutoScroll enables or disables auto-scrolling
func (vp *ViewportPanel) SetAutoScroll(enabled bool) {
	vp.autoScroll = enabled
}

// wrapText wraps long text to fit within specified width
func (vp *ViewportPanel) wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}
	
	var lines []string
	currentLine := ""
	
	for _, word := range words {
		// If adding this word would exceed width, start new line
		if len(currentLine)+len(word)+1 > width {
			if currentLine != "" {
				lines = append(lines, currentLine)
				currentLine = word
			} else {
				// Single word longer than width, truncate it
				if len(word) > width-3 {
					lines = append(lines, word[:width-3]+"...")
				} else {
					lines = append(lines, word)
				}
			}
		} else {
			if currentLine == "" {
				currentLine = word
			} else {
				currentLine += " " + word
			}
		}
	}
	
	// Add final line if not empty
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	
	return strings.Join(lines, "\n")
}