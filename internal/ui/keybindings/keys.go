package keybindings

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/your-org/ipcrawler/internal/ui"
)

// KeyMap defines all key bindings for the application
type KeyMap struct {
	// Global keys
	Quit        key.Binding
	Tab         key.Binding
	FocusList   key.Binding
	FocusMain   key.Binding
	FocusStatus key.Binding

	// List navigation
	ListUp     key.Binding
	ListDown   key.Binding
	ListSelect key.Binding
	ListFilter key.Binding

	// Viewport navigation
	ViewportUp       key.Binding
	ViewportDown     key.Binding
	ViewportPageUp   key.Binding
	ViewportPageDown key.Binding
	ViewportHome     key.Binding
	ViewportEnd      key.Binding
}

// NewKeyMap creates a new key map from configuration
func NewKeyMap(config *ui.Config) KeyMap {
	return KeyMap{
		// Global keys
		Quit: key.NewBinding(
			key.WithKeys(config.UI.Keys.Quit...),
			key.WithHelp("q/ctrl+c", "quit"),
		),
		Tab: key.NewBinding(
			key.WithKeys(config.UI.Keys.Tab...),
			key.WithHelp("tab", "cycle focus"),
		),
		FocusList: key.NewBinding(
			key.WithKeys(config.UI.Keys.FocusList...),
			key.WithHelp("1", "focus list"),
		),
		FocusMain: key.NewBinding(
			key.WithKeys(config.UI.Keys.FocusMain...),
			key.WithHelp("2", "focus main"),
		),
		FocusStatus: key.NewBinding(
			key.WithKeys(config.UI.Keys.FocusStatus...),
			key.WithHelp("3", "focus status"),
		),

		// List navigation
		ListUp: key.NewBinding(
			key.WithKeys(config.UI.Keys.ListUp...),
			key.WithHelp("↑/k", "move up"),
		),
		ListDown: key.NewBinding(
			key.WithKeys(config.UI.Keys.ListDown...),
			key.WithHelp("↓/j", "move down"),
		),
		ListSelect: key.NewBinding(
			key.WithKeys(config.UI.Keys.ListSelect...),
			key.WithHelp("enter/space", "select"),
		),
		ListFilter: key.NewBinding(
			key.WithKeys(config.UI.Keys.ListFilter...),
			key.WithHelp("/", "filter"),
		),

		// Viewport navigation
		ViewportUp: key.NewBinding(
			key.WithKeys(config.UI.Keys.ViewportUp...),
			key.WithHelp("↑/k", "scroll up"),
		),
		ViewportDown: key.NewBinding(
			key.WithKeys(config.UI.Keys.ViewportDown...),
			key.WithHelp("↓/j", "scroll down"),
		),
		ViewportPageUp: key.NewBinding(
			key.WithKeys(config.UI.Keys.ViewportPageUp...),
			key.WithHelp("ctrl+b/pgup", "page up"),
		),
		ViewportPageDown: key.NewBinding(
			key.WithKeys(config.UI.Keys.ViewportPageDown...),
			key.WithHelp("ctrl+f/pgdn", "page down"),
		),
		ViewportHome: key.NewBinding(
			key.WithKeys(config.UI.Keys.ViewportHome...),
			key.WithHelp("g", "go to top"),
		),
		ViewportEnd: key.NewBinding(
			key.WithKeys(config.UI.Keys.ViewportEnd...),
			key.WithHelp("G", "go to bottom"),
		),
	}
}

// FullHelp returns the full help for all key bindings
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// First column
		{k.Quit, k.Tab, k.FocusList, k.FocusMain, k.FocusStatus},
		// Second column  
		{k.ListUp, k.ListDown, k.ListSelect, k.ListFilter},
		// Third column
		{k.ViewportUp, k.ViewportDown, k.ViewportPageUp, k.ViewportPageDown, k.ViewportHome, k.ViewportEnd},
	}
}

// ShortHelp returns a condensed help for key bindings
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Quit, k.Tab, k.ListSelect, k.ViewportPageDown}
}

// DefaultKeyMap provides fallback key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q/ctrl+c", "quit"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "cycle focus"),
		),
		FocusList: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "focus list"),
		),
		FocusMain: key.NewBinding(
			key.WithKeys("2"), 
			key.WithHelp("2", "focus main"),
		),
		FocusStatus: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "focus status"),
		),
		ListUp: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("↑/k", "move up"),
		),
		ListDown: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("↓/j", "move down"),
		),
		ListSelect: key.NewBinding(
			key.WithKeys("enter", "space"),
			key.WithHelp("enter/space", "select"),
		),
		ListFilter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		ViewportUp: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("↑/k", "scroll up"),
		),
		ViewportDown: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("↓/j", "scroll down"),
		),
		ViewportPageUp: key.NewBinding(
			key.WithKeys("ctrl+b", "page_up"),
			key.WithHelp("ctrl+b/pgup", "page up"),
		),
		ViewportPageDown: key.NewBinding(
			key.WithKeys("ctrl+f", "page_down"),
			key.WithHelp("ctrl+f/pgdn", "page down"),
		),
		ViewportHome: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "go to top"),
		),
		ViewportEnd: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "go to bottom"),
		),
	}
}