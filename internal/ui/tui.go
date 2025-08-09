package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents the main Bubble Tea model for IPCrawler TUI
type Model struct {
	// WindowSizeMsg guard - CRITICAL for proper initial render
	ready  bool
	width  int
	height int

	// Target being scanned
	target string

	// Configuration
	config *UIConfig

	// Current state
	state AppState

	// Embedded Bubbles components - the Charmbracelet way
	workflowList list.Model
	toolTable    table.Model
	logViewport  viewport.Model
	progress     progress.Model
	spinner      spinner.Model
	help         help.Model

	// Layout state
	currentPanel FocusedPanel
	layoutMode   LayoutMode
	showHelp     bool

	// Application data
	workflows []WorkflowData
	tools     []ToolData
	logs      []LogEntry

	// Keybindings
	keyMap KeyMap

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// AppState represents the current application state
type AppState int

const (
	StateInitializing AppState = iota
	StateReady
	StateRunning
	StateCompleted
	StateError
)

// FocusedPanel represents which panel currently has focus
type FocusedPanel int

const (
	PanelWorkflows FocusedPanel = iota
	PanelTools
	PanelLogs
	PanelStatus
)

// LayoutMode represents responsive layout modes
type LayoutMode int

const (
	LayoutLarge  LayoutMode = iota // ≥120 cols: [Nav] | [Main] | [Status]
	LayoutMedium                   // 80-119 cols: [Nav] | [Main+Status] 
	LayoutSmall                    // 40-79 cols: Single panel with tabs
)

// Data structures for application state
type WorkflowData struct {
	ID          string
	Description string
	Status      string
	Progress    float64
	Duration    time.Duration
	StartTime   time.Time
	Error       error
}

type ToolData struct {
	Name       string
	WorkflowID string
	Status     string
	Duration   time.Duration
	Args       []string
	Output     string
	Error      error
}

type LogEntry struct {
	Timestamp time.Time
	Level     string
	Category  string
	Message   string
}

// KeyMap defines keybindings
type KeyMap struct {
	Quit      key.Binding
	Help      key.Binding
	NextPanel key.Binding
	PrevPanel key.Binding
	Up        key.Binding
	Down      key.Binding
	Enter     key.Binding
	Tab       key.Binding
}

// DefaultKeyMap returns the default keybindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		NextPanel: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next panel"),
		),
		PrevPanel: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev panel"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Tab: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "next view"),
		),
	}
}

// ShortHelp returns keybindings to be shown in the mini help view
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

// FullHelp returns keybindings for the expanded help view
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.NextPanel, k.PrevPanel},
		{k.Enter, k.Help, k.Quit},
	}
}

// NewModel creates a new TUI model with proper initialization
func NewModel(target string) Model {
	ctx, cancel := context.WithCancel(context.Background())

	// Load configuration
	config, err := LoadDefaultConfig()
	if err != nil {
		// Fall back to default config if loading fails
		config = DefaultConfig()
	}

	// Initialize components with minimal sizes (will be updated on first WindowSizeMsg)
	workflowList := list.New([]list.Item{}, NewWorkflowDelegate(), 20, 10)
	workflowList.Title = "Workflows"
	workflowList.SetShowStatusBar(false)
	workflowList.SetFilteringEnabled(false)

	toolTable := table.New(
		table.WithColumns([]table.Column{
			{Title: "Tool", Width: 15},
			{Title: "Status", Width: 10},
			{Title: "Duration", Width: 10},
		}),
		table.WithFocused(false),
		table.WithHeight(5),
	)

	logViewport := viewport.New(30, 10)
	logViewport.SetContent("Waiting for output...")

	// Initialize progress bar
	prog := progress.New(progress.WithDefaultGradient())

	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Initialize help
	h := help.New()

	return Model{
		ready:        false, // CRITICAL: Start with ready=false
		target:       target,
		config:       config,
		state:        StateInitializing,
		workflowList: workflowList,
		toolTable:    toolTable,
		logViewport:  logViewport,
		progress:     prog,
		spinner:      s,
		help:         h,
		currentPanel: PanelWorkflows,
		layoutMode:   LayoutLarge, // Will be recalculated on WindowSizeMsg
		showHelp:     false,
		workflows:    []WorkflowData{},
		tools:        []ToolData{},
		logs:         []LogEntry{},
		keyMap:       DefaultKeyMap(),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Init implements tea.Model interface
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return TickMsg{Time: t}
		}),
	)
}

