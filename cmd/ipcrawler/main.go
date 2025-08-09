package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/mattn/go-isatty"

	"github.com/your-org/ipcrawler/internal/tui/data/loader"
)

// focusState tracks which card is focused
type focusState int

const (
	workflowTreeFocus focusState = iota  // Was overviewFocus - now for selecting workflows
	scanOverviewFocus                    // Was workflowsFocus - now shows selected workflow details
	outputFocus                          // Live raw tool output
	logsFocus                           // System logs, debug, errors, warnings
	toolsFocus
	perfFocus
)

type model struct {
	// Layout
	width  int
	height int
	focus  focusState
	ready  bool

	// Data source
	workflows *loader.WorkflowData
	
	// Interactive list components
	workflowTreeList list.Model  // For selecting workflows
	scanOverviewList list.Model  // Shows execution queue and status
	
	// Multi-select workflow tracking
	selectedWorkflows map[string]bool  // Track which workflows are selected
	currentWorkflow   string           // Currently highlighted workflow
	executionQueue    []string         // Queue of workflows to execute
	
	// Live output (raw tool output)
	outputViewport viewport.Model
	liveOutput     []string  // Raw tool execution output
	
	// System logs (debug, errors, warnings)
	logsViewport viewport.Model
	systemLogs   []string  // System messages, debug info, errors
	logger       *log.Logger  // Charmbracelet structured logger
	
	// Running tools
	spinner spinner.Model
	tools   []toolExecution
	
	// Performance data
	perfData systemMetrics

	// Calculated card dimensions
	cardWidth  int
	cardHeight int

	// Styles for cards
	cardStyle        lipgloss.Style
	focusedCardStyle lipgloss.Style
	titleStyle       lipgloss.Style
	dimStyle         lipgloss.Style
	headerStyle      lipgloss.Style
}

type toolExecution struct {
	Name   string
	Status string
	Output string
}

type systemMetrics struct {
	MemoryMB    float64
	Goroutines  int
	LastUpdate  string
}

// List item implementations for bubbles/list component

// systemItem represents a system overview item
type systemItem struct {
	title string
	desc  string
}

func (i systemItem) Title() string       { return i.title }
func (i systemItem) Description() string { return i.desc }
func (i systemItem) FilterValue() string { return i.title }

// workflowItem represents a workflow in the tree with selection status
type workflowItem struct {
	name        string
	description string
	toolCount   int
	selected    bool
}

func (i workflowItem) Title() string { 
	checkbox := "[ ]"
	if i.selected {
		checkbox = "[X]"
	}
	return fmt.Sprintf("%s %s", checkbox, i.name)
}
func (i workflowItem) Description() string { 
	return fmt.Sprintf("%s (%d tools)", i.description, i.toolCount)
}
func (i workflowItem) FilterValue() string { return i.name }

// toolItem represents a tool within a workflow
type toolItem struct {
	name     string
	workflow string
}

func (i toolItem) Title() string       { return i.name }
func (i toolItem) Description() string { return fmt.Sprintf("in %s workflow", i.workflow) }
func (i toolItem) FilterValue() string { return i.name }

// executionItem represents a workflow in the execution queue
type executionItem struct {
	name        string
	description string
	status      string  // "queued", "running", "completed", "failed"
}

func (i executionItem) Title() string { 
	statusIcon := "[WAIT]"
	switch i.status {
	case "queued":
		statusIcon = "[QUEUE]"
	case "running":
		statusIcon = "[RUN]"
	case "completed":
		statusIcon = "[DONE]"
	case "failed":
		statusIcon = "[FAIL]"
	case "ready":
		statusIcon = "[READY]"
	case "info":
		statusIcon = "[INFO]"
	default:
		statusIcon = "[WAIT]"
	}
	return fmt.Sprintf("%s %s", statusIcon, i.name)
}
func (i executionItem) Description() string { 
	return fmt.Sprintf("%s - %s", i.description, i.status)
}
func (i executionItem) FilterValue() string { return i.name }

