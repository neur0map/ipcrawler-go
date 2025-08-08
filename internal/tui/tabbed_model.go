package tui

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

type TabType int

const (
	OverviewTab TabType = iota
	ToolsTab
	WorkflowsTab
	SystemTab
	LogsTab
)

var tabNames = []string{"Overview", "Tools", "Workflows", "System", "Logs"}

type TabKeyMap struct {
	NextTab     key.Binding
	PrevTab     key.Binding
	Tab1        key.Binding
	Tab2        key.Binding
	Tab3        key.Binding
	Tab4        key.Binding
	Tab5        key.Binding
	Up          key.Binding
	Down        key.Binding
	Left        key.Binding
	Right       key.Binding
	Enter       key.Binding
	Quit        key.Binding
	Cancel      key.Binding
	Refresh     key.Binding
	Help        key.Binding
}

func DefaultTabKeyMap() TabKeyMap {
	return TabKeyMap{
		NextTab: key.NewBinding(
			key.WithKeys("tab", "right"),
			key.WithHelp("tab/→", "next tab"),
		),
		PrevTab: key.NewBinding(
			key.WithKeys("shift+tab", "left"),
			key.WithHelp("shift+tab/←", "prev tab"),
		),
		Tab1: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "overview"),
		),
		Tab2: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "tools"),
		),
		Tab3: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "workflows"),
		),
		Tab4: key.NewBinding(
			key.WithKeys("4"),
			key.WithHelp("4", "system"),
		),
		Tab5: key.NewBinding(
			key.WithKeys("5"),
			key.WithHelp("5", "logs"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Left: key.NewBinding(
			key.WithKeys("h"),
			key.WithHelp("h", "move left"),
		),
		Right: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "move right"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel scan"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
	}
}

func (k TabKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.NextTab, k.PrevTab, k.Quit, k.Help}
}

func (k TabKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Tab1, k.Tab2, k.Tab3, k.Tab4, k.Tab5},
		{k.Up, k.Down, k.Left, k.Right, k.Enter},
		{k.NextTab, k.PrevTab, k.Cancel, k.Refresh},
		{k.Quit, k.Help},
	}
}

type ActiveTool struct {
	Name      string
	Workflow  string
	StartTime time.Time
	Status    string
	Args      []string
}

type SystemMetrics struct {
	RAMUsed       uint64
	RAMPercent    float64
	CPUPercent    float64
	Goroutines    int
	ProcessPID    int32
	UpdateTime    time.Time
}

type RecentCompletion struct {
	Tool     string
	Workflow string
	Duration time.Duration
	Status   string
	Output   string
	EndTime  time.Time
}

type TabbedTUIModel struct {
	mu           sync.RWMutex
	target       string
	cancelFunc   context.CancelFunc
	
	activeTab    TabType
	keyMap       TabKeyMap
	help         help.Model
	showHelp     bool
	
	// Widgets for Overview tab
	activeToolsList     list.Model
	systemMetrics       SystemMetrics
	systemProgress      progress.Model
	recentCompletions   table.Model
	workflowStatusList  list.Model
	liveOutputViewport  viewport.Model
	
	// Data
	activeTools      []ActiveTool
	workflows        []WorkflowStatus
	tools           []ToolStatus
	logs            []LogEntry
	recentCompList  []RecentCompletion
	
	// System monitoring
	process         *process.Process
	lastUpdate      time.Time
	
	// Layout
	width           int
	height          int
}

type ActiveToolItem struct {
	tool ActiveTool
}

func (a ActiveToolItem) FilterValue() string {
	return a.tool.Name
}

func (a ActiveToolItem) Title() string {
	return fmt.Sprintf("🚀 %s → %s", a.tool.Name, a.tool.Workflow)
}

func (a ActiveToolItem) Description() string {
	elapsed := time.Since(a.tool.StartTime)
	return fmt.Sprintf("⏱️  Started: %s | ⏳ Running: %s",
		a.tool.StartTime.Format("15:04:05"),
		formatDuration(elapsed))
}

type WorkflowStatusItem struct {
	workflow WorkflowStatus
}

func (w WorkflowStatusItem) FilterValue() string {
	return w.workflow.ID
}

