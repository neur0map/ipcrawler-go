package term

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/muesli/termenv"
)

// TTYInfo contains information about the terminal
type TTYInfo struct {
	IsTTY             bool
	Width             int
	Height            int
	SupportsColor     bool
	SupportsTrueColor bool
	Profile           termenv.Profile
	Output            *termenv.Output
}

// GetTTYInfo returns comprehensive terminal information
func GetTTYInfo() TTYInfo {
	// Initialize termenv output for the current terminal
	output := termenv.NewOutput(os.Stdout)
	profile := termenv.ColorProfile()
	
	info := TTYInfo{
		IsTTY:   IsTerminal(int(os.Stdout.Fd())),
		Width:   80,  // Default fallback
		Height:  24,  // Default fallback
		Profile: profile,
		Output:  output,
	}
	
	if info.IsTTY {
		width, height := GetTerminalSize()
		info.Width = width
		info.Height = height
		info.SupportsColor = SupportsColor()
		info.SupportsTrueColor = SupportsTrueColor()
	} else {
		// For non-TTY, force ASCII profile
		info.Profile = termenv.Ascii
	}
	
	// Update color support based on termenv profile
	switch info.Profile {
	case termenv.Ascii:
		info.SupportsColor = false
		info.SupportsTrueColor = false
	case termenv.ANSI:
		info.SupportsColor = true
		info.SupportsTrueColor = false
	case termenv.ANSI256:
		info.SupportsColor = true
		info.SupportsTrueColor = false
	case termenv.TrueColor:
		info.SupportsColor = true
		info.SupportsTrueColor = true
	}
	
	return info
}

// IsTerminal returns true if the file descriptor is a terminal
func IsTerminal(fd int) bool {
	var termios syscall.Termios
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), 
		0x5401, uintptr(unsafe.Pointer(&termios)), 0, 0, 0) // TCGETS
	return err == 0
}

// GetTerminalSize returns the terminal width and height
func GetTerminalSize() (width, height int) {
	// Default fallback values
	width, height = 80, 24
	
	// Try to get actual terminal size
	if w, h := getTerminalSizeUnix(); w > 0 && h > 0 {
		width, height = w, h
	}
	
	return
}

// SupportsColor checks if the terminal supports ANSI colors
func SupportsColor() bool {
	// Check for common environment variables that indicate color support
	term := os.Getenv("TERM")
	colorTerm := os.Getenv("COLORTERM")
	
	// Check if explicitly disabled
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	
	// Check for color terminal indicators
	if colorTerm != "" {
		return true
	}
	
	// Check TERM value for color support
	colorTerms := []string{
		"xterm", "xterm-color", "xterm-256color",
		"screen", "screen-256color",
		"tmux", "tmux-256color",
		"rxvt", "rxvt-unicode", "rxvt-256color",
		"linux", "cygwin", "putty",
	}
	
	for _, colorTermValue := range colorTerms {
		if term == colorTermValue {
			return true
		}
	}
	
	return false
}

// SupportsTrueColor checks if the terminal supports 24-bit color
func SupportsTrueColor() bool {
	// Check for explicit true color support
	colorTerm := os.Getenv("COLORTERM")
	if colorTerm == "truecolor" || colorTerm == "24bit" {
		return true
	}
	
	// Check TERM values that typically support true color
	term := os.Getenv("TERM")
	trueColorTerms := []string{
		"xterm-direct",
		"xterm-truecolor",
		"screen-truecolor",
		"tmux-truecolor",
	}
	
	for _, trueColorTerm := range trueColorTerms {
		if term == trueColorTerm {
			return true
		}
	}
	
	// Some terminals support true color but don't advertise it properly
	// Check for common terminal applications
	termProgram := os.Getenv("TERM_PROGRAM")
	switch termProgram {
	case "iTerm.app", "WezTerm", "Alacritty", "kitty":
		return true
	}
	
	return false
}

