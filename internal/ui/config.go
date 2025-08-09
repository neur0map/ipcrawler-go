package ui

import (
	"os"
	"gopkg.in/yaml.v3"
)

// Config represents the complete UI configuration structure
type Config struct {
	UI       UIConfig       `yaml:"ui"`
	Simulator SimulatorConfig `yaml:"simulator"`
}

// UIConfig holds all UI-related settings
type UIConfig struct {
	Layout      LayoutConfig      `yaml:"layout"`
	Theme       ThemeConfig       `yaml:"theme"`
	Components  ComponentsConfig  `yaml:"components"`
	Keys        KeysConfig        `yaml:"keys"`
	Performance PerformanceConfig `yaml:"performance"`
}

// LayoutConfig defines responsive layout settings
type LayoutConfig struct {
	Breakpoints BreakpointsConfig `yaml:"breakpoints"`
	Panels      PanelsConfig      `yaml:"panels"`
}

// BreakpointsConfig defines screen size breakpoints
type BreakpointsConfig struct {
	Small  int `yaml:"small"`  // Switch to stacked layout
	Medium int `yaml:"medium"` // Switch to three-column
}

// PanelsConfig defines panel sizing
type PanelsConfig struct {
	List   PanelSizeConfig `yaml:"list"`
	Main   PanelSizeConfig `yaml:"main"`
	Status StatusPanelConfig `yaml:"status"`
}

// PanelSizeConfig defines panel dimensions
type PanelSizeConfig struct {
	WidthLarge  float64 `yaml:"width_large"`
	WidthMedium float64 `yaml:"width_medium"`
}

// StatusPanelConfig defines status panel specific settings
type StatusPanelConfig struct {
	WidthLarge   float64 `yaml:"width_large"`
	HeightFooter int     `yaml:"height_footer"`
}

// ThemeConfig defines color and styling
type ThemeConfig struct {
	Colors   ColorConfig   `yaml:"colors"`
	Adaptive AdaptiveConfig `yaml:"adaptive"`
}

// ColorConfig defines the color palette
type ColorConfig struct {
	Primary   string `yaml:"primary"`
	Secondary string `yaml:"secondary"`
	Accent    string `yaml:"accent"`
	Success   string `yaml:"success"`
	Warning   string `yaml:"warning"`
	Error     string `yaml:"error"`
	Border    string `yaml:"border"`
}

// AdaptiveConfig defines adaptive colors for light/dark terminals
type AdaptiveConfig struct {
	TextPrimary AdaptiveColorConfig `yaml:"text_primary"`
	Border      AdaptiveColorConfig `yaml:"border"`
}

// AdaptiveColorConfig defines light/dark color pairs
type AdaptiveColorConfig struct {
	Light string `yaml:"light"`
	Dark  string `yaml:"dark"`
}

// ComponentsConfig defines component-specific settings
type ComponentsConfig struct {
	List     ListConfig     `yaml:"list"`
	Viewport ViewportConfig `yaml:"viewport"`
	Status   StatusConfig   `yaml:"status"`
}

// ListConfig defines list component settings
type ListConfig struct {
	Title            string `yaml:"title"`
	ShowStatusBar    bool   `yaml:"show_status_bar"`
	FilteringEnabled bool   `yaml:"filtering_enabled"`
	ItemHeight       int    `yaml:"item_height"`
}

// ViewportConfig defines viewport component settings
type ViewportConfig struct {
	HighPerformance bool `yaml:"high_performance"`
	AutoScroll      bool `yaml:"auto_scroll"`
	LineNumbers     bool `yaml:"line_numbers"`
}

// StatusConfig defines status component settings
type StatusConfig struct {
	Spinner        string `yaml:"spinner"`
	ShowStats      bool   `yaml:"show_stats"`
	UpdateInterval string `yaml:"update_interval"`
}

// KeysConfig defines keyboard bindings
type KeysConfig struct {
	Quit           []string `yaml:"quit"`
	Tab            []string `yaml:"tab"`
	FocusList      []string `yaml:"focus_list"`
	FocusMain      []string `yaml:"focus_main"`
	FocusStatus    []string `yaml:"focus_status"`
	ListUp         []string `yaml:"list_up"`
	ListDown       []string `yaml:"list_down"`
	ListSelect     []string `yaml:"list_select"`
	ListFilter     []string `yaml:"list_filter"`
	ViewportUp     []string `yaml:"viewport_up"`
	ViewportDown   []string `yaml:"viewport_down"`
	ViewportPageUp []string `yaml:"viewport_page_up"`
	ViewportPageDown []string `yaml:"viewport_page_down"`
	ViewportHome   []string `yaml:"viewport_home"`
	ViewportEnd    []string `yaml:"viewport_end"`
}

