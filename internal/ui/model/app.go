package model

import (
	"context"
	"fmt"
	"strings"
	"sync"
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
	"github.com/carlosm/ipcrawler/internal/ui/accessibility"
	"github.com/carlosm/ipcrawler/internal/ui/keymap"
	"github.com/carlosm/ipcrawler/internal/ui/layout"
	"github.com/carlosm/ipcrawler/internal/ui/theme"
)

// AppModel is the main Bubble Tea model for the IPCrawler TUI
type AppModel struct {
	// Core state
	mu           sync.RWMutex
	ctx          context.Context
	cancelFunc   context.CancelFunc
	
	// Layout and display
	layout       *layout.Layout
	styles       theme.Styles
	focused      FocusedComponent
	showHelp     bool
	terminalSize tea.WindowSizeMsg
	
	// Accessibility
	accessibility *accessibility.AccessibilityManager
	
	// Application state
	state        AppState
	target       string
	config       AppConfig
	
	// Components
	components   ComponentState
	
	// Data
	workflows    []WorkflowStatus
	tools        []ToolStatus
	logs         []LogEntry
	systemStats  SystemStats
	notifications []Notification
	
	// Keyboard and interaction
	keyMap       keymap.KeyMap
	keyContext   keymap.KeyMapContext
	
	// UI state
	currentView  string // for small layout tab switching
	lastUpdate   time.Time
	
	// Error handling
	lastError    error
	errorShown   bool
}

// NewAppModel creates a new application model
func NewAppModel(target string) AppModel {
	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	
	// Initialize with default terminal size
	initialSize := tea.WindowSizeMsg{Width: 80, Height: 24}
	
	// Create layout
	layout := layout.New(initialSize.Width, initialSize.Height)
	
	// Initialize accessibility manager
	accessibilityMgr := accessibility.NewAccessibilityManager()
	
	// Create styles from configuration (with accessibility adaptations)
	styles := theme.NewConfiguredStyles()
	
	// Initialize components
	components := initializeComponents(layout)
	
	// Default config
	config := AppConfig{
		Theme:             "monochrome",
		AnimationsEnabled: true,
		RefreshInterval:   time.Second,
		LogLevel:         "info",
		MaxLogEntries:    1000,
		ShowHelp:         false,
		ColorDisabled:    false,
	}
	
	return AppModel{
		ctx:          ctx,
		cancelFunc:   cancel,
		layout:       layout,
		styles:       *styles,
		accessibility: accessibilityMgr,
		focused:      FocusWorkflowList,
		showHelp:     false,
		terminalSize: initialSize,
		state:        StateInitializing,
		target:       target,
		config:       config,
		components:   components,
		workflows:    []WorkflowStatus{},
		tools:        []ToolStatus{},
		logs:         []LogEntry{},
		notifications: []Notification{},
		keyMap:       keymap.DefaultKeyMap(),
		keyContext:   keymap.GlobalContext,
		currentView:  "workflows",
		lastUpdate:   time.Now(),
	}
}

// Init implements tea.Model interface
func (m AppModel) Init() tea.Cmd {
	return tea.Batch(
		m.components.Spinner.Tick,
		tea.EnterAltScreen,
		func() tea.Msg {
			return InitCompleteMsg{}
		},
	)
}

// Update implements tea.Model interface
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Handle global messages first
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleResize(msg)
		
	case accessibility.DebouncedResizeMsg:
		return m.handleResize(msg.WindowSizeMsg)
		
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
		
	case InitCompleteMsg:
		m.state = StateReady
		return m, func() tea.Msg {
			return LogMsg{
				Level:     "info",
				Message:   "IPCrawler TUI initialized",
				Timestamp: time.Now(),
				Category:  "system",
			}
		}
		
	case WorkflowUpdateMsg:
		return m.handleWorkflowUpdate(msg)
		
	case ToolExecutionMsg:
		return m.handleToolExecution(msg)
		
	case LogMsg:
		return m.handleLogMessage(msg)
		
	case SystemStatsMsg:
		m.systemStats = SystemStats(msg)
		return m, nil
		
	case ErrorMsg:
		return m.handleError(msg)
		
	case QuitMsg:
		return m.handleQuit()
		
	case TickMsg:
		return m.handleTick(msg)
		
	default:
		// Handle component-specific updates
		return m.updateComponents(msg)
	}
}

