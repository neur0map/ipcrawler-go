package layout

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/carlosm/ipcrawler/internal/ui/config"
)

// LayoutMode represents different responsive layout modes
type LayoutMode int

const (
	// LargeLayout - Three panels: Navigation | Main | Status (≥120 cols)
	LargeLayout LayoutMode = iota
	// MediumLayout - Two panels: Navigation | Main+Status (80-119 cols)  
	MediumLayout
	// SmallLayout - Single panel with tabbed/stacked content (<80 cols)
	SmallLayout
)

// String returns the string representation of LayoutMode
func (l LayoutMode) String() string {
	switch l {
	case LargeLayout:
		return "large"
	case MediumLayout:
		return "medium"
	case SmallLayout:
		return "small"
	default:
		return "unknown"
	}
}

// Dimensions represents width and height measurements
type Dimensions struct {
	Width  int
	Height int
}

// LayoutDimensions contains calculated dimensions for all layout components
type LayoutDimensions struct {
	Terminal          Dimensions // Overall terminal size
	Header            Dimensions // Header area
	LeftPanel         Dimensions // Navigation/menu panel
	MainContent       Dimensions // Primary content area
	RightPanel        Dimensions // Status/info panel (large layout only)
	StatusFooter      Dimensions // Status footer (medium layout)
	HorizontalPadding int        // Padding for panels
	VerticalPadding   int        // Padding for panels
}

// DetermineLayout calculates the appropriate layout mode based on terminal size and configuration
func DetermineLayout(width, height int) LayoutMode {
	uiConfig := config.GetUIConfig()
	breakpoints := uiConfig.Layout.Breakpoints
	
	// Use configuration-driven breakpoints instead of hardcoded values
	if width >= breakpoints.Large {
		return LargeLayout // Three-panel layout for wide terminals
	} else if width >= breakpoints.Medium {
		return MediumLayout // Two-panel layout for medium terminals
	} else {
		return SmallLayout // Single-panel layout for narrow terminals
	}
}

// Layout handles responsive design calculations and rendering
type Layout struct {
	mode       LayoutMode
	dimensions LayoutDimensions
	styles     map[string]lipgloss.Style
}

// New creates a new layout instance
func New(width, height int) *Layout {
	mode := DetermineLayout(width, height)
	dimensions := calculateDimensions(mode, width, height)
	
	return &Layout{
		mode:       mode,
		dimensions: dimensions,
		styles:     createLayoutStyles(mode, dimensions),
	}
}

// Update recalculates layout for new terminal dimensions
func (l *Layout) Update(width, height int) {
	l.mode = DetermineLayout(width, height)
	l.dimensions = calculateDimensions(l.mode, width, height)
	l.styles = createLayoutStyles(l.mode, l.dimensions)
}

// Mode returns the current layout mode
func (l *Layout) Mode() LayoutMode {
	return l.mode
}

// Dimensions returns the current layout dimensions
func (l *Layout) Dimensions() LayoutDimensions {
	return l.dimensions
}

// RenderThreePanel renders the large layout: [Nav] | [Main] | [Status]
func (l *Layout) RenderThreePanel(nav, main, status string) string {
	if l.mode != LargeLayout {
		return l.RenderTwoPanel(nav, main) // Fallback
	}

	navStyled := l.styles["nav"].Render(nav)
	mainStyled := l.styles["main"].Render(main)
	statusStyled := l.styles["status"].Render(status)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		navStyled,
		mainStyled,
		statusStyled,
	)
}

// RenderTwoPanel renders the medium layout: [Nav] | [Main+Status]
func (l *Layout) RenderTwoPanel(nav, main string) string {
	if l.mode == SmallLayout {
		return l.RenderSinglePanel(main) // Fallback
	}

	navStyled := l.styles["nav"].Render(nav)
	mainStyled := l.styles["main_wide"].Render(main)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		navStyled,
		mainStyled,
	)
}

// RenderTwoPanelWithFooter renders medium layout with footer status
func (l *Layout) RenderTwoPanelWithFooter(nav, main, footer string) string {
	topPanel := l.RenderTwoPanel(nav, main)
	footerStyled := l.styles["footer"].Render(footer)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		topPanel,
		footerStyled,
	)
}

