package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config represents the complete application configuration
type Config struct {
	UI       UIConfig       `mapstructure:"ui"`
	Security SecurityConfig `mapstructure:"security"`
	Output   OutputConfig   `mapstructure:"output"`
	Tools    ToolsConfig    `mapstructure:"tools"`
}

// UIConfig represents UI configuration
type UIConfig struct {
	Layout      LayoutConfig      `mapstructure:"layout"`
	Theme       ThemeConfig       `mapstructure:"theme"`
	Components  ComponentsConfig  `mapstructure:"components"`
	Keys        KeysConfig        `mapstructure:"keys"`
	Performance PerformanceConfig `mapstructure:"performance"`
	Display     DisplayConfig     `mapstructure:"display"`
	Formatting  FormattingConfig  `mapstructure:"formatting"`
}

type LayoutConfig struct {
	Breakpoints map[string]int `mapstructure:"breakpoints"`
	Panels      PanelsConfig   `mapstructure:"panels"`
	Cards       CardsConfig    `mapstructure:"cards"`
}

type CardsConfig struct {
	Columns        int `mapstructure:"columns"`
	Rows           int `mapstructure:"rows"`
	MinWidth       int `mapstructure:"min_width"`
	MinHeight      int `mapstructure:"min_height"`
	Spacing        int `mapstructure:"spacing"`
	ScrollBarSpace int `mapstructure:"scroll_bar_space"`
	VerticalOffset int `mapstructure:"vertical_offset"`
}

type PanelsConfig struct {
	List   PanelSize `mapstructure:"list"`
	Main   PanelSize `mapstructure:"main"`
	Status PanelSize `mapstructure:"status"`
}

type PanelSize struct {
	WidthLarge   float64 `mapstructure:"width_large"`
	WidthMedium  float64 `mapstructure:"width_medium"`
	HeightFooter int     `mapstructure:"height_footer"`
}

type ThemeConfig struct {
	Colors   map[string]string            `mapstructure:"colors"`
	Adaptive map[string]map[string]string `mapstructure:"adaptive"`
}

type ComponentsConfig struct {
	List        ListConfig        `mapstructure:"list"`
	Viewport    ViewportConfig    `mapstructure:"viewport"`
	Status      StatusConfig      `mapstructure:"status"`
	ProgressBar ProgressBarConfig `mapstructure:"progress_bar"`
	Sparkline   SparklineConfig   `mapstructure:"sparkline"`
}

type ListConfig struct {
	Title            string `mapstructure:"title"`
	ShowStatusBar    bool   `mapstructure:"show_status_bar"`
	FilteringEnabled bool   `mapstructure:"filtering_enabled"`
	ItemHeight       int    `mapstructure:"item_height"`
	BorderPadding    int    `mapstructure:"border_padding"`
	ContentMargin    int    `mapstructure:"content_margin"`
}

type ViewportConfig struct {
	HighPerformance   bool    `mapstructure:"high_performance"`
	AutoScroll        bool    `mapstructure:"auto_scroll"`
	LineNumbers       bool    `mapstructure:"line_numbers"`
	ScrollSpeed       int     `mapstructure:"scroll_speed"` // Lines to scroll per key press
	BorderPadding     int     `mapstructure:"border_padding"`
	ContentPadding    int     `mapstructure:"content_padding"`
	SplitRatio        float64 `mapstructure:"split_ratio"`
	MouseWheelDelta   int     `mapstructure:"mouse_wheel_delta"`
	ScrollSensitivity float64 `mapstructure:"scroll_sensitivity"`
}

type StatusConfig struct {
	Spinner        string `mapstructure:"spinner"`
	ShowStats      bool   `mapstructure:"show_stats"`
	UpdateInterval string `mapstructure:"update_interval"`
	RefreshMs      int    `mapstructure:"refresh_ms"`
}

type ProgressBarConfig struct {
	LabelSpace int    `mapstructure:"label_space"`
	MinWidth   int    `mapstructure:"min_width"`
	FilledChar string `mapstructure:"filled_char"`
	EmptyChar  string `mapstructure:"empty_char"`
}

type SparklineConfig struct {
	Chars        []string `mapstructure:"chars"`
	MinWidth     int      `mapstructure:"min_width"`
	FallbackChar string   `mapstructure:"fallback_char"`
}