// GetColorProfile returns the appropriate color profile for the terminal
func GetColorProfile() ColorProfile {
	if !SupportsColor() {
		return ColorProfileNone
	}
	
	if SupportsTrueColor() {
		return ColorProfileTrueColor
	}
	
	// Check for 256 color support
	term := os.Getenv("TERM")
	if contains(term, "256color") || contains(term, "256") {
		return ColorProfile256
	}
	
	// Default to 16 colors
	return ColorProfile16
}

// GetTermenvProfile returns the termenv color profile for the current terminal
func GetTermenvProfile() termenv.Profile {
	return termenv.ColorProfile()
}

// GetTermenvOutput returns a configured termenv output for the current terminal
func GetTermenvOutput() *termenv.Output {
	return termenv.NewOutput(os.Stdout)
}

// ConvertToTermenvProfile converts our ColorProfile to termenv.Profile
func ConvertToTermenvProfile(profile ColorProfile) termenv.Profile {
	switch profile {
	case ColorProfileNone:
		return termenv.Ascii
	case ColorProfile16:
		return termenv.ANSI
	case ColorProfile256:
		return termenv.ANSI256
	case ColorProfileTrueColor:
		return termenv.TrueColor
	default:
		return termenv.Ascii
	}
}

// ConvertFromTermenvProfile converts termenv.Profile to our ColorProfile
func ConvertFromTermenvProfile(profile termenv.Profile) ColorProfile {
	switch profile {
	case termenv.Ascii:
		return ColorProfileNone
	case termenv.ANSI:
		return ColorProfile16
	case termenv.ANSI256:
		return ColorProfile256
	case termenv.TrueColor:
		return ColorProfileTrueColor
	default:
		return ColorProfileNone
	}
}

// ColorProfile represents different color capabilities
type ColorProfile int

const (
	ColorProfileNone ColorProfile = iota
	ColorProfile16
	ColorProfile256
	ColorProfileTrueColor
)

// String returns the string representation of ColorProfile
func (c ColorProfile) String() string {
	switch c {
	case ColorProfileNone:
		return "none"
	case ColorProfile16:
		return "16"
	case ColorProfile256:
		return "256"
	case ColorProfileTrueColor:
		return "truecolor"
	default:
		return "unknown"
	}
}

// FallbackOptions defines options for non-TTY output
type FallbackOptions struct {
	UseColor      bool
	UseUnicode    bool
	ProgressChars ProgressChars
	StatusIcons   StatusIcons
}

// ProgressChars defines characters used for progress indication
type ProgressChars struct {
	Complete   string
	Incomplete string
	Current    string
}

// StatusIcons defines icons used for status indication
type StatusIcons struct {
	Running   string
	Completed string
	Failed    string
	Pending   string
}

// GetFallbackOptions returns appropriate fallback options
func GetFallbackOptions() FallbackOptions {
	info := GetTTYInfo()
	
	// Default fallback options for non-TTY
	options := FallbackOptions{
		UseColor:   false,
		UseUnicode: false,
		ProgressChars: ProgressChars{
			Complete:   "=",
			Incomplete: "-",
			Current:    ">",
		},
		StatusIcons: StatusIcons{
			Running:   "[RUNNING]",
			Completed: "[DONE]",
			Failed:    "[FAIL]",
			Pending:   "[WAIT]",
		},
	}
	
	// If we have a TTY with color support, enable enhanced output
	if info.IsTTY && info.SupportsColor {
		options.UseColor = true
		options.UseUnicode = true
		options.ProgressChars = ProgressChars{
			Complete:   "█",
			Incomplete: "░",
			Current:    "▓",
		}
		options.StatusIcons = StatusIcons{
			Running:   "●",
			Completed: "✓",
			Failed:    "✗",
			Pending:   "○",
		}
	}
	
	return options
}