func newModel() *model {
	// Load workflows - try multiple paths
	var workflows *loader.WorkflowData
	var err error
	
	// Try current directory first
	workflows, err = loader.LoadWorkflowDescriptions(".")
	if err != nil || workflows == nil || len(workflows.Workflows) == 0 {
		// Try from executable directory
		if execPath, err := os.Executable(); err == nil {
			execDir := filepath.Dir(execPath)
			workflows, _ = loader.LoadWorkflowDescriptions(execDir)
			
			// If that fails, try parent directory of executable (for bin/ case)
			if workflows == nil || len(workflows.Workflows) == 0 {
				parentDir := filepath.Dir(execDir)
				workflows, _ = loader.LoadWorkflowDescriptions(parentDir)
			}
		}
	}
	
	// Final fallback: empty workflows
	if workflows == nil || len(workflows.Workflows) == 0 {
		workflows = &loader.WorkflowData{Workflows: make(map[string]loader.WorkflowConfig)}
	}

	// Create viewports for output and logs
	liveOutputVp := viewport.New(50, 10)
	logsVp := viewport.New(50, 10)
	
	// Initialize live output (raw tool execution output)
	liveOutputLines := []string{
		"IPCrawler Live Tool Output",
		"Ready to execute workflows...",
		"",
	}
	
	// Create structured logger for system logs
	var systemLogBuffer strings.Builder
	logger := log.NewWithOptions(&systemLogBuffer, log.Options{
		ReportTimestamp: true,
		TimeFormat:      "15:04:05",
		Prefix:          "IPCrawler ",
		Level:           log.DebugLevel,
	})
	
	// Build initial system logs with logger
	logger.Info("System initialized")
	logger.Info("Loading workflows from descriptions.yaml")
	
	// Add workflow loading result
	if len(workflows.Workflows) > 0 {
		logger.Info("Workflows loaded successfully", "count", len(workflows.Workflows))
		for name := range workflows.Workflows {
			logger.Debug("Found workflow", "name", name)
		}
	} else {
		logger.Warn("No workflows loaded - check workflows/descriptions.yaml")
		if err != nil {
			logger.Error("Workflow loading failed", "err", err)
		}
	}
	
	logger.Info("TUI ready - Use Tab to navigate cards")
	logger.Info("Press 1-6 for direct card focus")
	
	// Get the initial system logs
	systemLogLines := strings.Split(systemLogBuffer.String(), "\n")
	
	// Set content for both viewports
	liveOutputVp.SetContent(strings.Join(liveOutputLines, "\n"))
	logsVp.SetContent(strings.Join(systemLogLines, "\n"))

	// Create spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Create list delegates with custom styling
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("69")).
		BorderLeftForeground(lipgloss.Color("69"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle.Copy()

	// Build workflow tree items (for multi-selection)
	workflowTreeItems := []list.Item{}
	for name, workflow := range workflows.Workflows {
		workflowTreeItems = append(workflowTreeItems, workflowItem{
			name:        name,
			description: workflow.Name,
			toolCount:   len(workflow.Tools),
			selected:    false, // Initially not selected
		})
	}

	// Add fallback items if no workflows
	if len(workflows.Workflows) == 0 {
		workflowTreeItems = []list.Item{
			workflowItem{name: "No workflows found", description: "Check workflows/descriptions.yaml", toolCount: 0, selected: false},
		}
	}

	// Initialize with empty execution queue
	executionQueueItems := []list.Item{
		executionItem{name: "No workflows selected", description: "Select workflows and press Enter to execute", status: ""},
	}

	// Create the lists
	workflowTreeList := list.New(workflowTreeItems, delegate, 0, 0)
	workflowTreeList.Title = "Available Workflows"
	workflowTreeList.SetShowStatusBar(false)
	workflowTreeList.SetShowPagination(false)
	workflowTreeList.SetFilteringEnabled(false)

	scanOverviewList := list.New(executionQueueItems, delegate, 0, 0)
	scanOverviewList.Title = "Execution Queue"
	scanOverviewList.SetShowStatusBar(false)
	scanOverviewList.SetShowPagination(false)
	scanOverviewList.SetFilteringEnabled(false)

	m := &model{
		workflows:         workflows,
		workflowTreeList:  workflowTreeList,
		scanOverviewList:  scanOverviewList,
		selectedWorkflows: make(map[string]bool),
		executionQueue:    []string{},
		outputViewport:    liveOutputVp,
		liveOutput:        liveOutputLines,
		logsViewport:      logsVp,
		systemLogs:        systemLogLines,
		logger:            logger,
		spinner:           s,
		tools:             []toolExecution{},
		perfData:          systemMetrics{MemoryMB: 12.5, Goroutines: 5, LastUpdate: "12:34:56"},
		
		// Box card styles
		cardStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1),
		focusedCardStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("69")).
			Padding(0, 1),
		titleStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("69")).
			Bold(true).
			Align(lipgloss.Center),
		headerStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Align(lipgloss.Right),
		dimStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")),
	}
	
	return m
}