// View implements tea.Model interface
func (m AppModel) View() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.terminalSize.Width == 0 || m.terminalSize.Height == 0 {
		return "Initializing..."
	}
	
	// Check for terminal size requirements
	if m.terminalSize.Width < 40 || m.terminalSize.Height < 10 {
		return m.renderTooSmallMessage()
	}
	
	// Render based on layout mode
	content := m.renderMainContent()
	
	// Add header if space allows
	if m.terminalSize.Height > 15 {
		header := m.renderHeader()
		content = m.layout.RenderWithHeader(header, content)
	}
	
	// Add help panel if enabled
	if m.showHelp {
		help := m.renderHelp()
		content = lipgloss.JoinVertical(lipgloss.Left, content, help)
	}
	
	return content
}

// handleResize handles terminal resize events
func (m AppModel) handleResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	// Check if resize should be debounced
	if shouldDebounce, debounceCmd := m.accessibility.ShouldDebounceResize(msg); shouldDebounce {
		return m, debounceCmd
	}
	
	// Get previous mode for comparison
	oldMode := m.layout.Mode()
	
	// Update terminal size
	m.terminalSize = msg
	
	// Validate minimum size for accessibility
	minWidth, minHeight := m.accessibility.GetRecommendedMinimumSize()
	if msg.Width < minWidth || msg.Height < minHeight {
		// Generate accessibility warning
		return m, func() tea.Msg {
			return LogMsg{
				Level:     "warning",
				Message:   fmt.Sprintf("Terminal size (%dx%d) below recommended minimum (%dx%d) for accessibility", msg.Width, msg.Height, minWidth, minHeight),
				Timestamp: time.Now(),
				Category:  "accessibility",
			}
		}
	}
	
	// Update layout
	m.layout.Update(msg.Width, msg.Height)
	
	// Update component sizes
	m.updateComponentSizes()
	
	// Check for layout mode change
	newMode := m.layout.Mode()
	
	// Generate accessibility announcement for screen readers
	if oldMode != newMode {
		announcement := m.accessibility.GetAccessibilityAnnouncement("layout_change", map[string]interface{}{
			"mode": newMode.String(),
		})
		
		if announcement != "" {
			// Send accessibility announcement
			return m, func() tea.Msg {
				return LogMsg{
					Level:     "info",
					Message:   announcement,
					Timestamp: time.Now(),
					Category:  "accessibility",
				}
			}
		}
	}
	
	// Log layout change
	cmd := func() tea.Msg {
		return LogMsg{
			Level:     "debug",
			Message:   fmt.Sprintf("Layout changed to %s mode (%dx%d)", newMode.String(), msg.Width, msg.Height),
			Timestamp: time.Now(),
			Category:  "ui",
		}
	}
	
	return m, cmd
}

// handleKeyPress handles keyboard input
func (m AppModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle global keys first
	if keymap.IsGlobalKey(msg) {
		return m.handleGlobalKeys(msg)
	}
	
	// Handle panel switching
	if keymap.IsPanelSwitchKey(msg) {
		return m.handlePanelSwitch(msg)
	}
	
	// Route to focused component
	return m.routeKeyToComponent(msg)
}

// handleGlobalKeys processes globally available keys
func (m AppModel) handleGlobalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keyMap.Quit):
		if m.cancelFunc != nil {
			m.cancelFunc()
		}
		return m, tea.Quit
		
	case key.Matches(msg, m.keyMap.Help):
		m.showHelp = !m.showHelp
		return m, nil
		
	case key.Matches(msg, m.keyMap.Refresh):
		return m, func() tea.Msg {
			return DataRefreshMsg{Component: "all"}
		}
	}
	
	return m, nil
}