type KeysConfig struct {
	Quit             []string `mapstructure:"quit"`
	Tab              []string `mapstructure:"tab"`
	FocusList        []string `mapstructure:"focus_list"`
	FocusMain        []string `mapstructure:"focus_main"`
	FocusStatus      []string `mapstructure:"focus_status"`
	ListUp           []string `mapstructure:"list_up"`
	ListDown         []string `mapstructure:"list_down"`
	ListSelect       []string `mapstructure:"list_select"`
	ListFilter       []string `mapstructure:"list_filter"`
	ViewportUp       []string `mapstructure:"viewport_up"`
	ViewportDown     []string `mapstructure:"viewport_down"`
	ViewportPageUp   []string `mapstructure:"viewport_page_up"`
	ViewportPageDown []string `mapstructure:"viewport_page_down"`
	ViewportHome     []string `mapstructure:"viewport_home"`
	ViewportEnd      []string `mapstructure:"viewport_end"`
}

type PerformanceConfig struct {
	AltScreen            bool    `mapstructure:"alt_screen"`
	FramerateCap         int     `mapstructure:"framerate_cap"`
	BatchUpdates         bool    `mapstructure:"batch_updates"`
	LazyRendering        bool    `mapstructure:"lazy_rendering"`
	MaxConcurrent        int     `mapstructure:"max_concurrent"` // Max concurrent scans
	SystemMonitorRefresh int     `mapstructure:"system_monitor_refresh"`
	AnimationFactor      float64 `mapstructure:"animation_factor"`
	FallbackRefresh      int     `mapstructure:"fallback_refresh"`
}

type DisplayConfig struct {
	ShowTimestamps  bool `mapstructure:"show_timestamps"`
	ShowProgress    bool `mapstructure:"show_progress"`
	ShowSpinner     bool `mapstructure:"show_spinner"`
	WordWrap        bool `mapstructure:"word_wrap"`
	SyntaxHighlight bool `mapstructure:"syntax_highlight"`
}

type FormattingConfig struct {
	PercentageFormat     string `mapstructure:"percentage_format"`
	DecimalPlaces        int    `mapstructure:"decimal_places"`
	UnitSpacing          string `mapstructure:"unit_spacing"`
	TimeFormat           string `mapstructure:"time_format"`
	DebugComponentWidth  int    `mapstructure:"debug_component_width"`
	DebugComponentHeight int    `mapstructure:"debug_component_height"`
	DebugViewportWidth   int    `mapstructure:"debug_viewport_width"`
	DebugViewportHeight  int    `mapstructure:"debug_viewport_height"`
}

// SecurityConfig for security.yaml
type SecurityConfig struct {
	OSDetection bool                    `mapstructure:"os_detection"`
	Execution   SecurityExecutionConfig `mapstructure:"execution"`
	Scanning    ScanningConfig          `mapstructure:"scanning"`
	Detection   DetectionConfig         `mapstructure:"detection"`
	Reporting   ReportingConfig         `mapstructure:"reporting"`
}

type SecurityExecutionConfig struct {
	ToolsRoot       string `mapstructure:"tools_root"`
	ArgsValidation  bool   `mapstructure:"args_validation"`
	ExecValidation  bool   `mapstructure:"exec_validation"`
}

type ScanningConfig struct {
	MaxThreads     int      `mapstructure:"max_threads"`
	TimeoutSeconds int      `mapstructure:"timeout_seconds"`
	RetryAttempts  int      `mapstructure:"retry_attempts"`
	RateLimiting   bool     `mapstructure:"rate_limiting"`
	UserAgents     []string `mapstructure:"user_agents"`
	SkipSSLVerify  bool     `mapstructure:"skip_ssl_verify"`
}

type DetectionConfig struct {
	SeverityLevels   []string `mapstructure:"severity_levels"`
	IgnorePatterns   []string `mapstructure:"ignore_patterns"`
	CustomSignatures []string `mapstructure:"custom_signatures"`
	EnableHeuristics bool     `mapstructure:"enable_heuristics"`
}

type ReportingConfig struct {
	AutoGenerate bool     `mapstructure:"auto_generate"`
	Formats      []string `mapstructure:"formats"`
	IncludeRaw   bool     `mapstructure:"include_raw"`
	Redaction    bool     `mapstructure:"redaction"`
}