// FormatProgress creates a progress bar suitable for current terminal
func FormatProgress(progress float64, width int, options FallbackOptions) string {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	
	filled := int(progress * float64(width))
	
	if options.UseUnicode {
		// Unicode progress bar
		bar := ""
		for i := 0; i < width; i++ {
			if i < filled {
				bar += options.ProgressChars.Complete
			} else if i == filled && progress < 1 {
				bar += options.ProgressChars.Current
			} else {
				bar += options.ProgressChars.Incomplete
			}
		}
		return fmt.Sprintf("[%s] %.1f%%", bar, progress*100)
	}
	
	// ASCII progress bar
	bar := ""
	for i := 0; i < width; i++ {
		if i < filled {
			bar += options.ProgressChars.Complete
		} else {
			bar += options.ProgressChars.Incomplete
		}
	}
	return fmt.Sprintf("[%s] %.1f%%", bar, progress*100)
}

// FormatStatus returns an appropriate status indicator
func FormatStatus(status string, options FallbackOptions) string {
	var icon string
	switch status {
	case "running":
		icon = options.StatusIcons.Running
	case "completed":
		icon = options.StatusIcons.Completed
	case "failed":
		icon = options.StatusIcons.Failed
	case "pending":
		icon = options.StatusIcons.Pending
	default:
		icon = options.StatusIcons.Pending
	}
	
	return icon
}

// LogPlainText formats a log entry for plain text output
func LogPlainText(level, category, message string) string {
	timestamp := fmt.Sprintf("%s", "2006-01-02 15:04:05") // Will be replaced with actual timestamp
	return fmt.Sprintf("[%s] %s:%s: %s", timestamp, level, category, message)
}

// IsCI returns true if running in a CI environment
func IsCI() bool {
	ciEnvVars := []string{
		"CI", "CONTINUOUS_INTEGRATION",
		"GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI",
		"TRAVIS", "JENKINS_URL", "BUILDKITE",
		"APPVEYOR", "AZURE_HTTP_USER_AGENT",
	}
	
	for _, envVar := range ciEnvVars {
		if os.Getenv(envVar) != "" {
			return true
		}
	}
	
	return false
}

// IsDebugMode returns true if debug mode is enabled
func IsDebugMode() bool {
	debug := os.Getenv("DEBUG")
	return debug != "" && debug != "0" && debug != "false"
}

// GetOutputMode determines the appropriate output mode
func GetOutputMode() OutputMode {
	info := GetTTYInfo()
	
	// Force plain output in CI or when explicitly requested
	if IsCI() || os.Getenv("IPCRAWLER_PLAIN") != "" {
		return OutputModePlain
	}
	
	// Use TUI if we have a proper terminal
	if info.IsTTY && info.Width >= 40 && info.Height >= 10 {
		return OutputModeTUI
	}
	
	// Fallback to plain output
	return OutputModePlain
}

// OutputMode represents different output modes
type OutputMode int

const (
	OutputModePlain OutputMode = iota
	OutputModeTUI
)

// String returns the string representation of OutputMode
func (o OutputMode) String() string {
	switch o {
	case OutputModePlain:
		return "plain"
	case OutputModeTUI:
		return "tui"
	default:
		return "unknown"
	}
}

// Utility functions

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		(s == substr || 
		 (len(s) > len(substr) && 
		  (s[:len(substr)] == substr || 
		   s[len(s)-len(substr):] == substr ||
		   containsSubstring(s, substr))))
}

// containsSubstring checks if s contains substr anywhere
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Enhanced Terminal Functions for TUI Integration

// GetTerminalCapabilities returns a comprehensive overview of terminal capabilities
func GetTerminalCapabilities() TerminalCapabilities {
	info := GetTTYInfo()
	
	return TerminalCapabilities{
		IsTTY:             info.IsTTY,
		Width:             info.Width,
		Height:            info.Height,
		ColorProfile:      ConvertFromTermenvProfile(info.Profile),
		TermenvProfile:    info.Profile,
		SupportsColor:     info.SupportsColor,
		SupportsTrueColor: info.SupportsTrueColor,
		SupportsUnicode:   checkUnicodeSupport(),
		IsCI:              IsCI(),
		IsDebug:           IsDebugMode(),
		OutputMode:        GetOutputMode(),
	}
}