// Update implements tea.Model interface  
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// CRITICAL: This is the pattern from research - handle WindowSizeMsg first
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true // Only now are we ready to render
		
		// Update layout mode based on new size
		m.layoutMode = m.determineLayoutMode(msg.Width)
		
		// Update component sizes using proper Lipgloss calculations
		m.updateComponentSizes()
		
		return m, nil

	case tea.KeyMsg:
		// Only process keys if we're ready
		if !m.ready {
			return m, nil
		}
		
		// Handle global keys first
		switch {
		case key.Matches(msg, m.keyMap.Quit):
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
			
		case key.Matches(msg, m.keyMap.Help):
			m.showHelp = !m.showHelp
			return m, nil
			
		case key.Matches(msg, m.keyMap.NextPanel):
			m.nextPanel()
			return m, nil
			
		case key.Matches(msg, m.keyMap.PrevPanel):
			m.prevPanel()
			return m, nil
		}
		
		// Route to focused component
		return m.updateFocusedComponent(msg)

	case WorkflowUpdateMsg:
		return m.handleWorkflowUpdate(msg), nil

	case ToolUpdateMsg:
		return m.handleToolUpdate(msg), nil

	case LogMsg:
		return m.handleLogMessage(msg), nil

	case TickMsg:
		// Update spinner
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
		
		// Schedule next tick
		cmds = append(cmds, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return TickMsg{Time: t}
		}))
		
		return m, tea.Batch(cmds...)
	}

	return m, nil
}

// View implements tea.Model interface
func (m Model) View() string {
	// CRITICAL: WindowSizeMsg guard pattern from research
	if !m.ready {
		return "\n  Initializing..."
	}
	
	// Check minimum terminal size using configuration
	if m.width < m.config.Terminal.Constraints.MinimumWidth || m.height < m.config.Terminal.Constraints.MinimumHeight {
		return m.renderTooSmallMessage()
	}
	
	// Render based on layout mode
	content := m.renderLayout()
	
	// Add help panel if enabled
	if m.showHelp {
		help := m.renderHelpPanel()
		content = lipgloss.JoinVertical(lipgloss.Left, content, help)
	}
	
	// Ensure final content respects terminal boundaries
	finalContent := lipgloss.NewStyle().
		MaxWidth(m.width).
		MaxHeight(m.height).
		Render(content)
	
	return finalContent
}

// determineLayoutMode calculates layout mode based on width using configuration
func (m Model) determineLayoutMode(width int) LayoutMode {
	if width >= m.config.Layout.Breakpoints.Large {
		return LayoutLarge
	} else if width >= m.config.Layout.Breakpoints.Medium {
		return LayoutMedium
	} else {
		return LayoutSmall
	}
}

// updateComponentSizes updates component dimensions using GetFrameSize pattern
func (m *Model) updateComponentSizes() {
	switch m.layoutMode {
	case LayoutLarge:
		m.updateLargeLayoutSizes()
	case LayoutMedium:
		m.updateMediumLayoutSizes()
	case LayoutSmall:
		m.updateSmallLayoutSizes()
	}
}