// RenderSinglePanel renders the small layout with single focused panel
func (l *Layout) RenderSinglePanel(content string) string {
	return l.styles["single"].Render(content)
}

// RenderWithHeader adds a header to any layout
func (l *Layout) RenderWithHeader(header, content string) string {
	headerStyled := l.styles["header"].Render(header)
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		headerStyled,
		content,
	)
}

// RenderWithFooter adds a footer to any layout
func (l *Layout) RenderWithFooter(content, footer string) string {
	footerStyled := l.styles["footer"].Render(footer)
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		footerStyled,
	)
}

// GetNavPanelStyle returns the styled container for navigation panel
func (l *Layout) GetNavPanelStyle() lipgloss.Style {
	return l.styles["nav"]
}

// GetMainPanelStyle returns the styled container for main content
func (l *Layout) GetMainPanelStyle() lipgloss.Style {
	return l.styles["main"]
}

// GetStatusPanelStyle returns the styled container for status panel
func (l *Layout) GetStatusPanelStyle() lipgloss.Style {
	return l.styles["status"]
}

// GetHeaderStyle returns the styled container for header
func (l *Layout) GetHeaderStyle() lipgloss.Style {
	return l.styles["header"]
}

// GetFooterStyle returns the styled container for footer
func (l *Layout) GetFooterStyle() lipgloss.Style {
	return l.styles["footer"]
}

// calculateDimensions computes panel dimensions based on layout mode, terminal size, and configuration
func calculateDimensions(mode LayoutMode, width, height int) LayoutDimensions {
	// Get configuration instead of using hardcoded values
	uiConfig := config.GetUIConfig()
	layoutConfig := uiConfig.Layout
	
	// Use configuration-driven values
	borderWidth := layoutConfig.Borders.Width
	panelSpacing := layoutConfig.Spacing.Small // Space between panels
	headerHeight := layoutConfig.Header.Height
	footerHeight := layoutConfig.Footer.Height
	minPadding := layoutConfig.Spacing.Small

	dims := LayoutDimensions{
		Terminal: Dimensions{
			Width:  width,
			Height: height,
		},
		HorizontalPadding: minPadding,
		VerticalPadding:   minPadding,
	}

	// Calculate available space
	availableWidth := width - (borderWidth * 3) - (panelSpacing * 2) // 3 panels + 2 spaces
	availableHeight := height - headerHeight - dims.VerticalPadding*2

	switch mode {
	case LargeLayout:
		// Three panels using configuration-driven ratios
		navRatio := layoutConfig.Panels.Navigation.PreferredWidthRatio
		statusRatio := layoutConfig.Panels.Status.PreferredWidthRatio
		
		navWidth := int(float64(availableWidth) * navRatio)
		statusWidth := int(float64(availableWidth) * statusRatio)
		mainWidth := availableWidth - navWidth - statusWidth
		
		// Ensure minimum widths from configuration
		if navWidth < layoutConfig.Panels.Navigation.MinWidth {
			navWidth = layoutConfig.Panels.Navigation.MinWidth
		}
		if statusWidth < layoutConfig.Panels.Status.MinWidth {
			statusWidth = layoutConfig.Panels.Status.MinWidth
		}
		if mainWidth < layoutConfig.Panels.Main.MinWidth {
			mainWidth = layoutConfig.Panels.Main.MinWidth
		}

		dims.LeftPanel = Dimensions{Width: navWidth, Height: availableHeight}
		dims.MainContent = Dimensions{Width: mainWidth, Height: availableHeight}
		dims.RightPanel = Dimensions{Width: statusWidth, Height: availableHeight}

	case MediumLayout:
		// Two panels using configuration-driven ratios
		navRatio := layoutConfig.Panels.Navigation.PreferredWidthRatio
		navWidth := int(float64(availableWidth) * navRatio)
		mainWidth := availableWidth - navWidth
		contentHeight := availableHeight - footerHeight
		
		// Ensure minimum widths
		if navWidth < layoutConfig.Panels.Navigation.MinWidth {
			navWidth = layoutConfig.Panels.Navigation.MinWidth
			mainWidth = availableWidth - navWidth
		}
		if mainWidth < layoutConfig.Panels.Main.MinWidth {
			mainWidth = layoutConfig.Panels.Main.MinWidth
			navWidth = availableWidth - mainWidth
		}

		dims.LeftPanel = Dimensions{Width: navWidth, Height: contentHeight}
		dims.MainContent = Dimensions{Width: mainWidth, Height: contentHeight}
		dims.StatusFooter = Dimensions{Width: width - borderWidth, Height: footerHeight}

	case SmallLayout:
		// Single panel using full available space
		dims.MainContent = Dimensions{
			Width:  width - borderWidth,
			Height: availableHeight,
		}
	}

	// Header spans full width minus borders
	dims.Header = Dimensions{
		Width:  width - borderWidth,
		Height: headerHeight,
	}

	return dims
}

