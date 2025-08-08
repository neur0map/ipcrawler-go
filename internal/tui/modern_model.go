package tui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type FocusedComponent int

const (
	FocusWorkflowTable FocusedComponent = iota
	FocusToolTable
	FocusWorkflowList
	FocusOutput
)

type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	Left      key.Binding
	Right     key.Binding
	Tab       key.Binding
	Quit      key.Binding
	Cancel    key.Binding
	Help      key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "move left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "move right"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch focus"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel scan"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
	}
}

type WorkflowUpdateMsg struct {
	WorkflowID  string
	Status      string
	Duration    time.Duration
	Error       error
	Description string
}

type ModernTUIModel struct {
	mu               sync.RWMutex
	target           string
	cancelFunc       context.CancelFunc
	
	// Bubbles components
	workflowTable    table.Model
	toolTable        table.Model
	workflowList     list.Model
	outputViewport   viewport.Model
	progressBar      progress.Model
	spinner          spinner.Model
	help             help.Model
	
	// Component focus and state
	focused          FocusedComponent
	keyMap           KeyMap
	showHelp         bool
	
	// Data
	workflows        []WorkflowStatus
	tools           []ToolStatus
	logs            []LogEntry
	systemStats     SystemStats
	
	// Layout
	width           int
	height          int
	terminalSize    tea.WindowSizeMsg
}

type WorkflowListItem struct {
	workflow WorkflowStatus
}

func (w WorkflowListItem) FilterValue() string {
	return w.workflow.ID
}

func (w WorkflowListItem) Title() string {
	status := getStatusIcon(w.workflow.Status)
	return fmt.Sprintf("%s %s", status, w.workflow.ID)
}

func (w WorkflowListItem) Description() string {
	duration := formatDuration(w.workflow.Duration)
	if w.workflow.Status == "running" {
		duration = formatDuration(time.Since(w.workflow.StartTime))
	}
	return fmt.Sprintf("%s • %s", w.workflow.Description, duration)
}

func NewModernTUIModel(target string) ModernTUIModel {
	keyMap := DefaultKeyMap()
	
	// Initialize workflow table
	workflowColumns := []table.Column{
		{Title: "Workflow", Width: 25},
		{Title: "Status", Width: 10},
		{Title: "Duration", Width: 12},
		{Title: "Description", Width: 40},
	}
	
	workflowTable := table.New(
		table.WithColumns(workflowColumns),
		table.WithFocused(true),
		table.WithHeight(8),
	)
	
	// Initialize tool table
	toolColumns := []table.Column{
		{Title: "Tool", Width: 15},
		{Title: "Workflow", Width: 20},
		{Title: "Status", Width: 10},
		{Title: "Duration", Width: 12},
		{Title: "Output", Width: 30},
	}
	
	toolTable := table.New(
		table.WithColumns(toolColumns),
		table.WithHeight(8),
	)
	
	// Initialize workflow list
	workflowList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	workflowList.Title = "Active Workflows"
	workflowList.SetShowStatusBar(false)
	workflowList.SetFilteringEnabled(false)
	
	// Initialize output viewport
	outputViewport := viewport.New(0, 0)
	outputViewport.SetContent("Waiting for tool output...")
	
	// Initialize progress bar
	progressBar := progress.New(progress.WithDefaultGradient())
	
	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	
	// Initialize help
	helpModel := help.New()
	helpModel.ShowAll = false
	
	return ModernTUIModel{
		target:          target,
		workflowTable:   workflowTable,
		toolTable:      toolTable,
		workflowList:   workflowList,
		outputViewport: outputViewport,
		progressBar:    progressBar,
		spinner:        s,
		help:           helpModel,
		focused:        FocusWorkflowTable,
		keyMap:         keyMap,
		showHelp:       false,
		workflows:      []WorkflowStatus{},
		tools:          []ToolStatus{},
		logs:           []LogEntry{},
	}
}

func (m ModernTUIModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
	)
}

func (m ModernTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.mu.Lock()
		m.terminalSize = msg
		m.width = msg.Width
		m.height = msg.Height
		m.updateComponentSizes()
		m.mu.Unlock()
		return m, nil
		
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keyMap.Quit):
			if m.cancelFunc != nil {
				m.cancelFunc()
			}
			return m, tea.Quit
			
		case key.Matches(msg, m.keyMap.Cancel):
			if m.cancelFunc != nil {
				m.cancelFunc()
			}
			return m, nil
			
		case key.Matches(msg, m.keyMap.Help):
			m.showHelp = !m.showHelp
			return m, nil
			
		case key.Matches(msg, m.keyMap.Tab):
			m.cycleFocus()
			return m, nil
		}
		
		// Route input to focused component
		return m.updateFocusedComponent(msg)
		
	case ToolExecutionMsg:
		m.mu.Lock()
		status := "completed"
		if msg.Error != nil {
			status = "failed"
		}
		m.tools = append(m.tools, ToolStatus{
			Name:     msg.Tool,
			Workflow: msg.Workflow,
			Status:   status,
			Duration: msg.Duration,
			Error:    msg.Error,
			Args:     msg.Args,
			Output:   msg.Output,
		})
		m.updateToolTable()
		m.mu.Unlock()
		return m, nil
		
	case WorkflowUpdateMsg:
		m.mu.Lock()
		m.updateWorkflow(msg)
		m.updateWorkflowTable()
		m.updateWorkflowList()
		m.mu.Unlock()
		return m, nil
		
	case LogMsg:
		m.mu.Lock()
		m.logs = append(m.logs, LogEntry{
			Timestamp: msg.Timestamp,
			Level:     msg.Level,
			Category:  msg.Category,
			Message:   msg.Message,
			Data:      msg.Data,
		})
		m.updateOutputViewport()
		m.mu.Unlock()
		return m, nil
		
	case SystemStatsMsg:
		m.mu.Lock()
		m.systemStats = SystemStats(msg)
		m.mu.Unlock()
		return m, nil
	}
	
	// Update spinner
	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)
	
	return m, tea.Batch(cmds...)
}

