package styles

import (
	"strings"
	"github.com/charmbracelet/lipgloss"
	"github.com/your-org/ipcrawler/internal/ui"
)

// Theme contains all styling information for the TUI
type Theme struct {
	// Panel styles
	PanelStyle        lipgloss.Style
	FocusedPanelStyle lipgloss.Style
	FooterStyle       lipgloss.Style

	// Component styles  
	ListStyle     lipgloss.Style
	ViewportStyle lipgloss.Style
	StatusStyle   lipgloss.Style

	// Text styles
	TitleStyle       lipgloss.Style
	DescriptionStyle lipgloss.Style
	ErrorStyle       lipgloss.Style
	SuccessStyle     lipgloss.Style
	WarningStyle     lipgloss.Style

	// Colors
	Colors Colors
}

// Colors defines the color palette
type Colors struct {
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Accent    lipgloss.Color
	Success   lipgloss.Color
	Warning   lipgloss.Color
	Error     lipgloss.Color
	Border    lipgloss.Color

	// Adaptive colors for light/dark terminals
	TextPrimary lipgloss.AdaptiveColor
	BorderAdaptive lipgloss.AdaptiveColor
}

// NewTheme creates a new theme from configuration
func NewTheme(config *ui.Config) *Theme {
	// Initialize colors from config
	colors := Colors{
		Primary:   lipgloss.Color(config.UI.Theme.Colors.Primary),
		Secondary: lipgloss.Color(config.UI.Theme.Colors.Secondary),
		Accent:    lipgloss.Color(config.UI.Theme.Colors.Accent),
		Success:   lipgloss.Color(config.UI.Theme.Colors.Success),
		Warning:   lipgloss.Color(config.UI.Theme.Colors.Warning),
		Error:     lipgloss.Color(config.UI.Theme.Colors.Error),
		Border:    lipgloss.Color(config.UI.Theme.Colors.Border),
		TextPrimary: lipgloss.AdaptiveColor{
			Light: config.UI.Theme.Adaptive.TextPrimary.Light,
			Dark:  config.UI.Theme.Adaptive.TextPrimary.Dark,
		},
		BorderAdaptive: lipgloss.AdaptiveColor{
			Light: config.UI.Theme.Adaptive.Border.Light,
			Dark:  config.UI.Theme.Adaptive.Border.Dark,
		},
	}

	theme := &Theme{Colors: colors}
	theme.initializeStyles()
	return theme
}

// initializeStyles sets up all the styling rules
func (t *Theme) initializeStyles() {
	// Base panel style with borders and spacing
	t.PanelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Colors.BorderAdaptive).
		Padding(1).
		Margin(0, 1).
		Align(lipgloss.Left, lipgloss.Top)

	// Focused panel style with accent border
	t.FocusedPanelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Colors.Accent).
		Padding(1).
		Margin(0, 1).
		Align(lipgloss.Left, lipgloss.Top)

	// Footer style for two-column layout
	t.FooterStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Colors.BorderAdaptive).
		Padding(0, 1).
		Margin(1, 0, 0, 0).
		Align(lipgloss.Left, lipgloss.Center)

	// List component style
	t.ListStyle = lipgloss.NewStyle().
		Foreground(t.Colors.TextPrimary).
		MarginRight(1)

	// Viewport component style
	t.ViewportStyle = lipgloss.NewStyle().
		Foreground(t.Colors.TextPrimary)

	// Status component style
	t.StatusStyle = lipgloss.NewStyle().
		Foreground(t.Colors.TextPrimary).
		Align(lipgloss.Center)

	// Text styles
	t.TitleStyle = lipgloss.NewStyle().
		Foreground(t.Colors.Accent).
		Bold(true).
		MarginBottom(1)

	t.DescriptionStyle = lipgloss.NewStyle().
		Foreground(t.Colors.Secondary).
		Italic(true)

	t.ErrorStyle = lipgloss.NewStyle().
		Foreground(t.Colors.Error).
		Bold(true)

	t.SuccessStyle = lipgloss.NewStyle().
		Foreground(t.Colors.Success).
		Bold(true)

	t.WarningStyle = lipgloss.NewStyle().
		Foreground(t.Colors.Warning).
		Bold(true)
}

