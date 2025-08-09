package keymap

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// KeyMapContext defines different keyboard contexts
type KeyMapContext int

const (
	GlobalContext KeyMapContext = iota
	NavigationContext
	TableContext
	ListContext
	ViewportContext
	HelpContext
)

// KeyMap defines all keybindings for the application
type KeyMap struct {
	// Navigation
	Up    key.Binding
	Down  key.Binding
	Left  key.Binding
	Right key.Binding

	// Selection and interaction
	Select    key.Binding
	Confirm   key.Binding
	Cancel    key.Binding
	Back      key.Binding

	// Panel navigation
	NextPanel key.Binding
	PrevPanel key.Binding
	FocusNav  key.Binding
	FocusMain key.Binding
	FocusStatus key.Binding

	// Application controls
	Quit    key.Binding
	Help    key.Binding
	Refresh key.Binding

	// Viewport controls
	PageUp     key.Binding
	PageDown   key.Binding
	GoToTop    key.Binding
	GoToBottom key.Binding

	// List controls
	FilterToggle key.Binding
	ClearFilter  key.Binding

	// Table controls
	SortToggle   key.Binding
	ColumnNext   key.Binding
	ColumnPrev   key.Binding

	// Workflow controls
	StartWorkflow  key.Binding
	StopWorkflow   key.Binding
	RestartWorkflow key.Binding

	// View toggles
	ToggleLogs    key.Binding
	ToggleTable   key.Binding
	ToggleDetails key.Binding

	// Debug controls
	DebugToggle key.Binding
	DebugClear  key.Binding
}

// DefaultKeyMap returns the default key mappings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		// Navigation
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

		// Selection and interaction
		Select: key.NewBinding(
			key.WithKeys(" ", "space"),
			key.WithHelp("space", "select/toggle"),
		),
		Confirm: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
		Back: key.NewBinding(
			key.WithKeys("backspace"),
			key.WithHelp("backspace", "go back"),
		),

		// Panel navigation
		NextPanel: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next panel"),
		),
		PrevPanel: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "previous panel"),
		),
		FocusNav: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "focus navigation"),
		),
		FocusMain: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "focus main content"),
		),
		FocusStatus: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "focus status panel"),
		),

		// Application controls
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r", "f5"),
			key.WithHelp("r", "refresh"),
		),

		// Viewport controls
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "b"),
			key.WithHelp("pgup/b", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "f"),
			key.WithHelp("pgdn/f", "page down"),
		),
		GoToTop: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home/g", "go to top"),
		),
		GoToBottom: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "go to bottom"),
		),

		// List controls
		FilterToggle: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "toggle filter"),
		),
		ClearFilter: key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("ctrl+u", "clear filter"),
		),

		// Table controls
		SortToggle: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "sort column"),
		),
		ColumnNext: key.NewBinding(
			key.WithKeys("ctrl+right"),
			key.WithHelp("ctrl+→", "next column"),
		),
		ColumnPrev: key.NewBinding(
			key.WithKeys("ctrl+left"),
			key.WithHelp("ctrl+←", "prev column"),
		),

		// Workflow controls
		StartWorkflow: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "start workflow"),
		),
		StopWorkflow: key.NewBinding(
			key.WithKeys("ctrl+x"),
			key.WithHelp("ctrl+x", "stop workflow"),
		),
		RestartWorkflow: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "restart workflow"),
		),

		// View toggles
		ToggleLogs: key.NewBinding(
			key.WithKeys("v", "l"),
			key.WithHelp("v/l", "view logs"),
		),
		ToggleTable: key.NewBinding(
			key.WithKeys("v", "t"),
			key.WithHelp("v/t", "view table"),
		),
		ToggleDetails: key.NewBinding(
			key.WithKeys("v", "d"),
			key.WithHelp("v/d", "view details"),
		),

		// Debug controls
		DebugToggle: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("ctrl+d", "toggle debug"),
		),
		DebugClear: key.NewBinding(
			key.WithKeys("ctrl+shift+d"),
			key.WithHelp("ctrl+shift+d", "clear debug"),
		),
	}
}

// GlobalKeyMap returns keys that work in all contexts
func GlobalKeyMap() KeyMap {
	km := DefaultKeyMap()
	return KeyMap{
		Quit:      km.Quit,
		Help:      km.Help,
		Refresh:   km.Refresh,
		NextPanel: km.NextPanel,
		PrevPanel: km.PrevPanel,
		FocusNav:  km.FocusNav,
		FocusMain: km.FocusMain,
		FocusStatus: km.FocusStatus,
	}
}

// NavigationKeyMap returns keys for navigation contexts
func NavigationKeyMap() KeyMap {
	km := DefaultKeyMap()
	return KeyMap{
		Up:           km.Up,
		Down:         km.Down,
		Select:       km.Select,
		Confirm:      km.Confirm,
		Cancel:       km.Cancel,
		FilterToggle: km.FilterToggle,
		ClearFilter:  km.ClearFilter,
	}
}

