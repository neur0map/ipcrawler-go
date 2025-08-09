package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/your-org/ipcrawler/internal/ui/styles"
)

// LayoutManager handles responsive layout rendering
type LayoutManager struct {
	config *Config
	theme  *styles.Theme
}

// NewLayoutManager creates a new layout manager
func NewLayoutManager(config *Config, theme *styles.Theme) *LayoutManager {
	return &LayoutManager{
		config: config,
		theme:  theme,
	}
}

// RenderLayout renders the appropriate layout based on current mode
func (lm *LayoutManager) RenderLayout(app *App) string {
	switch app.GetLayout() {
	case LayoutLarge:
		return lm.renderThreeColumn(app)
	case LayoutMedium:
		return lm.renderTwoColumn(app)
	case LayoutSmall:
		return lm.renderStacked(app)
	default:
		return "Invalid layout mode"
	}
}

// renderThreeColumn renders three-column layout (≥120 cols)
// Layout: [Left Panel] [Main Content] [Right Status]
func (lm *LayoutManager) renderThreeColumn(app *App) string {
	leftW, mainW, rightW, contentH := app.calculatePanelSizes()

	// Render left panel (workflows/tools list)
	leftPanel := lm.theme.PanelStyle.
		Width(leftW).
		Height(contentH).
		Render(lm.renderPanelWithFocus(
			app.GetListPanel().View(),
			app.GetFocused() == FocusListPanel,
		))

	// Render main panel (viewport with logs)
	mainPanel := lm.theme.PanelStyle.
		Width(mainW).
		Height(contentH).
		Render(lm.renderPanelWithFocus(
			app.GetViewport().View(),
			app.GetFocused() == FocusViewportPanel,
		))

	// Render right panel (status and progress)
	rightPanel := lm.theme.PanelStyle.
		Width(rightW).
		Height(contentH).
		Render(lm.renderPanelWithFocus(
			app.GetStatusPanel().View(),
			app.GetFocused() == FocusStatusPanel,
		))

	// Join horizontally with consistent alignment
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftPanel,
		mainPanel,
		rightPanel,
	)
}

// renderTwoColumn renders two-column layout with footer (80-119 cols)
// Layout: [Left Panel] [Main Content]
//         [Footer Status            ]
func (lm *LayoutManager) renderTwoColumn(app *App) string {
	leftW, mainW, _, contentH := app.calculatePanelSizes()

	// Render left panel (workflows/tools list)
	leftPanel := lm.theme.PanelStyle.
		Width(leftW).
		Height(contentH).
		Render(lm.renderPanelWithFocus(
			app.GetListPanel().View(),
			app.GetFocused() == FocusListPanel,
		))

	// Render main panel (viewport with logs)
	mainPanel := lm.theme.PanelStyle.
		Width(mainW).
		Height(contentH).
		Render(lm.renderPanelWithFocus(
			app.GetViewport().View(),
			app.GetFocused() == FocusViewportPanel,
		))

	// Join top row horizontally
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, mainPanel)

	// Render footer status
	footer := lm.theme.FooterStyle.
		Width(app.GetWidth() - 2).
		Height(lm.config.UI.Layout.Panels.Status.HeightFooter).
		Render(lm.renderPanelWithFocus(
			app.GetStatusPanel().View(),
			app.GetFocused() == FocusStatusPanel,
		))

	// Join vertically
	return lipgloss.JoinVertical(lipgloss.Left, topRow, footer)
}

// renderStacked renders stacked layout for small screens (<80 cols)
// Layout: [Header - List Panel]
//         [Main - Viewport    ]
//         [Footer - Status    ]
func (lm *LayoutManager) renderStacked(app *App) string {
	panelW := app.GetWidth() - 4
	headerH := 8
	footerH := 4
	mainH := app.GetHeight() - headerH - footerH - 6 // Account for borders

	// Render header (condensed list view)
	header := lm.theme.PanelStyle.
		Width(panelW).
		Height(headerH).
		Render(lm.renderPanelWithFocus(
			app.GetListPanel().View(),
			app.GetFocused() == FocusListPanel,
		))

	// Render main content (viewport takes most space)
	main := lm.theme.PanelStyle.
		Width(panelW).
		Height(mainH).
		Render(lm.renderPanelWithFocus(
			app.GetViewport().View(),
			app.GetFocused() == FocusViewportPanel,
		))

	// Render footer (compact status)
	footer := lm.theme.FooterStyle.
		Width(panelW).
		Height(footerH).
		Render(lm.renderPanelWithFocus(
			app.GetStatusPanel().View(),
			app.GetFocused() == FocusStatusPanel,
		))

	// Join vertically with consistent spacing
	return lipgloss.JoinVertical(lipgloss.Left, header, main, footer)
}

// renderPanelWithFocus applies focus styling to panels
func (lm *LayoutManager) renderPanelWithFocus(content string, focused bool) string {
	if focused {
		return lm.theme.FocusedPanelStyle.Render(content)
	}
	return content
}

// calculateStableHeight ensures consistent height for flicker-free updates
func (lm *LayoutManager) calculateStableHeight(baseHeight int) int {
	// Always return the same height to prevent flicker
	// Content should scroll or truncate to fit
	return baseHeight
}

// ensureMinimumPanelSize prevents panels from becoming too small
func (lm *LayoutManager) ensureMinimumPanelSize(width, height int) (int, int) {
	const minWidth = 20
	const minHeight = 5

	if width < minWidth {
		width = minWidth
	}
	if height < minHeight {
		height = minHeight
	}

	return width, height
}