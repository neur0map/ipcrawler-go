package accessibility

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/carlosm/ipcrawler/internal/term"
	"github.com/carlosm/ipcrawler/internal/ui/config"
)

// AccessibilityManager handles all accessibility features
type AccessibilityManager struct {
	config           config.AccessibilityConfig
	terminalInfo     term.TTYInfo
	isScreenReader   bool
	isHighContrast   bool
	isColorDisabled  bool
	resizeDebouncer  *ResizeDebouncer
}

// ResizeDebouncer helps prevent excessive resize events
type ResizeDebouncer struct {
	lastResize    time.Time
	debounceDelay time.Duration
	pending       *tea.WindowSizeMsg
}

// NewAccessibilityManager creates a new accessibility manager
func NewAccessibilityManager() *AccessibilityManager {
	uiConfig := config.GetUIConfig()
	termInfo := term.GetTTYInfo()
	
	return &AccessibilityManager{
		config:          uiConfig.Theme.Accessibility,
		terminalInfo:    termInfo,
		isScreenReader:  detectScreenReader(),
		isHighContrast:  uiConfig.Theme.HighContrast || detectHighContrastPreference(),
		isColorDisabled: detectColorDisability(),
		resizeDebouncer: &ResizeDebouncer{
			debounceDelay: time.Millisecond * 150, // 150ms debounce
		},
	}
}

// detectScreenReader checks for screen reader presence
func detectScreenReader() bool {
	// Check common screen reader environment variables
	screenReaderVars := []string{
		"NVDA_PORT",           // NVDA
		"JAWS",                // JAWS
		"WINDOWEYES",          // Window-Eyes
		"DRAGON_NATURALLYSPEAKING", // Dragon
		"ORCA_PREFERENCES_PATH", // Orca (Linux)
	}
	
	for _, envVar := range screenReaderVars {
		if os.Getenv(envVar) != "" {
			return true
		}
	}
	
	// Check for accessibility programs in process list (simplified detection)
	if os.Getenv("ACCESSIBILITY_ENABLED") == "1" {
		return true
	}
	
	return false
}

// detectHighContrastPreference checks for high contrast preferences
func detectHighContrastPreference() bool {
	// Check environment variables
	if os.Getenv("HIGH_CONTRAST") == "1" || os.Getenv("FORCE_HIGH_CONTRAST") == "1" {
		return true
	}
	
	// Check TERM variable for high contrast indicators
	term := strings.ToLower(os.Getenv("TERM"))
	if strings.Contains(term, "mono") || strings.Contains(term, "contrast") {
		return true
	}
	
	return false
}

// detectColorDisability checks for color vision accessibility needs
func detectColorDisability() bool {
	// Check explicit environment variables
	if os.Getenv("NO_COLOR") != "" || os.Getenv("COLORBLIND") == "1" {
		return true
	}
	
	// Check if terminal doesn't support color
	if !term.SupportsColor() {
		return true
	}
	
	return false
}

// IsScreenReaderActive returns true if screen reader support is enabled
func (am *AccessibilityManager) IsScreenReaderActive() bool {
	return am.isScreenReader || am.config.ScreenReaderCompatible
}

// IsHighContrastMode returns true if high contrast mode is enabled
func (am *AccessibilityManager) IsHighContrastMode() bool {
	return am.isHighContrast
}

// IsColorDisabled returns true if color should be disabled
func (am *AccessibilityManager) IsColorDisabled() bool {
	return am.isColorDisabled || am.config.ColorDisabledFallback
}

// ShouldDebounceResize checks if a resize event should be debounced
func (am *AccessibilityManager) ShouldDebounceResize(msg tea.WindowSizeMsg) (bool, tea.Cmd) {
	now := time.Now()
	
	// If this is the first resize or enough time has passed, process immediately
	if am.resizeDebouncer.lastResize.IsZero() || 
	   now.Sub(am.resizeDebouncer.lastResize) > am.resizeDebouncer.debounceDelay {
		am.resizeDebouncer.lastResize = now
		am.resizeDebouncer.pending = nil
		return false, nil // Don't debounce, process now
	}
	
	// Store pending resize event
	am.resizeDebouncer.pending = &msg
	
	// Return a command to process the resize after debounce delay
	return true, tea.Tick(am.resizeDebouncer.debounceDelay, func(t time.Time) tea.Msg {
		if am.resizeDebouncer.pending != nil {
			// Return the stored resize event
			pendingMsg := *am.resizeDebouncer.pending
			am.resizeDebouncer.pending = nil
			am.resizeDebouncer.lastResize = t
			return DebouncedResizeMsg{
				WindowSizeMsg: pendingMsg,
			}
		}
		return nil
	})
}

