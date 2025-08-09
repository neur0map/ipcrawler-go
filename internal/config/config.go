package config

import (
	"path/filepath"
	"os"
	
	"github.com/spf13/viper"
)

// Config represents the complete application configuration
type Config struct {
	UI       UIConfig       `mapstructure:"ui"`
	Security SecurityConfig `mapstructure:"security"`
	Network  NetworkConfig  `mapstructure:"network"`
	Output   OutputConfig   `mapstructure:"output"`
	API      APIConfig      `mapstructure:"api"`
}

// UIConfig represents UI configuration from ui.yaml
type UIConfig struct {
	Layout      LayoutConfig      `mapstructure:"layout"`
	Theme       ThemeConfig       `mapstructure:"theme"`
	Components  ComponentsConfig  `mapstructure:"components"`
	Keys        KeysConfig        `mapstructure:"keys"`
	Performance PerformanceConfig `mapstructure:"performance"`
}

type LayoutConfig struct {
	Breakpoints map[string]int `mapstructure:"breakpoints"`
	Panels      PanelsConfig   `mapstructure:"panels"`
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
	Colors   map[string]string             `mapstructure:"colors"`
	Adaptive map[string]map[string]string `mapstructure:"adaptive"`
}

type ComponentsConfig struct {
	List     ListConfig     `mapstructure:"list"`
	Viewport ViewportConfig `mapstructure:"viewport"`
	Status   StatusConfig   `mapstructure:"status"`
}

type ListConfig struct {
	Title            string `mapstructure:"title"`
	ShowStatusBar    bool   `mapstructure:"show_status_bar"`
	FilteringEnabled bool   `mapstructure:"filtering_enabled"`
	ItemHeight       int    `mapstructure:"item_height"`
}

type ViewportConfig struct {
	HighPerformance bool `mapstructure:"high_performance"`
	AutoScroll      bool `mapstructure:"auto_scroll"`
	LineNumbers     bool `mapstructure:"line_numbers"`
	ScrollSpeed     int  `mapstructure:"scroll_speed"` // Lines to scroll per key press
}

type StatusConfig struct {
	Spinner        string `mapstructure:"spinner"`
	ShowStats      bool   `mapstructure:"show_stats"`
	UpdateInterval string `mapstructure:"update_interval"`
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
	AltScreen     bool `mapstructure:"alt_screen"`
	FramerateCap  int  `mapstructure:"framerate_cap"`
	BatchUpdates  bool `mapstructure:"batch_updates"`
	LazyRendering bool `mapstructure:"lazy_rendering"`
	MaxConcurrent int  `mapstructure:"max_concurrent"` // Max concurrent scans
}

// SecurityConfig for security.yaml
type SecurityConfig struct {
	Scanning  ScanningConfig  `mapstructure:"scanning"`
	Detection DetectionConfig `mapstructure:"detection"`
	Reporting ReportingConfig `mapstructure:"reporting"`
}

type ScanningConfig struct {
	MaxThreads      int      `mapstructure:"max_threads"`
	TimeoutSeconds  int      `mapstructure:"timeout_seconds"`
	RetryAttempts   int      `mapstructure:"retry_attempts"`
	RateLimiting    bool     `mapstructure:"rate_limiting"`
	UserAgents      []string `mapstructure:"user_agents"`
	SkipSSLVerify   bool     `mapstructure:"skip_ssl_verify"`
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

// NetworkConfig for network.yaml
type NetworkConfig struct {
	Proxy      ProxyConfig      `mapstructure:"proxy"`
	DNS        DNSConfig        `mapstructure:"dns"`
	Interfaces InterfacesConfig `mapstructure:"interfaces"`
}

type ProxyConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	HTTP     string `mapstructure:"http"`
	HTTPS    string `mapstructure:"https"`
	SOCKS5   string `mapstructure:"socks5"`
	NoProxy  []string `mapstructure:"no_proxy"`
}

type DNSConfig struct {
	Servers  []string `mapstructure:"servers"`
	Timeout  int      `mapstructure:"timeout"`
	Retries  int      `mapstructure:"retries"`
}

type InterfacesConfig struct {
	PreferIPv4 bool   `mapstructure:"prefer_ipv4"`
	Interface  string `mapstructure:"interface"`
}

// OutputConfig for output.yaml
type OutputConfig struct {
	Directory   string          `mapstructure:"directory"`
	Formats     FormatsConfig   `mapstructure:"formats"`
	Logging     LoggingConfig   `mapstructure:"logging"`
	Persistence PersistConfig   `mapstructure:"persistence"`
}