// createLayoutStyles creates lipgloss styles for each panel based on calculated dimensions and configuration
func createLayoutStyles(mode LayoutMode, dims LayoutDimensions) map[string]lipgloss.Style {
	styles := make(map[string]lipgloss.Style)
	
	// Get configuration for styling
	uiConfig := config.GetUIConfig()
	layoutConfig := uiConfig.Layout
	themeConfig := uiConfig.Theme
	
	// Determine border style from configuration
	var borderStyle lipgloss.Border
	switch layoutConfig.Borders.Style {
	case "rounded":
		borderStyle = lipgloss.RoundedBorder()
	case "normal":
		borderStyle = lipgloss.NormalBorder()
	case "thick":
		borderStyle = lipgloss.ThickBorder()
	case "double":
		borderStyle = lipgloss.DoubleBorder()
	default:
		borderStyle = lipgloss.RoundedBorder()
	}
	
	// Base style with configuration-driven border and colors
	baseStyle := lipgloss.NewStyle().
		BorderStyle(borderStyle).
		BorderForeground(lipgloss.Color(layoutConfig.Borders.Color)).
		Padding(dims.VerticalPadding, dims.HorizontalPadding)

	// Header style (no bottom border to avoid double lines)
	styles["header"] = lipgloss.NewStyle().
		Width(dims.Header.Width).
		Height(dims.Header.Height).
		BorderStyle(borderStyle).
		BorderForeground(lipgloss.Color(layoutConfig.Borders.Color)).
		BorderBottom(false).
		Padding(0, dims.HorizontalPadding).
		Foreground(lipgloss.Color(themeConfig.Colors["primary"])).
		Bold(true)

	// Footer style (no top border)
	styles["footer"] = lipgloss.NewStyle().
		Width(dims.StatusFooter.Width).
		Height(dims.StatusFooter.Height).
		BorderStyle(borderStyle).
		BorderForeground(lipgloss.Color(layoutConfig.Borders.Color)).
		BorderTop(false).
		Padding(0, dims.HorizontalPadding).
		Foreground(lipgloss.Color(themeConfig.Colors["muted"]))

	switch mode {
	case LargeLayout:
		// Navigation panel (left)
		styles["nav"] = baseStyle.Copy().
			Width(dims.LeftPanel.Width).
			Height(dims.LeftPanel.Height)

		// Main content panel (center)
		styles["main"] = baseStyle.Copy().
			Width(dims.MainContent.Width).
			Height(dims.MainContent.Height)

		// Status panel (right)
		styles["status"] = baseStyle.Copy().
			Width(dims.RightPanel.Width).
			Height(dims.RightPanel.Height)

	case MediumLayout:
		// Navigation panel (left)
		styles["nav"] = baseStyle.Copy().
			Width(dims.LeftPanel.Width).
			Height(dims.LeftPanel.Height)

		// Main content panel (right, wider for medium layout)
		styles["main_wide"] = baseStyle.Copy().
			Width(dims.MainContent.Width).
			Height(dims.MainContent.Height)

		// Use main_wide as main for consistency
		styles["main"] = styles["main_wide"]

	case SmallLayout:
		// Single panel using full space
		styles["single"] = baseStyle.Copy().
			Width(dims.MainContent.Width).
			Height(dims.MainContent.Height)

		// Use single as main for consistency
		styles["main"] = styles["single"]
	}

	return styles
}

