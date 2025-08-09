package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Bold(true).
		MarginBottom(1)

	panelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1).
		Margin(0, 1)

	focusedPanelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(1).
		Margin(0, 1)

	logStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("248"))

	timestampStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EF4444")).
		Bold(true)

	successStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#04B575")).
		Bold(true)
)

// List item implementation
type item struct {
	title, desc string
}

func (i item) FilterValue() string { return i.title }
func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }

// Layout modes
type layoutMode int

const (
	layoutSmall layoutMode = iota
	layoutMedium
	layoutLarge
)

// Focus states
type focusedPanel int

const (
	focusList focusedPanel = iota
	focusViewport
	focusStatus
)

// Main application model
type model struct {
	ready    bool
	width    int
	height   int
	layout   layoutMode
	focused  focusedPanel
	list     list.Model
	viewport viewport.Model
	logs     []string
	keys     keyMap
	
	// Production features
	activeWorkflow string
	activeTool     string
	systemStatus   string
	metrics        struct {
		completed int
		failed    int
		active    int
	}
}

// Key bindings
type keyMap struct {
	Up    key.Binding
	Down  key.Binding
	Left  key.Binding
	Right key.Binding
	Tab   key.Binding
	Enter key.Binding
	Quit  key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Tab, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Tab, k.Enter, k.Quit},
	}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "move left"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "move right"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next panel"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter", " "),
		key.WithHelp("enter/space", "select"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

// Create new model
func newModel() model {
	// Initialize workflows list
	items := []list.Item{
		item{title: "Port Scan Basic", desc: "Standard TCP port enumeration"},
		item{title: "Subdomain Discovery", desc: "Comprehensive subdomain enumeration"},
		item{title: "Web Application Scan", desc: "Automated web app security testing"},
		item{title: "Network Discovery", desc: "Host discovery and network mapping"},
		item{title: "Vulnerability Assessment", desc: "Comprehensive vulnerability scanning"},
		item{title: "SSL/TLS Analysis", desc: "Certificate and protocol security analysis"},
		item{title: "DNS Reconnaissance", desc: "DNS records and zone transfer testing"},
		item{title: "Service Fingerprinting", desc: "Identify services and versions"},
		item{title: "Content Discovery", desc: "Find hidden files and directories"},
		item{title: "API Security Scan", desc: "REST/GraphQL endpoint testing"},
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "IPCrawler Workflows"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	vp := viewport.New(0, 0)
	vp.HighPerformanceRendering = true

	// Initialize with system logs
	logs := []string{
		formatLogEntry("15:04:05", "INFO", "System", "IPCrawler v2.0.0 initialized"),
		formatLogEntry("15:04:06", "INFO", "Config", "Loaded configuration from configs/ui.yaml"),
		formatLogEntry("15:04:07", "INFO", "Tools", "Discovered 15 security tools"),
		formatLogEntry("15:04:08", "INFO", "Network", "Network interfaces detected: eth0, lo"),
		formatLogEntry("15:04:09", "INFO", "System", "Ready for operations"),
	}

	m := model{
		list:         l,
		viewport:     vp,
		logs:         logs,
		keys:         keys,
		focused:      focusList,
		systemStatus: "Ready",
		metrics: struct {
			completed int
			failed    int
			active    int
		}{
			completed: 0,
			failed:    0,
			active:    0,
		},
	}

	return m
}

// Format log entries with color coding
func formatLogEntry(timestamp, level, source, message string) string {
	var levelStyled string
	switch level {
	case "ERROR":
		levelStyled = errorStyle.Render("[ERROR]")
	case "WARN":
		levelStyled = errorStyle.Render("[WARN] ")
	case "INFO":
		levelStyled = successStyle.Render("[INFO] ")
	default:
		levelStyled = logStyle.Render("[" + level + "] ")
	}

	return fmt.Sprintf("%s %s %s %s",
		timestampStyle.Render(timestamp),
		levelStyled,
		lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4")).Width(10).Render(source),
		logStyle.Render(message),
	)
}

// Initialize the model
func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(time.Second*3, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}),
	)
}

type tickMsg time.Time

