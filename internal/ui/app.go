package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/key"
	"github.com/your-org/ipcrawler/internal/simulator"
	"github.com/your-org/ipcrawler/internal/ui/components"
	"github.com/your-org/ipcrawler/internal/ui/keybindings"
	"github.com/your-org/ipcrawler/internal/ui/styles"
)

// LayoutMode defines the current responsive layout mode
type LayoutMode int

const (
	LayoutSmall  LayoutMode = iota // <80 cols: stacked
	LayoutMedium                   // 80-119 cols: two-column
	LayoutLarge                    // ≥120 cols: three-column
)

// FocusedPanel defines which panel currently has focus
type FocusedPanel int

const (
	FocusListPanel FocusedPanel = iota
	FocusViewportPanel
	FocusStatusPanel
)

// App is the root Bubble Tea model managing all UI state
// CRITICAL: This is the ONLY tea.Model in the application
// All rendering goes through this single model
type App struct {
	// Terminal state - MUST wait for WindowSizeMsg before rendering
	ready  bool
	width  int
	height int

	// Layout state
	layout  LayoutMode
	focused FocusedPanel

	// Child components - all implement the Component interface
	listPanel   *components.ListPanel
	viewport    *components.ViewportPanel
	statusPanel *components.StatusPanel

	// Layout manager for responsive design
	layoutManager *LayoutManager

	// Configuration loaded from ui.yaml
	config *Config

	// Simulator interface for demo content
	simulator simulator.Simulator

	// Key bindings
	keys keybindings.KeyMap

	// Theme and styling
	theme *styles.Theme
}

// NewApp creates a new root application model
func NewApp(config *Config, sim simulator.Simulator) *App {
	// Initialize theme from config
	theme := styles.NewTheme(config)

	// Create layout manager
	layoutManager := NewLayoutManager(config, theme)

	// Initialize child components
	listPanel := components.NewListPanel(config, sim, theme)
	viewport := components.NewViewportPanel(config, theme)
	statusPanel := components.NewStatusPanel(config, theme)

	return &App{
		config:        config,
		simulator:     sim,
		listPanel:     listPanel,
		viewport:      viewport,
		statusPanel:   statusPanel,
		layoutManager: layoutManager,
		keys:          keybindings.NewKeyMap(config),
		theme:         theme,
		focused:       FocusListPanel,
	}
}

// Init initializes the application and child components
func (a *App) Init() tea.Cmd {
	// Batch initialization commands from all components
	return tea.Batch(
		a.listPanel.Init(),
		a.viewport.Init(),
		a.statusPanel.Init(),
		a.simulator.StreamLogs(), // Start log streaming
	)
}

// Update handles messages and updates the application state
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// CRITICAL: Handle resize first before any rendering
		// This is the foundation of resize-safe architecture
		a.width, a.height = msg.Width, msg.Height
		a.ready = true
		a.layout = a.calculateLayout()

		// Route WindowSizeMsg to all child components
		cmds = append(cmds, a.routeWindowSizeToChildren(msg))

	case tea.KeyMsg:
		// Handle global key bindings first
		switch {
		case key.Matches(msg, a.keys.Quit):
			return a, tea.Quit
		case key.Matches(msg, a.keys.Tab):
			a.cycleFocus()
			return a, nil
		case key.Matches(msg, a.keys.FocusList):
			a.focused = FocusListPanel
			return a, nil
		case key.Matches(msg, a.keys.FocusMain):
			a.focused = FocusViewportPanel
			return a, nil
		case key.Matches(msg, a.keys.FocusStatus):
			a.focused = FocusStatusPanel
			return a, nil
		}

		// Route to focused component
		cmd := a.routeKeyToFocused(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	default:
		// Route other messages to appropriate components
		cmds = append(cmds, a.routeMessageToComponents(msg))
	}

	return a, tea.Batch(cmds...)
}

// View renders the complete application UI
func (a *App) View() string {
	// CRITICAL: Don't render until WindowSizeMsg received
	// This prevents layout calculations with zero dimensions
	if !a.ready {
		return "Initializing IPCrawler TUI..."
	}

	// Use layout manager to render appropriate layout
	return a.layoutManager.RenderLayout(a)
}

