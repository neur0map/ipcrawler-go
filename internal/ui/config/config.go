package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// UIConfig represents the complete UI configuration
type UIConfig struct {
	Layout      LayoutConfig      `yaml:"layout"`
	Theme       ThemeConfig       `yaml:"theme"`
	Symbols     SymbolsConfig     `yaml:"symbols"`
	Keymap      KeymapConfig      `yaml:"keymap"`
	Components  ComponentsConfig  `yaml:"components"`
	Performance PerformanceConfig `yaml:"performance"`
	Debug       DebugConfig       `yaml:"debug"`
	Terminal    TerminalConfig    `yaml:"terminal"`
}

// LayoutConfig contains layout-related configuration
type LayoutConfig struct {
	Breakpoints BreakpointsConfig `yaml:"breakpoints"`
	Spacing     SpacingConfig     `yaml:"spacing"`
	Panels      PanelsConfig      `yaml:"panels"`
	Borders     BordersConfig     `yaml:"borders"`
	Header      HeaderConfig      `yaml:"header"`
	Footer      FooterConfig      `yaml:"footer"`
}

// BreakpointsConfig defines responsive breakpoints
type BreakpointsConfig struct {
	Large  int `yaml:"large"`
	Medium int `yaml:"medium"`
	Small  int `yaml:"small"`
}

// SpacingConfig defines spacing values
type SpacingConfig struct {
	None   int `yaml:"none"`
	Small  int `yaml:"small"`
	Medium int `yaml:"medium"`
	Large  int `yaml:"large"`
}

// PanelsConfig defines panel sizing
type PanelsConfig struct {
	Navigation PanelConfig `yaml:"navigation"`
	Main       PanelConfig `yaml:"main"`
	Status     PanelConfig `yaml:"status"`
}

// PanelConfig defines individual panel configuration
type PanelConfig struct {
	MinWidth             int     `yaml:"min_width"`
	PreferredWidthRatio  float64 `yaml:"preferred_width_ratio"`
}

// BordersConfig defines border styling
type BordersConfig struct {
	Style string `yaml:"style"`
	Color string `yaml:"color"`
	Width int    `yaml:"width"`
}

// HeaderConfig defines header configuration
type HeaderConfig struct {
	Height               int  `yaml:"height"`
	ShowWhenHeightAbove  int  `yaml:"show_when_height_above"`
}

// FooterConfig defines footer configuration
type FooterConfig struct {
	Height            int  `yaml:"height"`
	ShowHelpShortcut  bool `yaml:"show_help_shortcut"`
}

// ThemeConfig contains theme-related configuration
type ThemeConfig struct {
	Mode          string                `yaml:"mode"`
	HighContrast  bool                  `yaml:"high_contrast"`
	Accessibility AccessibilityConfig   `yaml:"accessibility"`
	Colors        map[string]string     `yaml:"colors"`
}

// AccessibilityConfig defines accessibility features
type AccessibilityConfig struct {
	ColorDisabledFallback   bool    `yaml:"color_disabled_fallback"`
	ScreenReaderCompatible  bool    `yaml:"screen_reader_compatible"`
	MinimumContrastRatio    float64 `yaml:"minimum_contrast_ratio"`
}

// SymbolsConfig defines all UI symbols
type SymbolsConfig struct {
	Status     map[string]string `yaml:"status"`
	Progress   ProgressSymbols   `yaml:"progress"`
	Navigation NavigationSymbols `yaml:"navigation"`
	Special    map[string]string `yaml:"special"`
}

// ProgressSymbols defines progress-related symbols
type ProgressSymbols struct {
	Spinner []string `yaml:"spinner"`
	Filled  string   `yaml:"filled"`
	Empty   string   `yaml:"empty"`
}

// NavigationSymbols defines navigation-related symbols
type NavigationSymbols struct {
	Cursor      string `yaml:"cursor"`
	FocusBorder string `yaml:"focus_border"`
}

// KeymapConfig defines keyboard shortcuts
type KeymapConfig struct {
	Global     map[string][]string `yaml:"global"`
	Navigation map[string][]string `yaml:"navigation"`
	Actions    map[string][]string `yaml:"actions"`
}

// ComponentsConfig defines component-specific settings
type ComponentsConfig struct {
	WorkflowList ComponentWorkflowList `yaml:"workflow_list"`
	ToolTable    ComponentToolTable    `yaml:"tool_table"`
	LogViewport  ComponentLogViewport  `yaml:"log_viewport"`
	Progress     ComponentProgress     `yaml:"progress"`
	Help         ComponentHelp         `yaml:"help"`
}