func (m *model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSizes()
		
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "tab":
			m.focus = focusState((int(m.focus) + 1) % 6)
		case "shift+tab":
			m.focus = focusState((int(m.focus) + 5) % 6)
		case "1":
			m.focus = workflowTreeFocus
		case "2":
			m.focus = scanOverviewFocus
		case "3":
			m.focus = outputFocus
		case "4":
			m.focus = logsFocus
		case "5":
			m.focus = toolsFocus
		case "6":
			m.focus = perfFocus
		case " ":
			// Handle workflow selection/deselection with spacebar
			if m.focus == workflowTreeFocus {
				// Move from workflow tree to execution queue
				selectedItem := m.workflowTreeList.SelectedItem()
				if workflowItem, ok := selectedItem.(workflowItem); ok {
					m.selectedWorkflows[workflowItem.name] = true
					m.updateWorkflowList()
					m.updateExecutionQueue()
					
					// Log workflow selection and update logs viewport
					selectedCount := 0
					for _, selected := range m.selectedWorkflows {
						if selected {
							selectedCount++
						}
					}
					m.logSystemMessage("info", "Workflow added to queue", "workflow", workflowItem.name, "total_queued", selectedCount)
				}
			} else if m.focus == scanOverviewFocus {
				// Move from execution queue back to workflow tree
				selectedItem := m.scanOverviewList.SelectedItem()
				if executionItem, ok := selectedItem.(executionItem); ok {
					// Only deselect if it's an actual workflow (not summary items)
					if _, exists := m.workflows.Workflows[executionItem.name]; exists {
						m.selectedWorkflows[executionItem.name] = false
						m.updateWorkflowList()
						m.updateExecutionQueue()
						
						// Log workflow deselection
						selectedCount := 0
						for _, selected := range m.selectedWorkflows {
							if selected {
								selectedCount++
							}
						}
						m.logSystemMessage("info", "Workflow removed from queue", "workflow", executionItem.name, "total_queued", selectedCount)
					}
				}
			}
		case "enter":
			// Execute selected workflows from either card
			hasSelectedWorkflows := false
			for _, selected := range m.selectedWorkflows {
				if selected {
					hasSelectedWorkflows = true
					break
				}
			}
			if (m.focus == workflowTreeFocus || m.focus == scanOverviewFocus) && hasSelectedWorkflows {
				m.executeSelectedWorkflows()
			}
		default:
			// Handle focused component input
			switch m.focus {
			case workflowTreeFocus:
				m.workflowTreeList, cmd = m.workflowTreeList.Update(msg)
				cmds = append(cmds, cmd)
			case scanOverviewFocus:
				m.scanOverviewList, cmd = m.scanOverviewList.Update(msg)
				cmds = append(cmds, cmd)
			case outputFocus:
				m.outputViewport, cmd = m.outputViewport.Update(msg)
				cmds = append(cmds, cmd)
			case logsFocus:
				m.logsViewport, cmd = m.logsViewport.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) View() string {
	if !m.ready || m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	// Create title
	title := m.titleStyle.Render("IPCrawler TUI - Dynamic Cards Dashboard")
	title = lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(title)

	// Create help with focus-specific instructions
	var helpText string
	selectedCount := 0
	for _, selected := range m.selectedWorkflows {
		if selected {
			selectedCount++
		}
	}
	
	switch m.focus {
	case workflowTreeFocus:
		if selectedCount > 0 {
			helpText = fmt.Sprintf("SPACE: add to queue • ENTER: execute %d queued • ↑↓: navigate • Tab: view queue", selectedCount)
		} else {
			helpText = "SPACE: add to queue • ↑↓: navigate • Tab: view queue • q: quit"
		}
	case scanOverviewFocus:
		if selectedCount > 0 {
			helpText = fmt.Sprintf("SPACE: remove from queue • ENTER: execute %d queued • ↑↓: navigate • Tab: add more", selectedCount)
		} else {
			helpText = "No workflows queued • Tab: go back to select workflows • q: quit"
		}
	case outputFocus:
		helpText = "Tab/Shift+Tab: cycle focus • 1-6: direct focus • ↑↓: scroll live output • q: quit"
	case logsFocus:
		helpText = "Tab/Shift+Tab: cycle focus • 1-6: direct focus • ↑↓: scroll system logs • q: quit"
	default:
		helpText = "Tab/Shift+Tab: cycle focus • 1-6: direct focus • Arrow keys: navigate • q: quit"
	}
	help := m.dimStyle.Render(helpText)
	help = lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(help)

	// Render cards with proper list components
	workflowSelection := m.renderWorkflowTreeCard()
	executionQueue := m.renderScanOverviewCard()
	tools := m.renderToolsCard()
	perf := m.renderPerfCard()
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, workflowSelection, "  ", executionQueue, "  ", tools, "  ", perf)

	// Bottom Row: Live Output and Logs (side by side)
	liveOutput := m.renderOutputCard()
	logs := m.renderLogsCard()
	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, liveOutput, "  ", logs)

	// Combine all
	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		topRow,
		"",
		bottomRow,
		"",
		help,
	)

	return content
}

