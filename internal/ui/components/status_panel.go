package components

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/your-org/ipcrawler/internal/simulator"
	"github.com/your-org/ipcrawler/internal/ui/styles"
)

// StatusPanel manages the right panel showing system status and progress
type StatusPanel struct {
	config *ui.Config
	theme  *styles.Theme

	// Bubbles components
	spinner  spinner.Model
	progress progress.Model

	// State
	width  int
	height int
	ready  bool

	// Status data
	systemStatus   simulator.SystemStatus
	metrics       simulator.Metrics
	activeTask    string
	taskProgress  float64
	lastUpdate    time.Time
	
	// Update ticker
	ticker *time.Ticker
}

// NewStatusPanel creates a new status panel component
func NewStatusPanel(config *ui.Config, theme *styles.Theme) *StatusPanel {
	// Initialize spinner with configured type
	s := spinner.New()
	spinnerType := config.UI.Components.Status.Spinner
	switch spinnerType {
	case "dot":
		s.Spinner = spinner.Dot
	case "line":
		s.Spinner = spinner.Line
	case "meter":
		s.Spinner = spinner.Meter
	case "hamburger":
		s.Spinner = spinner.Hamburger
	default:
		s.Spinner = spinner.Dot
	}
	
	// Style spinner with theme colors
	s.Style = lipgloss.NewStyle().Foreground(theme.Colors.Accent)

	// Initialize progress bar
	p := progress.New(progress.WithDefaultGradient())
	p.Width = 30 // Will be adjusted on resize

	return &StatusPanel{
		config:      config,
		theme:       theme,
		spinner:     s,
		progress:    p,
		lastUpdate:  time.Now(),
		activeTask:  "Initializing",
		taskProgress: 0.0,
	}
}

// Init initializes the status panel component
func (sp *StatusPanel) Init() tea.Cmd {
	// Start spinner and create update ticker
	updateInterval := sp.config.UI.Components.Status.UpdateInterval
	sp.ticker = time.NewTicker(updateInterval)
	
	return tea.Batch(
		sp.spinner.Tick,
		sp.tick(),
	)
}

// Update handles messages and updates the status panel
func (sp *StatusPanel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle status panel specific keys if needed
		switch msg.String() {
		case "r":
			// Refresh status
			sp.lastUpdate = time.Now()
			return sp, nil
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		sp.spinner, cmd = sp.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case simulator.StatusUpdateMsg:
		// Update system status
		sp.systemStatus = msg.Status
		sp.metrics = msg.Metrics
		sp.lastUpdate = time.Now()

	case simulator.WorkflowExecutionMsg:
		// Update active task info
		sp.activeTask = fmt.Sprintf("Workflow: %s", msg.WorkflowID)
		sp.taskProgress = msg.Progress
		sp.lastUpdate = time.Now()

	case simulator.ToolExecutionMsg:
		// Update active task info
		sp.activeTask = fmt.Sprintf("Tool: %s", msg.ToolID)
		sp.taskProgress = msg.Progress
		sp.lastUpdate = time.Now()

	case tickMsg:
		// Periodic update
		cmds = append(cmds, sp.tick())

	default:
		// Update progress bar
		var cmd tea.Cmd
		sp.progress, cmd = sp.progress.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return sp, tea.Batch(cmds...)
}

// View renders the status panel
func (sp *StatusPanel) View() string {
	if !sp.ready {
		return sp.theme.TitleStyle.Render("Loading status...")
	}

	var sections []string

	// System Status Section
	sections = append(sections, sp.renderSystemStatus())
	
	// Active Task Section
	sections = append(sections, sp.renderActiveTask())
	
	// Metrics Section
	if sp.config.UI.Components.Status.ShowStats {
		sections = append(sections, sp.renderMetrics())
	}
	
	// System Info Section
	sections = append(sections, sp.renderSystemInfo())

	// Join all sections with spacing
	return lipgloss.JoinVertical(
		lipgloss.Left,
		sections...,
	)
}

// SetSize updates the panel dimensions
func (sp *StatusPanel) SetSize(width, height int) tea.Cmd {
	sp.width = width
	sp.height = height
	sp.ready = true

	// Update progress bar width (account for borders and padding)
	progressWidth := width - 8
	if progressWidth < 10 {
		progressWidth = 10
	}
	sp.progress.Width = progressWidth

	return nil
}

// renderSystemStatus renders the current system status
func (sp *StatusPanel) renderSystemStatus() string {
	title := sp.theme.TitleStyle.Render("System Status")
	
	// Status indicator with spinner if active
	statusText := sp.systemStatus.Status
	if statusText == "" {
		statusText = "unknown"
	}
	
	var statusLine string
	if statusText == "running" || statusText == "active" {
		statusLine = lipgloss.JoinHorizontal(
			lipgloss.Left,
			sp.spinner.View(),
			" ",
			sp.theme.RenderStatus(statusText),
		)
	} else {
		statusLine = sp.theme.RenderStatus(statusText)
	}

	// Uptime
	uptimeText := "Uptime: " + sp.formatDuration(sp.systemStatus.Uptime)
	uptimeStyle := lipgloss.NewStyle().Foreground(sp.theme.Colors.Secondary)
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		statusLine,
		uptimeStyle.Render(uptimeText),
		"", // Spacing
	)
}

