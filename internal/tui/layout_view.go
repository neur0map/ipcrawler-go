package tui

import (
	"fmt"
	"strings"

	"github.com/76creates/stickers/flexbox"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Layout styles
	containerStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#3b82f6")).
			Padding(0, 1).
			Margin(0, 1)

	headerStyleLayout = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1e3a8a")).
			Background(lipgloss.Color("#f8fafc")).
			Padding(0, 1).
			MarginBottom(1)

	activeTabBorderStyle = lipgloss.Border{
		Top:         "─",
		Bottom:      " ",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "┘",
		BottomRight: "└",
	}

	inactiveTabBorderStyle = lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "┴",
		BottomRight: "┴",
	}

	activeTabStyleNew = lipgloss.NewStyle().
				Border(activeTabBorderStyle, true).
				BorderForeground(lipgloss.Color("#3b82f6")).
				Background(lipgloss.Color("#1e3a8a")).
				Foreground(lipgloss.Color("#ffffff")).
				Bold(true).
				Padding(0, 1)

	inactiveTabStyleNew = lipgloss.NewStyle().
				Border(inactiveTabBorderStyle, true).
				BorderForeground(lipgloss.Color("#64748b")).
				Background(lipgloss.Color("#f8fafc")).
				Foreground(lipgloss.Color("#475569")).
				Padding(0, 1)
)

func (m TabbedTUIModel) ViewWithProperLayout() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	// Minimum size check
	if m.width < 80 || m.height < 24 {
		return fmt.Sprintf("Terminal too small: %dx%d\nMinimum required: 80x24", m.width, m.height)
	}

	// Create main container using flexbox
	mainFlex := flexbox.New(0, 0)
	mainFlex.SetWidth(m.width)
	mainFlex.SetHeight(m.height)

	// Header section
	header := m.renderHeaderWithLayout()
	headerRow := mainFlex.NewRow()
	headerCell := flexbox.NewCell(1, 1)
	headerCell.SetContent(header)
	headerRow.AddCells(headerCell)

	// Tab bar section
	tabBar := m.renderTabBarWithLayout()
	tabBarRow := mainFlex.NewRow()
	tabBarCell := flexbox.NewCell(1, 1)
	tabBarCell.SetContent(tabBar)
	tabBarRow.AddCells(tabBarCell)

	// Content section (main area)
	content := m.renderContentWithLayout()
	contentRow := mainFlex.NewRow()
	contentCell := flexbox.NewCell(1, 8) // Give most space to content
	contentCell.SetContent(content)
	contentRow.AddCells(contentCell)

	// Help section (if shown)
	var helpRow *flexbox.Row
	if m.showHelp {
		help := m.renderHelpWithLayout()
		helpRow = mainFlex.NewRow()
		helpCell := flexbox.NewCell(1, 2)
		helpCell.SetContent(help)
		helpRow.AddCells(helpCell)
	}

	// Add rows to main flex container
	rows := []*flexbox.Row{headerRow, tabBarRow, contentRow}
	if helpRow != nil {
		rows = append(rows, helpRow)
	}
	mainFlex.AddRows(rows)

	return mainFlex.Render()
}

func (m TabbedTUIModel) renderHeaderWithLayout() string {
	title := fmt.Sprintf("🎯 IPCrawler TUI - Target: %s", m.target)
	
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
		
		status = fmt.Sprintf(" | Running: %d | Completed: %d | Failed: %d", running, completed, failed)
	}
	
	headerText := title + status
	return headerStyleLayout.Width(m.width - 2).Render(headerText)
}

func (m TabbedTUIModel) renderTabBarWithLayout() string {
	var tabs []string
	
	for i, tabName := range tabNames {
		var style lipgloss.Style
		if TabType(i) == m.activeTab {
			style = activeTabStyleNew
		} else {
			style = inactiveTabStyleNew
		}
		
		tabText := fmt.Sprintf("%d %s", i+1, tabName)
		tabs = append(tabs, style.Render(tabText))
	}
	
	row := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)
	
	// Fill remaining space with border
	gap := m.width - lipgloss.Width(row) - 2
	if gap > 0 {
		bottomBorder := lipgloss.NewStyle().
			BorderBottom(true).
			BorderBottomForeground(lipgloss.Color("#64748b")).
			Width(gap).
			Render("")
		row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, bottomBorder)
	}
	
	return row
}

func (m TabbedTUIModel) renderContentWithLayout() string {
	switch m.activeTab {
	case OverviewTab:
		return m.renderOverviewWithFlexbox()
	case ToolsTab:
		return m.renderToolsWithLayout()
	case WorkflowsTab:
		return m.renderWorkflowsWithLayout()
	case SystemTab:
		return m.renderSystemWithLayout()
	case LogsTab:
		return m.renderLogsWithLayout()
	default:
		return "Unknown tab"
	}
}