// handlePanelSwitch handles focus switching between panels
func (m AppModel) handlePanelSwitch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keyMap.NextPanel):
		m.cycleFocusForward()
		
	case key.Matches(msg, m.keyMap.PrevPanel):
		m.cycleFocusBackward()
		
	case key.Matches(msg, m.keyMap.FocusNav):
		m.focused = FocusWorkflowList
		
	case key.Matches(msg, m.keyMap.FocusMain):
		m.focused = FocusWorkflowTable
		
	case key.Matches(msg, m.keyMap.FocusStatus):
		m.focused = FocusStatusPanel
	}
	
	m.updateComponentFocus()
	
	return m, func() tea.Msg {
		return LogMsg{
			Level:     "debug",
			Message:   fmt.Sprintf("Focus changed to %s", m.focused.String()),
			Timestamp: time.Now(),
			Category:  "ui",
		}
	}
}

// routeKeyToComponent routes keys to the currently focused component
func (m AppModel) routeKeyToComponent(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	
	switch m.focused {
	case FocusWorkflowList:
		m.components.WorkflowList, cmd = m.components.WorkflowList.Update(msg)
		
	case FocusWorkflowTable:
		m.components.WorkflowTable, cmd = m.components.WorkflowTable.Update(msg)
		
	case FocusOutputViewport:
		m.components.OutputViewport, cmd = m.components.OutputViewport.Update(msg)
		
	case FocusHelpPanel:
		m.components.Help, cmd = m.components.Help.Update(msg)
	}
	
	return m, cmd
}

// cycleFocusForward moves focus to the next component
func (m *AppModel) cycleFocusForward() {
	switch m.layout.Mode() {
	case layout.LargeLayout:
		// Cycle through all components
		switch m.focused {
		case FocusWorkflowList:
			m.focused = FocusWorkflowTable
		case FocusWorkflowTable:
			m.focused = FocusOutputViewport
		case FocusOutputViewport:
			m.focused = FocusStatusPanel
		case FocusStatusPanel:
			m.focused = FocusWorkflowList
		default:
			m.focused = FocusWorkflowList
		}
		
	case layout.MediumLayout:
		// Cycle through nav, main, and viewport
		switch m.focused {
		case FocusWorkflowList:
			m.focused = FocusWorkflowTable
		case FocusWorkflowTable:
			m.focused = FocusOutputViewport
		case FocusOutputViewport:
			m.focused = FocusWorkflowList
		default:
			m.focused = FocusWorkflowList
		}
		
	case layout.SmallLayout:
		// Switch between different views
		m.switchSmallLayoutView()
	}
}

// cycleFocusBackward moves focus to the previous component
func (m *AppModel) cycleFocusBackward() {
	switch m.layout.Mode() {
	case layout.LargeLayout:
		switch m.focused {
		case FocusWorkflowList:
			m.focused = FocusStatusPanel
		case FocusStatusPanel:
			m.focused = FocusOutputViewport
		case FocusOutputViewport:
			m.focused = FocusWorkflowTable
		case FocusWorkflowTable:
			m.focused = FocusWorkflowList
		default:
			m.focused = FocusWorkflowList
		}
		
	case layout.MediumLayout:
		switch m.focused {
		case FocusWorkflowList:
			m.focused = FocusOutputViewport
		case FocusOutputViewport:
			m.focused = FocusWorkflowTable
		case FocusWorkflowTable:
			m.focused = FocusWorkflowList
		default:
			m.focused = FocusWorkflowList
		}
		
	case layout.SmallLayout:
		m.switchSmallLayoutViewBackward()
	}
}

// switchSmallLayoutView changes the current view in small layout mode
func (m *AppModel) switchSmallLayoutView() {
	switch m.currentView {
	case "workflows":
		m.currentView = "table"
		m.focused = FocusWorkflowTable
	case "table":
		m.currentView = "logs"
		m.focused = FocusOutputViewport
	case "logs":
		m.currentView = "workflows"
		m.focused = FocusWorkflowList
	default:
		m.currentView = "workflows"
		m.focused = FocusWorkflowList
	}
}

// switchSmallLayoutViewBackward changes view backward in small layout
func (m *AppModel) switchSmallLayoutViewBackward() {
	switch m.currentView {
	case "workflows":
		m.currentView = "logs"
		m.focused = FocusOutputViewport
	case "logs":
		m.currentView = "table"
		m.focused = FocusWorkflowTable
	case "table":
		m.currentView = "workflows"
		m.focused = FocusWorkflowList
	default:
		m.currentView = "workflows"
		m.focused = FocusWorkflowList
	}
}