// OutputConfig matches the current configs/output.yaml schema (multi-sink by level)
// Example (top-level file without an "output:" wrapper):
//
//	workspace_base: "./workspace"
//	timestamp: true
//	info: { directory: "{{workspace}}/logs/info/", level: "info" }
//
// It also supports the legacy wrapper form under the "output" key via loadConfigFile.
type OutputConfig struct {
	WorkspaceBase      string        `mapstructure:"workspace_base"`
	Timestamp          bool          `mapstructure:"timestamp"`
	TimeFormat         string        `mapstructure:"time_format"`
	ScanOutputMode     string        `mapstructure:"scan_output_mode"`
	CreateLatestLinks  bool          `mapstructure:"create_latest_links"`
	Info               LogSinkConfig `mapstructure:"info"`
	Error              LogSinkConfig `mapstructure:"error"`
	Warning            LogSinkConfig `mapstructure:"warning"`
	Debug              LogSinkConfig `mapstructure:"debug"`
	Raw                RawSinkConfig `mapstructure:"raw"`
}

type LogSinkConfig struct {
	Directory string `mapstructure:"directory"`
	Level     string `mapstructure:"level"`
}

type RawSinkConfig struct {
	Directory string `mapstructure:"directory"`
}

// ToolsConfig for tools.yaml configuration
type ToolsConfig struct {
	ToolExecution         ToolExecutionConfig         `mapstructure:"tool_execution"`
	WorkflowOrchestration WorkflowOrchestrationConfig `mapstructure:"workflow_orchestration"`
	DefaultTimeout        int                         `mapstructure:"default_timeout_seconds"`
	RetryAttempts         int                         `mapstructure:"retry_attempts"`
	ArgvPolicy            ArgvPolicyConfig            `mapstructure:"argv_policy"`
	Execution             ExecutionConfig             `mapstructure:"execution"`
	CLIMode               CLIModeConfig               `mapstructure:"cli_mode"`
}

type ToolExecutionConfig struct {
	MaxConcurrentExecutions int `mapstructure:"max_concurrent_executions"`
	MaxParallelExecutions   int `mapstructure:"max_parallel_executions"`
}

type WorkflowOrchestrationConfig struct {
	MaxConcurrentWorkflows   int                    `mapstructure:"max_concurrent_workflows"`
	MaxConcurrentToolsPerStep int                   `mapstructure:"max_concurrent_tools_per_step"`
	ResourceLimits           ResourceLimitsConfig   `mapstructure:"resource_limits"`
	PriorityWeights          PriorityWeightsConfig  `mapstructure:"priority_weights"`
	Scheduling               SchedulingConfig       `mapstructure:"scheduling"`
}

type ResourceLimitsConfig struct {
	MaxCPUUsage     float64 `mapstructure:"max_cpu_usage"`
	MaxMemoryUsage  float64 `mapstructure:"max_memory_usage"`
	MaxActiveTools  int     `mapstructure:"max_active_tools"`
}

type PriorityWeightsConfig struct {
	High             int `mapstructure:"high"`
	Medium           int `mapstructure:"medium"`
	Low              int `mapstructure:"low"`
	IndependentBonus int `mapstructure:"independent_bonus"`
	ParallelBonus    int `mapstructure:"parallel_bonus"`
}

type SchedulingConfig struct {
	QueueCheckIntervalMs    int `mapstructure:"queue_check_interval_ms"`
	ResourceCheckIntervalMs int `mapstructure:"resource_check_interval_ms"`
}

type ArgvPolicyConfig struct {
	MaxArgs             int      `mapstructure:"max_args"`
	MaxArgBytes         int      `mapstructure:"max_arg_bytes"`
	MaxArgvBytes        int      `mapstructure:"max_argv_bytes"`
	DenyShellMetachars  bool     `mapstructure:"deny_shell_metachars"`
	AllowedCharClasses  []string `mapstructure:"allowed_char_classes"`
}

type ExecutionConfig struct {
	ToolsPath       string `mapstructure:"tools_path"`
	ArgsValidation  bool   `mapstructure:"args_validation"`
	ExecValidation  bool   `mapstructure:"exec_validation"`
}

type CLIModeConfig struct {
	ExecutionTimeoutSeconds int  `mapstructure:"execution_timeout_seconds"`
	WorkflowTimeoutSeconds  int  `mapstructure:"workflow_timeout_seconds"`
	StepTimeoutSeconds      int  `mapstructure:"step_timeout_seconds"`
	ValidateOutput          bool `mapstructure:"validate_output"`
}

