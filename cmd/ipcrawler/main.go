package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"

	"github.com/your-org/ipcrawler/internal/tui/data/loader"
)

// focusState tracks which card is focused
type focusState int

const (
	overviewFocus focusState = iota
	workflowsFocus
	outputFocus
	toolsFocus
	perfFocus
)

type model struct {
	// Layout
	width  int
	height int
	focus  focusState
	ready  bool

	// Components (not separate models, just data)
	workflows *loader.WorkflowData
	
	// Live output
	outputViewport viewport.Model
	outputLogs     []string
	
	// Running tools
	spinner spinner.Model
	tools   []toolExecution
	
	// Performance data
	perfData systemMetrics

	// Calculated card dimensions (fixed to prevent flickering)
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

	// Create viewport for output
	vp := viewport.New(50, 10)
	
	// Build initial output with workflow loading status
	outputLines := []string{
		"[12:34:56] INFO  System initialized",
		"[12:34:57] INFO  Loading workflows from descriptions.yaml",
	}
	
	// Add workflow loading result
	if len(workflows.Workflows) > 0 {
		outputLines = append(outputLines, fmt.Sprintf("[12:34:58] INFO  Loaded %d workflows successfully", len(workflows.Workflows)))
		for name := range workflows.Workflows {
			outputLines = append(outputLines, fmt.Sprintf("[12:34:59] DEBUG Found workflow: %s", name))
		}
	} else {
		outputLines = append(outputLines, "[12:34:58] WARN  No workflows loaded - check workflows/descriptions.yaml")
		if err != nil {
			outputLines = append(outputLines, fmt.Sprintf("[12:34:59] ERROR %s", err.Error()))
		}
	}
	
	outputLines = append(outputLines, []string{
		"[12:35:00] INFO  TUI ready - Use Tab to navigate cards",
		"[12:35:01] INFO  Press 1-5 for direct card focus",
	}...)
	
	vp.SetContent(strings.Join(outputLines, "\n"))

	// Create spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &model{
		workflows:      workflows,
		outputViewport: vp,
		outputLogs:     []string{"System initialized", "Waiting for commands..."},
		spinner:        s,
		tools:          []toolExecution{},
		perfData:       systemMetrics{MemoryMB: 12.5, Goroutines: 5, LastUpdate: "12:34:56"},
		
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
}

func (m *model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "tab":
			m.focus = focusState((int(m.focus) + 1) % 5)
		case "shift+tab":
			m.focus = focusState((int(m.focus) + 4) % 5)
		case "1":
			m.focus = overviewFocus
		case "2":
			m.focus = workflowsFocus
		case "3":
			m.focus = outputFocus
		case "4":
			m.focus = toolsFocus
		case "5":
			m.focus = perfFocus
		default:
			// Handle focused card input
			if m.focus == outputFocus {
				m.outputViewport, cmd = m.outputViewport.Update(msg)
			}
		}

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
	}

	return m, cmd
}

func (m *model) View() string {
	if !m.ready || m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	// Create title
	title := m.titleStyle.Render("IPCrawler TUI - Dynamic Cards Dashboard")
	title = lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(title)

	// Create help
	help := m.dimStyle.Render("Tab/Shift+Tab: cycle focus • 1-5: direct focus • q: quit")
	help = lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(help)

	// Use pre-calculated card sizes (horizontal layout for 190 width)
	// Top Row: Overview | Workflows | Tools | Performance (4 cards horizontal)
	overview := m.renderOverviewCard()
	workflows := m.renderWorkflowsCard()
	tools := m.renderToolsCard()
	perf := m.renderPerfCard()
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, overview, "  ", workflows, "  ", tools, "  ", perf)

	// Bottom Row: Output (full width)
	output := m.renderOutputCard()

	// Combine all
	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		topRow,
		"",
		output,
		"",
		help,
	)

	return content
}