// updateComponentFocus updates focus state in components
func (m *AppModel) updateComponentFocus() {
	// Update table focus
	if m.focused == FocusWorkflowTable {
		m.components.WorkflowTable.Focus()
	} else {
		m.components.WorkflowTable.Blur()
	}
	
	// Workflow list maintains its own focus via bubbles
	// Viewport updates automatically
}

// handleWorkflowUpdate processes workflow status updates
func (m AppModel) handleWorkflowUpdate(msg WorkflowUpdateMsg) (tea.Model, tea.Cmd) {
	// Find existing workflow or create new one
	found := false
	for i := range m.workflows {
		if m.workflows[i].ID == msg.WorkflowID {
			m.workflows[i].Status = msg.Status
			m.workflows[i].Progress = msg.Progress
			m.workflows[i].Description = msg.Description
			m.workflows[i].Duration = msg.Duration
			m.workflows[i].Error = msg.Error
			if msg.Status == "running" && m.workflows[i].StartTime.IsZero() {
				m.workflows[i].StartTime = msg.StartTime
			}
			if msg.Status == "completed" || msg.Status == "failed" {
				m.workflows[i].EndTime = time.Now()
			}
			found = true
			break
		}
	}
	
	if !found {
		workflow := WorkflowStatus{
			ID:          msg.WorkflowID,
			Status:      msg.Status,
			Progress:    msg.Progress,
			Description: msg.Description,
			Duration:    msg.Duration,
			Error:       msg.Error,
			StartTime:   msg.StartTime,
		}
		m.workflows = append(m.workflows, workflow)
	}
	
	// Update components
	m.updateWorkflowList()
	m.updateWorkflowTable()
	
	return m, nil
}

// handleToolExecution processes tool execution updates
func (m AppModel) handleToolExecution(msg ToolExecutionMsg) (tea.Model, tea.Cmd) {
	tool := ToolStatus{
		Name:       msg.ToolName,
		WorkflowID: msg.WorkflowID,
		Status:     msg.Status,
		Progress:   msg.Progress,
		Duration:   msg.Duration,
		Output:     msg.Output,
		Error:      msg.Error,
		Args:       msg.Args,
	}
	
	// Add to tools list (keeping only recent entries)
	m.tools = append(m.tools, tool)
	if len(m.tools) > 100 { // Keep last 100 tool executions
		m.tools = m.tools[1:]
	}
	
	// Update table
	m.updateToolTable()
	
	return m, nil
}

// handleLogMessage processes log entries
func (m AppModel) handleLogMessage(msg LogMsg) (tea.Model, tea.Cmd) {
	// Add to logs (keeping only recent entries)
	m.logs = append(m.logs, LogEntry{
		Level:     msg.Level,
		Message:   msg.Message,
		Timestamp: msg.Timestamp,
		Category:  msg.Category,
		Data:      msg.Data,
	})
	
	// Keep only recent logs
	if len(m.logs) > m.config.MaxLogEntries {
		m.logs = m.logs[1:]
	}
	
	// Update viewport
	m.updateOutputViewport()
	
	return m, nil
}

// handleError processes error messages
func (m AppModel) handleError(msg ErrorMsg) (tea.Model, tea.Cmd) {
	m.lastError = msg.Error
	m.errorShown = false
	
	// Add error notification
	notification := Notification{
		Level:   NotificationError,
		Title:   "Error",
		Message: msg.Error.Error(),
		Timeout: 5 * time.Second,
		Created: time.Now(),
	}
	m.notifications = append(m.notifications, notification)
	
	// Log the error
	return m, func() tea.Msg {
		return LogMsg{
			Level:     "error",
			Message:   fmt.Sprintf("%s: %s", msg.Context, msg.Error.Error()),
			Timestamp: time.Now(),
			Category:  "system",
		}
	}
}

// handleQuit processes quit messages
func (m AppModel) handleQuit() (tea.Model, tea.Cmd) {
	if m.cancelFunc != nil {
		m.cancelFunc()
	}
	m.state = StateShuttingDown
	return m, tea.Quit
}