// ComponentWorkflowList defines workflow list configuration
type ComponentWorkflowList struct {
	ShowStatusIcons bool `yaml:"show_status_icons"`
	ShowProgress    bool `yaml:"show_progress"`
	AutoScroll      bool `yaml:"auto_scroll"`
	MaxItems        int  `yaml:"max_items"`
	FilterEnabled   bool `yaml:"filter_enabled"`
}

// ComponentToolTable defines tool table configuration
type ComponentToolTable struct {
	ShowRecentOnly    bool `yaml:"show_recent_only"`
	RecentLimit       int  `yaml:"recent_limit"`
	ShowDuration      bool `yaml:"show_duration"`
	ShowArgs          bool `yaml:"show_args"`
	AutoResizeColumns bool `yaml:"auto_resize_columns"`
}

// ComponentLogViewport defines log viewport configuration
type ComponentLogViewport struct {
	MaxEntries     int  `yaml:"max_entries"`
	AutoScroll     bool `yaml:"auto_scroll"`
	ShowTimestamps bool `yaml:"show_timestamps"`
	ShowLevels     bool `yaml:"show_levels"`
	WrapLines      bool `yaml:"wrap_lines"`
}

// ComponentProgress defines progress component configuration
type ComponentProgress struct {
	ShowPercentage   bool `yaml:"show_percentage"`
	AnimateSpinner   bool `yaml:"animate_spinner"`
	UpdateFrequency  int  `yaml:"update_frequency"`
}

// ComponentHelp defines help component configuration
type ComponentHelp struct {
	ShowAllShortcuts  bool `yaml:"show_all_shortcuts"`
	MarkdownRendering bool `yaml:"markdown_rendering"`
	AutoHideTimeout   int  `yaml:"auto_hide_timeout"`
}

// PerformanceConfig defines performance settings
type PerformanceConfig struct {
	UpdateIntervals map[string]int `yaml:"update_intervals"`
	Limits          LimitsConfig   `yaml:"limits"`
	Responsiveness  ResponsivenessConfig `yaml:"responsiveness"`
}

// LimitsConfig defines resource limits
type LimitsConfig struct {
	MaxLogBuffer          int `yaml:"max_log_buffer"`
	MaxToolHistory        int `yaml:"max_tool_history"`
	MaxNotificationQueue  int `yaml:"max_notification_queue"`
}

// ResponsivenessConfig defines responsiveness settings
type ResponsivenessConfig struct {
	ResizeDebounce   int  `yaml:"resize_debounce"`
	InputDebounce    int  `yaml:"input_debounce"`
	AsyncRendering   bool `yaml:"async_rendering"`
}

// DebugConfig defines debug settings
type DebugConfig struct {
	Enabled              bool `yaml:"enabled"`
	ShowRenderTime       bool `yaml:"show_render_time"`
	LogMessages          bool `yaml:"log_messages"`
	ShowComponentBounds  bool `yaml:"show_component_bounds"`
	MockData             bool `yaml:"mock_data"`
}

// TerminalConfig defines terminal compatibility settings
type TerminalConfig struct {
	NonTTY       NonTTYConfig       `yaml:"non_tty"`
	Capabilities CapabilitiesConfig `yaml:"capabilities"`
	Constraints  ConstraintsConfig  `yaml:"constraints"`
}

// NonTTYConfig defines non-TTY fallback behavior
type NonTTYConfig struct {
	Enabled       bool `yaml:"enabled"`
	UseColor      bool `yaml:"use_color"`
	ShowProgress  bool `yaml:"show_progress"`
	CompactOutput bool `yaml:"compact_output"`
}

// CapabilitiesConfig defines terminal capability detection
type CapabilitiesConfig struct {
	DetectColorSupport   bool `yaml:"detect_color_support"`
	DetectUnicodeSupport bool `yaml:"detect_unicode_support"`
	FallbackToASCII      bool `yaml:"fallback_to_ascii"`
}

// ConstraintsConfig defines terminal size constraints
type ConstraintsConfig struct {
	MinimumWidth       int  `yaml:"minimum_width"`
	MinimumHeight      int  `yaml:"minimum_height"`
	PreferWideLayout   bool `yaml:"prefer_wide_layout"`
}

var (
	// Global UI configuration instance
	globalConfig *UIConfig
)