// TerminalCapabilities represents comprehensive terminal capability information
type TerminalCapabilities struct {
	IsTTY             bool
	Width             int
	Height            int
	ColorProfile      ColorProfile
	TermenvProfile    termenv.Profile
	SupportsColor     bool
	SupportsTrueColor bool
	SupportsUnicode   bool
	IsCI              bool
	IsDebug           bool
	OutputMode        OutputMode
}

// checkUnicodeSupport determines if the terminal supports Unicode characters
func checkUnicodeSupport() bool {
	// Check environment variables that indicate Unicode support
	lang := os.Getenv("LANG")
	lcAll := os.Getenv("LC_ALL")
	lcCtype := os.Getenv("LC_CTYPE")
	
	// If any locale setting contains UTF-8, assume Unicode support
	locales := []string{lang, lcAll, lcCtype}
	for _, locale := range locales {
		if contains(locale, "UTF-8") || contains(locale, "utf8") {
			return true
		}
	}
	
	// Check TERM environment for Unicode-capable terminals
	term := os.Getenv("TERM")
	unicodeTerms := []string{
		"xterm", "screen", "tmux", "alacritty", "kitty", "wezterm",
	}
	
	for _, unicodeTerm := range unicodeTerms {
		if contains(term, unicodeTerm) {
			return true
		}
	}
	
	// Default to false for safety
	return false
}

// ShouldUseTUI determines if TUI mode should be used based on terminal capabilities
func ShouldUseTUI() bool {
	caps := GetTerminalCapabilities()
	
	// Check for explicit plain mode request first
	if os.Getenv("IPCRAWLER_PLAIN") != "" || os.Getenv("NO_TUI") != "" {
		return false
	}
	
	// Allow forcing TUI mode
	if os.Getenv("FORCE_TUI") != "" {
		return true
	}
	
	// Don't use TUI in CI environments unless explicitly requested
	if caps.IsCI {
		return false
	}
	
	// Must meet minimum size requirements
	if caps.Width < 40 || caps.Height < 10 {
		return false
	}
	
	// Check if we have a terminal-like environment even without strict TTY
	// This handles cases where terminal emulators don't provide full TTY access
	// but still support TUI features
	if !caps.IsTTY {
		// Allow TUI if we have terminal environment variables suggesting capability
		term := os.Getenv("TERM")
		termProgram := os.Getenv("TERM_PROGRAM")
		
		// Known terminal programs that support TUI
		supportedTerminals := []string{
			"iTerm.app", "Apple_Terminal", "WarpTerminal", "Alacritty", 
			"kitty", "WezTerm", "Hyper", "Terminal.app", "VSCode",
		}
		
		// Check for supported terminal programs
		for _, supported := range supportedTerminals {
			if termProgram == supported {
				return true
			}
		}
		
		// Check for capable TERM values
		if term != "" && (contains(term, "xterm") || contains(term, "screen") || 
			contains(term, "tmux") || term == "alacritty" || term == "kitty") {
			return true
		}
		
		// If no TTY and no recognizable terminal, don't use TUI
		return false
	}
	
	return true
}

// GetRecommendedLayoutMode returns the recommended layout mode for current terminal
func GetRecommendedLayoutMode() string {
	caps := GetTerminalCapabilities()
	
	if caps.Width >= 120 {
		return "large"   // Three-panel layout
	} else if caps.Width >= 80 {
		return "medium"  // Two-panel layout
	} else if caps.Width >= 40 {
		return "small"   // Single-panel with tabs
	} else {
		return "minimal" // Error state or forced minimal
	}
}