func (m *ModernTUIModel) updateComponentSizes() {
	headerHeight := 3
	helpHeight := 0
	if m.showHelp {
		helpHeight = 10
	}
	
	availableHeight := m.height - headerHeight - helpHeight - 2
	tableHeight := availableHeight / 3
	
	// Update workflow table
	m.workflowTable.SetHeight(tableHeight)
	workflowCols := []table.Column{
		{Title: "Workflow", Width: m.width / 4},
		{Title: "Status", Width: 10},
		{Title: "Duration", Width: 12},
		{Title: "Description", Width: m.width - (m.width/4) - 22 - 4},
	}
	m.workflowTable.SetColumns(workflowCols)
	
	// Update tool table
	m.toolTable.SetHeight(tableHeight)
	toolCols := []table.Column{
		{Title: "Tool", Width: 15},
		{Title: "Workflow", Width: m.width / 5},
		{Title: "Status", Width: 10},
		{Title: "Duration", Width: 12},
		{Title: "Output", Width: m.width - 47 - (m.width/5)},
	}
	m.toolTable.SetColumns(toolCols)
	
	// Update workflow list
	listWidth := m.width / 3
	m.workflowList.SetSize(listWidth, tableHeight)
	
	// Update output viewport
	outputWidth := m.width - listWidth - 4
	m.outputViewport.Width = outputWidth
	m.outputViewport.Height = tableHeight
}

func (m *ModernTUIModel) cycleFocus() {
	switch m.focused {
	case FocusWorkflowTable:
		m.focused = FocusToolTable
		m.workflowTable.Blur()
		m.toolTable.Focus()
	case FocusToolTable:
		m.focused = FocusWorkflowList
		m.toolTable.Blur()
	case FocusWorkflowList:
		m.focused = FocusOutput
	case FocusOutput:
		m.focused = FocusWorkflowTable
		m.workflowTable.Focus()
	}
}

func (m ModernTUIModel) updateFocusedComponent(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	
	switch m.focused {
	case FocusWorkflowTable:
		m.workflowTable, cmd = m.workflowTable.Update(msg)
	case FocusToolTable:
		m.toolTable, cmd = m.toolTable.Update(msg)
	case FocusWorkflowList:
		m.workflowList, cmd = m.workflowList.Update(msg)
	case FocusOutput:
		m.outputViewport, cmd = m.outputViewport.Update(msg)
	}
	
	return m, cmd
}

func (m *ModernTUIModel) updateWorkflowTable() {
	rows := make([]table.Row, len(m.workflows))
	for i, wf := range m.workflows {
		status := getStatusIcon(wf.Status)
		duration := formatDuration(wf.Duration)
		if wf.Status == "running" {
			duration = formatDuration(time.Since(wf.StartTime))
		}
		
		rows[i] = table.Row{
			wf.ID,
			status,
			duration,
			wf.Description,
		}
	}
	m.workflowTable.SetRows(rows)
}

func (m *ModernTUIModel) updateToolTable() {
	start := 0
	if len(m.tools) > 15 {
		start = len(m.tools) - 15
	}
	
	rows := make([]table.Row, len(m.tools)-start)
	for i, tool := range m.tools[start:] {
		status := getStatusIcon(tool.Status)
		duration := formatDuration(tool.Duration)
		output := truncateString(tool.Output, 25)
		
		rows[i] = table.Row{
			tool.Name,
			tool.Workflow,
			status,
			duration,
			output,
		}
	}
	m.toolTable.SetRows(rows)
}

func (m *ModernTUIModel) updateWorkflowList() {
	items := make([]list.Item, len(m.workflows))
	for i, wf := range m.workflows {
		items[i] = WorkflowListItem{workflow: wf}
	}
	m.workflowList.SetItems(items)
}

func (m *ModernTUIModel) updateOutputViewport() {
	content := ""
	start := 0
	if len(m.logs) > 100 {
		start = len(m.logs) - 100
	}
	
	for _, log := range m.logs[start:] {
		timestamp := log.Timestamp.Format("15:04:05")
		content += fmt.Sprintf("[%s] %s: %s\n", timestamp, log.Level, log.Message)
	}
	
	m.outputViewport.SetContent(content)
	m.outputViewport.GotoBottom()
}

func (m *ModernTUIModel) updateWorkflow(msg WorkflowUpdateMsg) {
	for i, wf := range m.workflows {
		if wf.ID == msg.WorkflowID {
			m.workflows[i].Status = msg.Status
			m.workflows[i].Duration = msg.Duration
			m.workflows[i].Error = msg.Error
			if msg.Status == "running" && m.workflows[i].StartTime.IsZero() {
				m.workflows[i].StartTime = time.Now()
			}
			return
		}
	}
	
	// New workflow
	m.workflows = append(m.workflows, WorkflowStatus{
		ID:          msg.WorkflowID,
		Status:      msg.Status,
		Duration:    msg.Duration,
		Error:       msg.Error,
		Description: msg.Description,
		StartTime:   time.Now(),
	})
}

func (m *ModernTUIModel) SetCancelFunc(cancelFunc context.CancelFunc) {
	m.cancelFunc = cancelFunc
}