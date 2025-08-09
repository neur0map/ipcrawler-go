package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
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

// LayoutConfig defines responsive layout settings
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
	MinWidth            int     `yaml:"min_width"`
	PreferredWidthRatio float64 `yaml:"preferred_width_ratio"`
}

type BordersConfig struct {
	Style string `yaml:"style"`
	Color string `yaml:"color"`
	Width int    `yaml:"width"`
}

type HeaderConfig struct {
	Height              int  `yaml:"height"`
	ShowWhenHeightAbove int  `yaml:"show_when_height_above"`
}

type FooterConfig struct {
	Height           int  `yaml:"height"`
	ShowHelpShortcut bool `yaml:"show_help_shortcut"`
}

// ThemeConfig defines styling and accessibility settings
type ThemeConfig struct {
	Mode          string                `yaml:"mode"`
	HighContrast  bool                  `yaml:"high_contrast"`
	Accessibility AccessibilityConfig   `yaml:"accessibility"`
	Colors        ColorsConfig          `yaml:"colors"`
}

type AccessibilityConfig struct {
	ColorDisabledFallback  bool    `yaml:"color_disabled_fallback"`
	ScreenReaderCompatible bool    `yaml:"screen_reader_compatible"`
	MinimumContrastRatio   float64 `yaml:"minimum_contrast_ratio"`
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

// SymbolsConfig defines all UI symbols and icons
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

// KeymapConfig defines keyboard shortcuts
type KeymapConfig struct {
	Global     GlobalKeysConfig     `yaml:"global"`
	Navigation NavigationKeysConfig `yaml:"navigation"`
	Actions    ActionKeysConfig     `yaml:"actions"`
}

type GlobalKeysConfig struct {
	Quit    []string `yaml:"quit"`
	Help    []string `yaml:"help"`
	Refresh []string `yaml:"refresh"`
	Debug   []string `yaml:"debug"`
}

type NavigationKeysConfig struct {
	Up          []string `yaml:"up"`
	Down        []string `yaml:"down"`
	Left        []string `yaml:"left"`
	Right       []string `yaml:"right"`
	NextPanel   []string `yaml:"next_panel"`
	PrevPanel   []string `yaml:"prev_panel"`
	FocusNav    []string `yaml:"focus_nav"`
	FocusMain   []string `yaml:"focus_main"`
	FocusStatus []string `yaml:"focus_status"`
	NextView    []string `yaml:"next_view"`
	PrevView    []string `yaml:"prev_view"`
}

type ActionKeysConfig struct {
	Select       []string `yaml:"select"`
	Cancel       []string `yaml:"cancel"`
	Toggle       []string `yaml:"toggle"`
	PageUp       []string `yaml:"page_up"`
	PageDown     []string `yaml:"page_down"`
	ScrollTop    []string `yaml:"scroll_top"`
	ScrollBottom []string `yaml:"scroll_bottom"`
}

// ComponentsConfig defines component-specific settings
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
	ShowRecentOnly    bool `yaml:"show_recent_only"`
	RecentLimit       int  `yaml:"recent_limit"`
	ShowDuration      bool `yaml:"show_duration"`
	ShowArgs          bool `yaml:"show_args"`
	AutoResizeColumns bool `yaml:"auto_resize_columns"`
}

type LogViewportConfig struct {
	MaxEntries     int  `yaml:"max_entries"`
	AutoScroll     bool `yaml:"auto_scroll"`
	ShowTimestamps bool `yaml:"show_timestamps"`
	ShowLevels     bool `yaml:"show_levels"`
	WrapLines      bool `yaml:"wrap_lines"`
}

type ProgressConfig struct {
	ShowPercentage  bool `yaml:"show_percentage"`
	AnimateSpinner  bool `yaml:"animate_spinner"`
	UpdateFrequency int  `yaml:"update_frequency"`
}

type HelpConfig struct {
	ShowAllShortcuts  bool `yaml:"show_all_shortcuts"`
	MarkdownRendering bool `yaml:"markdown_rendering"`
	AutoHideTimeout   int  `yaml:"auto_hide_timeout"`
}