func (m *model) renderWorkflowTreeCard() string {
	style := m.cardStyle
	if m.focus == workflowTreeFocus {
		style = m.focusedCardStyle
	}

	// Use the list component's view
	return style.Width(m.cardWidth).Height(m.cardHeight).Render(m.workflowTreeList.View())
}

func (m *model) renderScanOverviewCard() string {
	style := m.cardStyle
	if m.focus == scanOverviewFocus {
		style = m.focusedCardStyle
	}

	// Use the list component's view
	return style.Width(m.cardWidth).Height(m.cardHeight).Render(m.scanOverviewList.View())
}

func (m *model) renderOutputCard() string {
	style := m.cardStyle
	if m.focus == outputFocus {
		style = m.focusedCardStyle
	}

	// Calculate width for side-by-side layout (split bottom row)
	outputWidth := (m.width - 6) / 2  // Split width, account for spacing

	// Card header with title and scroll info
	titleText := m.titleStyle.Render("Live Output")
	scrollInfo := m.headerStyle.Render(fmt.Sprintf("%.1f%%", m.outputViewport.ScrollPercent()*100))
	titleWidth := outputWidth - 4
	header := lipgloss.JoinHorizontal(lipgloss.Left, titleText, 
		strings.Repeat(" ", titleWidth-lipgloss.Width(titleText)-lipgloss.Width(scrollInfo)), scrollInfo)
	
	// Card content
	content := m.outputViewport.View()

	// Combine header and content
	cardContent := lipgloss.JoinVertical(lipgloss.Left,
		header,
		strings.Repeat("─", titleWidth),
		content,
	)

	return style.Width(outputWidth).Height(m.cardHeight).Render(cardContent)
}

func (m *model) renderLogsCard() string {
	style := m.cardStyle
	if m.focus == logsFocus {
		style = m.focusedCardStyle
	}

	// Calculate width for side-by-side layout (split bottom row)
	logsWidth := (m.width - 6) / 2  // Split width, account for spacing

	// Card header with title and scroll info
	titleText := m.titleStyle.Render("Logs")
	scrollInfo := m.headerStyle.Render(fmt.Sprintf("%.1f%%", m.logsViewport.ScrollPercent()*100))
	titleWidth := logsWidth - 4
	header := lipgloss.JoinHorizontal(lipgloss.Left, titleText, 
		strings.Repeat(" ", titleWidth-lipgloss.Width(titleText)-lipgloss.Width(scrollInfo)), scrollInfo)
	
	// Card content
	content := m.logsViewport.View()

	// Combine header and content
	cardContent := lipgloss.JoinVertical(lipgloss.Left,
		header,
		strings.Repeat("─", titleWidth),
		content,
	)

	return style.Width(logsWidth).Height(m.cardHeight).Render(cardContent)
}