// Update handles all messages
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// CRITICAL: Handle WindowSizeMsg first - prevents text truncation
		m.width, m.height = msg.Width, msg.Height
		m.ready = true
		m.layout = m.calculateLayout()
		m.updatePanelSizes()

	case tickMsg:
		// Simulate activity
		if m.activeWorkflow != "" {
			newLog := formatLogEntry(
				time.Now().Format("15:04:05"),
				"INFO",
				m.activeWorkflow,
				fmt.Sprintf("Processing target 192.168.1.%d", len(m.logs)%255),
			)
			m.logs = append(m.logs, newLog)
			if len(m.logs) > 100 {
				m.logs = m.logs[1:]
			}
			m.updateViewportContent()
		}
		return m, tea.Tick(time.Second*3, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Tab):
			m.cycleFocus()
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			if m.focused == focusList {
				if i, ok := m.list.SelectedItem().(item); ok {
					// Start workflow
					m.activeWorkflow = i.title
					m.systemStatus = "Running"
					m.metrics.active++
					newLog := formatLogEntry(
						time.Now().Format("15:04:05"),
						"INFO",
						"Workflow",
						fmt.Sprintf("Started workflow: %s", i.title),
					)
					m.logs = append(m.logs, newLog)
					m.updateViewportContent()
				}
			}
		}

		// Route keys to focused panel
		switch m.focused {
		case focusList:
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		case focusViewport:
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// Main view rendering
func (m model) View() string {
	if !m.ready {
		return titleStyle.Render("Initializing IPCrawler...")
	}

	switch m.layout {
	case layoutLarge:
		return m.renderThreeColumn()
	case layoutMedium:
		return m.renderTwoColumn()
	default:
		return m.renderStacked()
	}
}

// Calculate responsive layout
func (m *model) calculateLayout() layoutMode {
	switch {
	case m.width >= 120:
		return layoutLarge
	case m.width >= 80:
		return layoutMedium
	default:
		return layoutSmall
	}
}

// Update panel sizes based on layout
func (m *model) updatePanelSizes() {
	contentHeight := m.height - 4

	switch m.layout {
	case layoutLarge:
		leftW := int(float64(m.width) * 0.25)
		mainW := int(float64(m.width) * 0.55)
		
		m.list.SetSize(leftW-4, contentHeight-2)
		m.viewport.Width = mainW - 4
		m.viewport.Height = contentHeight - 4
		
	case layoutMedium:
		leftW := int(float64(m.width) * 0.30)
		mainW := int(float64(m.width) * 0.70)
		
		m.list.SetSize(leftW-4, contentHeight-2)
		m.viewport.Width = mainW - 4
		m.viewport.Height = contentHeight - 8 // Footer space
		
	case layoutSmall:
		m.list.SetSize(m.width-8, 8)
		m.viewport.Width = m.width - 8
		m.viewport.Height = contentHeight - 16
	}

	m.updateViewportContent()
}

// Update viewport with wrapped text
func (m *model) updateViewportContent() {
	if len(m.logs) == 0 {
		m.viewport.SetContent("No logs available...")
		return
	}
	
	// CRITICAL: Wrap text to viewport width to prevent truncation
	wrappedLogs := make([]string, 0, len(m.logs))
	for _, logLine := range m.logs {
		wrapped := m.wrapText(logLine, m.viewport.Width)
		wrappedLogs = append(wrappedLogs, wrapped)
	}
	
	content := strings.Join(wrappedLogs, "\n")
	m.viewport.SetContent(content)
	
	// Auto scroll to bottom
	m.viewport.GotoBottom()
}

// Text wrapping to prevent truncation
func (m *model) wrapText(text string, width int) string {
	if width <= 0 || len(text) <= width {
		return text
	}
	
	// Simple word wrapping
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}
	
	var lines []string
	currentLine := ""
	
	for _, word := range words {
		if len(currentLine)+len(word)+1 > width {
			if currentLine != "" {
				lines = append(lines, currentLine)
				currentLine = word
			} else {
				// Word longer than width
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
	
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	
	return strings.Join(lines, "\n")
}

// Cycle focus between panels
func (m *model) cycleFocus() {
	switch m.focused {
	case focusList:
		m.focused = focusViewport
	case focusViewport:
		if m.layout == layoutLarge {
			m.focused = focusStatus
		} else {
			m.focused = focusList
		}
	case focusStatus:
		m.focused = focusList
	}
}

// Three-column layout for large screens
func (m model) renderThreeColumn() string {
	leftW := int(float64(m.width) * 0.25)
	mainW := int(float64(m.width) * 0.55)
	rightW := int(float64(m.width) * 0.20)
	contentH := m.height - 4

	// Left panel (list)
	listPanel := titleStyle.Render("Workflows") + "\n" + m.list.View()
	leftStyled := panelStyle.Width(leftW).Height(contentH).Render(listPanel)
	if m.focused == focusList {
		leftStyled = focusedPanelStyle.Width(leftW).Height(contentH).Render(listPanel)
	}

	// Main panel (viewport)
	header := titleStyle.Render("System Logs & Output")
	viewportContent := m.viewport.View()
	mainContent := header + "\n" + viewportContent
	mainStyled := panelStyle.Width(mainW).Height(contentH).Render(mainContent)
	if m.focused == focusViewport {
		mainStyled = focusedPanelStyle.Width(mainW).Height(contentH).Render(mainContent)
	}

	// Right panel (status)
	statusContent := m.renderStatus()
	rightStyled := panelStyle.Width(rightW).Height(contentH).Render(statusContent)
	if m.focused == focusStatus {
		rightStyled = focusedPanelStyle.Width(rightW).Height(contentH).Render(statusContent)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, leftStyled, mainStyled, rightStyled)
}

// Two-column layout for medium screens
func (m model) renderTwoColumn() string {
	leftW := int(float64(m.width) * 0.30)
	mainW := int(float64(m.width) * 0.70)
	contentH := m.height - 8

	// Left panel
	listPanel := titleStyle.Render("Workflows") + "\n" + m.list.View()
	leftStyled := panelStyle.Width(leftW).Height(contentH).Render(listPanel)
	if m.focused == focusList {
		leftStyled = focusedPanelStyle.Width(leftW).Height(contentH).Render(listPanel)
	}

	// Main panel
	header := titleStyle.Render("System Logs & Output")
	viewportContent := m.viewport.View()
	mainContent := header + "\n" + viewportContent
	mainStyled := panelStyle.Width(mainW).Height(contentH).Render(mainContent)
	if m.focused == focusViewport {
		mainStyled = focusedPanelStyle.Width(mainW).Height(contentH).Render(mainContent)
	}

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, leftStyled, mainStyled)

	// Footer status
	footer := panelStyle.Width(m.width-2).Height(4).Render(m.renderStatus())
	
	return lipgloss.JoinVertical(lipgloss.Left, topRow, footer)
}

// Stacked layout for small screens
func (m model) renderStacked() string {
	panelW := m.width - 4

	selected := "None"
	if i, ok := m.list.SelectedItem().(item); ok {
		selected = i.Title()
	}

	// Header (condensed list)
	header := panelStyle.Width(panelW).Height(8).Render(
		titleStyle.Render("Workflows") + "\n" + 
		fmt.Sprintf("Selected: %s", selected),
	)

	// Main (viewport)
	main := panelStyle.Width(panelW).Height(m.height-16).Render(
		titleStyle.Render("System Logs") + "\n" + m.viewport.View(),
	)

	// Footer (status)
	footer := panelStyle.Width(panelW).Height(4).Render(m.renderStatus())

	return lipgloss.JoinVertical(lipgloss.Left, header, main, footer)
}

// Render status panel
func (m model) renderStatus() string {
	var statusIcon string
	var statusColor lipgloss.Style
	
	switch m.systemStatus {
	case "Running":
		statusIcon = "●"
		statusColor = successStyle
	case "Error":
		statusIcon = "●"
		statusColor = errorStyle
	default:
		statusIcon = "●"
		statusColor = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	}
	
	status := fmt.Sprintf("Active: %d | Completed: %d | Failed: %d | Logs: %d",
		m.metrics.active, m.metrics.completed, m.metrics.failed, len(m.logs))
	
	activeWorkflow := m.activeWorkflow
	if activeWorkflow == "" {
		activeWorkflow = "None"
	}
	
	return titleStyle.Render("System Status") + "\n" + 
		statusColor.Render(statusIcon + " " + m.systemStatus) + "\n" +
		"Workflow: " + activeWorkflow + "\n" +
		status
}

// Main entry point
func main() {
	// Create the application model
	app := newModel()
	
	// CRITICAL: Single tea.Program with alt-screen
	// This is the ONLY renderer in the entire application
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
	
	// Run the TUI
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running IPCrawler: %v\n", err)
		os.Exit(1)
	}
}