// PerformanceConfig defines performance settings
type PerformanceConfig struct {
	UpdateIntervals UpdateIntervalsConfig `yaml:"update_intervals"`
	Limits          LimitsConfig          `yaml:"limits"`
	Responsiveness  ResponsivenessConfig  `yaml:"responsiveness"`
}

type UpdateIntervalsConfig struct {
	UIRefresh        int `yaml:"ui_refresh"`
	ProgressUpdate   int `yaml:"progress_update"`
	SpinnerAnimation int `yaml:"spinner_animation"`
	LogRefresh       int `yaml:"log_refresh"`
	SystemStats      int `yaml:"system_stats"`
}

type LimitsConfig struct {
	MaxLogBuffer         int `yaml:"max_log_buffer"`
	MaxToolHistory       int `yaml:"max_tool_history"`
	MaxNotificationQueue int `yaml:"max_notification_queue"`
}

type ResponsivenessConfig struct {
	ResizeDebounce int  `yaml:"resize_debounce"`
	InputDebounce  int  `yaml:"input_debounce"`
	AsyncRendering bool `yaml:"async_rendering"`
}

// DebugConfig defines debug settings
type DebugConfig struct {
	Enabled             bool `yaml:"enabled"`
	ShowRenderTime      bool `yaml:"show_render_time"`
	LogMessages         bool `yaml:"log_messages"`
	ShowComponentBounds bool `yaml:"show_component_bounds"`
	MockData            bool `yaml:"mock_data"`
}

// TerminalConfig defines terminal compatibility settings
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
	MinimumWidth     int  `yaml:"minimum_width"`
	MinimumHeight    int  `yaml:"minimum_height"`
	PreferWideLayout bool `yaml:"prefer_wide_layout"`
}

// Styles represents the complete styling system
type Styles struct {
	// Base styles
	Base          lipgloss.Style
	Title         lipgloss.Style
	Subtitle      lipgloss.Style
	Text          lipgloss.Style
	TextBold      lipgloss.Style
	TextMuted     lipgloss.Style
	TextSecondary lipgloss.Style
	TextError     lipgloss.Style

	// Layout styles
	Panel         lipgloss.Style
	PanelFocused  lipgloss.Style
	Border        lipgloss.Style
	BorderFocused lipgloss.Style
	Header        lipgloss.Style
	Footer        lipgloss.Style

	// Component styles
	StatusRunning   lipgloss.Style
	StatusCompleted lipgloss.Style
	StatusFailed    lipgloss.Style
	StatusPending   lipgloss.Style

	// Progress styles
	ProgressBar     lipgloss.Style
	ProgressPercent lipgloss.Style
	Spinner         lipgloss.Style

	// Table styles
	TableHeader     lipgloss.Style
	TableRow        lipgloss.Style
	TableRowFocused lipgloss.Style

	// List styles
	ListItem         lipgloss.Style
	ListItemFocused  lipgloss.Style
	ListItemSelected lipgloss.Style

	// Configuration
	config *UIConfig
}

// Global configuration instance
var globalConfig *UIConfig
var globalStyles *Styles