// updateLargeLayoutSizes calculates sizes for three-panel layout using configuration
func (m *Model) updateLargeLayoutSizes() {
	// Use configuration ratios instead of magic numbers
	navWidth := int(float64(m.width) * m.config.Layout.Panels.Navigation.PreferredWidthRatio)
	statusWidth := int(float64(m.width) * m.config.Layout.Panels.Status.PreferredWidthRatio)
	mainWidth := m.width - navWidth - statusWidth
	
	// Ensure minimum widths
	if navWidth < m.config.Layout.Panels.Navigation.MinWidth {
		navWidth = m.config.Layout.Panels.Navigation.MinWidth
	}
	if statusWidth < m.config.Layout.Panels.Status.MinWidth {
		statusWidth = m.config.Layout.Panels.Status.MinWidth
	}
	if mainWidth < m.config.Layout.Panels.Main.MinWidth {
		mainWidth = m.config.Layout.Panels.Main.MinWidth
	}
	
	// Account for header and borders using configuration
	headerHeight := 0
	if m.height > m.config.Layout.Header.ShowWhenHeightAbove {
		headerHeight = m.config.Layout.Header.Height
	}
	contentHeight := m.height - headerHeight - m.config.Layout.Footer.Height - m.config.Layout.Spacing.Medium
	
	// Update workflow list (left panel)
	borderPadding := m.config.Layout.Borders.Width
	m.workflowList.SetSize(navWidth-borderPadding, contentHeight)
	
	// Update tool table (main panel, top half)
	tableHeight := contentHeight / 2
	m.toolTable.SetHeight(tableHeight)
	
	// Update log viewport (main panel, bottom half)
	logHeight := contentHeight - tableHeight
	m.logViewport.Width = mainWidth - borderPadding
	m.logViewport.Height = logHeight
}

// updateMediumLayoutSizes calculates sizes for two-panel layout using configuration
func (m *Model) updateMediumLayoutSizes() {
	// In medium layout, navigation gets more space, no separate status panel
	navWidth := int(float64(m.width) * 0.33) // 33%
	mainWidth := m.width - navWidth          // 67%
	
	// Ensure minimum widths
	if navWidth < m.config.Layout.Panels.Navigation.MinWidth {
		navWidth = m.config.Layout.Panels.Navigation.MinWidth
		mainWidth = m.width - navWidth
	}
	if mainWidth < m.config.Layout.Panels.Main.MinWidth {
		mainWidth = m.config.Layout.Panels.Main.MinWidth
		navWidth = m.width - mainWidth
	}
	
	// Account for header and footer using configuration
	headerHeight := 0
	if m.height > m.config.Layout.Header.ShowWhenHeightAbove {
		headerHeight = m.config.Layout.Header.Height
	}
	contentHeight := m.height - headerHeight - m.config.Layout.Footer.Height - m.config.Layout.Spacing.Medium
	
	// Update workflow list (left panel)
	borderPadding := m.config.Layout.Borders.Width
	m.workflowList.SetSize(navWidth-borderPadding, contentHeight)
	
	// Update tool table (main panel, top half)
	tableHeight := contentHeight / 2
	m.toolTable.SetHeight(tableHeight)
	
	// Update log viewport (main panel, bottom half)
	logHeight := contentHeight - tableHeight
	m.logViewport.Width = mainWidth - borderPadding
	m.logViewport.Height = logHeight
}

// updateSmallLayoutSizes calculates sizes for single-panel layout using configuration
func (m *Model) updateSmallLayoutSizes() {
	borderPadding := m.config.Layout.Borders.Width
	contentWidth := m.width - borderPadding
	
	// Account for header and tab bar using configuration
	headerHeight := 0
	if m.height > m.config.Layout.Header.ShowWhenHeightAbove {
		headerHeight = m.config.Layout.Header.Height
	}
	tabBarHeight := 2 // Small layout always shows tab bar
	contentHeight := m.height - headerHeight - tabBarHeight - m.config.Layout.Spacing.Small
	
	// All components use full width in small layout
	m.workflowList.SetSize(contentWidth, contentHeight)
	m.toolTable.SetHeight(contentHeight)
	m.logViewport.Width = contentWidth
	m.logViewport.Height = contentHeight
}

// renderLayout renders the main layout using Lipgloss
func (m Model) renderLayout() string {
	switch m.layoutMode {
	case LayoutLarge:
		return m.renderLargeLayout()
	case LayoutMedium:
		return m.renderMediumLayout()
	case LayoutSmall:
		return m.renderSmallLayout()
	default:
		return "Unknown layout mode"
	}
}