func (m *model) renderToolsCard() string {
	style := m.cardStyle
	if m.focus == toolsFocus {
		style = m.focusedCardStyle
	}

	// Card header with title
	title := m.titleStyle.Width(m.cardWidth - 2).Render("Running Tools")
	
	// Card content
	content := strings.Builder{}
	if len(m.tools) == 0 {
		content.WriteString("No tools running\n")
		content.WriteString(m.spinner.View() + " Waiting for jobs...")
	} else {
		for _, tool := range m.tools {
			status := "✓"
			if tool.Status == "running" {
				status = m.spinner.View()
			}
			content.WriteString(fmt.Sprintf("%s %s\n", status, tool.Name))
		}
	}

	// Combine title and content
	cardContent := lipgloss.JoinVertical(lipgloss.Left,
		title,
		strings.Repeat("─", m.cardWidth-2),
		content.String(),
	)

	return style.Width(m.cardWidth).Height(m.cardHeight).Render(cardContent)
}

func (m *model) renderPerfCard() string {
	style := m.cardStyle
	if m.focus == perfFocus {
		style = m.focusedCardStyle
	}

	// Card header with title
	title := m.titleStyle.Width(m.cardWidth - 2).Render("Performance Monitor")
	
	// Card content
	content := fmt.Sprintf(`Memory: %.1f MB
Goroutines: %d
Last Update: %s

System: Operational
Status: Active`,
		m.perfData.MemoryMB,
		m.perfData.Goroutines,
		m.perfData.LastUpdate,
	)

	// Combine title and content
	cardContent := lipgloss.JoinVertical(lipgloss.Left,
		title,
		strings.Repeat("─", m.cardWidth-2),
		content,
	)

	return style.Width(m.cardWidth).Height(m.cardHeight).Render(cardContent)
}

func (m *model) updateSizes() {
	if m.width <= 10 || m.height <= 10 {
		return
	}

	// Calculate card dimensions for horizontal layout (4 cards across)
	// Account for borders and spacing
	m.cardWidth = (m.width - 8) / 4  // 4 cards horizontal with spacing
	m.cardHeight = (m.height - 10) / 2 // 2 rows: top cards + output

	// Ensure reasonable minimums for readability
	if m.cardWidth < 35 {
		m.cardWidth = 35
	}
	if m.cardHeight < 12 {
		m.cardHeight = 12
	}

	// Update list sizes (account for borders and padding)
	listWidth := m.cardWidth - 4  // Subtract border and padding
	listHeight := m.cardHeight - 4 // Subtract border and padding
	
	if listWidth > 0 && listHeight > 0 {
		m.workflowTreeList.SetSize(listWidth, listHeight)
		m.scanOverviewList.SetSize(listWidth, listHeight)
	}

	// Update viewport sizes for side-by-side layout (live output and logs)
	bottomCardWidth := (m.width - 6) / 2  // Split bottom row
	viewportWidth := bottomCardWidth - 8  // Account for card borders and padding
	viewportHeight := m.cardHeight - 4
	
	if viewportHeight < 8 {
		viewportHeight = 8
	}
	if viewportWidth < 20 {
		viewportWidth = 20
	}
	
	// Update both viewports with same dimensions
	m.outputViewport.Width = viewportWidth
	m.outputViewport.Height = viewportHeight
	m.logsViewport.Width = viewportWidth
	m.logsViewport.Height = viewportHeight
}

// updateWorkflowList refreshes the workflow list to show only unselected workflows
func (m *model) updateWorkflowList() {
	items := []list.Item{}
	availableCount := 0
	
	for name, workflow := range m.workflows.Workflows {
		// Only show workflows that are NOT selected
		if !m.selectedWorkflows[name] {
			items = append(items, workflowItem{
				name:        name,
				description: workflow.Name,
				toolCount:   len(workflow.Tools),
				selected:    false, // Always false since we only show unselected
			})
			availableCount++
		}
	}
	
	if len(m.workflows.Workflows) == 0 {
		items = []list.Item{
			workflowItem{name: "No workflows found", description: "Check workflows/descriptions.yaml", toolCount: 0, selected: false},
		}
	} else if availableCount == 0 {
		items = []list.Item{
			workflowItem{name: "All workflows selected", description: "Go to Execution Queue to deselect workflows", toolCount: 0, selected: false},
		}
	}
	
	m.workflowTreeList.SetItems(items)
}