// Persistence config removed (not used)

// LoadConfig loads all configuration files
func LoadConfig() (*Config, error) {
	config := &Config{}

	// Try to find config directory in multiple locations
	configPath := findConfigPath()

	// Load UI config
	if err := loadConfigFile(configPath, "ui", &config.UI); err != nil {
		// Use defaults if file not found
		setUIDefaults(&config.UI)
	}

	// Load Security config
	if err := loadConfigFile(configPath, "security", &config.Security); err != nil {
		setSecurityDefaults(&config.Security)
	}

	// Load Output config
	if err := loadConfigFile(configPath, "output", &config.Output); err != nil {
		setOutputDefaults(&config.Output)
	}

	// Load Tools config
	if err := loadConfigFile(configPath, "tools", &config.Tools); err != nil {
		setToolsDefaults(&config.Tools)
	}

	return config, nil
}

// findConfigPath tries to locate the configs directory in multiple locations
func findConfigPath() string {
	// Try multiple paths in order of preference
	paths := []string{
		"configs",                      // Current directory
		"./configs",                    // Explicit current directory
		filepath.Join("..", "configs"), // Parent directory
	}

	// Add path relative to executable
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		paths = append(paths,
			filepath.Join(execDir, "configs"),       // Same dir as executable
			filepath.Join(execDir, "..", "configs"), // Parent of executable
		)
	}

	// Add standard system paths
	paths = append(paths,
		"/etc/ipcrawler",
		filepath.Join(os.Getenv("HOME"), ".ipcrawler"),
	)

	// Find first existing path
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}

	// Default to configs in current directory
	return "configs"
}

func loadConfigFile(basePath, name string, target interface{}) error {
	v := viper.New()
	v.SetConfigName(name)
	v.SetConfigType("yaml")

	// Add the primary base path first
	if basePath != "" {
		v.AddConfigPath(basePath)
	}

	// Add config paths
	v.AddConfigPath("configs")
	v.AddConfigPath("./configs")
	v.AddConfigPath("../configs")

	// Add paths relative to executable
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		v.AddConfigPath(filepath.Join(execDir, "configs"))
		v.AddConfigPath(filepath.Join(execDir, "..", "configs"))
	}

	// Add system paths
	v.AddConfigPath("/etc/ipcrawler")
	v.AddConfigPath("$HOME/.ipcrawler")

	if err := v.ReadInConfig(); err != nil {
		return err
	}

	// The YAML files usually have a top-level key (ui, security, output)
	// We unmarshal into a map first, then extract the named section.
	// If the section is missing, we fall back to unmarshalling the whole file
	// directly into the target to support flat schemas (e.g., new output.yaml).
	var rawConfig map[string]interface{}
	if err := v.Unmarshal(&rawConfig); err != nil {
		return err
	}

	// Extract the specific section (ui, security, or output)
	if section, exists := rawConfig[name]; exists {
		// Use viper to convert the section to the target struct
		sectionViper := viper.New()
		if err := sectionViper.MergeConfigMap(map[string]interface{}{name: section}); err != nil {
			return err
		}
		return sectionViper.UnmarshalKey(name, target)
	}
	// Fallback: try whole-file unmarshal (supports flat schema files)
	if err := v.Unmarshal(target); err == nil {
		return nil
	}
	return fmt.Errorf("section '%s' not found in config and whole-file unmarshal failed", name)
}