// handleTick processes periodic updates
func (m AppModel) handleTick(msg TickMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	
	// Update spinner
	var cmd tea.Cmd
	m.components.Spinner, cmd = m.components.Spinner.Update(msg)
	cmds = append(cmds, cmd)
	
	// Clean up old notifications
	m.cleanupNotifications()
	
	// Schedule next tick
	cmds = append(cmds, tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg{Time: t}
	}))
	
	return m, tea.Batch(cmds...)
}

// updateComponents handles component-specific updates
func (m AppModel) updateComponents(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd
	
	// Update individual components
	m.components.Spinner, cmd = m.components.Spinner.Update(msg)
	cmds = append(cmds, cmd)
	
	m.components.Help, cmd = m.components.Help.Update(msg)
	cmds = append(cmds, cmd)
	
	return m, tea.Batch(cmds...)
}

// renderMainContent renders the main content based on layout mode
func (m AppModel) renderMainContent() string {
	switch m.layout.Mode() {
	case layout.LargeLayout:
		return m.renderThreePanelLayout()
	case layout.MediumLayout:
		return m.renderTwoPanelLayout()
	case layout.SmallLayout:
		return m.renderSinglePanelLayout()
	default:
		return "Unknown layout mode"
	}
}

// renderThreePanelLayout renders the large layout using proper Lipgloss joining
func (m AppModel) renderThreePanelLayout() string {
	// Calculate dimensions for proper layout
	sizes := m.layout.CalculateComponentSizes()
	
	// Render panels with proper sizing
	nav := m.renderNavigationPanel()
	main := m.renderMainPanel()
	status := m.renderStatusPanel()
	
	// Use Lipgloss JoinHorizontal for proper layout
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(sizes["workflow_list"].Width).Render(nav),
		lipgloss.NewStyle().Width(sizes["workflow_table"].Width).Render(main),
		lipgloss.NewStyle().Width(sizes["status_panel"].Width).Render(status),
	)
}

// renderTwoPanelLayout renders the medium layout using proper Lipgloss joining
func (m AppModel) renderTwoPanelLayout() string {
	// Calculate dimensions for proper layout
	sizes := m.layout.CalculateComponentSizes()
	
	// Render panels with proper sizing
	nav := m.renderNavigationPanel()
	main := m.renderMainPanel()
	
	// Use Lipgloss JoinHorizontal for proper layout
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(sizes["workflow_list"].Width).Render(nav),
		lipgloss.NewStyle().Width(sizes["workflow_table"].Width + sizes["output_viewport"].Width).Render(main),
	)
}

// renderSinglePanelLayout renders the small layout
func (m AppModel) renderSinglePanelLayout() string {
	// Calculate dimensions for proper layout
	sizes := m.layout.CalculateComponentSizes()
	
	var content string
	
	// Show current view with navigation hints
	switch m.currentView {
	case "workflows":
		content = m.renderNavigationPanel()
	case "table":
		content = m.renderTablePanel()
	case "logs":
		content = m.renderLogPanel()
	default:
		content = m.renderNavigationPanel()
	}
	
	// Add view navigation footer
	footer := m.renderSmallLayoutFooter()
	
	// Use proper Lipgloss vertical joining with height management
	fullHeight := sizes["workflow_list"].Height + 3 // Add space for footer
	return lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Height(fullHeight-3).Render(content),
		footer,
	)
}

// renderNavigationPanel renders the workflow list panel
func (m AppModel) renderNavigationPanel() string {
	// Create title with focus indicator
	title := "Workflows"
	if m.focused == FocusWorkflowList {
		title = "> " + title + " <"
	}
	
	// Get the workflow list content
	listContent := m.components.WorkflowList.View()
	
	// Create bordered panel using simple Lipgloss styling
	border := lipgloss.RoundedBorder()
	borderColor := lipgloss.Color("#444444")
	if m.focused == FocusWorkflowList {
		borderColor = lipgloss.Color("#888888")
	}
	
	return lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderColor).
		Padding(1).
		Render(lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render(title),
			listContent,
		))
}