// GetStatusColor returns the appropriate color for a status
func (t *Theme) GetStatusColor(status string) lipgloss.Color {
	switch status {
	case "running", "active":
		return t.Colors.Success
	case "pending", "waiting":
		return t.Colors.Warning
	case "failed", "error":
		return t.Colors.Error
	case "completed", "success":
		return t.Colors.Success
	default:
		return t.Colors.Secondary
	}
}

// GetLogLevelColor returns the appropriate color for a log level
func (t *Theme) GetLogLevelColor(level string) lipgloss.Color {
	switch level {
	case "error":
		return t.Colors.Error
	case "warn", "warning":
		return t.Colors.Warning
	case "info":
		return t.Colors.Primary
	case "debug":
		return t.Colors.Secondary
	default:
		return lipgloss.Color("248")
	}
}

// RenderTitle renders a styled title
func (t *Theme) RenderTitle(title string) string {
	return t.TitleStyle.Render(title)
}

// RenderDescription renders a styled description
func (t *Theme) RenderDescription(description string) string {
	return t.DescriptionStyle.Render(description)
}

// RenderStatus renders a status with appropriate color
func (t *Theme) RenderStatus(status string) string {
	color := t.GetStatusColor(status)
	style := lipgloss.NewStyle().Foreground(color).Bold(true)
	return style.Render(status)
}

// RenderLogEntry renders a log entry with level-based coloring and proper wrapping
func (t *Theme) RenderLogEntry(timestamp, level, source, message string) string {
	levelColor := t.GetLogLevelColor(level)
	
	timestampStyle := lipgloss.NewStyle().Foreground(t.Colors.Secondary)
	levelStyle := lipgloss.NewStyle().Foreground(levelColor).Bold(true).Width(5).Align(lipgloss.Right)
	sourceStyle := lipgloss.NewStyle().Foreground(t.Colors.Accent).Width(10).Align(lipgloss.Left)
	messageStyle := lipgloss.NewStyle().Foreground(t.Colors.TextPrimary)

	// CRITICAL: Handle multiline messages properly (from text wrapping)
	messageLines := strings.Split(message, "\n")
	firstLine := messageStyle.Render(messageLines[0])
	
	// Format first line with full prefix
	result := lipgloss.JoinHorizontal(
		lipgloss.Top,
		timestampStyle.Render(timestamp),
		" ",
		levelStyle.Render(level),
		" ",
		sourceStyle.Render(source),
		" ",
		firstLine,
	)
	
	// Add continuation lines with proper indentation
	if len(messageLines) > 1 {
		indentWidth := 8 + 1 + 5 + 1 + 10 + 1 // timestamp + space + level + space + source + space
		indent := strings.Repeat(" ", indentWidth)
		
		for _, line := range messageLines[1:] {
			if strings.TrimSpace(line) != "" {
				result += "\n" + indent + messageStyle.Render(line)
			}
		}
	}
	
	return result
}

// Default theme colors for fallback
var DefaultColors = Colors{
	Primary:   lipgloss.Color("#FAFAFA"),
	Secondary: lipgloss.Color("#3C3C3C"), 
	Accent:    lipgloss.Color("#7D56F4"),
	Success:   lipgloss.Color("#04B575"),
	Warning:   lipgloss.Color("#F59E0B"),
	Error:     lipgloss.Color("#EF4444"),
	Border:    lipgloss.Color("#E5E5E5"),
	TextPrimary: lipgloss.AdaptiveColor{
		Light: "236",
		Dark:  "248",
	},
	BorderAdaptive: lipgloss.AdaptiveColor{
		Light: "240",
		Dark:  "238",
	},
}