// EnsureNoOverlap validates that rendered content doesn't exceed terminal bounds
func (l *Layout) EnsureNoOverlap(content string) string {
	// Get actual rendered dimensions
	contentWidth := lipgloss.Width(content)
	contentHeight := lipgloss.Height(content)

	// Check if content exceeds terminal bounds
	if contentWidth > l.dimensions.Terminal.Width {
		// Truncate content to fit
		lines := lipgloss.Height(content)
		maxWidth := l.dimensions.Terminal.Width
		
		// Split into lines and truncate each
		truncated := ""
		for i := 0; i < lines; i++ {
			line := getLine(content, i)
			if len(line) > maxWidth {
				line = line[:maxWidth-3] + "..."
			}
			if i > 0 {
				truncated += "\n"
			}
			truncated += line
		}
		content = truncated
	}

	if contentHeight > l.dimensions.Terminal.Height {
		// Truncate to fit height
		maxHeight := l.dimensions.Terminal.Height
		lines := splitLines(content)
		if len(lines) > maxHeight {
			lines = lines[:maxHeight-1]
			lines = append(lines, "... (content truncated)")
		}
		content = joinLines(lines)
	}

	return content
}

// Helper functions for string manipulation

// getLine extracts a specific line from multi-line content
func getLine(content string, lineNum int) string {
	lines := splitLines(content)
	if lineNum >= 0 && lineNum < len(lines) {
		return lines[lineNum]
	}
	return ""
}

// splitLines splits content into individual lines
func splitLines(content string) []string {
	if content == "" {
		return []string{}
	}
	
	lines := []string{}
	current := ""
	
	for _, char := range content {
		if char == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(char)
		}
	}
	
	// Add the last line if it doesn't end with newline
	if current != "" {
		lines = append(lines, current)
	}
	
	return lines
}

// joinLines combines lines back into a single string
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	
	result := lines[0]
	for i := 1; i < len(lines); i++ {
		result += "\n" + lines[i]
	}
	
	return result
}

// CalculateComponentSizes returns optimal sizes for specific components
func (l *Layout) CalculateComponentSizes() map[string]Dimensions {
	sizes := make(map[string]Dimensions)

	switch l.mode {
	case LargeLayout:
		// Component sizes for large layout
		sizes["workflow_list"] = Dimensions{
			Width:  l.dimensions.LeftPanel.Width - 2, // Account for padding
			Height: l.dimensions.LeftPanel.Height / 2,
		}
		sizes["workflow_table"] = Dimensions{
			Width:  l.dimensions.MainContent.Width - 2,
			Height: l.dimensions.MainContent.Height * 2 / 3,
		}
		sizes["output_viewport"] = Dimensions{
			Width:  l.dimensions.MainContent.Width - 2,
			Height: l.dimensions.MainContent.Height / 3,
		}
		sizes["status_panel"] = Dimensions{
			Width:  l.dimensions.RightPanel.Width - 2,
			Height: l.dimensions.RightPanel.Height,
		}

	case MediumLayout:
		// Component sizes for medium layout
		sizes["workflow_list"] = Dimensions{
			Width:  l.dimensions.LeftPanel.Width - 2,
			Height: l.dimensions.LeftPanel.Height,
		}
		sizes["workflow_table"] = Dimensions{
			Width:  l.dimensions.MainContent.Width - 2,
			Height: l.dimensions.MainContent.Height * 2 / 3,
		}
		sizes["output_viewport"] = Dimensions{
			Width:  l.dimensions.MainContent.Width - 2,
			Height: l.dimensions.MainContent.Height / 3,
		}

	case SmallLayout:
		// Component sizes for small layout (full width)
		componentHeight := l.dimensions.MainContent.Height / 3
		sizes["workflow_list"] = Dimensions{
			Width:  l.dimensions.MainContent.Width - 2,
			Height: componentHeight,
		}
		sizes["workflow_table"] = Dimensions{
			Width:  l.dimensions.MainContent.Width - 2,
			Height: componentHeight,
		}
		sizes["output_viewport"] = Dimensions{
			Width:  l.dimensions.MainContent.Width - 2,
			Height: componentHeight,
		}
	}

	return sizes
}

// IsLayoutTransition checks if a size change requires layout mode transition
func IsLayoutTransition(oldWidth, oldHeight, newWidth, newHeight int) bool {
	oldMode := DetermineLayout(oldWidth, oldHeight)
	newMode := DetermineLayout(newWidth, newHeight)
	return oldMode != newMode
}