// DebouncedResizeMsg wraps a debounced resize event
type DebouncedResizeMsg struct {
	WindowSizeMsg tea.WindowSizeMsg
}

// AdaptThemeForAccessibility modifies theme colors based on accessibility needs
func (am *AccessibilityManager) AdaptThemeForAccessibility(theme map[string]string) map[string]string {
	adapted := make(map[string]string)
	
	// Copy original theme
	for key, value := range theme {
		adapted[key] = value
	}
	
	// Apply color disability adaptations
	if am.IsColorDisabled() {
		adapted = am.applyColorBlindAdaptations(adapted)
	}
	
	// Apply high contrast adaptations
	if am.IsHighContrastMode() {
		adapted = am.applyHighContrastAdaptations(adapted)
	}
	
	// Apply screen reader adaptations
	if am.IsScreenReaderActive() {
		adapted = am.applyScreenReaderAdaptations(adapted)
	}
	
	return adapted
}

// applyColorBlindAdaptations removes problematic color combinations
func (am *AccessibilityManager) applyColorBlindAdaptations(theme map[string]string) map[string]string {
	// Use high contrast monochrome palette for color blindness
	return map[string]string{
		"primary":    "#ffffff",
		"secondary":  "#cccccc", 
		"background": "#000000",
		"border":     "#808080",
		"success":    "#ffffff",
		"warning":    "#cccccc",
		"error":      "#ffffff",
		"muted":      "#666666",
		"accent":     "#999999",
	}
}

// applyHighContrastAdaptations increases contrast ratios
func (am *AccessibilityManager) applyHighContrastAdaptations(theme map[string]string) map[string]string {
	// Enhanced contrast theme
	return map[string]string{
		"primary":    "#ffffff",
		"secondary":  "#ffffff",
		"background": "#000000",
		"border":     "#ffffff",
		"success":    "#ffffff",
		"warning":    "#ffffff", 
		"error":      "#ffffff",
		"muted":      "#cccccc",
		"accent":     "#ffffff",
	}
}

// applyScreenReaderAdaptations optimizes for screen reader compatibility
func (am *AccessibilityManager) applyScreenReaderAdaptations(theme map[string]string) map[string]string {
	// Screen readers work better with high contrast
	return am.applyHighContrastAdaptations(theme)
}

// FormatContentForAccessibility formats content for screen readers
func (am *AccessibilityManager) FormatContentForAccessibility(content string, contentType string) string {
	if !am.IsScreenReaderActive() {
		return content
	}
	
	switch contentType {
	case "status":
		return am.formatStatusForScreenReader(content)
	case "progress":
		return am.formatProgressForScreenReader(content)
	case "table":
		return am.formatTableForScreenReader(content)
	case "list":
		return am.formatListForScreenReader(content)
	default:
		return content
	}
}

// formatStatusForScreenReader adds descriptive text for status indicators
func (am *AccessibilityManager) formatStatusForScreenReader(content string) string {
	// Replace visual symbols with descriptive text
	replacements := map[string]string{
		"●": "Running: ",
		"✓": "Completed: ",
		"✗": "Failed: ",
		"○": "Pending: ",
		"⚠": "Warning: ",
	}
	
	result := content
	for symbol, description := range replacements {
		result = strings.ReplaceAll(result, symbol, description)
	}
	
	return result
}

// formatProgressForScreenReader adds percentage descriptions
func (am *AccessibilityManager) formatProgressForScreenReader(content string) string {
	// Add "percent complete" to progress indicators
	if strings.Contains(content, "%") {
		return content + " complete"
	}
	return content
}

// formatTableForScreenReader adds table navigation cues
func (am *AccessibilityManager) formatTableForScreenReader(content string) string {
	// Add table structure descriptions
	lines := strings.Split(content, "\n")
	if len(lines) > 0 {
		// Add table header indication
		lines[0] = "Table headers: " + lines[0]
		
		// Add row number for each data row
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) != "" {
				lines[i] = fmt.Sprintf("Row %d: %s", i, lines[i])
			}
		}
	}
	
	return strings.Join(lines, "\n")
}

// formatListForScreenReader adds list navigation cues
func (am *AccessibilityManager) formatListForScreenReader(content string) string {
	lines := strings.Split(content, "\n")
	itemCount := 0
	
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			itemCount++
			lines[i] = fmt.Sprintf("Item %d of %d: %s", itemCount, len(lines), line)
		}
	}
	
	if itemCount > 0 {
		return fmt.Sprintf("List with %d items:\n%s", itemCount, strings.Join(lines, "\n"))
	}
	
	return content
}