func (w WorkflowStatusItem) Title() string {
	icon := getStatusIcon(w.workflow.Status)
	return fmt.Sprintf("%s %s", icon, w.workflow.ID)
}

func (w WorkflowStatusItem) Description() string {
	if w.workflow.Status == "running" {
		elapsed := time.Since(w.workflow.StartTime)
		return fmt.Sprintf("Running for %s", formatDuration(elapsed))
	}
	return fmt.Sprintf("Completed in %s", formatDuration(w.workflow.Duration))
}

func NewTabbedTUIModel(target string) TabbedTUIModel {
	keyMap := DefaultTabKeyMap()
	
	// Initialize help
	helpModel := help.New()
	helpModel.ShowAll = false
	
	// Initialize active tools list
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#3b82f6")).
		BorderLeftForeground(lipgloss.Color("#3b82f6"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#3b82f6"))
	
	activeToolsList := list.New([]list.Item{}, delegate, 0, 0)
	activeToolsList.SetShowStatusBar(false)
	activeToolsList.SetFilteringEnabled(false)
	activeToolsList.Title = "Active Tools"
	
	// Initialize system progress
	systemProgress := progress.New(progress.WithDefaultGradient())
	
	// Initialize recent completions table
	recentCompletions := table.New(
		table.WithColumns([]table.Column{
			{Title: "Tool", Width: 12},
			{Title: "Workflow", Width: 20},
			{Title: "Duration", Width: 10},
			{Title: "Status", Width: 8},
		}),
		table.WithHeight(5),
		table.WithFocused(false),
	)
	
	// Initialize workflow status list
	workflowDelegate := list.NewDefaultDelegate()
	workflowDelegate.Styles.SelectedTitle = workflowDelegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#059669")).
		BorderLeftForeground(lipgloss.Color("#059669"))
	workflowDelegate.Styles.SelectedDesc = workflowDelegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#059669"))
	
	workflowStatusList := list.New([]list.Item{}, workflowDelegate, 0, 0)
	workflowStatusList.SetShowStatusBar(false)
	workflowStatusList.SetFilteringEnabled(false)
	workflowStatusList.Title = "Workflow Status"
	
	// Initialize live output viewport
	liveOutputViewport := viewport.New(0, 0)
	liveOutputViewport.SetContent("Waiting for tool output...")
	
	// Get current process for monitoring - handle potential errors
	var currentProcess *process.Process
	if pid := int32(os.Getpid()); pid > 0 {
		if proc, err := process.NewProcess(pid); err == nil {
			currentProcess = proc
		}
	}
	
	return TabbedTUIModel{
		target:             target,
		activeTab:          OverviewTab,
		keyMap:             keyMap,
		help:               helpModel,
		showHelp:           false,
		activeToolsList:    activeToolsList,
		systemProgress:     systemProgress,
		recentCompletions:  recentCompletions,
		workflowStatusList: workflowStatusList,
		liveOutputViewport: liveOutputViewport,
		activeTools:        []ActiveTool{},
		workflows:          []WorkflowStatus{},
		tools:             []ToolStatus{},
		logs:              []LogEntry{},
		recentCompList:    []RecentCompletion{},
		process:           currentProcess,
		lastUpdate:        time.Now(),
	}
}

func (m *TabbedTUIModel) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
			return SystemUpdateMsg{Time: t}
		}),
	)
}

type SystemUpdateMsg struct {
	Time time.Time
}