func (m *model) renderOverviewCard() string {
	style := m.cardStyle
	if m.focus == overviewFocus {
		style = m.focusedCardStyle
	}

	// Card header with title
	title := m.titleStyle.Width(m.cardWidth - 2).Render("System Overview")
	
	// Card content
	content := strings.Builder{}
	content.WriteString(fmt.Sprintf("Workflows: %d\n", len(m.workflows.Workflows)))
	content.WriteString(fmt.Sprintf("Tools Available: %d\n", len(m.workflows.GetAllTools())))
	content.WriteString("\nCategories:\n")
	
	for _, workflow := range m.workflows.Workflows {
		line := fmt.Sprintf("• %s (%d tools)", workflow.Name, len(workflow.Tools))
		content.WriteString(line + "\n")
	}

	// Combine title and content
	cardContent := lipgloss.JoinVertical(lipgloss.Left,
		title,
		strings.Repeat("─", m.cardWidth-2),
		content.String(),
	)

	return style.Width(m.cardWidth).Height(m.cardHeight).Render(cardContent)
}

func (m *model) renderWorkflowsCard() string {
	style := m.cardStyle
	if m.focus == workflowsFocus {
		style = m.focusedCardStyle
	}

	// Card header with title
	title := m.titleStyle.Width(m.cardWidth - 2).Render("Workflows Tree")
	
	// Card content
	content := strings.Builder{}
	for _, workflow := range m.workflows.Workflows {
		content.WriteString(fmt.Sprintf("▶ %s\n", workflow.Name))
		for _, tool := range workflow.Tools {
			content.WriteString(fmt.Sprintf("  - %s\n", tool))
		}
		content.WriteString("\n")
		
		// Limit display
		if len(content.String()) > 200 {
			break
		}
	}

	if len(m.workflows.Workflows) == 0 {
		content.WriteString("No workflows found\nCheck workflows/descriptions.yaml")
	}

	// Combine title and content
	cardContent := lipgloss.JoinVertical(lipgloss.Left,
		title,
		strings.Repeat("─", m.cardWidth-2),
		content.String(),
	)

	return style.Width(m.cardWidth).Height(m.cardHeight).Render(cardContent)
}

func (m *model) renderOutputCard() string {
	style := m.cardStyle
	if m.focus == outputFocus {
		style = m.focusedCardStyle
	}

	// Card header with title and scroll info
	titleText := m.titleStyle.Render("Live Output")
	scrollInfo := m.headerStyle.Render(fmt.Sprintf("%.1f%%", m.outputViewport.ScrollPercent()*100))
	header := lipgloss.JoinHorizontal(lipgloss.Left, titleText, 
		strings.Repeat(" ", (m.width-4)-lipgloss.Width(titleText)-lipgloss.Width(scrollInfo)), scrollInfo)
	
	// Card content
	content := m.outputViewport.View()

	// Combine header and content
	cardContent := lipgloss.JoinVertical(lipgloss.Left,
		header,
		strings.Repeat("─", m.width-6),
		content,
	)

	return style.Width(m.width-4).Height(m.cardHeight).Render(cardContent)
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
	// 200 width = 4 cards + 3 spacers (2 chars each) = 4 cards in (200-6) = 194 width
	m.cardWidth = (m.width - 8) / 4  // 4 cards horizontal with spacing
	m.cardHeight = (m.height - 10) / 2 // 2 rows: top cards + output

	// Ensure reasonable minimums for readability
	if m.cardWidth < 35 {
		m.cardWidth = 35
	}
	if m.cardHeight < 12 {
		m.cardHeight = 12
	}

	// Update viewport size for output card (full width, remaining height)
	m.outputViewport.Width = m.width - 8
	m.outputViewport.Height = m.cardHeight - 4
	if m.outputViewport.Height < 8 {
		m.outputViewport.Height = 8
	}
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

	// Create model with optimal fixed size
	model := newModel()
	model.width = 200   // Fixed optimal width (no overlaps)
	model.height = 70   // Fixed optimal height (full visibility)
	model.updateSizes()
	model.ready = true

	// Run TUI with fixed dimensions (no resize handling)
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		// Remove mouse support to reduce flicker
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}