// updateExecutionQueue builds the execution queue display
func (m *model) updateExecutionQueue() {
	items := []list.Item{}
	
	if len(m.selectedWorkflows) == 0 {
		items = append(items, executionItem{
			name:        "No workflows selected", 
			description: "Select workflows with SPACE and press ENTER to execute", 
			status:      "",
		})
	} else {
		selectedCount := 0
		for workflowName, selected := range m.selectedWorkflows {
			if selected {
				selectedCount++
				if workflow, exists := m.workflows.Workflows[workflowName]; exists {
					items = append(items, executionItem{
						name:        workflowName,
						description: workflow.Name,
						status:      "queued",
					})
				}
			}
		}
		
		// Add summary
		if selectedCount > 0 {
			items = append(items, executionItem{
				name:        fmt.Sprintf("Total: %d workflows", selectedCount),
				description: "Press ENTER to execute all selected workflows",
				status:      "ready",
			})
		}
	}
	
	m.scanOverviewList.SetItems(items)
}

// executeSelectedWorkflows starts execution of selected workflows (UI simulation only)
func (m *model) executeSelectedWorkflows() {
	// This is UI-only for now - no actual backend execution
	items := []list.Item{}
	
	for workflowName, selected := range m.selectedWorkflows {
		if selected {
			if workflow, exists := m.workflows.Workflows[workflowName]; exists {
				items = append(items, executionItem{
					name:        workflowName,
					description: workflow.Name,
					status:      "running",
				})
			}
		}
	}
	
	// Add execution summary
	executedCount := len(items)
	items = append(items, executionItem{
		name:        fmt.Sprintf("Executing %d workflows", executedCount),
		description: "Backend execution logic will be implemented later",
		status:      "info",
	})
	
	m.scanOverviewList.SetItems(items)
	
	// Update live output with tool execution simulation
	m.liveOutput = []string{
		"=== IPCrawler Tool Execution Started ===",
		"",
	}
	
	// Add individual workflow execution to live output (simulated tool output)
	for workflowName, selected := range m.selectedWorkflows {
		if selected {
			m.liveOutput = append(m.liveOutput, 
				fmt.Sprintf(">>> Starting workflow: %s", workflowName),
				"[tool] nmap -sS target.com",
				"[tool] Starting Nmap scan...",
				"[tool] Discovered 3 open ports",
				"",
			)
		}
	}
	
	m.liveOutput = append(m.liveOutput, "=== Execution in progress ===")
	
	// Update live output viewport
	liveOutputContent := strings.Join(m.liveOutput, "\n")
	m.outputViewport.SetContent(liveOutputContent)
	
	// Log execution start to system logs
	m.logSystemMessage("info", "Workflow execution started", "count", executedCount)
	for workflowName, selected := range m.selectedWorkflows {
		if selected {
			m.logSystemMessage("debug", "Executing workflow", "workflow", workflowName)
		}
	}
}

// logSystemMessage adds a structured log message and updates the logs viewport
func (m *model) logSystemMessage(level, message string, keyvals ...interface{}) {
	switch level {
	case "debug":
		m.logger.Debug(message, keyvals...)
	case "info":
		m.logger.Info(message, keyvals...)
	case "warn":
		m.logger.Warn(message, keyvals...)
	case "error":
		m.logger.Error(message, keyvals...)
	default:
		m.logger.Info(message, keyvals...)
	}
	
	// Note: Since we're using a buffer, we need to get the current content
	// This is a simplified approach - in production you'd want a more efficient method
	m.systemLogs = append(m.systemLogs, fmt.Sprintf("[%s] %s %v", time.Now().Format("15:04:05"), strings.ToUpper(level), message))
	logContent := strings.Join(m.systemLogs, "\n")
	m.logsViewport.SetContent(logContent)
}

