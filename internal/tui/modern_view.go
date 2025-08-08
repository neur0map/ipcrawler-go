package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1e3a8a")).
			Background(lipgloss.Color("#f8fafc")).
			Padding(0, 2).
			MarginBottom(1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1e40af")).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#3b82f6")).
			Padding(0, 1).
			MarginBottom(1)

	focusedStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#3b82f6")).
			BorderTop(true).
			BorderBottom(true).
			BorderLeft(true).
			BorderRight(true)

	blurredStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#64748b")).
			BorderTop(true).
			BorderBottom(true).
			BorderLeft(true).
			BorderRight(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#059669")).
			Bold(true)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3b82f6"))
)

func (m ModernTUIModel) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	// Header
	header := m.renderHeader()

	// Main content area
	content := m.renderMainContent()

	// Help
	help := ""
	if m.showHelp {
		help = m.renderHelp()
	}

	// Status bar
	statusBar := m.renderStatusBar()

	sections := []string{header}
	if content != "" {
		sections = append(sections, content)
	}
	if help != "" {
		sections = append(sections, help)
	}
	sections = append(sections, statusBar)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m ModernTUIModel) renderHeader() string {
	title := titleStyle.Render(fmt.Sprintf("IPCrawler TUI - Target: %s", m.target))
	
	status := ""
	if len(m.workflows) > 0 {
		running := 0
		completed := 0
		failed := 0
		
		for _, wf := range m.workflows {
			switch wf.Status {
			case "running":
				running++
			case "completed":
				completed++
			case "failed":
				failed++
			}
		}
		
		if running > 0 {
			status = fmt.Sprintf("%s Running: %d | Completed: %d | Failed: %d", 
				m.spinner.View(), running, completed, failed)
		} else {
			status = fmt.Sprintf("Completed: %d | Failed: %d", completed, failed)
		}
	} else {
		status = "No workflows running"
	}
	
	statusLine := statusStyle.Render(status)
	
	return lipgloss.JoinVertical(lipgloss.Left, title, statusLine)
}

func (m ModernTUIModel) renderMainContent() string {
	if m.width < 50 || m.height < 20 {
		return "Terminal too small. Please resize to at least 50x20."
	}

	// Left side - Tables
	leftSide := m.renderTables()
	
	// Right side - List and Output
	rightSide := m.renderSidebar()
	
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftSide,
		rightSide,
	)
}

func (m ModernTUIModel) renderTables() string {
	// Workflow table
	workflowHeader := headerStyle.Render("Workflows")
	workflowTableStyle := blurredStyle
	if m.focused == FocusWorkflowTable {
		workflowTableStyle = focusedStyle
	}
	workflowContent := workflowTableStyle.Render(m.workflowTable.View())
	workflowSection := lipgloss.JoinVertical(lipgloss.Left, workflowHeader, workflowContent)

	// Tool table
	toolHeader := headerStyle.Render("Tool Executions")
	toolTableStyle := blurredStyle
	if m.focused == FocusToolTable {
		toolTableStyle = focusedStyle
	}
	toolContent := toolTableStyle.Render(m.toolTable.View())
	toolSection := lipgloss.JoinVertical(lipgloss.Left, toolHeader, toolContent)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		workflowSection,
		"",
		toolSection,
	)
}

func (m ModernTUIModel) renderSidebar() string {
	// Active workflows list
	listHeader := headerStyle.Render("Active Workflows")
	listStyle := blurredStyle
	if m.focused == FocusWorkflowList {
		listStyle = focusedStyle
	}
	listContent := listStyle.Render(m.workflowList.View())
	listSection := lipgloss.JoinVertical(lipgloss.Left, listHeader, listContent)

	// Output viewport
	outputHeader := headerStyle.Render("Output & Logs")
	outputStyle := blurredStyle
	if m.focused == FocusOutput {
		outputStyle = focusedStyle
	}
	outputContent := outputStyle.Render(m.outputViewport.View())
	outputSection := lipgloss.JoinVertical(lipgloss.Left, outputHeader, outputContent)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		listSection,
		"",
		outputSection,
	)
}

func (m ModernTUIModel) renderHelp() string {
	helpContent := m.help.View(m.keyMap)
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#64748b")).
		Padding(1).
		Render(helpContent)
}

func (m ModernTUIModel) renderStatusBar() string {
	leftStatus := ""
	if len(m.tools) > 0 {
		lastTool := m.tools[len(m.tools)-1]
		leftStatus = fmt.Sprintf("Last: %s (%s)", lastTool.Name, lastTool.Status)
	}

	rightStatus := fmt.Sprintf("Focus: %s | Press ? for help", m.getFocusName())

	statusWidth := m.width - 2
	leftWidth := len(leftStatus)
	rightWidth := len(rightStatus)
	padding := statusWidth - leftWidth - rightWidth

	if padding < 0 {
		padding = 0
		if leftWidth+rightWidth > statusWidth {
			leftStatus = truncateString(leftStatus, statusWidth/2)
			rightStatus = truncateString(rightStatus, statusWidth/2)
			padding = 0
		}
	}

	statusLine := leftStatus + strings.Repeat(" ", padding) + rightStatus

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#64748b")).
		Background(lipgloss.Color("#f1f5f9")).
		Width(m.width).
		Render(statusLine)
}

func (m ModernTUIModel) getFocusName() string {
	switch m.focused {
	case FocusWorkflowTable:
		return "Workflows"
	case FocusToolTable:
		return "Tools"
	case FocusWorkflowList:
		return "Active"
	case FocusOutput:
		return "Output"
	default:
		return "Unknown"
	}
}

// Implement help.KeyMap interface for the help component
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Tab, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Tab, k.Cancel, k.Quit, k.Help},
	}
}