// renderMainPanel renders the main content panel
func (m AppModel) renderMainPanel() string {
	// Split between table and logs using proper Lipgloss vertical joining
	table := m.renderTablePanel()
	logs := m.renderLogPanel()
	
	// Calculate available space
	sizes := m.layout.CalculateComponentSizes()
	tableHeight := sizes["workflow_table"].Height
	logHeight := sizes["output_viewport"].Height
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Height(tableHeight).Render(table),
		lipgloss.NewStyle().Height(logHeight).Render(logs),
	)
}

// renderTablePanel renders the workflow/tool table
func (m AppModel) renderTablePanel() string {
	// Create title with focus indicator
	title := "Tool Executions"
	if m.focused == FocusWorkflowTable {
		title = "> " + title + " <"
	}
	
	// Get the table content
	tableContent := m.components.WorkflowTable.View()
	
	// Create bordered panel using simple Lipgloss styling
	border := lipgloss.RoundedBorder()
	borderColor := lipgloss.Color("#444444")
	if m.focused == FocusWorkflowTable {
		borderColor = lipgloss.Color("#888888")
	}
	
	return lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderColor).
		Padding(1).
		Render(lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render(title),
			tableContent,
		))
}

// renderLogPanel renders the log output panel
func (m AppModel) renderLogPanel() string {
	// Create title with focus indicator
	title := "Output & Logs"
	if m.focused == FocusOutputViewport {
		title = "> " + title + " <"
	}
	
	// Get the viewport content
	viewportContent := m.components.OutputViewport.View()
	
	// Create bordered panel using simple Lipgloss styling
	border := lipgloss.RoundedBorder()
	borderColor := lipgloss.Color("#444444")
	if m.focused == FocusOutputViewport {
		borderColor = lipgloss.Color("#888888")
	}
	
	return lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderColor).
		Padding(1).
		Render(lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render(title),
			viewportContent,
		))
}

// renderStatusPanel renders the status information panel
func (m AppModel) renderStatusPanel() string {
	// Create title with focus indicator
	title := "Status"
	if m.focused == FocusStatusPanel {
		title = "> " + title + " <"
	}
	
	// Get the status content
	statusContent := m.renderStatusContent()
	
	// Create bordered panel using simple Lipgloss styling
	border := lipgloss.RoundedBorder()
	borderColor := lipgloss.Color("#444444")
	if m.focused == FocusStatusPanel {
		borderColor = lipgloss.Color("#888888")
	}
	
	return lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderColor).
		Padding(1).
		Render(lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render(title),
			statusContent,
		))
}

// renderStatusContent renders the status panel content
func (m AppModel) renderStatusContent() string {
	var content []string
	
	// Workflow counts
	running, completed, failed := m.getWorkflowCounts()
	content = append(content, fmt.Sprintf("Running: %d", running))
	content = append(content, fmt.Sprintf("Completed: %d", completed))
	content = append(content, fmt.Sprintf("Failed: %d", failed))
	
	// System stats if available
	if !m.systemStats.Timestamp.IsZero() {
		content = append(content, "")
		content = append(content, "System:")
		content = append(content, fmt.Sprintf("CPU: %.1f%%", m.systemStats.CPUUsage))
		content = append(content, fmt.Sprintf("Memory: %.1f%%", m.systemStats.MemoryUsage))
	}
	
	// Current focus indicator
	content = append(content, "")
	content = append(content, fmt.Sprintf("Focus: %s", m.focused.String()))
	
	return strings.Join(content, "\n")
}

// renderHeader renders the application header
func (m AppModel) renderHeader() string {
	title := fmt.Sprintf("IPCrawler TUI - Target: %s", m.target)
	
	// Add status indicator
	var status string
	if len(m.workflows) > 0 {
		running, completed, failed := m.getWorkflowCounts()
		if running > 0 {
			status = fmt.Sprintf("%s %d running, %d completed, %d failed", 
				m.components.Spinner.View(), running, completed, failed)
		} else {
			status = fmt.Sprintf("✓ %d completed, %d failed", completed, failed)
		}
	} else {
		status = "Ready"
	}
	
	titleStyled := m.styles.Title.Render(title)
	statusStyled := m.styles.TextSecondary.Render(status)
	
	return lipgloss.JoinVertical(lipgloss.Left, titleStyled, statusStyled)
}

