package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// LayoutCalculator handles responsive layout calculations
type LayoutCalculator struct {
	config *UIConfig
	width  int
	height int
	mode   LayoutMode
}

// NewLayoutCalculator creates a new layout calculator
func NewLayoutCalculator(config *UIConfig, width, height int, mode LayoutMode) *LayoutCalculator {
	return &LayoutCalculator{
		config: config,
		width:  width,
		height: height,
		mode:   mode,
	}
}

// CalculatePanelDimensions returns the available dimensions for each panel
func (lc *LayoutCalculator) CalculatePanelDimensions() PanelDimensions {
	switch lc.mode {
	case LayoutLarge:
		return lc.calculateLargePanelDimensions()
	case LayoutMedium:
		return lc.calculateMediumPanelDimensions()
	case LayoutSmall:
		return lc.calculateSmallPanelDimensions()
	default:
		return lc.calculateSmallPanelDimensions()
	}
}

// PanelDimensions holds the calculated dimensions for each panel
type PanelDimensions struct {
	NavWidth     int
	NavHeight    int
	MainWidth    int
	MainHeight   int
	StatusWidth  int
	StatusHeight int
	
	// Content area (excluding borders and padding)
	NavContentWidth     int
	NavContentHeight    int
	MainContentWidth    int
	MainContentHeight   int
	StatusContentWidth  int
	StatusContentHeight int
}

func (lc *LayoutCalculator) calculateLargePanelDimensions() PanelDimensions {
	// Calculate raw panel widths
	navWidth := int(float64(lc.width) * lc.config.Layout.Panels.Navigation.PreferredWidthRatio)
	statusWidth := int(float64(lc.width) * lc.config.Layout.Panels.Status.PreferredWidthRatio)
	mainWidth := lc.width - navWidth - statusWidth

	// Ensure minimum widths
	if navWidth < lc.config.Layout.Panels.Navigation.MinWidth {
		navWidth = lc.config.Layout.Panels.Navigation.MinWidth
	}
	if statusWidth < lc.config.Layout.Panels.Status.MinWidth {
		statusWidth = lc.config.Layout.Panels.Status.MinWidth
	}
	if mainWidth < lc.config.Layout.Panels.Main.MinWidth {
		mainWidth = lc.config.Layout.Panels.Main.MinWidth
	}

	// If panels don't fit, adjust proportionally
	totalRequired := navWidth + mainWidth + statusWidth
	if totalRequired > lc.width {
		scale := float64(lc.width) / float64(totalRequired)
		navWidth = int(float64(navWidth) * scale)
		statusWidth = int(float64(statusWidth) * scale)
		mainWidth = lc.width - navWidth - statusWidth
	}

	// Calculate height (same for all panels in large mode)
	headerHeight := 0
	if lc.height > lc.config.Layout.Header.ShowWhenHeightAbove {
		headerHeight = lc.config.Layout.Header.Height
	}
	panelHeight := lc.height - headerHeight

	// Calculate content dimensions (account for borders and padding)
	borderPadding := 4 // 2 for border + 2 for padding (rounded border + padding(1))
	
	return PanelDimensions{
		NavWidth:    navWidth,
		NavHeight:   panelHeight,
		MainWidth:   mainWidth,
		MainHeight:  panelHeight,
		StatusWidth: statusWidth,
		StatusHeight: panelHeight,
		
		NavContentWidth:    maxInt(1, navWidth-borderPadding),
		NavContentHeight:   maxInt(1, panelHeight-borderPadding),
		MainContentWidth:   maxInt(1, mainWidth-borderPadding),
		MainContentHeight:  maxInt(1, panelHeight-borderPadding),
		StatusContentWidth: maxInt(1, statusWidth-borderPadding),
		StatusContentHeight: maxInt(1, panelHeight-borderPadding),
	}
}

func (lc *LayoutCalculator) calculateMediumPanelDimensions() PanelDimensions {
	// Two panel layout: nav + main
	navWidth := int(float64(lc.width) * 0.33) // 33%
	mainWidth := lc.width - navWidth          // 67%

	// Ensure minimum widths
	if navWidth < lc.config.Layout.Panels.Navigation.MinWidth {
		navWidth = lc.config.Layout.Panels.Navigation.MinWidth
		mainWidth = lc.width - navWidth
	}
	if mainWidth < lc.config.Layout.Panels.Main.MinWidth {
		mainWidth = lc.config.Layout.Panels.Main.MinWidth
		navWidth = lc.width - mainWidth
	}

	// Calculate height
	headerHeight := 0
	if lc.height > lc.config.Layout.Header.ShowWhenHeightAbove {
		headerHeight = lc.config.Layout.Header.Height
	}
	panelHeight := lc.height - headerHeight - lc.config.Layout.Footer.Height

	// Calculate content dimensions
	borderPadding := 4
	
	return PanelDimensions{
		NavWidth:    navWidth,
		NavHeight:   panelHeight,
		MainWidth:   mainWidth,
		MainHeight:  panelHeight,
		StatusWidth: lc.width, // Status becomes footer in medium mode
		StatusHeight: lc.config.Layout.Footer.Height,
		
		NavContentWidth:    maxInt(1, navWidth-borderPadding),
		NavContentHeight:   maxInt(1, panelHeight-borderPadding),
		MainContentWidth:   maxInt(1, mainWidth-borderPadding),
		MainContentHeight:  maxInt(1, panelHeight-borderPadding),
		StatusContentWidth: maxInt(1, lc.width-borderPadding),
		StatusContentHeight: maxInt(1, lc.config.Layout.Footer.Height-2), // Less padding for footer
	}
}

func (lc *LayoutCalculator) calculateSmallPanelDimensions() PanelDimensions {
	// Single panel layout
	headerHeight := 0
	if lc.height > lc.config.Layout.Header.ShowWhenHeightAbove {
		headerHeight = lc.config.Layout.Header.Height
	}
	tabHeight := 2 // Tab bar height
	contentHeight := lc.height - headerHeight - tabHeight

	borderPadding := 4
	
	return PanelDimensions{
		NavWidth:    lc.width,
		NavHeight:   contentHeight,
		MainWidth:   lc.width,
		MainHeight:  contentHeight,
		StatusWidth: lc.width,
		StatusHeight: contentHeight,
		
		NavContentWidth:    maxInt(1, lc.width-borderPadding),
		NavContentHeight:   maxInt(1, contentHeight-borderPadding),
		MainContentWidth:   maxInt(1, lc.width-borderPadding),
		MainContentHeight:  maxInt(1, contentHeight-borderPadding),
		StatusContentWidth: maxInt(1, lc.width-borderPadding),
		StatusContentHeight: maxInt(1, contentHeight-borderPadding),
	}
}

// RenderConstrainedPanel renders a panel with exact size constraints
func (lc *LayoutCalculator) RenderConstrainedPanel(content string, width, height int, focused bool) string {
	// Get border style from config
	borderStyle := lipgloss.RoundedBorder()
	borderColor := lipgloss.Color(lc.config.Layout.Borders.Color)
	if focused {
		borderColor = lipgloss.Color(lc.config.Theme.Colors.Accent)
	}

	// Create constrained style - use MaxWidth/MaxHeight for hard limits
	style := lipgloss.NewStyle().
		MaxWidth(width).
		MaxHeight(height).
		Border(borderStyle).
		BorderForeground(borderColor).
		Padding(1)

	return style.Render(content)
}

// maxInt returns the maximum of two integers
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}