// PerformanceConfig defines performance settings
type PerformanceConfig struct {
	AltScreen      bool `yaml:"alt_screen"`
	FramerateCap   int  `yaml:"framerate_cap"`
	BatchUpdates   bool `yaml:"batch_updates"`
	LazyRendering  bool `yaml:"lazy_rendering"`
}

// SimulatorConfig defines simulator settings
type SimulatorConfig struct {
	Workflows []WorkflowConfig `yaml:"workflows"`
	Tools     []ToolConfig     `yaml:"tools"`
	Logs      LogsConfig       `yaml:"logs"`
	Status    StatusSimConfig  `yaml:"status"`
}

// WorkflowConfig defines mock workflow data
type WorkflowConfig struct {
	ID          string   `yaml:"id"`
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Status      string   `yaml:"status"`
	Tools       []string `yaml:"tools"`
}

// ToolConfig defines mock tool data
type ToolConfig struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Category    string `yaml:"category"`
	Status      string `yaml:"status"`
}

// LogsConfig defines log simulation settings
type LogsConfig struct {
	UpdateInterval string   `yaml:"update_interval"`
	MaxEntries     int      `yaml:"max_entries"`
	SampleMessages []string `yaml:"sample_messages"`
}

// StatusSimConfig defines status simulation settings
type StatusSimConfig struct {
	ActiveTasks int    `yaml:"active_tasks"`
	Completed   int    `yaml:"completed"`
	Failed      int    `yaml:"failed"`
	UptimeStart string `yaml:"uptime_start"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(configPath string) (*Config, error) {
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return getDefaultConfig(), nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	// Parse YAML
	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	// Apply defaults for any missing values
	config = *mergeWithDefaults(&config)

	return &config, nil
}

// getDefaultConfig returns default configuration values
func getDefaultConfig() *Config {
	return &Config{
		UI: UIConfig{
			Layout: LayoutConfig{
				Breakpoints: BreakpointsConfig{
					Small:  80,
					Medium: 120,
				},
				Panels: PanelsConfig{
					List: PanelSizeConfig{
						WidthLarge:  0.25,
						WidthMedium: 0.30,
					},
					Main: PanelSizeConfig{
						WidthLarge:  0.55,
						WidthMedium: 0.70,
					},
					Status: StatusPanelConfig{
						WidthLarge:   0.20,
						HeightFooter: 4,
					},
				},
			},
			Theme: ThemeConfig{
				Colors: ColorConfig{
					Primary:   "#FAFAFA",
					Secondary: "#3C3C3C",
					Accent:    "#7D56F4",
					Success:   "#04B575",
					Warning:   "#F59E0B",
					Error:     "#EF4444",
					Border:    "#E5E5E5",
				},
				Adaptive: AdaptiveConfig{
					TextPrimary: AdaptiveColorConfig{
						Light: "236",
						Dark:  "248",
					},
					Border: AdaptiveColorConfig{
						Light: "240",
						Dark:  "238",
					},
				},
			},
			Components: ComponentsConfig{
				List: ListConfig{
					Title:            "Workflows & Tools",
					ShowStatusBar:    false,
					FilteringEnabled: true,
					ItemHeight:       3,
				},
				Viewport: ViewportConfig{
					HighPerformance: true,
					AutoScroll:      true,
					LineNumbers:     false,
				},
				Status: StatusConfig{
					Spinner:        "dot",
					ShowStats:      true,
					UpdateInterval: "100ms",
				},
			},
			Performance: PerformanceConfig{
				AltScreen:     true,
				FramerateCap:  60,
				BatchUpdates:  true,
				LazyRendering: true,
			},
		},
	}
}

// mergeWithDefaults merges config with defaults for missing values
func mergeWithDefaults(config *Config) *Config {
	defaults := getDefaultConfig()
	
	// This is a simplified merge - in production you'd want more sophisticated merging
	if config.UI.Layout.Breakpoints.Small == 0 {
		config.UI.Layout.Breakpoints.Small = defaults.UI.Layout.Breakpoints.Small
	}
	if config.UI.Layout.Breakpoints.Medium == 0 {
		config.UI.Layout.Breakpoints.Medium = defaults.UI.Layout.Breakpoints.Medium
	}
	
	return config
}