// renderLargeLayout renders three-panel layout: [Nav] | [Main] | [Status]
func (m Model) renderLargeLayout() string {
	// Use layout calculator for precise dimensions
	calc := NewLayoutCalculator(m.config, m.width, m.height, LayoutLarge)
	dims := calc.CalculatePanelDimensions()
	
	// Render panels with exact constraints
	nav := calc.RenderConstrainedPanel(
		m.renderWorkflowContent(dims.NavContentWidth, dims.NavContentHeight),
		dims.NavWidth,
		dims.NavHeight,
		m.currentPanel == PanelWorkflows,
	)
	
	main := calc.RenderConstrainedPanel(
		m.renderMainContent(dims.MainContentWidth, dims.MainContentHeight),
		dims.MainWidth,
		dims.MainHeight,
		m.currentPanel == PanelTools || m.currentPanel == PanelLogs,
	)
	
	status := calc.RenderConstrainedPanel(
		m.renderStatusContent(dims.StatusContentWidth, dims.StatusContentHeight),
		dims.StatusWidth,
		dims.StatusHeight,
		m.currentPanel == PanelStatus,
	)
	
	// Join panels horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, nav, main, status)
}

// renderMediumLayout renders two-panel layout: [Nav] | [Main]
func (m Model) renderMediumLayout() string {
	// Calculate panel widths for medium layout
	navWidth := int(float64(m.width) * 0.33) // 33%
	mainWidth := m.width - navWidth          // 67%
	
	// Ensure minimum widths
	if navWidth < m.config.Layout.Panels.Navigation.MinWidth {
		navWidth = m.config.Layout.Panels.Navigation.MinWidth
		mainWidth = m.width - navWidth
	}
	if mainWidth < m.config.Layout.Panels.Main.MinWidth {
		mainWidth = m.config.Layout.Panels.Main.MinWidth
		navWidth = m.width - mainWidth
	}
	
	// Create panels with explicit width constraints
	nav := lipgloss.NewStyle().Width(navWidth).Render(m.renderWorkflowPanel())
	main := lipgloss.NewStyle().Width(mainWidth).Render(m.renderMainPanel())
	status := m.renderStatusFooter()
	
	// Two panels horizontally, then add status footer
	panels := lipgloss.JoinHorizontal(lipgloss.Top, nav, main)
	
	// Constrain final width
	panelsConstrained := lipgloss.NewStyle().MaxWidth(m.width).Render(panels)
	
	return lipgloss.JoinVertical(lipgloss.Left, panelsConstrained, status)
}

// renderSmallLayout renders single-panel layout with tabs
func (m Model) renderSmallLayout() string {
	var content string
	
	// Show current panel based on focus
	switch m.currentPanel {
	case PanelWorkflows:
		content = m.renderWorkflowPanel()
	case PanelTools:
		content = m.renderToolPanel()
	case PanelLogs:
		content = m.renderLogPanel()
	default:
		content = m.renderWorkflowPanel()
	}
	
	// Add tab navigation bar
	tabs := m.renderTabBar()
	
	// Constrain content to terminal width
	contentConstrained := lipgloss.NewStyle().MaxWidth(m.width).Render(content)
	tabsConstrained := lipgloss.NewStyle().MaxWidth(m.width).Render(tabs)
	
	return lipgloss.JoinVertical(lipgloss.Left, tabsConstrained, contentConstrained)
}

// Component rendering methods
func (m Model) renderWorkflowPanel() string {
	title := "Workflows"
	if m.currentPanel == PanelWorkflows {
		title = "> " + title + " <"
	}
	
	border := lipgloss.RoundedBorder()
	borderColor := lipgloss.Color(m.config.Layout.Borders.Color)
	if m.currentPanel == PanelWorkflows {
		borderColor = lipgloss.Color(m.config.Theme.Colors.Accent)
	}
	
	// Create base style
	style := lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderColor).
		Padding(1)
	
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render(title),
		m.workflowList.View(),
	)
	
	return style.Render(content)
}

// Content rendering methods with exact dimensions
func (m Model) renderWorkflowContent(width, height int) string {
	title := "Workflows"
	if m.currentPanel == PanelWorkflows {
		title = "> " + title + " <"
	}
	
	// Constrain workflow list to exact dimensions
	titleHeight := 1
	listHeight := height - titleHeight - 1 // -1 for spacing
	if listHeight < 1 {
		listHeight = 1
	}
	
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Width(width).
		Render(title)
	
	// Create a constrained workflow list view
	listContent := m.workflowList.View()
	listStyle := lipgloss.NewStyle().
		Width(width).
		Height(listHeight).
		Render(listContent)
	
	return lipgloss.JoinVertical(lipgloss.Left, titleStyle, listStyle)
}