// LoadUIConfig loads the UI configuration from configs/ui.yaml
func LoadUIConfig() (*UIConfig, error) {
	if globalConfig != nil {
		return globalConfig, nil
	}

	// Find config file
	configPath := findConfigFile()
	if configPath == "" {
		return nil, fmt.Errorf("ui config file not found")
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	// Parse YAML
	var config UIConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	// Validate and set defaults
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Cache globally
	globalConfig = &config
	return globalConfig, nil
}

// GetUIConfig returns the global UI configuration
func GetUIConfig() *UIConfig {
	if globalConfig == nil {
		// Load with fallback defaults if not loaded
		config, err := LoadUIConfig()
		if err != nil {
			// Return safe defaults
			return getDefaultConfig()
		}
		return config
	}
	return globalConfig
}

// NewConfiguredStyles creates a Styles instance from the UI configuration
func NewConfiguredStyles() *Styles {
	if globalStyles != nil {
		return globalStyles
	}

	config := GetUIConfig()
	styles := createStylesFromConfig(config)

	// Cache globally
	globalStyles = styles
	return globalStyles
}

// GetStyles returns the global styles instance
func GetStyles() *Styles {
	if globalStyles == nil {
		return NewConfiguredStyles()
	}
	return globalStyles
}

// createStylesFromConfig builds Lipgloss styles from configuration
func createStylesFromConfig(config *UIConfig) *Styles {
	colors := config.Theme.Colors
	borders := config.Layout.Borders

	// Base style definitions
	baseStyle := lipgloss.NewStyle()

	// Create border style
	borderStyle := getBorderStyle(borders.Style)
	borderColor := lipgloss.Color(colors.Border)
	focusBorderColor := lipgloss.Color(colors.Accent)

	return &Styles{
		config: config,

		// Base styles
		Base:          baseStyle.Foreground(lipgloss.Color(colors.Primary)),
		Title:         baseStyle.Bold(true).Foreground(lipgloss.Color(colors.Primary)),
		Subtitle:      baseStyle.Foreground(lipgloss.Color(colors.Secondary)),
		Text:          baseStyle.Foreground(lipgloss.Color(colors.Primary)),
		TextBold:      baseStyle.Bold(true).Foreground(lipgloss.Color(colors.Primary)),
		TextMuted:     baseStyle.Foreground(lipgloss.Color(colors.Muted)),
		TextSecondary: baseStyle.Foreground(lipgloss.Color(colors.Secondary)),
		TextError:     baseStyle.Bold(true).Foreground(lipgloss.Color(colors.Error)),

		// Layout styles
		Panel: baseStyle.
			Border(borderStyle).
			BorderForeground(borderColor).
			Padding(0, 1),
		PanelFocused: baseStyle.
			Border(borderStyle).
			BorderForeground(focusBorderColor).
			Padding(0, 1),
		Border:        baseStyle.Border(borderStyle).BorderForeground(borderColor),
		BorderFocused: baseStyle.Border(borderStyle).BorderForeground(focusBorderColor),
		Header:        baseStyle.Bold(true).Foreground(lipgloss.Color(colors.Primary)).MarginBottom(1),
		Footer:        baseStyle.Foreground(lipgloss.Color(colors.Muted)).MarginTop(1),

		// Component styles
		StatusRunning:   baseStyle.Foreground(lipgloss.Color(colors.Info)),
		StatusCompleted: baseStyle.Foreground(lipgloss.Color(colors.Success)),
		StatusFailed:    baseStyle.Foreground(lipgloss.Color(colors.Error)),
		StatusPending:   baseStyle.Foreground(lipgloss.Color(colors.Muted)),

		// Progress styles
		ProgressBar:     baseStyle.Foreground(lipgloss.Color(colors.Accent)),
		ProgressPercent: baseStyle.Foreground(lipgloss.Color(colors.Secondary)),
		Spinner:         baseStyle.Foreground(lipgloss.Color(colors.Accent)),

		// Table styles
		TableHeader: baseStyle.
			Bold(true).
			Foreground(lipgloss.Color(colors.Primary)).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(colors.Border)),
		TableRow: baseStyle.Foreground(lipgloss.Color(colors.Primary)),
		TableRowFocused: baseStyle.
			Foreground(lipgloss.Color(colors.Primary)).
			Background(lipgloss.Color(colors.Accent)).
			Bold(true),

		// List styles
		ListItem: baseStyle.Foreground(lipgloss.Color(colors.Primary)),
		ListItemFocused: baseStyle.
			Foreground(lipgloss.Color(colors.Primary)).
			Background(lipgloss.Color(colors.Accent)).
			Bold(true),
		ListItemSelected: baseStyle.
			Foreground(lipgloss.Color(colors.Success)).
			Bold(true),
	}
}

// getBorderStyle converts string border style to Lipgloss border
func getBorderStyle(style string) lipgloss.Border {
	switch strings.ToLower(style) {
	case "rounded":
		return lipgloss.RoundedBorder()
	case "normal":
		return lipgloss.NormalBorder()
	case "thick":
		return lipgloss.ThickBorder()
	case "double":
		return lipgloss.DoubleBorder()
	default:
		return lipgloss.RoundedBorder()
	}
}