// renderActiveTask renders information about the current task
func (sp *StatusPanel) renderActiveTask() string {
	title := sp.theme.TitleStyle.Render("Active Task")
	
	taskText := sp.activeTask
	if taskText == "" {
		taskText = "No active tasks"
	}
	
	taskStyle := lipgloss.NewStyle().
		Foreground(sp.theme.Colors.TextPrimary).
		Width(sp.width - 4).
		Align(lipgloss.Left)
	
	// Progress bar
	progressBar := sp.progress.ViewAs(sp.taskProgress)
	progressText := fmt.Sprintf("%.1f%%", sp.taskProgress*100)
	
	progressSection := lipgloss.JoinVertical(
		lipgloss.Left,
		progressBar,
		lipgloss.NewStyle().Foreground(sp.theme.Colors.Secondary).Render(progressText),
	)
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		taskStyle.Render(taskText),
		progressSection,
		"", // Spacing
	)
}

// renderMetrics renders system performance metrics
func (sp *StatusPanel) renderMetrics() string {
	title := sp.theme.TitleStyle.Render("Metrics")
	
	// Format metrics
	cpuText := fmt.Sprintf("CPU: %.1f%%", sp.metrics.CPU)
	memoryText := fmt.Sprintf("Memory: %.1f%%", sp.metrics.Memory)
	tasksText := fmt.Sprintf("Tasks: %d", sp.metrics.ActiveTasks)
	throughputText := fmt.Sprintf("Throughput: %.1f/s", sp.metrics.Throughput)
	errorRateText := fmt.Sprintf("Errors: %.2f%%", sp.metrics.ErrorRate*100)
	
	// Style metrics based on values
	metricStyle := lipgloss.NewStyle().Foreground(sp.theme.Colors.TextPrimary)
	
	// Color-code based on thresholds
	cpuColor := sp.theme.Colors.Success
	if sp.metrics.CPU > 70 {
		cpuColor = sp.theme.Colors.Warning
	}
	if sp.metrics.CPU > 90 {
		cpuColor = sp.theme.Colors.Error
	}
	
	memoryColor := sp.theme.Colors.Success
	if sp.metrics.Memory > 80 {
		memoryColor = sp.theme.Colors.Warning
	}
	if sp.metrics.Memory > 95 {
		memoryColor = sp.theme.Colors.Error
	}
	
	cpuStyled := lipgloss.NewStyle().Foreground(cpuColor).Render(cpuText)
	memoryStyled := lipgloss.NewStyle().Foreground(memoryColor).Render(memoryText)
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		cpuStyled,
		memoryStyled,
		metricStyle.Render(tasksText),
		metricStyle.Render(throughputText),
		metricStyle.Render(errorRateText),
		"", // Spacing
	)
}

// renderSystemInfo renders additional system information
func (sp *StatusPanel) renderSystemInfo() string {
	title := sp.theme.TitleStyle.Render("System Info")
	
	completedText := fmt.Sprintf("Completed: %d", sp.systemStatus.Completed)
	failedText := fmt.Sprintf("Failed: %d", sp.systemStatus.Failed)
	versionText := fmt.Sprintf("Version: %s", sp.systemStatus.Version)
	
	lastUpdateText := fmt.Sprintf("Updated: %s", sp.lastUpdate.Format("15:04:05"))
	
	infoStyle := lipgloss.NewStyle().Foreground(sp.theme.Colors.Secondary)
	
	// Color failed count if > 0
	failedColor := sp.theme.Colors.Secondary
	if sp.systemStatus.Failed > 0 {
		failedColor = sp.theme.Colors.Warning
	}
	failedStyled := lipgloss.NewStyle().Foreground(failedColor).Render(failedText)
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		infoStyle.Render(completedText),
		failedStyled,
		infoStyle.Render(versionText),
		infoStyle.Render(lastUpdateText),
	)
}

// formatDuration formats a duration in a human-readable way
func (sp *StatusPanel) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	} else {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
}

// tick creates a periodic update command
func (sp *StatusPanel) tick() tea.Cmd {
	return tea.Tick(sp.config.UI.Components.Status.UpdateInterval, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

// Custom message for periodic updates
type tickMsg struct{}

// GetSystemStatus returns current system status
func (sp *StatusPanel) GetSystemStatus() simulator.SystemStatus {
	return sp.systemStatus
}

// GetMetrics returns current metrics
func (sp *StatusPanel) GetMetrics() simulator.Metrics {
	return sp.metrics
}

// SetActiveTask updates the current active task
func (sp *StatusPanel) SetActiveTask(task string, progress float64) {
	sp.activeTask = task
	sp.taskProgress = progress
	sp.lastUpdate = time.Now()
}