type FormatsConfig struct {
	JSON  bool `mapstructure:"json"`
	CSV   bool `mapstructure:"csv"`
	HTML  bool `mapstructure:"html"`
	PDF   bool `mapstructure:"pdf"`
	XML   bool `mapstructure:"xml"`
}

type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	File       string `mapstructure:"file"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

type PersistConfig struct {
	Database string `mapstructure:"database"`
	AutoSave bool   `mapstructure:"auto_save"`
	Interval int    `mapstructure:"interval"`
}

// APIConfig for api.yaml
type APIConfig struct {
	Keys       map[string]string `mapstructure:"keys"`
	Endpoints  EndpointsConfig   `mapstructure:"endpoints"`
	RateLimits RateLimitsConfig  `mapstructure:"rate_limits"`
}

type EndpointsConfig struct {
	Shodan        string `mapstructure:"shodan"`
	Censys        string `mapstructure:"censys"`
	VirusTotal    string `mapstructure:"virustotal"`
	SecurityTrails string `mapstructure:"securitytrails"`
	HunterIO      string `mapstructure:"hunter_io"`
}

type RateLimitsConfig struct {
	RequestsPerMinute int `mapstructure:"requests_per_minute"`
	BurstSize         int `mapstructure:"burst_size"`
	BackoffSeconds    int `mapstructure:"backoff_seconds"`
}

// LoadConfig loads all configuration files
func LoadConfig() (*Config, error) {
	config := &Config{}
	
	// Get base path
	execPath, _ := os.Executable()
	basePath := filepath.Dir(execPath)
	configPath := filepath.Join(basePath, "..", "configs")
	
	// Load UI config
	if err := loadConfigFile(configPath, "ui", &config.UI); err != nil {
		// Use defaults if file not found
		setUIDefaults(&config.UI)
	}
	
	// Load Security config
	if err := loadConfigFile(configPath, "security", &config.Security); err != nil {
		setSecurityDefaults(&config.Security)
	}
	
	// Load Network config
	if err := loadConfigFile(configPath, "network", &config.Network); err != nil {
		setNetworkDefaults(&config.Network)
	}
	
	// Load Output config
	if err := loadConfigFile(configPath, "output", &config.Output); err != nil {
		setOutputDefaults(&config.Output)
	}
	
	// Load API config
	if err := loadConfigFile(configPath, "api", &config.API); err != nil {
		setAPIDefaults(&config.API)
	}
	
	return config, nil
}

func loadConfigFile(basePath, name string, target interface{}) error {
	v := viper.New()
	v.SetConfigName(name)
	v.SetConfigType("yaml")
	
	// Add config paths
	v.AddConfigPath(basePath)
	v.AddConfigPath("configs")
	v.AddConfigPath("./configs")
	v.AddConfigPath("../configs")
	v.AddConfigPath("/etc/ipcrawler")
	v.AddConfigPath("$HOME/.ipcrawler")
	
	if err := v.ReadInConfig(); err != nil {
		return err
	}
	
	return v.Unmarshal(target)
}

func setUIDefaults(ui *UIConfig) {
	ui.Theme.Colors = map[string]string{
		"primary":   "#FAFAFA",
		"secondary": "#3C3C3C",
		"accent":    "#7D56F4",
		"success":   "#04B575",
		"warning":   "#F59E0B",
		"error":     "#EF4444",
		"border":    "#E5E5E5",
	}
	ui.Components.Viewport.AutoScroll = true
	ui.Components.Viewport.ScrollSpeed = 3
	ui.Performance.FramerateCap = 60
	ui.Performance.MaxConcurrent = 5
}

func setSecurityDefaults(sec *SecurityConfig) {
	sec.Scanning.MaxThreads = 10
	sec.Scanning.TimeoutSeconds = 30
	sec.Scanning.RetryAttempts = 3
	sec.Scanning.RateLimiting = true
	sec.Detection.EnableHeuristics = true
	sec.Reporting.Formats = []string{"json", "html"}
}

func setNetworkDefaults(net *NetworkConfig) {
	net.DNS.Servers = []string{"8.8.8.8", "1.1.1.1"}
	net.DNS.Timeout = 5
	net.DNS.Retries = 3
	net.Interfaces.PreferIPv4 = true
}

func setOutputDefaults(out *OutputConfig) {
	out.Directory = "./output"
	out.Formats.JSON = true
	out.Logging.Level = "info"
	out.Logging.MaxSize = 100
	out.Logging.MaxBackups = 3
	out.Logging.MaxAge = 30
	out.Persistence.AutoSave = true
	out.Persistence.Interval = 60
}

func setAPIDefaults(api *APIConfig) {
	api.RateLimits.RequestsPerMinute = 60
	api.RateLimits.BurstSize = 10
	api.RateLimits.BackoffSeconds = 60
}