// LoadUIConfig loads the UI configuration from file
func LoadUIConfig(configPath string) (*UIConfig, error) {
	// If no path specified, look for configs/ui.yaml
	if configPath == "" {
		configPath = findDefaultUIConfigPath()
	}
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read UI config file %s: %w", configPath, err)
	}
	
	var config UIConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse UI config file %s: %w", configPath, err)
	}
	
	// Validate configuration
	if err := validateUIConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid UI configuration: %w", err)
	}
	
	globalConfig = &config
	return &config, nil
}

// GetUIConfig returns the global UI configuration
func GetUIConfig() *UIConfig {
	if globalConfig == nil {
		// Load default configuration if none loaded
		config, err := LoadUIConfig("")
		if err != nil {
			// Return safe defaults if config loading fails
			return getDefaultUIConfig()
		}
		return config
	}
	return globalConfig
}

// findDefaultUIConfigPath searches for the UI config file
func findDefaultUIConfigPath() string {
	// Common locations to search for ui.yaml
	searchPaths := []string{
		"configs/ui.yaml",
		"../configs/ui.yaml",
		"../../configs/ui.yaml",
		"/etc/ipcrawler/ui.yaml",
		filepath.Join(os.Getenv("HOME"), ".ipcrawler", "ui.yaml"),
	}
	
	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	
	// Default to the first path if none found
	return searchPaths[0]
}

// validateUIConfig validates the configuration for required fields and sensible values
func validateUIConfig(config *UIConfig) error {
	// Validate breakpoints
	if config.Layout.Breakpoints.Large <= config.Layout.Breakpoints.Medium {
		return fmt.Errorf("large breakpoint (%d) must be greater than medium (%d)", 
			config.Layout.Breakpoints.Large, config.Layout.Breakpoints.Medium)
	}
	if config.Layout.Breakpoints.Medium <= config.Layout.Breakpoints.Small {
		return fmt.Errorf("medium breakpoint (%d) must be greater than small (%d)", 
			config.Layout.Breakpoints.Medium, config.Layout.Breakpoints.Small)
	}
	
	// Validate panel ratios sum to reasonable value
	totalRatio := config.Layout.Panels.Navigation.PreferredWidthRatio + 
				 config.Layout.Panels.Main.PreferredWidthRatio + 
				 config.Layout.Panels.Status.PreferredWidthRatio
	if totalRatio > 1.0 {
		return fmt.Errorf("panel width ratios sum to %.2f, which exceeds 1.0", totalRatio)
	}
	
	// Validate minimum dimensions
	if config.Terminal.Constraints.MinimumWidth < 20 {
		return fmt.Errorf("minimum width (%d) too small, must be at least 20", 
			config.Terminal.Constraints.MinimumWidth)
	}
	if config.Terminal.Constraints.MinimumHeight < 5 {
		return fmt.Errorf("minimum height (%d) too small, must be at least 5", 
			config.Terminal.Constraints.MinimumHeight)
	}
	
	return nil
}

// getDefaultUIConfig returns safe default configuration
func getDefaultUIConfig() *UIConfig {
	return &UIConfig{
		Layout: LayoutConfig{
			Breakpoints: BreakpointsConfig{
				Large:  120,
				Medium: 80,
				Small:  40,
			},
			Spacing: SpacingConfig{
				None:   0,
				Small:  1,
				Medium: 2,
				Large:  4,
			},
			Panels: PanelsConfig{
				Navigation: PanelConfig{MinWidth: 20, PreferredWidthRatio: 0.25},
				Main:       PanelConfig{MinWidth: 40, PreferredWidthRatio: 0.50},
				Status:     PanelConfig{MinWidth: 15, PreferredWidthRatio: 0.25},
			},
			Borders: BordersConfig{
				Style: "rounded",
				Color: "#444444",
				Width: 2,
			},
			Header: HeaderConfig{
				Height:              3,
				ShowWhenHeightAbove: 15,
			},
			Footer: FooterConfig{
				Height:           2,
				ShowHelpShortcut: true,
			},
		},
		Theme: ThemeConfig{
			Mode:         "monochrome",
			HighContrast: true,
			Colors: map[string]string{
				"primary":    "#ffffff",
				"secondary":  "#cccccc",
				"background": "#000000",
				"border":     "#444444",
			},
		},
		Terminal: TerminalConfig{
			Constraints: ConstraintsConfig{
				MinimumWidth:     40,
				MinimumHeight:    10,
				PreferWideLayout: true,
			},
		},
	}
}