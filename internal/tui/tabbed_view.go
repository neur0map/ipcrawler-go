package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Tab styles
	activeTabBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      " ",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "┘",
		BottomRight: "└",
	}

	tabBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "┴",
		BottomRight: "┴",
	}

	activeTabStyle = lipgloss.NewStyle().
			Border(activeTabBorder, true).
			BorderForeground(lipgloss.Color("#3b82f6")).
			Background(lipgloss.Color("#1e3a8a")).
			Foreground(lipgloss.Color("#ffffff")).
			Bold(true).
			Padding(0, 1)

	inactiveTabStyle = lipgloss.NewStyle().
				Border(tabBorder, true).
				BorderForeground(lipgloss.Color("#64748b")).
				Background(lipgloss.Color("#f8fafc")).
				Foreground(lipgloss.Color("#475569")).
				Padding(0, 1)

	// Widget styles
	widgetTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#1e40af")).
				Background(lipgloss.Color("#f8fafc")).
				Padding(0, 1).
				MarginBottom(1)

	widgetBorderStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#3b82f6")).
				Padding(1)

	systemMetricStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#059669"))

	tabHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1e3a8a")).
			Background(lipgloss.Color("#f8fafc")).
			Padding(0, 2).
			Width(100)

	contentStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#3b82f6")).
			BorderTop(false).
			Padding(1)
)

func (m TabbedTUIModel) View() string {
	// Use new proper layout system with flexbox
	return m.ViewWithProperLayout()
}

func (m TabbedTUIModel) renderHeader() string {
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
	return tabHeaderStyle.Width(m.width).Render(headerText)
}

func (m TabbedTUIModel) renderTabBar() string {
	var tabs []string
	
	for i, tabName := range tabNames {
		var style lipgloss.Style
		if TabType(i) == m.activeTab {
			style = activeTabStyle
		} else {
			style = inactiveTabStyle
		}
		
		// Add tab number for quick navigation
		tabText := fmt.Sprintf("%d %s", i+1, tabName)
		tabs = append(tabs, style.Render(tabText))
	}
	
	row := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)
	
	// Add bottom border to fill remaining space
	gap := m.width - lipgloss.Width(row)
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

func (m TabbedTUIModel) renderActiveTabContent() string {
	var content string
	
	switch m.activeTab {
	case OverviewTab:
		content = m.renderOverviewTab()
	case ToolsTab:
		content = m.renderToolsTab()
	case WorkflowsTab:
		content = m.renderWorkflowsTab()
	case SystemTab:
		content = m.renderSystemTab()
	case LogsTab:
		content = m.renderLogsTab()
	}
	
	if m.width < 50 || m.height < 20 {
		content = "Terminal too small. Please resize to at least 50x20."
	}
	
	return contentStyle.Width(m.width - 4).Height(m.height - 8).Render(content)
}

func (m TabbedTUIModel) renderOverviewTab() string {
	if m.width < 60 {
		return "Terminal too narrow for overview layout."
	}
	
	// Top row: Active Tools (left) | System Resources (right)
	topLeft := m.renderActiveToolsWidget()
	topRight := m.renderSystemResourcesWidget()
	
	topRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		topLeft,
		topRight,
	)
	
	// Middle row: Recent Completions (full width)
	middleRow := m.renderRecentCompletionsWidget()
	
	// Bottom row: Workflow Status (left) | Live Output (right)
	bottomLeft := m.renderWorkflowStatusWidget()
	bottomRight := m.renderLiveOutputWidget()
	
	bottomRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		bottomLeft,
		bottomRight,
	)
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		topRow,
		"",
		middleRow,
		"",
		bottomRow,
	)
}

func (m TabbedTUIModel) renderActiveToolsWidget() string {
	title := widgetTitleStyle.Render("🚀 Active Tools")
	
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
	
	return widgetBorderStyle.
		Width(m.width/2 - 4).
		Height(m.height/3 - 2).
		Render(widget)
}

func (m TabbedTUIModel) renderSystemResourcesWidget() string {
	title := widgetTitleStyle.Render("💻 System Resources")
	
	// Format memory in MB
	ramMB := float64(m.systemMetrics.RAMUsed) / 1024 / 1024
	
	ramText := systemMetricStyle.Render(fmt.Sprintf("💾 RAM: %.1fMB (%.1f%%)", 
		ramMB, m.systemMetrics.RAMPercent))
	
	cpuText := systemMetricStyle.Render(fmt.Sprintf("🔧 CPU: %.1f%%", 
		m.systemMetrics.CPUPercent))
	
	pidText := systemMetricStyle.Render(fmt.Sprintf("🆔 PID: %d", 
		m.systemMetrics.ProcessPID))
	
	updateText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#64748b")).
		Render(fmt.Sprintf("Updated: %s", 
			m.systemMetrics.UpdateTime.Format("15:04:05")))
	
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		ramText,
		cpuText,
		pidText,
		"",
		updateText,
	)
	
	widget := lipgloss.JoinVertical(lipgloss.Left, title, content)
	
	return widgetBorderStyle.
		Width(m.width/2 - 4).
		Height(m.height/3 - 2).
		Render(widget)
}

func (m TabbedTUIModel) renderRecentCompletionsWidget() string {
	title := widgetTitleStyle.Render("✅ Recent Completions")
	
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
	
	return widgetBorderStyle.
		Width(m.width - 6).
		Height(m.height/4).
		Render(widget)
}

func (m TabbedTUIModel) renderWorkflowStatusWidget() string {
	title := widgetTitleStyle.Render("📋 Workflow Status")
	
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
	
	return widgetBorderStyle.
		Width(m.width/2 - 4).
		Height(m.height/3 - 2).
		Render(widget)
}

func (m TabbedTUIModel) renderLiveOutputWidget() string {
	title := widgetTitleStyle.Render("📡 Live Output")
	
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
	
	return widgetBorderStyle.
		Width(m.width/2 - 4).
		Height(m.height/3 - 2).
		Render(widget)
}

func (m TabbedTUIModel) renderToolsTab() string {
	title := widgetTitleStyle.Render("🔧 Tool Execution History")
	
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
	return widget
}

func (m TabbedTUIModel) renderWorkflowsTab() string {
	title := widgetTitleStyle.Render("📋 Workflow Management")
	
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
	return widget
}

func (m TabbedTUIModel) renderSystemTab() string {
	title := widgetTitleStyle.Render("💻 System Diagnostics")
	
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
	return widget
}

func (m TabbedTUIModel) renderLogsTab() string {
	title := widgetTitleStyle.Render("📝 Detailed Logs")
	
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
	return widget
}

func (m TabbedTUIModel) renderHelp() string {
	helpContent := m.help.View(m.keyMap)
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#64748b")).
		Padding(1).
		Render(helpContent)
}