// TableKeyMap returns keys for table contexts
func TableKeyMap() KeyMap {
	km := DefaultKeyMap()
	return KeyMap{
		Up:         km.Up,
		Down:       km.Down,
		Left:       km.Left,
		Right:      km.Right,
		Select:     km.Select,
		Confirm:    km.Confirm,
		SortToggle: km.SortToggle,
		ColumnNext: km.ColumnNext,
		ColumnPrev: km.ColumnPrev,
	}
}

// ViewportKeyMap returns keys for viewport contexts
func ViewportKeyMap() KeyMap {
	km := DefaultKeyMap()
	return KeyMap{
		Up:         km.Up,
		Down:       km.Down,
		PageUp:     km.PageUp,
		PageDown:   km.PageDown,
		GoToTop:    km.GoToTop,
		GoToBottom: km.GoToBottom,
	}
}

// WorkflowKeyMap returns keys for workflow management
func WorkflowKeyMap() KeyMap {
	km := DefaultKeyMap()
	return KeyMap{
		StartWorkflow:   km.StartWorkflow,
		StopWorkflow:    km.StopWorkflow,
		RestartWorkflow: km.RestartWorkflow,
		ToggleLogs:      km.ToggleLogs,
		ToggleTable:     km.ToggleTable,
		ToggleDetails:   km.ToggleDetails,
	}
}

// HelpKeyMap returns keys for help contexts
func HelpKeyMap() KeyMap {
	km := DefaultKeyMap()
	return KeyMap{
		Up:         km.Up,
		Down:       km.Down,
		PageUp:     km.PageUp,
		PageDown:   km.PageDown,
		GoToTop:    km.GoToTop,
		GoToBottom: km.GoToBottom,
		Cancel:     km.Cancel,
		Help:       km.Help,
	}
}

// ShortHelp returns key bindings to be shown in the mini help view
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up, k.Down, k.Select, k.NextPanel, k.Help, k.Quit,
	}
}

// FullHelp returns keybindings for the expanded help view
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Navigation
		{k.Up, k.Down, k.Left, k.Right},
		{k.Select, k.Confirm, k.Cancel, k.Back},
		
		// Panel management
		{k.NextPanel, k.PrevPanel, k.FocusNav, k.FocusMain},
		
		// Viewport controls
		{k.PageUp, k.PageDown, k.GoToTop, k.GoToBottom},
		
		// List and table
		{k.FilterToggle, k.ClearFilter, k.SortToggle},
		
		// Workflow controls
		{k.StartWorkflow, k.StopWorkflow, k.RestartWorkflow},
		
		// View toggles
		{k.ToggleLogs, k.ToggleTable, k.ToggleDetails},
		
		// Application
		{k.Help, k.Refresh, k.Quit},
	}
}

// ContextualHelp returns help for a specific context
func ContextualHelp(context string) [][]key.Binding {
	switch context {
	case "navigation":
		km := NavigationKeyMap()
		return [][]key.Binding{
			{km.Up, km.Down, km.Select, km.Confirm},
			{km.FilterToggle, km.ClearFilter},
		}
	case "table":
		km := TableKeyMap()
		return [][]key.Binding{
			{km.Up, km.Down, km.Left, km.Right},
			{km.Select, km.SortToggle},
			{km.ColumnNext, km.ColumnPrev},
		}
	case "viewport":
		km := ViewportKeyMap()
		return [][]key.Binding{
			{km.Up, km.Down, km.PageUp, km.PageDown},
			{km.GoToTop, km.GoToBottom},
		}
	case "workflow":
		km := WorkflowKeyMap()
		return [][]key.Binding{
			{km.StartWorkflow, km.StopWorkflow, km.RestartWorkflow},
			{km.ToggleLogs, km.ToggleTable, km.ToggleDetails},
		}
	default:
		km := DefaultKeyMap()
		return km.FullHelp()
	}
}

// IsNavigationKey checks if a key is for navigation
func IsNavigationKey(msg tea.KeyMsg) bool {
	km := NavigationKeyMap()
	return key.Matches(msg, km.Up, km.Down, km.Left, km.Right)
}

// IsSelectionKey checks if a key is for selection
func IsSelectionKey(msg tea.KeyMsg) bool {
	km := DefaultKeyMap()
	return key.Matches(msg, km.Select, km.Confirm)
}

// IsGlobalKey checks if a key is global (works in all contexts)
func IsGlobalKey(msg tea.KeyMsg) bool {
	km := GlobalKeyMap()
	return key.Matches(msg, km.Quit, km.Help, km.Refresh, km.NextPanel, km.PrevPanel)
}

// IsPanelSwitchKey checks if a key switches panels
func IsPanelSwitchKey(msg tea.KeyMsg) bool {
	km := DefaultKeyMap()
	return key.Matches(msg, km.NextPanel, km.PrevPanel, km.FocusNav, km.FocusMain, km.FocusStatus)
}

// IsViewToggleKey checks if a key toggles views
func IsViewToggleKey(msg tea.KeyMsg) bool {
	km := DefaultKeyMap()
	return key.Matches(msg, km.ToggleLogs, km.ToggleTable, km.ToggleDetails)
}