// GetAccessibilityReport generates a report of current accessibility settings
func (am *AccessibilityManager) GetAccessibilityReport() string {
	var report strings.Builder
	
	report.WriteString("Accessibility Status:\n")
	report.WriteString(fmt.Sprintf("  Screen Reader: %v\n", am.IsScreenReaderActive()))
	report.WriteString(fmt.Sprintf("  High Contrast: %v\n", am.IsHighContrastMode()))
	report.WriteString(fmt.Sprintf("  Color Disabled: %v\n", am.IsColorDisabled()))
	report.WriteString(fmt.Sprintf("  Terminal Color Support: %v\n", am.terminalInfo.SupportsColor))
	report.WriteString(fmt.Sprintf("  Terminal Size: %dx%d\n", am.terminalInfo.Width, am.terminalInfo.Height))
	
	if am.config.MinimumContrastRatio > 0 {
		report.WriteString(fmt.Sprintf("  Min Contrast Ratio: %.1f:1\n", am.config.MinimumContrastRatio))
	}
	
	return report.String()
}

// ValidateContrastRatio checks if colors meet minimum contrast requirements
func (am *AccessibilityManager) ValidateContrastRatio(foreground, background string) bool {
	if am.config.MinimumContrastRatio <= 0 {
		return true // No requirement set
	}
	
	// Simple contrast validation (would need full color parsing for complete implementation)
	// For now, validate common high-contrast combinations
	highContrastPairs := map[string][]string{
		"#ffffff": {"#000000"},
		"#000000": {"#ffffff"},
		"#cccccc": {"#000000"},
		"#333333": {"#ffffff"},
	}
	
	if validBackgrounds, exists := highContrastPairs[foreground]; exists {
		for _, validBg := range validBackgrounds {
			if validBg == background {
				return true
			}
		}
	}
	
	return false
}

// CreateAccessibleStyle creates a style adapted for accessibility
func (am *AccessibilityManager) CreateAccessibleStyle(baseStyle lipgloss.Style) lipgloss.Style {
	// Remove problematic styling for screen readers
	if am.IsScreenReaderActive() {
		return baseStyle.
			UnsetBlink().        // Remove blinking
			UnsetFaint().        // Remove faint text
			UnsetItalic()        // Remove italic for clarity
	}
	
	// Enhance contrast for high contrast mode
	if am.IsHighContrastMode() {
		return baseStyle.
			Bold(true).          // Make text bold
			UnsetFaint()         // Remove faint styling
	}
	
	return baseStyle
}

// GetRecommendedMinimumSize returns minimum recommended terminal size
func (am *AccessibilityManager) GetRecommendedMinimumSize() (width, height int) {
	// Default minimums
	width, height = 40, 10
	
	// Increase minimums for accessibility
	if am.IsScreenReaderActive() {
		width = 60  // More space for descriptive text
		height = 15 // More space for navigation cues
	}
	
	if am.IsHighContrastMode() {
		width = 50  // Slightly larger for better readability
		height = 12
	}
	
	return width, height
}

// ShouldUseAlternativeLayout returns true if alternative layout should be used
func (am *AccessibilityManager) ShouldUseAlternativeLayout() bool {
	return am.IsScreenReaderActive() || 
		   (am.terminalInfo.Width < 60 && am.IsHighContrastMode())
}

// GetAccessibilityAnnouncement creates announcements for screen readers
func (am *AccessibilityManager) GetAccessibilityAnnouncement(event string, data map[string]interface{}) string {
	if !am.IsScreenReaderActive() {
		return ""
	}
	
	switch event {
	case "workflow_start":
		if workflow, ok := data["workflow"].(string); ok {
			return fmt.Sprintf("Started workflow: %s", workflow)
		}
	case "workflow_complete":
		if workflow, ok := data["workflow"].(string); ok {
			if success, ok := data["success"].(bool); ok {
				status := "completed successfully"
				if !success {
					status = "failed"
				}
				return fmt.Sprintf("Workflow %s %s", workflow, status)
			}
		}
	case "tool_execution":
		if tool, ok := data["tool"].(string); ok {
			return fmt.Sprintf("Tool %s finished execution", tool)
		}
	case "focus_change":
		if component, ok := data["component"].(string); ok {
			return fmt.Sprintf("Focus moved to %s", component)
		}
	case "layout_change":
		if mode, ok := data["mode"].(string); ok {
			return fmt.Sprintf("Layout changed to %s mode", mode)
		}
	}
	
	return ""
}