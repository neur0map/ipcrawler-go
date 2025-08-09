package components

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/your-org/ipcrawler/internal/simulator"
	"github.com/your-org/ipcrawler/internal/ui/styles"
)

// Config interface to avoid import cycle
type Config interface {
	GetListConfig() ListConfig
}

type ListConfig struct {
	Title            string
	ShowStatusBar    bool
	FilteringEnabled bool
	ItemHeight       int
}

// ListPanel manages the left panel showing workflows and tools
type ListPanel struct {
	config    Config
	simulator simulator.Simulator
	theme     *styles.Theme

	// Bubbles list component
	list list.Model

	// State
	width  int
	height int
	ready  bool
	
	// View mode: "workflows" or "tools"
	viewMode string
}

// NewListPanel creates a new list panel component
func NewListPanel(config Config, sim simulator.Simulator, theme *styles.Theme) *ListPanel {
	// Create Bubbles list with custom styling
	delegate := list.NewDefaultDelegate()
	
	// Configure delegate styling from theme
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Foreground(theme.Colors.Accent).
		Bold(true)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(theme.Colors.Secondary).
		Italic(true)
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(theme.Colors.TextPrimary)
	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		Foreground(theme.Colors.Secondary)

	// Set item height from config
	delegate.SetHeight(config.UI.Components.List.ItemHeight)

	// Initialize list model
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = config.UI.Components.List.Title
	l.SetShowStatusBar(config.UI.Components.List.ShowStatusBar)
	l.SetFilteringEnabled(config.UI.Components.List.FilteringEnabled)

	// Apply list styling from theme
	l.Styles.Title = theme.TitleStyle
	l.Styles.FilterPrompt = lipgloss.NewStyle().
		Foreground(theme.Colors.Accent)
	l.Styles.FilterCursor = lipgloss.NewStyle().
		Foreground(theme.Colors.Accent)

	lp := &ListPanel{
		config:    config,
		simulator: sim,
		theme:     theme,
		list:      l,
		viewMode:  "workflows", // Start with workflows
	}

	// Load initial data
	lp.loadWorkflows()

	return lp
}

// Init initializes the list panel component
func (lp *ListPanel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the list panel
func (lp *ListPanel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle view mode switching
		switch msg.String() {
		case "w":
			lp.switchToWorkflows()
			return lp, nil
		case "t":
			lp.switchToTools()
			return lp, nil
		case "enter", " ":
			// Handle item selection
			if lp.list.SelectedItem() != nil {
				cmd := lp.handleItemSelection()
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}

	case simulator.LogStreamMsg:
		// Handle log updates (could affect workflow status)
		// For now, just pass through
		break
	}

	// Update the underlying list
	var cmd tea.Cmd
	lp.list, cmd = lp.list.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return lp, tea.Batch(cmds...)
}

// View renders the list panel
func (lp *ListPanel) View() string {
	if !lp.ready {
		return lp.theme.TitleStyle.Render("Loading...")
	}

	// Add view mode indicator
	modeIndicator := lp.renderModeIndicator()
	
	// Render list with mode indicator
	listView := lp.list.View()
	
	// Combine mode indicator with list
	return lipgloss.JoinVertical(
		lipgloss.Left,
		modeIndicator,
		listView,
	)
}

// SetSize updates the panel dimensions
func (lp *ListPanel) SetSize(width, height int) tea.Cmd {
	lp.width = width
	lp.height = height
	lp.ready = true

	// Update list size (accounting for borders and mode indicator)
	listHeight := height - 4 // Reserve space for borders and mode indicator
	lp.list.SetSize(width-4, listHeight)

	return nil
}

// loadWorkflows loads workflow items into the list
func (lp *ListPanel) loadWorkflows() {
	workflows := lp.simulator.GetWorkflows()
	items := make([]list.Item, len(workflows))
	
	for i, workflow := range workflows {
		items[i] = workflow
	}
	
	lp.list.SetItems(items)
	lp.list.Title = "Workflows & Tools"
	lp.viewMode = "workflows"
}

// loadTools loads tool items into the list
func (lp *ListPanel) loadTools() {
	tools := lp.simulator.GetTools()
	items := make([]list.Item, len(tools))
	
	for i, tool := range tools {
		items[i] = tool
	}
	
	lp.list.SetItems(items)
	lp.list.Title = "Available Tools"
	lp.viewMode = "tools"
}

// switchToWorkflows switches the list to show workflows
func (lp *ListPanel) switchToWorkflows() {
	lp.loadWorkflows()
}

// switchToTools switches the list to show tools
func (lp *ListPanel) switchToTools() {
	lp.loadTools()
}

// handleItemSelection handles when an item is selected
func (lp *ListPanel) handleItemSelection() tea.Cmd {
	selectedItem := lp.list.SelectedItem()
	if selectedItem == nil {
		return nil
	}

	switch lp.viewMode {
	case "workflows":
		if workflow, ok := selectedItem.(simulator.WorkflowItem); ok {
			return lp.simulator.ExecuteWorkflow(workflow.ID)
		}
	case "tools":
		if tool, ok := selectedItem.(simulator.ToolItem); ok {
			return lp.simulator.ExecuteTool(tool.ID, map[string]interface{}{})
		}
	}

	return nil
}

// renderModeIndicator renders the current view mode
func (lp *ListPanel) renderModeIndicator() string {
	workflowStyle := lipgloss.NewStyle().
		Foreground(lp.theme.Colors.Secondary).
		Padding(0, 1)
	
	toolStyle := lipgloss.NewStyle().
		Foreground(lp.theme.Colors.Secondary).
		Padding(0, 1)
	
	if lp.viewMode == "workflows" {
		workflowStyle = workflowStyle.
			Foreground(lp.theme.Colors.Accent).
			Bold(true)
	} else {
		toolStyle = toolStyle.
			Foreground(lp.theme.Colors.Accent).
			Bold(true)
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		workflowStyle.Render("Workflows(w)"),
		lipgloss.NewStyle().Foreground(lp.theme.Colors.Border).Render(" | "),
		toolStyle.Render("Tools(t)"),
	)
}

// GetSelectedItem returns the currently selected item
func (lp *ListPanel) GetSelectedItem() list.Item {
	return lp.list.SelectedItem()
}

// GetViewMode returns the current view mode
func (lp *ListPanel) GetViewMode() string {
	return lp.viewMode
}

// Refresh reloads the current view's data
func (lp *ListPanel) Refresh() {
	switch lp.viewMode {
	case "workflows":
		lp.loadWorkflows()
	case "tools":
		lp.loadTools()
	}
}