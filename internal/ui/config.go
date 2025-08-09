package ui

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// UIConfig represents the complete UI configuration loaded from configs/ui.yaml
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

type LayoutConfig struct {
	Breakpoints BreakpointsConfig `yaml:"breakpoints"`
	Spacing     SpacingConfig     `yaml:"spacing"`
	Panels      PanelsConfig      `yaml:"panels"`
	Borders     BordersConfig     `yaml:"borders"`
	Header      HeaderConfig      `yaml:"header"`
	Footer      FooterConfig      `yaml:"footer"`
}

type BreakpointsConfig struct {
	Large  int `yaml:"large"`
	Medium int `yaml:"medium"`
	Small  int `yaml:"small"`
}

type SpacingConfig struct {
	None   int `yaml:"none"`
	Small  int `yaml:"small"`
	Medium int `yaml:"medium"`
	Large  int `yaml:"large"`
}

type PanelsConfig struct {
	Navigation PanelSizeConfig `yaml:"navigation"`
	Main       PanelSizeConfig `yaml:"main"`
	Status     PanelSizeConfig `yaml:"status"`
}

type PanelSizeConfig struct {
	MinWidth              int     `yaml:"min_width"`
	PreferredWidthRatio   float64 `yaml:"preferred_width_ratio"`
}

type BordersConfig struct {
	Style string `yaml:"style"`
	Color string `yaml:"color"`
	Width int    `yaml:"width"`
}

type HeaderConfig struct {
	Height             int  `yaml:"height"`
	ShowWhenHeightAbove int `yaml:"show_when_height_above"`
}

type FooterConfig struct {
	Height           int  `yaml:"height"`
	ShowHelpShortcut bool `yaml:"show_help_shortcut"`
}

type ThemeConfig struct {
	Mode           string            `yaml:"mode"`
	HighContrast   bool              `yaml:"high_contrast"`
	Accessibility  AccessibilityConfig `yaml:"accessibility"`
	Colors         ColorsConfig      `yaml:"colors"`
}

type AccessibilityConfig struct {
	ColorDisabledFallback    bool    `yaml:"color_disabled_fallback"`
	ScreenReaderCompatible   bool    `yaml:"screen_reader_compatible"`
	MinimumContrastRatio     float64 `yaml:"minimum_contrast_ratio"`
}

type ColorsConfig struct {
	Primary    string `yaml:"primary"`
	Secondary  string `yaml:"secondary"`
	Accent     string `yaml:"accent"`
	Background string `yaml:"background"`
	Border     string `yaml:"border"`
	Muted      string `yaml:"muted"`
	Success    string `yaml:"success"`
	Warning    string `yaml:"warning"`
	Error      string `yaml:"error"`
	Info       string `yaml:"info"`
}

type SymbolsConfig struct {
	Status     StatusSymbolsConfig     `yaml:"status"`
	Progress   ProgressSymbolsConfig   `yaml:"progress"`
	Navigation NavigationSymbolsConfig `yaml:"navigation"`
	Special    SpecialSymbolsConfig    `yaml:"special"`
}

type StatusSymbolsConfig struct {
	Running   string `yaml:"running"`
	Completed string `yaml:"completed"`
	Failed    string `yaml:"failed"`
	Pending   string `yaml:"pending"`
	Paused    string `yaml:"paused"`
	Cancelled string `yaml:"cancelled"`
}

type ProgressSymbolsConfig struct {
	Spinner []string `yaml:"spinner"`
	Filled  string   `yaml:"filled"`
	Empty   string   `yaml:"empty"`
}

type NavigationSymbolsConfig struct {
	Cursor      string `yaml:"cursor"`
	FocusBorder string `yaml:"focus_border"`
}

type SpecialSymbolsConfig struct {
	Bullet     string `yaml:"bullet"`
	ArrowRight string `yaml:"arrow_right"`
	ArrowLeft  string `yaml:"arrow_left"`
	Ellipsis   string `yaml:"ellipsis"`
}

type KeymapConfig struct {
	Global     GlobalKeysConfig     `yaml:"global"`
	Navigation NavigationKeysConfig `yaml:"navigation"`
	Actions    ActionsKeysConfig    `yaml:"actions"`
}

type GlobalKeysConfig struct {
	Quit    []string `yaml:"quit"`
	Help    []string `yaml:"help"`
	Refresh []string `yaml:"refresh"`
	Debug   []string `yaml:"debug"`
}

type NavigationKeysConfig struct {
	Up         []string `yaml:"up"`
	Down       []string `yaml:"down"`
	Left       []string `yaml:"left"`
	Right      []string `yaml:"right"`
	NextPanel  []string `yaml:"next_panel"`
	PrevPanel  []string `yaml:"prev_panel"`
	FocusNav   []string `yaml:"focus_nav"`
	FocusMain  []string `yaml:"focus_main"`
	FocusStatus []string `yaml:"focus_status"`
	NextView   []string `yaml:"next_view"`
	PrevView   []string `yaml:"prev_view"`
}

type ActionsKeysConfig struct {
	Select     []string `yaml:"select"`
	Cancel     []string `yaml:"cancel"`
	Toggle     []string `yaml:"toggle"`
	PageUp     []string `yaml:"page_up"`
	PageDown   []string `yaml:"page_down"`
	ScrollTop  []string `yaml:"scroll_top"`
	ScrollBottom []string `yaml:"scroll_bottom"`
}