// calculateLayout determines layout mode based on terminal width
func (a *App) calculateLayout() LayoutMode {
	switch {
	case a.width >= a.config.UI.Layout.Breakpoints.Medium:
		return LayoutLarge
	case a.width >= a.config.UI.Layout.Breakpoints.Small:
		return LayoutMedium
	default:
		return LayoutSmall
	}
}

// calculatePanelSizes returns panel dimensions for current layout
func (a *App) calculatePanelSizes() (leftW, mainW, rightW, contentH int) {
	contentH = a.height - 4 // Reserve space for borders

	switch a.layout {
	case LayoutLarge:
		leftW = int(float64(a.width) * a.config.UI.Layout.Panels.List.WidthLarge)
		rightW = int(float64(a.width) * a.config.UI.Layout.Panels.Status.WidthLarge)
		mainW = a.width - leftW - rightW - 6 // Minus borders/margins

	case LayoutMedium:
		leftW = int(float64(a.width) * a.config.UI.Layout.Panels.List.WidthMedium)
		rightW = 0
		mainW = a.width - leftW - 4
		contentH = a.height - a.config.UI.Layout.Panels.Status.HeightFooter - 2

	case LayoutSmall:
		leftW = a.width - 4
		mainW = a.width - 4
		rightW = a.width - 4
		contentH = a.height - 12 // Multiple panels stacked
	}

	return
}

// cycleFocus moves focus to the next panel
func (a *App) cycleFocus() {
	switch a.focused {
	case FocusListPanel:
		a.focused = FocusViewportPanel
	case FocusViewportPanel:
		if a.layout == LayoutLarge {
			a.focused = FocusStatusPanel
		} else {
			a.focused = FocusListPanel
		}
	case FocusStatusPanel:
		a.focused = FocusListPanel
	}
}

// routeWindowSizeToChildren routes WindowSizeMsg to all child components
func (a *App) routeWindowSizeToChildren(msg tea.WindowSizeMsg) tea.Cmd {
	leftW, mainW, rightW, contentH := a.calculatePanelSizes()

	var cmds []tea.Cmd

	// Resize list panel
	if a.listPanel != nil {
		cmd := a.listPanel.SetSize(leftW, contentH)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Resize viewport panel
	if a.viewport != nil {
		cmd := a.viewport.SetSize(mainW, contentH)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Resize status panel
	if a.statusPanel != nil {
		cmd := a.statusPanel.SetSize(rightW, contentH)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return tea.Batch(cmds...)
}

// routeKeyToFocused routes key messages to the focused component
func (a *App) routeKeyToFocused(msg tea.KeyMsg) tea.Cmd {
	switch a.focused {
	case FocusListPanel:
		if a.listPanel != nil {
			_, cmd := a.listPanel.Update(msg)
			return cmd
		}
	case FocusViewportPanel:
		if a.viewport != nil {
			_, cmd := a.viewport.Update(msg)
			return cmd
		}
	case FocusStatusPanel:
		if a.statusPanel != nil {
			_, cmd := a.statusPanel.Update(msg)
			return cmd
		}
	}
	return nil
}

// routeMessageToComponents routes other messages to appropriate components
func (a *App) routeMessageToComponents(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	// Route to all components that might handle this message type
	if a.listPanel != nil {
		_, cmd := a.listPanel.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if a.viewport != nil {
		_, cmd := a.viewport.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if a.statusPanel != nil {
		_, cmd := a.statusPanel.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return tea.Batch(cmds...)
}

// Getters for layout manager
func (a *App) GetWidth() int          { return a.width }
func (a *App) GetHeight() int         { return a.height }
func (a *App) GetLayout() LayoutMode  { return a.layout }
func (a *App) GetFocused() FocusedPanel { return a.focused }
func (a *App) GetListPanel() *components.ListPanel { return a.listPanel }
func (a *App) GetViewport() *components.ViewportPanel { return a.viewport }
func (a *App) GetStatusPanel() *components.StatusPanel { return a.statusPanel }