func (m TabbedTUIModel) renderOverviewWithFlexbox() string {
	// Calculate available space for content
	contentWidth := m.width - 4  // Account for borders/padding
	contentHeight := m.height - 8 // Account for header, tabs, help space
	
	if contentWidth < 40 || contentHeight < 12 {
		return "Content area too small for overview layout"
	}

	// Create single main flexbox for overview
	overviewFlex := flexbox.New(0, 0)
	overviewFlex.SetWidth(contentWidth)
	overviewFlex.SetHeight(contentHeight)

	// Row 1: Active Tools | System Resources (side by side)
	topRow := overviewFlex.NewRow()
	
	activeToolsContent := m.renderActiveToolsContent()
	activeToolsCell := flexbox.NewCell(1, 1)
	activeToolsCell.SetContent(activeToolsContent)
	
	systemResourcesContent := m.renderSystemResourcesContent()
	systemResourcesCell := flexbox.NewCell(1, 1)
	systemResourcesCell.SetContent(systemResourcesContent)

	topRow.AddCells(activeToolsCell, systemResourcesCell)

	// Row 2: Recent Completions (full width)
	middleRow := overviewFlex.NewRow()
	
	recentCompletionsContent := m.renderRecentCompletionsContent()
	recentCompletionsCell := flexbox.NewCell(1, 1)
	recentCompletionsCell.SetContent(recentCompletionsContent)
	
	middleRow.AddCells(recentCompletionsCell)

	// Row 3: Workflow Status | Live Output (side by side)
	bottomRow := overviewFlex.NewRow()

	workflowStatusContent := m.renderWorkflowStatusContent()
	workflowStatusCell := flexbox.NewCell(1, 1)
	workflowStatusCell.SetContent(workflowStatusContent)

	liveOutputContent := m.renderLiveOutputContent()
	liveOutputCell := flexbox.NewCell(1, 1)
	liveOutputCell.SetContent(liveOutputContent)

	bottomRow.AddCells(workflowStatusCell, liveOutputCell)

	// Add all rows to main flexbox
	overviewFlex.AddRows([]*flexbox.Row{topRow, middleRow, bottomRow})

	return overviewFlex.Render()
}

func (m TabbedTUIModel) renderActiveToolsContent() string {
	title := headerStyleLayout.Render("🚀 Active Tools")
	
	content := ""
	if len(m.activeTools) == 0 {
		content = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#64748b")).
			Italic(true).
			Render("No tools currently running")
	} else {
		content = m.activeToolsList.View()
	}
	
	widget := lipgloss.JoinVertical(lipgloss.Left, title, content)
	return containerStyle.Render(widget)
}

func (m TabbedTUIModel) renderSystemResourcesContent() string {
	title := headerStyleLayout.Render("💻 System Resources")
	
	ramMB := float64(m.systemMetrics.RAMUsed) / 1024 / 1024
	
	ramText := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#059669")).
		Render(fmt.Sprintf("💾 RAM: %.1fMB (%.1f%%)", ramMB, m.systemMetrics.RAMPercent))
	
	cpuText := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#059669")).
		Render(fmt.Sprintf("🔧 CPU: %.1f%%", m.systemMetrics.CPUPercent))
	
	pidText := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#059669")).
		Render(fmt.Sprintf("🆔 PID: %d", m.systemMetrics.ProcessPID))
	
	updateText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#64748b")).
		Render(fmt.Sprintf("Updated: %s", m.systemMetrics.UpdateTime.Format("15:04:05")))
	
	content := lipgloss.JoinVertical(lipgloss.Left, ramText, cpuText, pidText, "", updateText)
	widget := lipgloss.JoinVertical(lipgloss.Left, title, content)
	
	return containerStyle.Render(widget)
}

func (m TabbedTUIModel) renderRecentCompletionsContent() string {
	title := headerStyleLayout.Render("✅ Recent Completions")
	
	content := ""
	if len(m.recentCompList) == 0 {
		content = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#64748b")).
			Italic(true).
			Render("No completed tools yet")
	} else {
		content = m.recentCompletions.View()
	}
	
	widget := lipgloss.JoinVertical(lipgloss.Left, title, content)
	return containerStyle.Render(widget)
}

func (m TabbedTUIModel) renderWorkflowStatusContent() string {
	title := headerStyleLayout.Render("📋 Workflow Status")
	
	content := ""
	if len(m.workflows) == 0 {
		content = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#64748b")).
			Italic(true).
			Render("No workflows started yet")
	} else {
		content = m.workflowStatusList.View()
	}
	
	widget := lipgloss.JoinVertical(lipgloss.Left, title, content)
	return containerStyle.Render(widget)
}