func setUIDefaults(ui *UIConfig) {
	// Set minimal defaults if config file is missing
	if ui.Theme.Colors == nil {
		ui.Theme.Colors = make(map[string]string)
	}

	// Set default color palette
	defaultColors := map[string]string{
		"primary":       "#FAFAFA",
		"secondary":     "#3C3C3C",
		"accent":        "#7D56F4",
		"success":       "#04B575",
		"warning":       "#F59E0B",
		"error":         "#EF4444",
		"border":        "#E5E5E5",
		"focused":       "#06B6D4",
		"info":          "#0EA5E9",
		"debug":         "#6B7280",
		"timestamp":     "#9CA3AF",
		"prefix":        "#8B5CF6",
		"key":           "#06B6D4",
		"value":         "#F9FAFB",
		"workflow":      "#10B981",
		"count":         "#F59E0B",
		"progress":      "#22C55E",
		"cpu":           "#F97316",
		"memory":        "#3B82F6",
		"disk":          "#10B981",
		"network":       "#EC4899",
		"running":       "#F97316",
		"pending":       "#6B7280",
		"completed":     "#06B6D4",
		"failed":        "#EF4444",
		"spinner":       "#EC4899",
		"list_selected": "#8B5CF6",
	}

	// Apply defaults only for missing colors
	for key, value := range defaultColors {
		if _, exists := ui.Theme.Colors[key]; !exists {
			ui.Theme.Colors[key] = value
		}
	}

	// Set layout defaults
	if ui.Layout.Cards.Columns == 0 {
		ui.Layout.Cards.Columns = 4
		ui.Layout.Cards.Rows = 2
		ui.Layout.Cards.MinWidth = 28
		ui.Layout.Cards.MinHeight = 10
		ui.Layout.Cards.Spacing = 2        // Match the "  " spacing in JoinHorizontal
		ui.Layout.Cards.ScrollBarSpace = 2 // Minimal scroll bar space
		ui.Layout.Cards.VerticalOffset = 4 // Space for title and help text
	}

	// Set component defaults
	if ui.Components.Viewport.MouseWheelDelta == 0 {
		ui.Components.Viewport.MouseWheelDelta = 3
	}
	if ui.Components.List.ShowStatusBar == false {
		ui.Components.List.ShowStatusBar = false // explicitly set
	}
	if ui.Components.Status.RefreshMs == 0 {
		ui.Components.Status.RefreshMs = 1500
	}

	// Set performance defaults
	if ui.Performance.FramerateCap == 0 {
		ui.Performance.FramerateCap = 60
	}
	if ui.Performance.MaxConcurrent == 0 {
		ui.Performance.MaxConcurrent = 5
	}
	if ui.Performance.SystemMonitorRefresh == 0 {
		ui.Performance.SystemMonitorRefresh = 1500
	}
	if ui.Performance.AnimationFactor == 0 {
		ui.Performance.AnimationFactor = 0.15
	}

	// Set default formatting values
	if ui.Formatting.DebugViewportWidth == 0 {
		ui.Formatting.DebugViewportWidth = 50
	}
	if ui.Formatting.DebugViewportHeight == 0 {
		ui.Formatting.DebugViewportHeight = 10
	}
}

func setSecurityDefaults(sec *SecurityConfig) {
	// Set minimal defaults if config file is missing
	if !sec.OSDetection {
		sec.OSDetection = true
	}
	
	// Set defaults for execution settings
	if sec.Execution.ToolsRoot == "" {
		sec.Execution.ToolsRoot = "" // Empty means allow system PATH
	}
	if !sec.Execution.ArgsValidation {
		sec.Execution.ArgsValidation = true
	}
	if !sec.Execution.ExecValidation {
		sec.Execution.ExecValidation = true
	}
	
	if sec.Scanning.MaxThreads == 0 {
		sec.Scanning.MaxThreads = 10
	}
	if sec.Scanning.TimeoutSeconds == 0 {
		sec.Scanning.TimeoutSeconds = 30
	}
	if sec.Scanning.RetryAttempts == 0 {
		sec.Scanning.RetryAttempts = 3
	}
}

func setOutputDefaults(out *OutputConfig) {
	// Minimal defaults if config is missing
	if out.WorkspaceBase == "" {
		out.WorkspaceBase = "./workspace"
	}
	if out.TimeFormat == "" {
		out.TimeFormat = "RFC3339Nano"
	}
	if out.ScanOutputMode == "" {
		out.ScanOutputMode = "both"
	}
	// CreateLatestLinks defaults to true if not explicitly set
	if !out.CreateLatestLinks {
		out.CreateLatestLinks = true
	}
	if out.Info.Directory == "" {
		out.Info.Directory = "{{workspace}}/logs/info/"
	}
	if out.Info.Level == "" {
		out.Info.Level = "info"
	}
	if out.Error.Directory == "" {
		out.Error.Directory = "{{workspace}}/logs/error/"
	}
	if out.Error.Level == "" {
		out.Error.Level = "error"
	}
	if out.Warning.Directory == "" {
		out.Warning.Directory = "{{workspace}}/logs/warning/"
	}
	if out.Warning.Level == "" {
		out.Warning.Level = "warn"
	}
	if out.Debug.Directory == "" {
		out.Debug.Directory = "{{workspace}}/logs/debug/"
	}
	if out.Debug.Level == "" {
		out.Debug.Level = "debug"
	}
	if out.Raw.Directory == "" {
		out.Raw.Directory = "{{workspace}}/raw/"
	}
}