// renderSmallLayoutFooter renders navigation hints for small layout
func (m AppModel) renderSmallLayoutFooter() string {
	views := []string{"workflows", "table", "logs"}
	var indicators []string
	
	for _, view := range views {
		if view == m.currentView {
			indicators = append(indicators, fmt.Sprintf("[%s]", view))
		} else {
			indicators = append(indicators, view)
		}
	}
	
	navigation := lipgloss.JoinHorizontal(lipgloss.Left, indicators...)
	help := "Tab: switch • ?: help • q: quit"
	
	return m.styles.Footer.Render(
		lipgloss.JoinVertical(lipgloss.Left, navigation, help))
}

// renderHelp renders the help panel
func (m AppModel) renderHelp() string {
	return m.styles.Panel.Render(m.components.Help.View(m.keyMap))
}

// renderTooSmallMessage renders a message when terminal is too small
func (m AppModel) renderTooSmallMessage() string {
	msg := fmt.Sprintf("Terminal too small: %dx%d\nMinimum required: 40x10", 
		m.terminalSize.Width, m.terminalSize.Height)
	return m.styles.TextError.Render(msg)
}

// Utility methods

// initializeComponents creates and configures all Bubbles components
func initializeComponents(layout *layout.Layout) ComponentState {
	// Calculate component sizes
	sizes := layout.CalculateComponentSizes()
	
	// Initialize workflow list
	workflowList := list.New([]list.Item{}, list.NewDefaultDelegate(), 
		sizes["workflow_list"].Width, sizes["workflow_list"].Height)
	workflowList.Title = "Workflows"
	workflowList.SetShowStatusBar(false)
	workflowList.SetFilteringEnabled(true)
	
	// Initialize workflow table
	workflowColumns := []table.Column{
		{Title: "Tool", Width: 15},
		{Title: "Workflow", Width: 20},
		{Title: "Status", Width: 10},
		{Title: "Duration", Width: 12},
		{Title: "Output", Width: 30},
	}
	
	workflowTable := table.New(
		table.WithColumns(workflowColumns),
		table.WithFocused(false),
		table.WithHeight(sizes["workflow_table"].Height),
	)
	
	// Initialize output viewport
	outputViewport := viewport.New(
		sizes["output_viewport"].Width, 
		sizes["output_viewport"].Height)
	outputViewport.SetContent("Waiting for output...")
	
	// Initialize progress bar
	progressBar := progress.New(progress.WithDefaultGradient())
	
	// Initialize spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	
	// Initialize help
	helpModel := help.New()
	helpModel.ShowAll = false
	
	return ComponentState{
		WorkflowList:    workflowList,
		WorkflowTable:   workflowTable,
		OutputViewport:  outputViewport,
		ProgressBar:     progressBar,
		Spinner:         sp,
		Help:            helpModel,
		StatusPanel:     StatusPanelState{},
		NavPanel:        NavPanelState{},
		MainPanel:       MainPanelState{CurrentView: "table", AutoScroll: true},
	}
}

// updateComponentSizes updates component dimensions when layout changes
func (m *AppModel) updateComponentSizes() {
	sizes := m.layout.CalculateComponentSizes()
	
	// Update workflow list
	if size, ok := sizes["workflow_list"]; ok {
		m.components.WorkflowList.SetSize(size.Width, size.Height)
	}
	
	// Update workflow table
	if size, ok := sizes["workflow_table"]; ok {
		m.components.WorkflowTable.SetHeight(size.Height)
	}
	
	// Update output viewport
	if size, ok := sizes["output_viewport"]; ok {
		m.components.OutputViewport.Width = size.Width
		m.components.OutputViewport.Height = size.Height
	}
}

// updateWorkflowList updates the workflow list component
func (m *AppModel) updateWorkflowList() {
	items := make([]list.Item, len(m.workflows))
	for i, workflow := range m.workflows {
		items[i] = WorkflowListItem{Workflow: workflow}
	}
	m.components.WorkflowList.SetItems(items)
}