func (m *TabbedTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.mu.Lock()
		m.width = msg.Width
		m.height = msg.Height
		m.updateComponentSizes()
		m.mu.Unlock()
		return m, nil
		
	case SystemUpdateMsg:
		m.mu.Lock()
		m.updateSystemMetrics()
		m.mu.Unlock()
		cmds = append(cmds, tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
			return SystemUpdateMsg{Time: t}
		}))
		
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
			
		case key.Matches(msg, m.keyMap.NextTab):
			m.activeTab = TabType((int(m.activeTab) + 1) % len(tabNames))
			return m, nil
			
		case key.Matches(msg, m.keyMap.PrevTab):
			m.activeTab = TabType((int(m.activeTab) - 1 + len(tabNames)) % len(tabNames))
			return m, nil
			
		case key.Matches(msg, m.keyMap.Tab1):
			m.activeTab = OverviewTab
			return m, nil
		case key.Matches(msg, m.keyMap.Tab2):
			m.activeTab = ToolsTab
			return m, nil
		case key.Matches(msg, m.keyMap.Tab3):
			m.activeTab = WorkflowsTab
			return m, nil
		case key.Matches(msg, m.keyMap.Tab4):
			m.activeTab = SystemTab
			return m, nil
		case key.Matches(msg, m.keyMap.Tab5):
			m.activeTab = LogsTab
			return m, nil
			
		case key.Matches(msg, m.keyMap.Refresh):
			m.updateSystemMetrics()
			return m, nil
		}
		
		// Route input to active tab components
		return m.updateActiveTabComponents(msg)
		
	case ToolStartMsg:
		m.mu.Lock()
		m.handleToolStart(msg)
		m.DebugModel()
		m.mu.Unlock()
		return m, nil
		
	case ToolExecutionMsg:
		m.mu.Lock()
		m.handleToolExecution(msg)
		m.DebugModel()
		m.mu.Unlock()
		return m, nil
		
	case WorkflowUpdateMsg:
		m.mu.Lock()
		m.updateWorkflow(msg)
		m.updateWorkflowStatusList()
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
		m.updateLiveOutput()
		m.mu.Unlock()
		return m, nil
		
	case SystemStatsMsg:
		m.mu.Lock()
		// Handle existing system stats if needed
		m.mu.Unlock()
		return m, nil
	}
	
	return m, tea.Batch(cmds...)
}

func (m *TabbedTUIModel) updateComponentSizes() {
	if m.width < 50 || m.height < 20 {
		return
	}
	
	tabHeight := 3
	helpHeight := 0
	if m.showHelp {
		helpHeight = 10
	}
	
	contentHeight := m.height - tabHeight - helpHeight - 2
	contentWidth := m.width - 2
	
	// Overview tab specific sizing
	if m.activeTab == OverviewTab {
		// Active tools list (left top)
		activeToolsWidth := contentWidth / 2 - 2
		activeToolsHeight := contentHeight / 2 - 1
		m.activeToolsList.SetSize(activeToolsWidth, activeToolsHeight)
		
		// Recent completions table (bottom)
		m.recentCompletions.SetHeight(contentHeight / 3)
		recentCols := []table.Column{
			{Title: "Tool", Width: 12},
			{Title: "Workflow", Width: contentWidth / 4},
			{Title: "Duration", Width: 10},
			{Title: "Status", Width: 8},
		}
		m.recentCompletions.SetColumns(recentCols)
		
		// Workflow status list (right top)
		workflowWidth := contentWidth / 2 - 2
		workflowHeight := contentHeight / 2 - 1
		m.workflowStatusList.SetSize(workflowWidth, workflowHeight)
		
		// Live output viewport (right bottom)
		m.liveOutputViewport.Width = workflowWidth
		m.liveOutputViewport.Height = contentHeight / 3
	}
}

func (m *TabbedTUIModel) updateActiveTabComponents(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.activeTab {
	case OverviewTab:
		// Route to different widgets based on focus
		m.activeToolsList, _ = m.activeToolsList.Update(msg)
	case ToolsTab:
		// Handle tools tab navigation
	case WorkflowsTab:
		// Handle workflows tab navigation
	case SystemTab:
		// Handle system tab navigation  
	case LogsTab:
		// Handle logs tab navigation
	}
	
	return m, nil
}

func (m *TabbedTUIModel) handleToolStart(msg ToolStartMsg) {
	activeTool := ActiveTool{
		Name:      msg.Tool,
		Workflow:  msg.Workflow,
		StartTime: time.Now(),
		Status:    "running",
		Args:      msg.Args,
	}
	
	m.activeTools = append(m.activeTools, activeTool)
	m.updateActiveToolsList()
}