func setToolsDefaults(tools *ToolsConfig) {
	// Set defaults for tool execution settings
	if tools.ToolExecution.MaxConcurrentExecutions == 0 {
		tools.ToolExecution.MaxConcurrentExecutions = 3
	}
	if tools.ToolExecution.MaxParallelExecutions == 0 {
		tools.ToolExecution.MaxParallelExecutions = 2
	}
	if tools.DefaultTimeout == 0 {
		tools.DefaultTimeout = 1800 // 30 minutes
	}
	if tools.RetryAttempts == 0 {
		tools.RetryAttempts = 1
	}
	
	// Set defaults for workflow orchestration
	if tools.WorkflowOrchestration.MaxConcurrentWorkflows == 0 {
		tools.WorkflowOrchestration.MaxConcurrentWorkflows = 3
	}
	if tools.WorkflowOrchestration.MaxConcurrentToolsPerStep == 0 {
		tools.WorkflowOrchestration.MaxConcurrentToolsPerStep = 10
	}
	if tools.WorkflowOrchestration.ResourceLimits.MaxCPUUsage == 0 {
		tools.WorkflowOrchestration.ResourceLimits.MaxCPUUsage = 80.0
	}
	if tools.WorkflowOrchestration.ResourceLimits.MaxMemoryUsage == 0 {
		tools.WorkflowOrchestration.ResourceLimits.MaxMemoryUsage = 80.0
	}
	if tools.WorkflowOrchestration.ResourceLimits.MaxActiveTools == 0 {
		tools.WorkflowOrchestration.ResourceLimits.MaxActiveTools = 15
	}
	if tools.WorkflowOrchestration.PriorityWeights.High == 0 {
		tools.WorkflowOrchestration.PriorityWeights.High = 30
	}
	if tools.WorkflowOrchestration.PriorityWeights.Medium == 0 {
		tools.WorkflowOrchestration.PriorityWeights.Medium = 10
	}
	if tools.WorkflowOrchestration.PriorityWeights.Low == 0 {
		tools.WorkflowOrchestration.PriorityWeights.Low = -10
	}
	if tools.WorkflowOrchestration.PriorityWeights.IndependentBonus == 0 {
		tools.WorkflowOrchestration.PriorityWeights.IndependentBonus = 20
	}
	if tools.WorkflowOrchestration.PriorityWeights.ParallelBonus == 0 {
		tools.WorkflowOrchestration.PriorityWeights.ParallelBonus = 5
	}
	if tools.WorkflowOrchestration.Scheduling.QueueCheckIntervalMs == 0 {
		tools.WorkflowOrchestration.Scheduling.QueueCheckIntervalMs = 500
	}
	if tools.WorkflowOrchestration.Scheduling.ResourceCheckIntervalMs == 0 {
		tools.WorkflowOrchestration.Scheduling.ResourceCheckIntervalMs = 1000
	}
	
	// Set defaults for argv policy
	if tools.ArgvPolicy.MaxArgs == 0 {
		tools.ArgvPolicy.MaxArgs = 64
	}
	if tools.ArgvPolicy.MaxArgBytes == 0 {
		tools.ArgvPolicy.MaxArgBytes = 512
	}
	if tools.ArgvPolicy.MaxArgvBytes == 0 {
		tools.ArgvPolicy.MaxArgvBytes = 16384
	}
	if !tools.ArgvPolicy.DenyShellMetachars {
		tools.ArgvPolicy.DenyShellMetachars = true
	}
	if len(tools.ArgvPolicy.AllowedCharClasses) == 0 {
		tools.ArgvPolicy.AllowedCharClasses = []string{"alnum", "-", "_", ".", ":", "/", "=", ","}
	}
	
	// Set defaults for execution settings
	if tools.Execution.ToolsPath == "" {
		tools.Execution.ToolsPath = "" // Empty means allow system PATH
	}
	if !tools.Execution.ArgsValidation {
		tools.Execution.ArgsValidation = true
	}
	if !tools.Execution.ExecValidation {
		tools.Execution.ExecValidation = true
	}
}