func (m Model) renderMainContent(width, height int) string {
	// Split between tool table and logs
	toolHeight := height / 2
	logHeight := height - toolHeight
	
	toolContent := m.renderToolContent(width, toolHeight)
	logContent := m.renderLogContent(width, logHeight)
	
	return lipgloss.JoinVertical(lipgloss.Left, toolContent, logContent)
}

func (m Model) renderToolContent(width, height int) string {
	title := "Tools"
	if m.currentPanel == PanelTools {
		title = "> " + title + " <"
	}
	
	titleHeight := 1
	tableHeight := height - titleHeight - 1
	if tableHeight < 1 {
		tableHeight = 1
	}
	
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Width(width).
		Render(title)
	
	tableContent := m.toolTable.View()
	tableStyle := lipgloss.NewStyle().
		Width(width).
		Height(tableHeight).
		Render(tableContent)
	
	return lipgloss.JoinVertical(lipgloss.Left, titleStyle, tableStyle)
}

func (m Model) renderLogContent(width, height int) string {
	title := "Logs"
	if m.currentPanel == PanelLogs {
		title = "> " + title + " <"
	}
	
	titleHeight := 1
	logHeight := height - titleHeight - 1
	if logHeight < 1 {
		logHeight = 1
	}
	
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Width(width).
		Render(title)
	
	logContent := m.logViewport.View()
	logStyle := lipgloss.NewStyle().
		Width(width).
		Height(logHeight).
		Render(logContent)
	
	return lipgloss.JoinVertical(lipgloss.Left, titleStyle, logStyle)
}

func (m Model) renderStatusContent(width, height int) string {
	title := "Status"
	if m.currentPanel == PanelStatus {
		title = "> " + title + " <"
	}
	
	// Status content
	var content []string
	content = append(content, fmt.Sprintf("Target: %s", m.target))
	content = append(content, fmt.Sprintf("Workflows: %d", len(m.workflows)))
	content = append(content, fmt.Sprintf("Tools: %d", len(m.tools)))
	
	// Add spinner if running
	if m.state == StateRunning {
		content = append(content, "")
		content = append(content, m.spinner.View()+" Running...")
	}
	
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Width(width).
		Render(title)
	
	statusText := lipgloss.JoinVertical(lipgloss.Left, content...)
	statusStyle := lipgloss.NewStyle().
		Width(width).
		Height(height-2). // Account for title
		Render(statusText)
	
	return lipgloss.JoinVertical(lipgloss.Left, titleStyle, statusStyle)
}

func (m Model) renderMainPanel() string {
	// Split between tool table and logs
	toolPanel := m.renderToolPanel()
	logPanel := m.renderLogPanel()
	
	return lipgloss.JoinVertical(lipgloss.Left, toolPanel, logPanel)
}

func (m Model) renderToolPanel() string {
	title := "Tools"
	if m.currentPanel == PanelTools {
		title = "> " + title + " <"
	}
	
	border := lipgloss.RoundedBorder()
	borderColor := lipgloss.Color("#444444")
	if m.currentPanel == PanelTools {
		borderColor = lipgloss.Color("#888888")
	}
	
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render(title),
		m.toolTable.View(),
	)
	
	return lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderColor).
		Padding(1).
		Render(content)
}

func (m Model) renderLogPanel() string {
	title := "Logs"
	if m.currentPanel == PanelLogs {
		title = "> " + title + " <"
	}
	
	border := lipgloss.RoundedBorder()
	borderColor := lipgloss.Color("#444444")
	if m.currentPanel == PanelLogs {
		borderColor = lipgloss.Color("#888888")
	}
	
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render(title),
		m.logViewport.View(),
	)
	
	return lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderColor).
		Padding(1).
		Render(content)
}