func (m TabbedTUIModel) renderLiveOutputContent() string {
	title := headerStyleLayout.Render("📡 Live Output")
	
	content := ""
	if len(m.logs) == 0 {
		content = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#64748b")).
			Italic(true).
			Render("Waiting for tool output...")
	} else {
		content = m.liveOutputViewport.View()
	}
	
	widget := lipgloss.JoinVertical(lipgloss.Left, title, content)
	return containerStyle.Render(widget)
}

func (m TabbedTUIModel) renderToolsWithLayout() string {
	title := headerStyleLayout.Render("🔧 Tool Execution History")
	
	content := "Tool execution details will be shown here."
	if len(m.tools) > 0 {
		var toolLines []string
		start := 0
		if len(m.tools) > 20 {
			start = len(m.tools) - 20
		}
		
		for _, tool := range m.tools[start:] {
			status := "✅"
			if tool.Error != nil {
				status = "❌"
			}
			
			line := fmt.Sprintf("%s %s (%s) - %s - %s",
				status,
				tool.Name,
				tool.Workflow,
				formatDuration(tool.Duration),
				truncateString(strings.Join(tool.Args, " "), 40))
			
			toolLines = append(toolLines, line)
		}
		content = strings.Join(toolLines, "\n")
	}
	
	widget := lipgloss.JoinVertical(lipgloss.Left, title, content)
	return containerStyle.Width(m.width - 6).Render(widget)
}

func (m TabbedTUIModel) renderWorkflowsWithLayout() string {
	title := headerStyleLayout.Render("📋 Workflow Management")
	
	content := "Workflow details will be shown here."
	if len(m.workflows) > 0 {
		var workflowLines []string
		
		for _, wf := range m.workflows {
			status := getStatusIcon(wf.Status)
			duration := formatDuration(wf.Duration)
			if wf.Status == "running" {
				duration = formatDuration(m.systemMetrics.UpdateTime.Sub(wf.StartTime))
			}
			
			line := fmt.Sprintf("%s %s - %s - %s",
				status,
				wf.ID,
				wf.Status,
				duration)
			
			workflowLines = append(workflowLines, line)
		}
		content = strings.Join(workflowLines, "\n")
	}
	
	widget := lipgloss.JoinVertical(lipgloss.Left, title, content)
	return containerStyle.Width(m.width - 6).Render(widget)
}

func (m TabbedTUIModel) renderSystemWithLayout() string {
	title := headerStyleLayout.Render("💻 System Diagnostics")
	
	ramMB := float64(m.systemMetrics.RAMUsed) / 1024 / 1024
	
	content := fmt.Sprintf(`Process Information:
🆔 PID: %d
💾 Memory Usage: %.1f MB (%.2f%%)
🔧 CPU Usage: %.2f%%
📊 Active Tools: %d
📋 Workflows: %d
📝 Log Entries: %d

Last Updated: %s`,
		m.systemMetrics.ProcessPID,
		ramMB,
		m.systemMetrics.RAMPercent,
		m.systemMetrics.CPUPercent,
		len(m.activeTools),
		len(m.workflows),
		len(m.logs),
		m.systemMetrics.UpdateTime.Format("15:04:05"))
	
	widget := lipgloss.JoinVertical(lipgloss.Left, title, content)
	return containerStyle.Width(m.width - 6).Render(widget)
}

func (m TabbedTUIModel) renderLogsWithLayout() string {
	title := headerStyleLayout.Render("📝 Detailed Logs")
	
	content := "No logs available yet."
	if len(m.logs) > 0 {
		var logLines []string
		start := 0
		if len(m.logs) > 50 {
			start = len(m.logs) - 50
		}
		
		for _, log := range m.logs[start:] {
			levelIcon := getLevelIcon(log.Level)
			timestamp := log.Timestamp.Format("15:04:05")
			
			line := fmt.Sprintf("[%s] %s %s: %s",
				timestamp,
				levelIcon,
				log.Category,
				log.Message)
			
			logLines = append(logLines, line)
		}
		content = strings.Join(logLines, "\n")
	}
	
	widget := lipgloss.JoinVertical(lipgloss.Left, title, content)
	return containerStyle.Width(m.width - 6).Render(widget)
}

func (m TabbedTUIModel) renderHelpWithLayout() string {
	helpContent := m.help.View(m.keyMap)
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#64748b")).
		Padding(1).
		Width(m.width - 6).
		Render(helpContent)
}