// getTerminalSize returns the actual terminal dimensions
func getTerminalSize() (int, int) {
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		// Fallback to tput if stty fails
		rowsCmd := exec.Command("tput", "lines")
		colsCmd := exec.Command("tput", "cols")
		
		rowsOut, err1 := rowsCmd.Output()
		colsOut, err2 := colsCmd.Output()
		
		if err1 == nil && err2 == nil {
			rows := strings.TrimSpace(string(rowsOut))
			cols := strings.TrimSpace(string(colsOut))
			
			var height, width int
			fmt.Sscanf(rows, "%d", &height)
			fmt.Sscanf(cols, "%d", &width)
			
			return width, height
		}
		
		// Final fallback
		return 80, 24
	}
	
	var height, width int
	fmt.Sscanf(string(out), "%d %d", &height, &width)
	return width, height
}

func main() {
	// Check for --new-window flag
	openNewWindow := len(os.Args) > 1 && os.Args[1] == "--new-window"

	if !openNewWindow {
		// Launch in new terminal window with optimal size
		launchInNewTerminal()
		return
	}

	// This is the actual TUI execution in the new terminal
	runTUI()
}

func launchInNewTerminal() {
	// Get the executable path
	executable, err := os.Executable()
	if err != nil {
		fmt.Printf("Error getting executable path: %v\n", err)
		os.Exit(1)
	}

	// Optimal dimensions for IPCrawler TUI (no overlaps, horizontal layout)
	width := 200
	height := 70

	// Try different terminal applications based on OS
	var cmd *exec.Cmd
	
	switch runtime.GOOS {
	case "darwin": // macOS
		// Try Terminal.app first
		cmd = exec.Command("osascript", "-e", fmt.Sprintf(`
tell application "Terminal"
	activate
	set newWindow to do script "%s --new-window"
	tell newWindow's tab 1
		set the size to {%d, %d}
	end tell
end tell`, executable, width, height))
		
		if err := cmd.Run(); err != nil {
			// Fallback to iTerm2
			cmd = exec.Command("osascript", "-e", fmt.Sprintf(`
tell application "iTerm"
	create window with default profile
	tell current session of current window
		write text "%s --new-window"
		set rows to %d
		set columns to %d
	end tell
end tell`, executable, height, width))
		}

	case "linux":
		// Try gnome-terminal first
		cmd = exec.Command("gnome-terminal", 
			"--geometry", fmt.Sprintf("%dx%d", width, height),
			"--title", "IPCrawler TUI",
			"--", executable, "--new-window")
		
		if err := cmd.Start(); err != nil {
			// Fallback to xterm
			cmd = exec.Command("xterm", 
				"-geometry", fmt.Sprintf("%dx%d", width, height),
				"-title", "IPCrawler TUI",
				"-e", executable, "--new-window")
		}

	default:
		// Fallback: run in current terminal
		fmt.Println("Opening TUI in current terminal (new window not supported on this platform)")
		runTUI()
		return
	}

	// Execute the command
	if err := cmd.Run(); err != nil {
		fmt.Printf("Could not open new terminal window: %v\n", err)
		fmt.Println("Falling back to current terminal...")
		runTUI()
	}
}

func runTUI() {
	// Check TTY
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Println("IPCrawler TUI requires a terminal")
		os.Exit(1)
	}

	// Check terminal size and give user feedback
	actualWidth, actualHeight := getTerminalSize()
	if actualWidth < 200 || actualHeight < 70 {
		fmt.Printf("⚠️  Terminal size: %dx%d (detected)\n", actualWidth, actualHeight)
		fmt.Printf("💡 Optimal size: 200x70 for best experience\n")
		fmt.Printf("📖 See RESIZE_GUIDE.md for instructions\n")
		fmt.Printf("\nContinue anyway? (y/N): ")
		
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Run 'make run' after resizing your terminal to 200x70")
			os.Exit(0)
		}
	}

	// Create model with optimal size
	model := newModel()
	
	// Get actual terminal size for responsive design
	termWidth, termHeight := getTerminalSize()
	model.width = termWidth
	model.height = termHeight
	model.updateSizes()
	model.ready = true

	// Run TUI with proper window size handling
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		// Enable resize messages to handle window changes properly
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}