func (m Model) renderStatusPanel() string {
	title := "Status"
	if m.currentPanel == PanelStatus {
		title = "> " + title + " <"
	}
	
	// Status content
	var content []string
	content = append(content, fmt.Sprintf("Target: %s", m.target))
	content = append(content, fmt.Sprintf("Workflows: %d", len(m.workflows)))
	content = append(content, fmt.Sprintf("Tools: %d", len(m.tools)))
	
	// Add spinner if running
	if m.state == StateRunning {
		content = append(content, "")
		content = append(content, m.spinner.View()+" Running...")
	}
	
	border := lipgloss.RoundedBorder()
	borderColor := lipgloss.Color("#444444")
	if m.currentPanel == PanelStatus {
		borderColor = lipgloss.Color("#888888")
	}
	
	statusContent := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render(title),
		lipgloss.JoinVertical(lipgloss.Left, content...),
	)
	
	return lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderColor).
		Padding(1).
		Render(statusContent)
}

func (m Model) renderStatusFooter() string {
	content := fmt.Sprintf("Target: %s | Workflows: %d | Tools: %d", 
		m.target, len(m.workflows), len(m.tools))
	
	if m.state == StateRunning {
		content = m.spinner.View() + " " + content
	}
	
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#444444")).
		Padding(0, 1).
		Render(content)
}

func (m Model) renderTabBar() string {
	var tabs []string
	
	panels := []struct {
		name  string
		panel FocusedPanel
	}{
		{"Workflows", PanelWorkflows},
		{"Tools", PanelTools},
		{"Logs", PanelLogs},
	}
	
	for _, p := range panels {
		if p.panel == m.currentPanel {
			tabs = append(tabs, fmt.Sprintf("[%s]", p.name))
		} else {
			tabs = append(tabs, p.name)
		}
	}
	
	content := lipgloss.JoinHorizontal(lipgloss.Left, tabs...)
	help := "Space: next • ?: help • q: quit"
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render(help),
	)
}

func (m Model) renderHelpPanel() string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#888888")).
		Padding(1).
		Render(m.help.View(m.keyMap))
}

func (m Model) renderTooSmallMessage() string {
	msg := fmt.Sprintf("Terminal too small: %dx%d\nMinimum required: 40x10", 
		m.width, m.height)
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ff0000")).
		Bold(true).
		Render(msg)
}

// Panel navigation
func (m *Model) nextPanel() {
	switch m.layoutMode {
	case LayoutLarge:
		switch m.currentPanel {
		case PanelWorkflows:
			m.currentPanel = PanelTools
		case PanelTools:
			m.currentPanel = PanelLogs
		case PanelLogs:
			m.currentPanel = PanelStatus
		case PanelStatus:
			m.currentPanel = PanelWorkflows
		}
	case LayoutMedium:
		switch m.currentPanel {
		case PanelWorkflows:
			m.currentPanel = PanelTools
		case PanelTools:
			m.currentPanel = PanelLogs
		case PanelLogs:
			m.currentPanel = PanelWorkflows
		default:
			m.currentPanel = PanelWorkflows
		}
	case LayoutSmall:
		switch m.currentPanel {
		case PanelWorkflows:
			m.currentPanel = PanelTools
		case PanelTools:
			m.currentPanel = PanelLogs
		case PanelLogs:
			m.currentPanel = PanelWorkflows
		default:
			m.currentPanel = PanelWorkflows
		}
	}
}

func (m *Model) prevPanel() {
	switch m.layoutMode {
	case LayoutLarge:
		switch m.currentPanel {
		case PanelWorkflows:
			m.currentPanel = PanelStatus
		case PanelStatus:
			m.currentPanel = PanelLogs
		case PanelLogs:
			m.currentPanel = PanelTools
		case PanelTools:
			m.currentPanel = PanelWorkflows
		}
	case LayoutMedium:
		switch m.currentPanel {
		case PanelWorkflows:
			m.currentPanel = PanelLogs
		case PanelLogs:
			m.currentPanel = PanelTools
		case PanelTools:
			m.currentPanel = PanelWorkflows
		default:
			m.currentPanel = PanelWorkflows
		}
	case LayoutSmall:
		switch m.currentPanel {
		case PanelWorkflows:
			m.currentPanel = PanelLogs
		case PanelLogs:
			m.currentPanel = PanelTools
		case PanelTools:
			m.currentPanel = PanelWorkflows
		default:
			m.currentPanel = PanelWorkflows
		}
	}
}

