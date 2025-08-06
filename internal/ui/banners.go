package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Banners handles all banner and header displays
type Banners struct{}

// NewBanners creates a new Banners instance
func NewBanners() *Banners {
	return &Banners{}
}

// ShowApplicationBanner displays the main application banner
func (b *Banners) ShowApplicationBanner(version, target, template string) {
	// Clean monochrome banner - no background fills
	brandName := lipgloss.NewStyle().
		Foreground(TextPrimary).
		Bold(true).
		Padding(1, 2).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor).
		Render("🚀 IPCrawler v" + version + " 🚀")

	targetInfo := lipgloss.NewStyle().
		Foreground(TextPrimary).
		Bold(true).
		Padding(0, 1).
		Render("🎯 Target: " + HighlightStyle.Render(target))

	templateInfo := lipgloss.NewStyle().
		Foreground(TextSecondary).
		Bold(true).
		Padding(0, 1).
		Render("🔧 Template: " + HighlightStyle.Render(template))

	// Create a clean layout with proper alignment
	headerContent := lipgloss.JoinVertical(
		lipgloss.Center,
		brandName,
		lipgloss.JoinHorizontal(lipgloss.Left, targetInfo, "     ", templateInfo),
	)

	// Center everything with clean styling
	centeredBanner := lipgloss.NewStyle().
		Align(lipgloss.Center).
		Width(80).
		Render(headerContent)
	fmt.Println(centeredBanner)
	fmt.Println()
}

// ShowSectionHeader displays a section header
func (b *Banners) ShowSectionHeader(title string) {
	header := CreateSection(title, "")
	fmt.Println(header)
}

// ShowHeader displays a main header with custom styling
func (b *Banners) ShowHeader(title, subtitle string) {
	// Create a beautiful header card
	headerContent := PrimaryText.Render("▊ " + title)
	
	if subtitle != "" {
		subtitleContent := SecondaryText.Render(subtitle)
		headerContent = lipgloss.JoinVertical(
			lipgloss.Left,
			headerContent,
			subtitleContent,
		)
	}
	
	cardHeader := ModernCardStyle.Render(headerContent)
	fmt.Println(cardHeader)
}