func (m *TabbedTUIModel) handleToolExecution(msg ToolExecutionMsg) {
	status := "completed"
	if msg.Error != nil {
		status = "failed"
	}
	
	// Add to tools list
	m.tools = append(m.tools, ToolStatus{
		Name:     msg.Tool,
		Workflow: msg.Workflow,
		Status:   status,
		Duration: msg.Duration,
		Error:    msg.Error,
		Args:     msg.Args,
		Output:   msg.Output,
	})
	
	// Add to recent completions
	m.recentCompList = append(m.recentCompList, RecentCompletion{
		Tool:     msg.Tool,
		Workflow: msg.Workflow,
		Duration: msg.Duration,
		Status:   status,
		Output:   msg.Output,
		EndTime:  time.Now(),
	})
	
	// Keep only last 10 recent completions
	if len(m.recentCompList) > 10 {
		m.recentCompList = m.recentCompList[len(m.recentCompList)-10:]
	}
	
	// Remove from active tools if present
	for i, activeTool := range m.activeTools {
		if activeTool.Name == msg.Tool && activeTool.Workflow == msg.Workflow {
			m.activeTools = append(m.activeTools[:i], m.activeTools[i+1:]...)
			break
		}
	}
	
	m.updateActiveToolsList()
	m.updateRecentCompletions()
}

func (m *TabbedTUIModel) updateSystemMetrics() {
	if m.process == nil {
		// Set default values when process monitoring is unavailable
		m.systemMetrics = SystemMetrics{
			RAMUsed:    0,
			RAMPercent: 0,
			CPUPercent: 0,
			ProcessPID: int32(os.Getpid()),
			UpdateTime: time.Now(),
		}
		return
	}
	
	// Initialize with current time and PID
	m.systemMetrics.ProcessPID = m.process.Pid
	m.systemMetrics.UpdateTime = time.Now()
	
	// Get memory info with error handling
	if memInfo, err := m.process.MemoryInfo(); err == nil && memInfo != nil {
		m.systemMetrics.RAMUsed = memInfo.RSS
		
		// Get system memory for percentage calculation
		if vmem, err := mem.VirtualMemory(); err == nil && vmem != nil && vmem.Total > 0 {
			m.systemMetrics.RAMPercent = float64(memInfo.RSS) / float64(vmem.Total) * 100
		}
	}
	
	// Get CPU percentage with error handling
	if cpuPercent, err := m.process.CPUPercent(); err == nil {
		m.systemMetrics.CPUPercent = cpuPercent
	}
}

func (m *TabbedTUIModel) updateActiveToolsList() {
	items := make([]list.Item, len(m.activeTools))
	for i, tool := range m.activeTools {
		items[i] = ActiveToolItem{tool: tool}
	}
	m.activeToolsList.SetItems(items)
}

func (m *TabbedTUIModel) updateRecentCompletions() {
	rows := make([]table.Row, len(m.recentCompList))
	for i, completion := range m.recentCompList {
		statusIcon := "✅"
		if completion.Status == "failed" {
			statusIcon = "❌"
		}
		
		rows[i] = table.Row{
			completion.Tool,
			completion.Workflow,
			formatDuration(completion.Duration),
			statusIcon,
		}
	}
	m.recentCompletions.SetRows(rows)
}

func (m *TabbedTUIModel) updateWorkflowStatusList() {
	items := make([]list.Item, len(m.workflows))
	for i, wf := range m.workflows {
		items[i] = WorkflowStatusItem{workflow: wf}
	}
	m.workflowStatusList.SetItems(items)
}

func (m *TabbedTUIModel) updateLiveOutput() {
	content := ""
	start := 0
	if len(m.logs) > 50 {
		start = len(m.logs) - 50
	}
	
	for _, log := range m.logs[start:] {
		timestamp := log.Timestamp.Format("15:04:05")
		content += fmt.Sprintf("[%s] %s: %s\n", timestamp, log.Category, log.Message)
	}
	
	m.liveOutputViewport.SetContent(content)
	m.liveOutputViewport.GotoBottom()
}

func (m *TabbedTUIModel) updateWorkflow(msg WorkflowUpdateMsg) {
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

func (m *TabbedTUIModel) SetCancelFunc(cancelFunc context.CancelFunc) {
	m.cancelFunc = cancelFunc
}

func (m *TabbedTUIModel) StartTool(tool, workflow string, args []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	activeTool := ActiveTool{
		Name:      tool,
		Workflow:  workflow,
		StartTime: time.Now(),
		Status:    "running",
		Args:      args,
	}
	
	m.activeTools = append(m.activeTools, activeTool)
	m.updateActiveToolsList()
}