// updateFocusedComponent routes key events to the currently focused component
func (m Model) updateFocusedComponent(msg tea.KeyMsg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	
	switch m.currentPanel {
	case PanelWorkflows:
		m.workflowList, cmd = m.workflowList.Update(msg)
	case PanelTools:
		m.toolTable, cmd = m.toolTable.Update(msg)
	case PanelLogs:
		m.logViewport, cmd = m.logViewport.Update(msg)
	}
	
	return m, cmd
}

// Message handling methods
func (m Model) handleWorkflowUpdate(msg WorkflowUpdateMsg) Model {
	// Find existing workflow or create new one
	for i := range m.workflows {
		if m.workflows[i].ID == msg.ID {
			m.workflows[i].Status = msg.Status
			m.workflows[i].Progress = msg.Progress
			m.workflows[i].Duration = msg.Duration
			if msg.Error != nil {
				m.workflows[i].Error = msg.Error
			}
			return m.updateWorkflowList()
		}
	}
	
	// New workflow
	workflow := WorkflowData{
		ID:          msg.ID,
		Description: msg.Description,
		Status:      msg.Status,
		Progress:    msg.Progress,
		Duration:    msg.Duration,
		StartTime:   time.Now(),
		Error:       msg.Error,
	}
	m.workflows = append(m.workflows, workflow)
	
	return m.updateWorkflowList()
}

func (m Model) handleToolUpdate(msg ToolUpdateMsg) Model {
	tool := ToolData{
		Name:       msg.Name,
		WorkflowID: msg.WorkflowID,
		Status:     msg.Status,
		Duration:   msg.Duration,
		Args:       msg.Args,
		Output:     msg.Output,
		Error:      msg.Error,
	}
	
	m.tools = append(m.tools, tool)
	
	// Keep only recent tools
	if len(m.tools) > 100 {
		m.tools = m.tools[1:]
	}
	
	return m.updateToolTable()
}

func (m Model) handleLogMessage(msg LogMsg) Model {
	entry := LogEntry{
		Timestamp: msg.Timestamp,
		Level:     msg.Level,
		Category:  msg.Category,
		Message:   msg.Message,
	}
	
	m.logs = append(m.logs, entry)
	
	// Keep only recent logs
	if len(m.logs) > 1000 {
		m.logs = m.logs[1:]
	}
	
	return m.updateLogViewport()
}

// Component update methods
func (m Model) updateWorkflowList() Model {
	items := make([]list.Item, len(m.workflows))
	for i, workflow := range m.workflows {
		items[i] = WorkflowItem{
			id:          workflow.ID,
			description: workflow.Description,
			status:      workflow.Status,
			progress:    workflow.Progress,
		}
	}
	m.workflowList.SetItems(items)
	return m
}

func (m Model) updateToolTable() Model {
	rows := make([]table.Row, 0)
	
	// Show recent tools only
	start := 0
	if len(m.tools) > 20 {
		start = len(m.tools) - 20
	}
	
	for _, tool := range m.tools[start:] {
		status := tool.Status
		if tool.Error != nil {
			status = "failed"
		}
		
		rows = append(rows, table.Row{
			tool.Name,
			status,
			tool.Duration.String(),
		})
	}
	
	m.toolTable.SetRows(rows)
	return m
}

func (m Model) updateLogViewport() Model {
	var content []string
	
	// Show recent logs only
	start := 0
	if len(m.logs) > 50 {
		start = len(m.logs) - 50
	}
	
	for _, log := range m.logs[start:] {
		timestamp := log.Timestamp.Format("15:04:05")
		line := fmt.Sprintf("[%s] %s: %s", timestamp, log.Level, log.Message)
		content = append(content, line)
	}
	
	if len(content) == 0 {
		content = []string{"Waiting for output..."}
	}
	
	m.logViewport.SetContent(lipgloss.JoinVertical(lipgloss.Left, content...))
	m.logViewport.GotoBottom()
	
	return m
}

// Context for cancellation
func (m Model) Context() context.Context {
	return m.ctx
}

func (m *Model) Cancel() {
	if m.cancel != nil {
		m.cancel()
	}
}