package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// 🎨 Modern Monochrome Palette - Clean & Minimal
var (
	// Error Color - Only color used in the theme
	ErrorColor = lipgloss.Color("#EF4444") // Red for errors only
	
	// Monochrome Colors - Grayscale only
	White      = lipgloss.Color("#FFFFFF") // Pure White
	Gray50     = lipgloss.Color("#FAFAFA") // Almost White
	Gray100    = lipgloss.Color("#F5F5F5") // Very Light Gray
	Gray200    = lipgloss.Color("#E5E5E5") // Light Gray
	Gray300    = lipgloss.Color("#D4D4D4") // Gray
	Gray400    = lipgloss.Color("#A3A3A3") // Medium Gray
	Gray500    = lipgloss.Color("#737373") // Dark Gray
	Gray600    = lipgloss.Color("#525252") // Darker Gray
	Gray700    = lipgloss.Color("#404040") // Very Dark Gray
	Gray800    = lipgloss.Color("#262626") // Almost Black
	Gray900    = lipgloss.Color("#171717") // Near Black
	Black      = lipgloss.Color("#000000") // Pure Black
	
	// Adaptive Colors for Light/Dark themes (monochrome only)
	TextPrimary   = lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}
	TextSecondary = lipgloss.AdaptiveColor{Light: "#525252", Dark: "#A3A3A3"}
	TextMuted     = lipgloss.AdaptiveColor{Light: "#737373", Dark: "#737373"}
	BorderColor   = lipgloss.AdaptiveColor{Light: "#D4D4D4", Dark: "#404040"}
)

// 🎯 Modern Monochrome Message Styles - Clean badges without fills
var (
	// Badge-style status messages - only borders, no background fills
	ModernSuccessStyle = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Bold(true).
		Padding(0, 1).
		MarginRight(1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Gray600)

	ModernInfoStyle = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Bold(true).
		Padding(0, 1).
		MarginRight(1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Gray500)

	ModernWarningStyle = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Bold(true).
		Padding(0, 1).
		MarginRight(1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Gray700)

	ModernErrorStyle = lipgloss.NewStyle().
		Foreground(White).
		Background(ErrorColor).
		Bold(true).
		Padding(0, 1).
		MarginRight(1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ErrorColor)

	// Text-only styles for inline use - all monochrome except errors
	SuccessText   = lipgloss.NewStyle().Foreground(TextPrimary).Bold(true)
	InfoText      = lipgloss.NewStyle().Foreground(TextPrimary).Bold(true)
	WarningText   = lipgloss.NewStyle().Foreground(TextPrimary).Bold(true)
	ErrorText     = lipgloss.NewStyle().Foreground(ErrorColor).Bold(true)
	PrimaryText   = lipgloss.NewStyle().Foreground(TextPrimary).Bold(true)
	SecondaryText = lipgloss.NewStyle().Foreground(TextSecondary)

	// 🎪 Banner and Header Styles - Clean monochrome, no fills
	ModernBannerStyle = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Bold(true).
		Padding(1, 2).
		Margin(1, 0, 1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor).
		Align(lipgloss.Center)

	ModernSectionHeaderStyle = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Bold(true).
		Padding(0, 2).
		Margin(1, 0).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor)

	// Layout utilities - clean cards with borders only
	ModernCardStyle = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Padding(1, 2).
		Margin(0, 0, 1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor)

	// 📊 Modern Table Styling - Clean monochrome
	ModernTableHeaderStyle = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Bold(true).
		Align(lipgloss.Center).
		Padding(0, 2)

	ModernTableCellStyle = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Padding(0, 2)

	ModernTableOddRowStyle = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Padding(0, 2)

	ModernTableEvenRowStyle = lipgloss.NewStyle().
		Foreground(TextSecondary).
		Padding(0, 2)

	ModernTableBorderStyle = lipgloss.NewStyle().
		Foreground(BorderColor)

	// 🔄 Progress and Interactive Styles - Clean monochrome
	ModernSpinnerStyle = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Bold(true).
		Padding(0, 1)

	ModernProgressStyle = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Bold(true).
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor)

	ModernPromptStyle = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Bold(true).
		Padding(0, 1)

	// Highlight styles for important text - minimal styling
	HighlightStyle = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Bold(true).
		Padding(0, 1)

	CodeStyle = lipgloss.NewStyle().
		Foreground(TextPrimary).
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor)
)

// 🔤 Modern Icons and Prefixes
var (
	SuccessIcon = "✅"
	InfoIcon    = "ℹ️"
	WarningIcon = "⚠️"
	ErrorIcon   = "❌"
	SecurityIcon = "🔒"
	NetworkIcon = "🌐"
	ToolIcon    = "🔧"
	ReportIcon  = "📊"
	SpinnerIcon = "⏳"
	RocketIcon  = "🚀"
	TargetIcon  = "🎯"
	FolderIcon  = "📁"
	CheckIcon   = "✓"
	CrossIcon   = "✗"
	ArrowIcon   = "→"
	BulletIcon  = "•"
)

// 🎨 Layout Helper Functions - Clean monochrome
func CreateCard(title, content string) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(TextPrimary).
		Bold(true).
		Padding(0, 1).
		MarginBottom(1)
	
	cardContent := lipgloss.JoinVertical(
		lipgloss.Left,
		headerStyle.Render("▊ "+title),
		content,
	)
	
	return ModernCardStyle.Render(cardContent)
}

func CreateSection(title, content string) string {
	header := ModernSectionHeaderStyle.Render(" " + title + " ")
	body := lipgloss.NewStyle().Padding(1, 2).Render(content)
	
	return lipgloss.JoinVertical(lipgloss.Left, header, body)
}

func CreateBadge(text, badgeType string) string {
	var style lipgloss.Style
	var icon string
	
	switch badgeType {
	case "success":
		style = ModernSuccessStyle
		icon = "✓"
	case "error":
		style = ModernErrorStyle
		icon = "✗"
	case "warning":
		style = ModernWarningStyle
		icon = "!"
	case "info":
		style = ModernInfoStyle
		icon = "i"
	default:
		style = ModernInfoStyle
		icon = "i"
	}
	
	return style.Render(icon + " " + text)
}

func CreateInfoPanel(items [][]string) string {
	var rows []string
	
	for _, item := range items {
		if len(item) >= 2 {
			key := PrimaryText.Render(item[0] + ":")
			value := SecondaryText.Render(item[1])
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left, key, " ", value))
		}
	}
	
	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return ModernCardStyle.Render(content)
}

// Layout utilities - Clean alignment
var (
	CenterStyle = lipgloss.NewStyle().Align(lipgloss.Center)
	LeftStyle   = lipgloss.NewStyle().Align(lipgloss.Left)
	RightStyle  = lipgloss.NewStyle().Align(lipgloss.Right)
)