// findConfigFile locates the ui.yaml configuration file
func findConfigFile() string {
	// Check relative to current directory
	if _, err := os.Stat("configs/ui.yaml"); err == nil {
		return "configs/ui.yaml"
	}

	// Check in project root
	if _, err := os.Stat("../configs/ui.yaml"); err == nil {
		return "../configs/ui.yaml"
	}

	// Check in parent directories
	for i := 0; i < 5; i++ {
		path := strings.Repeat("../", i) + "configs/ui.yaml"
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Check absolute paths
	possiblePaths := []string{
		"/etc/ipcrawler/ui.yaml",
		filepath.Join(os.Getenv("HOME"), ".config", "ipcrawler", "ui.yaml"),
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// validateConfig validates and sets defaults for configuration
func validateConfig(config *UIConfig) error {
	// Validate breakpoints
	if config.Layout.Breakpoints.Large < config.Layout.Breakpoints.Medium {
		return fmt.Errorf("large breakpoint must be >= medium breakpoint")
	}
	if config.Layout.Breakpoints.Medium < config.Layout.Breakpoints.Small {
		return fmt.Errorf("medium breakpoint must be >= small breakpoint")
	}
	if config.Layout.Breakpoints.Small < 40 {
		return fmt.Errorf("small breakpoint must be >= 40")
	}

	// Validate panel ratios
	totalRatio := config.Layout.Panels.Navigation.PreferredWidthRatio +
		config.Layout.Panels.Main.PreferredWidthRatio +
		config.Layout.Panels.Status.PreferredWidthRatio
	if totalRatio != 1.0 {
		return fmt.Errorf("panel width ratios must sum to 1.0, got %.2f", totalRatio)
	}

	// Set default values for empty fields
	setConfigDefaults(config)

	return nil
}

// setConfigDefaults sets default values for unspecified config fields
func setConfigDefaults(config *UIConfig) {
	// Set default theme mode if empty
	if config.Theme.Mode == "" {
		config.Theme.Mode = "monochrome"
	}

	// Set default spinner if empty
	if len(config.Symbols.Progress.Spinner) == 0 {
		config.Symbols.Progress.Spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	}

	// Set default performance limits
	if config.Performance.Limits.MaxLogBuffer == 0 {
		config.Performance.Limits.MaxLogBuffer = 1000
	}
	if config.Performance.Limits.MaxToolHistory == 0 {
		config.Performance.Limits.MaxToolHistory = 100
	}

	// Set default update intervals
	if config.Performance.UpdateIntervals.UIRefresh == 0 {
		config.Performance.UpdateIntervals.UIRefresh = 100
	}
	if config.Performance.UpdateIntervals.SpinnerAnimation == 0 {
		config.Performance.UpdateIntervals.SpinnerAnimation = 80
	}
}

// getDefaultConfig returns a safe default configuration
func getDefaultConfig() *UIConfig {
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
			Accessibility: AccessibilityConfig{
				ColorDisabledFallback:  true,
				ScreenReaderCompatible: true,
				MinimumContrastRatio:   7.0,
			},
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
		Symbols: SymbolsConfig{
			Status: StatusSymbolsConfig{
				Running:   "●",
				Completed: "✓",
				Failed:    "✗",
				Pending:   "○",
				Paused:    "⏸",
				Cancelled: "⏹",
			},
			Progress: ProgressSymbolsConfig{
				Spinner: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
				Filled:  "█",
				Empty:   " ",
			},
			Navigation: NavigationSymbolsConfig{
				Cursor:      ">",
				FocusBorder: "heavy",
			},
			Special: SpecialSymbolsConfig{
				Bullet:     "•",
				ArrowRight: "→",
				ArrowLeft:  "←",
				Ellipsis:   "...",
			},
		},
		Performance: PerformanceConfig{
			UpdateIntervals: UpdateIntervalsConfig{
				UIRefresh:        100,
				ProgressUpdate:   50,
				SpinnerAnimation: 80,
				LogRefresh:       200,
				SystemStats:      1000,
			},
			Limits: LimitsConfig{
				MaxLogBuffer:         1000,
				MaxToolHistory:       100,
				MaxNotificationQueue: 10,
			},
			Responsiveness: ResponsivenessConfig{
				ResizeDebounce: 100,
				InputDebounce:  50,
				AsyncRendering: true,
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

// AdaptForTerminalCapabilities adapts styles based on terminal capabilities
func (s *Styles) AdaptForTerminalCapabilities(profile termenv.Profile) {
	// Adapt color profile based on terminal capabilities
	switch profile {
	case termenv.Ascii:
		// Remove all colors and fancy borders
		s.adaptForAscii()
	case termenv.ANSI:
		// Use basic 16 colors only
		s.adaptForANSI()
	case termenv.ANSI256:
		// Use 256 color palette
		s.adaptFor256Color()
	case termenv.TrueColor:
		// Full color support - no changes needed
		break
	}
}

// adaptForAscii removes all styling for ASCII-only terminals
func (s *Styles) adaptForAscii() {
	baseStyle := lipgloss.NewStyle()

	// Remove all colors and borders
	s.Base = baseStyle
	s.Title = baseStyle.Bold(true)
	s.Text = baseStyle
	s.TextBold = baseStyle.Bold(true)
	s.Panel = baseStyle.Border(lipgloss.NormalBorder())
	s.Border = baseStyle.Border(lipgloss.NormalBorder())

	// Simplify all component styles
	s.StatusRunning = baseStyle
	s.StatusCompleted = baseStyle
	s.StatusFailed = baseStyle
	s.StatusPending = baseStyle
	s.ProgressBar = baseStyle
	s.Spinner = baseStyle
	s.TableHeader = baseStyle.Bold(true).BorderBottom(true)
	s.TableRow = baseStyle
	s.ListItem = baseStyle
}

// adaptForANSI adapts styles for 16-color terminals
func (s *Styles) adaptForANSI() {
	// Use basic ANSI colors
	s.Text = s.Text.Foreground(lipgloss.Color("15"))                            // White
	s.TextMuted = s.TextMuted.Foreground(lipgloss.Color("8"))                   // Dark gray
	s.TextError = s.TextError.Foreground(lipgloss.Color("9"))                   // Red
	s.StatusCompleted = s.StatusCompleted.Foreground(lipgloss.Color("10"))      // Green
	s.StatusFailed = s.StatusFailed.Foreground(lipgloss.Color("9"))             // Red
}

// adaptFor256Color adapts styles for 256-color terminals
func (s *Styles) adaptFor256Color() {
	// Use enhanced 256-color palette
	s.Text = s.Text.Foreground(lipgloss.Color("255"))                           // Bright white
	s.TextMuted = s.TextMuted.Foreground(lipgloss.Color("244"))                 // Medium gray
	s.TextError = s.TextError.Foreground(lipgloss.Color("196"))                 // Bright red
	s.StatusCompleted = s.StatusCompleted.Foreground(lipgloss.Color("46"))      // Bright green
	s.StatusFailed = s.StatusFailed.Foreground(lipgloss.Color("196"))           // Bright red
}

// GetStatusSymbol returns the appropriate status symbol
func (s *Styles) GetStatusSymbol(status string) string {
	switch status {
	case "running":
		return s.config.Symbols.Status.Running
	case "completed":
		return s.config.Symbols.Status.Completed
	case "failed":
		return s.config.Symbols.Status.Failed
	case "pending":
		return s.config.Symbols.Status.Pending
	case "paused":
		return s.config.Symbols.Status.Paused
	case "cancelled":
		return s.config.Symbols.Status.Cancelled
	default:
		return s.config.Symbols.Status.Pending
	}
}

// GetStatusStyle returns the appropriate status style
func (s *Styles) GetStatusStyle(status string) lipgloss.Style {
	switch status {
	case "running":
		return s.StatusRunning
	case "completed":
		return s.StatusCompleted
	case "failed":
		return s.StatusFailed
	default:
		return s.StatusPending
	}
}

// RenderStatus renders a status with appropriate symbol and styling
func (s *Styles) RenderStatus(status string) string {
	symbol := s.GetStatusSymbol(status)
	style := s.GetStatusStyle(status)
	return style.Render(symbol + " " + strings.ToUpper(status))
}

// GetSpinnerFrames returns the spinner animation frames
func (s *Styles) GetSpinnerFrames() []string {
	return s.config.Symbols.Progress.Spinner
}

// StatusIcon returns the appropriate status icon for the given status
func StatusIcon(status string) string {
	styles := GetStyles()
	return styles.GetStatusSymbol(status)
}

// Reset clears the global configuration cache (useful for testing)
func Reset() {
	globalConfig = nil
	globalStyles = nil
}