// updateWorkflowTable updates the workflow table component
func (m *AppModel) updateWorkflowTable() {
	// Get recent tool executions
	start := 0
	if len(m.tools) > 50 {
		start = len(m.tools) - 50
	}
	
	rows := make([]table.Row, len(m.tools)-start)
	for i, tool := range m.tools[start:] {
		rows[i] = tool.ToTableRow()
	}
	
	m.components.WorkflowTable.SetRows(rows)
}

// updateToolTable updates the tool table component (alias for updateWorkflowTable)
func (m *AppModel) updateToolTable() {
	m.updateWorkflowTable()
}

// updateOutputViewport updates the output viewport with recent logs
func (m *AppModel) updateOutputViewport() {
	var content []string
	
	// Get recent logs
	start := 0
	if len(m.logs) > 100 {
		start = len(m.logs) - 100
	}
	
	for _, log := range m.logs[start:] {
		timestamp := log.Timestamp.Format("15:04:05")
		level := log.Level
		if level == "error" {
			level = "ERROR"
		} else if level == "warn" {
			level = "WARN"
		} else {
			level = strings.ToUpper(level)
		}
		
		line := fmt.Sprintf("[%s] %s: %s", timestamp, level, log.Message)
		content = append(content, line)
	}
	
	if len(content) == 0 {
		content = []string{"Waiting for output..."}
	}
	
	m.components.OutputViewport.SetContent(strings.Join(content, "\n"))
	
	// Auto-scroll to bottom if enabled
	if m.components.MainPanel.AutoScroll {
		m.components.OutputViewport.GotoBottom()
	}
}

// getWorkflowCounts returns workflow status counts
func (m *AppModel) getWorkflowCounts() (running, completed, failed int) {
	for _, workflow := range m.workflows {
		switch workflow.Status {
		case "running":
			running++
		case "completed":
			completed++
		case "failed":
			failed++
		}
	}
	return
}

// cleanupNotifications removes expired notifications
func (m *AppModel) cleanupNotifications() {
	now := time.Now()
	var active []Notification
	
	for _, notification := range m.notifications {
		if notification.Timeout > 0 && now.Sub(notification.Created) < notification.Timeout {
			active = append(active, notification)
		} else if notification.Timeout == 0 {
			// Permanent notification
			active = append(active, notification)
		}
	}
	
	m.notifications = active
}

// SetCancelFunc sets the cancellation function for graceful shutdown
func (m *AppModel) SetCancelFunc(cancel context.CancelFunc) {
	m.cancelFunc = cancel
}

// GetContext returns the application context
func (m *AppModel) GetContext() context.Context {
	return m.ctx
}

// GetState returns the current application state
func (m *AppModel) GetState() AppState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// GetTarget returns the target being scanned
func (m *AppModel) GetTarget() string {
	return m.target
}

// IsRunning returns true if workflows are actively running
func (m *AppModel) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for _, workflow := range m.workflows {
		if workflow.Status == "running" {
			return true
		}
	}
	return false
}

// GetWorkflows returns a copy of current workflows
func (m *AppModel) GetWorkflows() []WorkflowStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	workflows := make([]WorkflowStatus, len(m.workflows))
	copy(workflows, m.workflows)
	return workflows
}

// AddWorkflow adds a new workflow to the model
func (m *AppModel) AddWorkflow(workflow WorkflowStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.workflows = append(m.workflows, workflow)
	m.updateWorkflowList()
}

// UpdateWorkflow updates an existing workflow
func (m *AppModel) UpdateWorkflow(workflowID string, update func(*WorkflowStatus)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for i := range m.workflows {
		if m.workflows[i].ID == workflowID {
			update(&m.workflows[i])
			m.updateWorkflowList()
			return
		}
	}
}

// AddLogEntry adds a new log entry
func (m *AppModel) AddLogEntry(entry LogEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.logs = append(m.logs, entry)
	if len(m.logs) > m.config.MaxLogEntries {
		m.logs = m.logs[1:]
	}
	m.updateOutputViewport()
}

// SetSystemStats updates system statistics
func (m *AppModel) SetSystemStats(stats SystemStats) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.systemStats = stats
}

// GetLastError returns the last error encountered
func (m *AppModel) GetLastError() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastError
}

// ClearError clears the last error
func (m *AppModel) ClearError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastError = nil
	m.errorShown = false
}