type ComponentsConfig struct {
	WorkflowList WorkflowListConfig `yaml:"workflow_list"`
	ToolTable    ToolTableConfig    `yaml:"tool_table"`
	LogViewport  LogViewportConfig  `yaml:"log_viewport"`
	Progress     ProgressConfig     `yaml:"progress"`
	Help         HelpConfig         `yaml:"help"`
}

type WorkflowListConfig struct {
	ShowStatusIcons bool `yaml:"show_status_icons"`
	ShowProgress    bool `yaml:"show_progress"`
	AutoScroll      bool `yaml:"auto_scroll"`
	MaxItems        int  `yaml:"max_items"`
	FilterEnabled   bool `yaml:"filter_enabled"`
}

type ToolTableConfig struct {
	ShowRecentOnly     bool `yaml:"show_recent_only"`
	RecentLimit        int  `yaml:"recent_limit"`
	ShowDuration       bool `yaml:"show_duration"`
	ShowArgs           bool `yaml:"show_args"`
	AutoResizeColumns  bool `yaml:"auto_resize_columns"`
}

type LogViewportConfig struct {
	MaxEntries     int  `yaml:"max_entries"`
	AutoScroll     bool `yaml:"auto_scroll"`
	ShowTimestamps bool `yaml:"show_timestamps"`
	ShowLevels     bool `yaml:"show_levels"`
	WrapLines      bool `yaml:"wrap_lines"`
}

type ProgressConfig struct {
	ShowPercentage   bool `yaml:"show_percentage"`
	AnimateSpinner   bool `yaml:"animate_spinner"`
	UpdateFrequency  int  `yaml:"update_frequency"`
}

type HelpConfig struct {
	ShowAllShortcuts  bool `yaml:"show_all_shortcuts"`
	MarkdownRendering bool `yaml:"markdown_rendering"`
	AutoHideTimeout   int  `yaml:"auto_hide_timeout"`
}

type PerformanceConfig struct {
	UpdateIntervals UpdateIntervalsConfig `yaml:"update_intervals"`
	Limits          LimitsConfig          `yaml:"limits"`
	Responsiveness  ResponsivenessConfig  `yaml:"responsiveness"`
}

type UpdateIntervalsConfig struct {
	UIRefresh       int `yaml:"ui_refresh"`
	ProgressUpdate  int `yaml:"progress_update"`
	SpinnerAnimation int `yaml:"spinner_animation"`
	LogRefresh      int `yaml:"log_refresh"`
	SystemStats     int `yaml:"system_stats"`
}

type LimitsConfig struct {
	MaxLogBuffer        int `yaml:"max_log_buffer"`
	MaxToolHistory      int `yaml:"max_tool_history"`
	MaxNotificationQueue int `yaml:"max_notification_queue"`
}

type ResponsivenessConfig struct {
	ResizeDebounce  int  `yaml:"resize_debounce"`
	InputDebounce   int  `yaml:"input_debounce"`
	AsyncRendering  bool `yaml:"async_rendering"`
}

type DebugConfig struct {
	Enabled             bool `yaml:"enabled"`
	ShowRenderTime      bool `yaml:"show_render_time"`
	LogMessages         bool `yaml:"log_messages"`
	ShowComponentBounds bool `yaml:"show_component_bounds"`
	MockData            bool `yaml:"mock_data"`
}

type TerminalConfig struct {
	NonTTY       NonTTYConfig       `yaml:"non_tty"`
	Capabilities CapabilitiesConfig `yaml:"capabilities"`
	Constraints  ConstraintsConfig  `yaml:"constraints"`
}

type NonTTYConfig struct {
	Enabled       bool `yaml:"enabled"`
	UseColor      bool `yaml:"use_color"`
	ShowProgress  bool `yaml:"show_progress"`
	CompactOutput bool `yaml:"compact_output"`
}

type CapabilitiesConfig struct {
	DetectColorSupport   bool `yaml:"detect_color_support"`
	DetectUnicodeSupport bool `yaml:"detect_unicode_support"`
	FallbackToASCII      bool `yaml:"fallback_to_ascii"`
}

type ConstraintsConfig struct {
	MinimumWidth      int  `yaml:"minimum_width"`
	MinimumHeight     int  `yaml:"minimum_height"`
	PreferWideLayout  bool `yaml:"prefer_wide_layout"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *UIConfig {
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
				Navigation: PanelSizeConfig{MinWidth: 20, PreferredWidthRatio: 0.25},
				Main:       PanelSizeConfig{MinWidth: 40, PreferredWidthRatio: 0.50},
				Status:     PanelSizeConfig{MinWidth: 15, PreferredWidthRatio: 0.25},
			},
			Borders: BordersConfig{
				Style: "rounded",
				Color: "#444444",
				Width: 2,
			},
			Header: HeaderConfig{
				Height:               3,
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
			Colors: ColorsConfig{
				Primary:    "#ffffff",
				Secondary:  "#cccccc",
				Accent:     "#888888",
				Background: "#000000",
				Border:     "#444444",
				Muted:      "#666666",
				Success:    "#ffffff",
				Warning:    "#cccccc",
				Error:      "#ffffff",
				Info:       "#888888",
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

// LoadConfig loads the UI configuration from the specified file
func LoadConfig(configPath string) (*UIConfig, error) {
	// Start with defaults
	config := DefaultConfig()
	
	// Try to load from file
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, use defaults
			return config, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	// Parse YAML
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}
	
	return config, nil
}

// LoadDefaultConfig loads the configuration from the default location
func LoadDefaultConfig() (*UIConfig, error) {
	return LoadConfig("configs/ui.yaml")
}