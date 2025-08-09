# IPCrawler TUI Architecture Design

## 1. Go Module Structure

### Dependencies (go.mod)
```go
module github.com/your-org/ipcrawler

go 1.21

require (
    github.com/charmbracelet/bubbletea v0.25.0
    github.com/charmbracelet/bubbles v0.18.0
    github.com/charmbracelet/lipgloss v0.9.1
    github.com/charmbracelet/glamour v0.6.0
    github.com/charmbracelet/log v0.3.1
    gopkg.in/yaml.v3 v3.0.1
)
```

### Directory Structure
```
ipcrawler/
├── cmd/
│   └── ipcrawler/
│       └── main.go              # Minimal entry point
├── internal/
│   ├── ui/
│   │   ├── app.go              # Root Bubble Tea model
│   │   ├── layout.go           # Responsive layout manager
│   │   ├── components/
│   │   │   ├── list_panel.go   # Left panel: workflows/tools
│   │   │   ├── viewport.go     # Center panel: logs/content
│   │   │   └── status_panel.go # Right/footer: status/progress
│   │   ├── styles/
│   │   │   ├── theme.go        # Color and style definitions
│   │   │   └── adaptive.go     # Dark/light theme adaptation
│   │   └── keybindings/
│   │       └── keys.go         # Keyboard navigation
│   ├── term/
│   │   ├── size.go             # Terminal size handling
│   │   └── renderer.go         # Rendering utilities
│   └── simulator/
│       ├── interface.go        # Backend simulation interface
│       ├── mock_data.go        # Demo content generation
│       └── events.go           # Event simulation
├── configs/
│   └── ui.yaml                 # Configuration file
└── docs/
    ├── charmbracelet-research-brief.md
    └── tui-architecture-design.md
```

## 2. Core Architecture Design

### Root Bubble Tea Model (internal/ui/app.go)

```go
package ui

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/charmbracelet/bubbles/key"
)

// App is the root model managing all UI state
type App struct {
    // Terminal state
    ready    bool
    width    int
    height   int
    
    // Layout state
    layout   LayoutMode
    focused  FocusedPanel
    
    // Child components
    listPanel   *ListPanel
    viewport    *ViewportPanel
    statusPanel *StatusPanel
    
    // Configuration
    config *Config
    
    // Simulator interface
    simulator Simulator
    
    // Key bindings
    keys KeyMap
}

type LayoutMode int
const (
    LayoutSmall LayoutMode = iota  // <80 cols: stacked
    LayoutMedium                   // 80-119 cols: two-column
    LayoutLarge                    // ≥120 cols: three-column
)

type FocusedPanel int
const (
    FocusListPanel FocusedPanel = iota
    FocusViewportPanel
    FocusStatusPanel
)

func NewApp(config *Config, simulator Simulator) *App {
    return &App{
        config:      config,
        simulator:   simulator,
        listPanel:   NewListPanel(config, simulator),
        viewport:    NewViewportPanel(config),
        statusPanel: NewStatusPanel(config),
        keys:        DefaultKeyMap,
        focused:     FocusListPanel,
    }
}

func (a *App) Init() tea.Cmd {
    return tea.Batch(
        a.listPanel.Init(),
        a.viewport.Init(),
        a.statusPanel.Init(),
    )
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmds []tea.Cmd
    
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        // Critical: Handle resize first
        a.width, a.height = msg.Width, msg.Height
        a.ready = true
        a.layout = a.calculateLayout()
        
        // Route to child components
        cmds = append(cmds, 
            a.routeToChild(a.listPanel, msg),
            a.routeToChild(a.viewport, msg),
            a.routeToChild(a.statusPanel, msg),
        )
        
    case tea.KeyMsg:
        // Handle global keys first
        switch {
        case key.Matches(msg, a.keys.Quit):
            return a, tea.Quit
        case key.Matches(msg, a.keys.Tab):
            a.cycleFocus()
            return a, nil
        }
        
        // Route to focused component
        cmd := a.routeToFocused(msg)
        if cmd != nil {
            cmds = append(cmds, cmd)
        }
    }
    
    return a, tea.Batch(cmds...)
}

func (a *App) View() string {
    if !a.ready {
        return "Initializing IPCrawler..."
    }
    
    return a.renderLayout()
}

func (a *App) calculateLayout() LayoutMode {
    switch {
    case a.width >= 120:
        return LayoutLarge
    case a.width >= 80:
        return LayoutMedium
    default:
        return LayoutSmall
    }
}
```

### WindowSizeMsg Handling Pattern

```go
// Centralized resize handler
func (a *App) handleResize(width, height int) []tea.Cmd {
    a.width, a.height = width, height
    a.layout = a.calculateLayout()
    
    // Calculate panel dimensions
    leftW, mainW, rightW, contentH := a.calculatePanelSizes()
    
    var cmds []tea.Cmd
    
    // Resize child components
    if a.listPanel != nil {
        cmds = append(cmds, a.listPanel.SetSize(leftW, contentH))
    }
    if a.viewport != nil {
        cmds = append(cmds, a.viewport.SetSize(mainW, contentH))
    }
    if a.statusPanel != nil {
        cmds = append(cmds, a.statusPanel.SetSize(rightW, contentH))
    }
    
    return cmds
}
```

## 3. Responsive Layout Design

### Layout Manager (internal/ui/layout.go)

```go
package ui

import "github.com/charmbracelet/lipgloss"

type LayoutManager struct {
    config *Config
    theme  *Theme
}

func (lm *LayoutManager) RenderLayout(app *App) string {
    switch app.layout {
    case LayoutLarge:
        return lm.renderThreeColumn(app)
    case LayoutMedium:
        return lm.renderTwoColumn(app)
    case LayoutSmall:
        return lm.renderStacked(app)
    default:
        return "Invalid layout"
    }
}

// Three-column layout (≥120 cols)
func (lm *LayoutManager) renderThreeColumn(app *App) string {
    leftW, mainW, rightW, _ := app.calculatePanelSizes()
    
    leftPanel := lm.theme.PanelStyle.
        Width(leftW).
        Height(app.height - 4).
        Render(app.listPanel.View())
    
    mainPanel := lm.theme.PanelStyle.
        Width(mainW).
        Height(app.height - 4).
        Render(app.viewport.View())
    
    rightPanel := lm.theme.PanelStyle.
        Width(rightW).
        Height(app.height - 4).
        Render(app.statusPanel.View())
    
    return lipgloss.JoinHorizontal(
        lipgloss.Top,
        leftPanel,
        mainPanel,
        rightPanel,
    )
}

// Two-column layout (80-119 cols)
func (lm *LayoutManager) renderTwoColumn(app *App) string {
    leftW, mainW, _, contentH := app.calculatePanelSizes()
    
    leftPanel := lm.theme.PanelStyle.
        Width(leftW).
        Height(contentH).
        Render(app.listPanel.View())
    
    mainPanel := lm.theme.PanelStyle.
        Width(mainW).
        Height(contentH).
        Render(app.viewport.View())
    
    topRow := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, mainPanel)
    
    footer := lm.theme.FooterStyle.
        Width(app.width).
        Render(app.statusPanel.View())
    
    return lipgloss.JoinVertical(lipgloss.Left, topRow, footer)
}

// Stacked layout (<80 cols)
func (lm *LayoutManager) renderStacked(app *App) string {
    panelW := app.width - 4
    headerH := 8
    footerH := 4
    mainH := app.height - headerH - footerH - 2
    
    header := lm.theme.PanelStyle.
        Width(panelW).
        Height(headerH).
        Render(app.listPanel.View())
    
    main := lm.theme.PanelStyle.
        Width(panelW).
        Height(mainH).
        Render(app.viewport.View())
    
    footer := lm.theme.FooterStyle.
        Width(panelW).
        Height(footerH).
        Render(app.statusPanel.View())
    
    return lipgloss.JoinVertical(lipgloss.Left, header, main, footer)
}

// Breakpoint calculations
func (a *App) calculatePanelSizes() (leftW, mainW, rightW, contentH int) {
    contentH = a.height - 4 // Reserve space for borders
    
    switch a.layout {
    case LayoutLarge:
        leftW = int(float64(a.width) * 0.25)
        rightW = int(float64(a.width) * 0.20)
        mainW = a.width - leftW - rightW - 6 // Minus borders/margins
        
    case LayoutMedium:
        leftW = int(float64(a.width) * 0.30)
        rightW = 0
        mainW = a.width - leftW - 4
        contentH = a.height - 6 // Reserve footer space
        
    case LayoutSmall:
        leftW = a.width - 4
        mainW = a.width - 4
        rightW = a.width - 4
        contentH = a.height - 12 // Multiple panels stacked
    }
    
    return
}
```

## 4. Component System Design

### List Panel (internal/ui/components/list_panel.go)

```go
package components

import (
    "github.com/charmbracelet/bubbles/list"
    tea "github.com/charmbracelet/bubbletea"
)

type ListPanel struct {
    list      list.Model
    items     []list.Item
    simulator Simulator
    config    *Config
}

type WorkflowItem struct {
    id          string
    title       string
    description string
    status      string
}

func (i WorkflowItem) FilterValue() string { return i.title }
func (i WorkflowItem) Title() string       { return i.title }
func (i WorkflowItem) Description() string { 
    return fmt.Sprintf("%s • %s", i.description, i.status)
}

func NewListPanel(config *Config, sim Simulator) *ListPanel {
    items := sim.GetWorkflows() // Get demo workflows
    
    l := list.New(items, NewWorkflowDelegate(), 0, 0)
    l.Title = "Workflows & Tools"
    l.SetShowStatusBar(false)
    l.SetFilteringEnabled(true)
    
    return &ListPanel{
        list:      l,
        items:     items,
        simulator: sim,
        config:    config,
    }
}

func (lp *ListPanel) Init() tea.Cmd {
    return nil
}

func (lp *ListPanel) Update(msg tea.Msg) (*ListPanel, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        lp.list.SetWidth(msg.Width)
        lp.list.SetHeight(msg.Height)
    case tea.KeyMsg:
        if msg.String() == "enter" {
            if item, ok := lp.list.SelectedItem().(WorkflowItem); ok {
                return lp, lp.simulator.ExecuteWorkflow(item.id)
            }
        }
    }
    
    var cmd tea.Cmd
    lp.list, cmd = lp.list.Update(msg)
    return lp, cmd
}

func (lp *ListPanel) View() string {
    return lp.list.View()
}

func (lp *ListPanel) SetSize(width, height int) tea.Cmd {
    lp.list.SetSize(width, height)
    return nil
}
```

### Viewport Panel (internal/ui/components/viewport.go)

```go
package components

import (
    "github.com/charmbracelet/bubbles/viewport"
    tea "github.com/charmbracelet/bubbletea"
)

type ViewportPanel struct {
    viewport    viewport.Model
    content     []string
    config      *Config
    autoScroll  bool
}

func NewViewportPanel(config *Config) *ViewportPanel {
    vp := viewport.New(0, 0)
    vp.HighPerformanceRendering = true // Critical for performance
    
    return &ViewportPanel{
        viewport:   vp,
        content:    []string{},
        config:     config,
        autoScroll: true,
    }
}

func (vp *ViewportPanel) Init() tea.Cmd {
    return nil
}

func (vp *ViewportPanel) Update(msg tea.Msg) (*ViewportPanel, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        vp.viewport.Width = msg.Width
        vp.viewport.Height = msg.Height
        
    case LogMessage:
        // Append new log content
        vp.content = append(vp.content, msg.Content)
        vp.viewport.SetContent(strings.Join(vp.content, "\n"))
        
        if vp.autoScroll {
            vp.viewport.GotoBottom()
        }
        
    case tea.KeyMsg:
        // Handle scroll keys when focused
        var cmd tea.Cmd
        vp.viewport, cmd = vp.viewport.Update(msg)
        return vp, cmd
    }
    
    return vp, nil
}

func (vp *ViewportPanel) View() string {
    return vp.viewport.View()
}

func (vp *ViewportPanel) SetSize(width, height int) tea.Cmd {
    vp.viewport.Width = width
    vp.viewport.Height = height
    return nil
}

// Custom message types
type LogMessage struct {
    Content string
}
```

### Status Panel (internal/ui/components/status_panel.go)

```go
package components

import (
    "github.com/charmbracelet/bubbles/spinner"
    "github.com/charmbracelet/bubbles/progress"
    tea "github.com/charmbracelet/bubbletea"
)

type StatusPanel struct {
    spinner   spinner.Model
    progress  progress.Model
    status    string
    stats     StatusStats
    config    *Config
}

type StatusStats struct {
    ActiveTasks int
    Completed   int
    Failed      int
    Uptime      time.Duration
}

func NewStatusPanel(config *Config) *StatusPanel {
    s := spinner.New()
    s.Spinner = spinner.Dot
    
    p := progress.New(progress.WithDefaultGradient())
    
    return &StatusPanel{
        spinner:  s,
        progress: p,
        status:   "Ready",
        config:   config,
    }
}

func (sp *StatusPanel) Init() tea.Cmd {
    return sp.spinner.Tick
}

func (sp *StatusPanel) Update(msg tea.Msg) (*StatusPanel, tea.Cmd) {
    switch msg := msg.(type) {
    case StatusUpdate:
        sp.status = msg.Status
        sp.stats = msg.Stats
        
    case ProgressUpdate:
        cmd := sp.progress.SetPercent(msg.Percent)
        return sp, cmd
        
    case tea.WindowSizeMsg:
        sp.progress.Width = msg.Width - 4
        
    default:
        var cmd tea.Cmd
        sp.spinner, cmd = sp.spinner.Update(msg)
        return sp, cmd
    }
    
    return sp, nil
}

func (sp *StatusPanel) View() string {
    return fmt.Sprintf(
        "%s %s\n\n%s\n\nActive: %d • Completed: %d • Failed: %d",
        sp.spinner.View(),
        sp.status,
        sp.progress.View(),
        sp.stats.ActiveTasks,
        sp.stats.Completed,
        sp.stats.Failed,
    )
}

// Custom message types
type StatusUpdate struct {
    Status string
    Stats  StatusStats
}

type ProgressUpdate struct {
    Percent float64
}
```

## 5. Configuration Schema

### UI Configuration (configs/ui.yaml)

```yaml
# IPCrawler TUI Configuration
ui:
  # Layout settings
  layout:
    breakpoints:
      small: 80    # Switch to stacked layout
      medium: 120  # Switch to three-column
    
    # Panel sizing (percentages)
    panels:
      list:
        width_large: 0.25   # 25% in large layout
        width_medium: 0.30  # 30% in medium layout
      main:
        width_large: 0.55   # 55% in large layout  
        width_medium: 0.70  # 70% in medium layout
      status:
        width_large: 0.20   # 20% in large layout
        height_footer: 4    # Footer height in medium/small
  
  # Theme configuration
  theme:
    # Color palette
    colors:
      primary: "#FAFAFA"
      secondary: "#3C3C3C" 
      accent: "#7D56F4"
      success: "#04B575"
      warning: "#F59E0B"
      error: "#EF4444"
      border: "#E5E5E5"
      
    # Adaptive colors for light/dark terminals
    adaptive:
      text_primary:
        light: "236"
        dark: "248"
      border:
        light: "240" 
        dark: "238"
        
  # Component settings
  components:
    list:
      title: "Workflows & Tools"
      show_status_bar: false
      filtering_enabled: true
      item_height: 3
      
    viewport:
      high_performance: true
      auto_scroll: true
      line_numbers: false
      
    status:
      spinner: "dot"
      show_stats: true
      update_interval: 100ms
      
  # Keyboard bindings
  keys:
    quit: ["q", "ctrl+c"]
    tab: ["tab"]
    focus_list: ["1"]
    focus_main: ["2"] 
    focus_status: ["3"]
    
    # List navigation
    list_up: ["k", "up"]
    list_down: ["j", "down"]
    list_select: ["enter", "space"]
    list_filter: ["/"]
    
    # Viewport navigation  
    viewport_up: ["k", "up"]
    viewport_down: ["j", "down"]
    viewport_page_up: ["ctrl+b", "page_up"]
    viewport_page_down: ["ctrl+f", "page_down"]
    viewport_home: ["g"]
    viewport_end: ["G"]
    
  # Performance settings
  performance:
    alt_screen: true
    framerate_cap: 60
    batch_updates: true
    lazy_rendering: true
```

## 6. Simulator Interface Design

### Backend Simulation Interface (internal/simulator/interface.go)

```go
package simulator

import (
    tea "github.com/charmbracelet/bubbletea"
    "time"
)

// Simulator provides demo content without actual tool execution
type Simulator interface {
    // Workflow management
    GetWorkflows() []WorkflowItem
    ExecuteWorkflow(id string) tea.Cmd
    GetWorkflowStatus(id string) WorkflowStatus
    
    // Tool management  
    GetTools() []ToolItem
    ExecuteTool(id string, args map[string]interface{}) tea.Cmd
    
    // Log streaming
    GetLogs() []LogEntry
    StreamLogs() tea.Cmd
    
    // Status information
    GetSystemStatus() SystemStatus
    GetMetrics() Metrics
}

type WorkflowItem struct {
    ID          string            `json:"id"`
    Title       string            `json:"title"`
    Description string            `json:"description"`
    Status      string            `json:"status"`
    Tools       []string          `json:"tools"`
    Config      map[string]interface{} `json:"config"`
}

type ToolItem struct {
    ID          string            `json:"id"`
    Name        string            `json:"name"`
    Description string            `json:"description"`
    Category    string            `json:"category"`
    Status      string            `json:"status"`
    Config      map[string]interface{} `json:"config"`
}

type LogEntry struct {
    Timestamp time.Time `json:"timestamp"`
    Level     string    `json:"level"`
    Source    string    `json:"source"`
    Message   string    `json:"message"`
}

type SystemStatus struct {
    Status      string    `json:"status"`
    Uptime      time.Duration `json:"uptime"`
    ActiveTasks int       `json:"active_tasks"`
    Completed   int       `json:"completed"`
    Failed      int       `json:"failed"`
}

type Metrics struct {
    CPU    float64 `json:"cpu"`
    Memory float64 `json:"memory"`
    Tasks  int     `json:"tasks"`
}
```

### Mock Data Generator (internal/simulator/mock_data.go)

```go
package simulator

import (
    "fmt"
    "math/rand"
    "time"
    tea "github.com/charmbracelet/bubbletea"
)

type MockSimulator struct {
    workflows []WorkflowItem
    tools     []ToolItem
    logs      []LogEntry
    status    SystemStatus
    startTime time.Time
}

func NewMockSimulator() *MockSimulator {
    return &MockSimulator{
        workflows: generateMockWorkflows(),
        tools:     generateMockTools(),
        logs:      generateMockLogs(),
        startTime: time.Now(),
        status: SystemStatus{
            Status:      "Running",
            ActiveTasks: 2,
            Completed:   15,
            Failed:      1,
        },
    }
}

func generateMockWorkflows() []WorkflowItem {
    return []WorkflowItem{
        {
            ID:          "port-scan-basic",
            Title:       "Basic Port Scan",
            Description: "Standard TCP port enumeration",
            Status:      "Ready",
            Tools:       []string{"nmap", "masscan"},
        },
        {
            ID:          "subdomain-discovery", 
            Title:       "Subdomain Discovery",
            Description: "Comprehensive subdomain enumeration",
            Status:      "Running",
            Tools:       []string{"subfinder", "amass", "dnsgen"},
        },
        {
            ID:          "web-app-scan",
            Title:       "Web Application Scan",
            Description: "Automated web app security testing",
            Status:      "Pending",
            Tools:       []string{"gobuster", "feroxbuster", "nuclei"},
        },
    }
}

func generateMockTools() []ToolItem {
    return []ToolItem{
        {
            ID:          "nmap",
            Name:        "Nmap",
            Description: "Network exploration and port scanner",
            Category:    "Port Scanning",
            Status:      "Available",
        },
        {
            ID:          "subfinder",
            Name:        "Subfinder", 
            Description: "Passive subdomain discovery",
            Category:    "Reconnaissance",
            Status:      "Running",
        },
    }
}

func (ms *MockSimulator) ExecuteWorkflow(id string) tea.Cmd {
    return tea.Tick(time.Millisecond*100, func(time.Time) tea.Msg {
        return WorkflowExecutionMsg{
            WorkflowID: id,
            Status:     "started",
            Message:    fmt.Sprintf("Starting workflow: %s", id),
        }
    })
}

func (ms *MockSimulator) StreamLogs() tea.Cmd {
    return tea.Tick(time.Millisecond*500, func(time.Time) tea.Msg {
        messages := []string{
            "Initializing port scan on 192.168.1.0/24",
            "Found open port 22/tcp on 192.168.1.100",
            "Found open port 80/tcp on 192.168.1.100", 
            "Found open port 443/tcp on 192.168.1.100",
            "Subdomain discovery completed: 15 subdomains found",
            "Running nuclei security checks...",
        }
        
        return LogMessage{
            Content: fmt.Sprintf("[%s] %s", 
                time.Now().Format("15:04:05"),
                messages[rand.Intn(len(messages))],
            ),
        }
    })
}
```

## 7. Performance Architecture

### High-Performance Patterns

```go
// Alt-screen usage (cmd/ipcrawler/main.go)
func main() {
    config, err := LoadConfig("configs/ui.yaml")
    if err != nil {
        log.Fatal(err)
    }
    
    simulator := simulator.NewMockSimulator()
    app := ui.NewApp(config, simulator)
    
    // Single tea.Program with alt-screen
    program := tea.NewProgram(
        app,
        tea.WithAltScreen(),           // Enable alt-screen
        tea.WithMouseCellMotion(),     // Enhanced mouse support
    )
    
    if _, err := program.Run(); err != nil {
        fmt.Printf("Error: %v\n", err)
        os.Exit(1)
    }
}

// Stable line count pattern (viewport rendering)
func (vp *ViewportPanel) renderContent() string {
    lines := make([]string, vp.viewport.Height)
    
    // Fill available space to maintain stable line count
    for i := 0; i < vp.viewport.Height; i++ {
        if i < len(vp.content) {
            lines[i] = vp.content[i]
        } else {
            lines[i] = "" // Empty lines maintain height
        }
    }
    
    return strings.Join(lines, "\n")
}

// Batch update pattern
func (a *App) handleBatchUpdate(updates []tea.Msg) tea.Cmd {
    var cmds []tea.Cmd
    
    for _, msg := range updates {
        cmd := a.routeMessage(msg)
        if cmd != nil {
            cmds = append(cmds, cmd)
        }
    }
    
    return tea.Batch(cmds...)
}
```

### Memory Optimization

```go
// Circular buffer for logs
type CircularBuffer struct {
    buffer []string
    size   int
    head   int
    tail   int
    full   bool
}

func (cb *CircularBuffer) Add(item string) {
    cb.buffer[cb.head] = item
    cb.head = (cb.head + 1) % cb.size
    
    if cb.full {
        cb.tail = (cb.tail + 1) % cb.size
    }
    
    cb.full = cb.head == cb.tail
}

// Lazy rendering for off-screen content
func (lp *ListPanel) View() string {
    if !lp.visible {
        return "" // Skip rendering if not visible
    }
    return lp.list.View()
}
```

## 8. Implementation Roadmap

### Phase 1: Foundation (Week 1)
1. Set up Go module with dependencies
2. Implement basic app structure and WindowSizeMsg handling
3. Create configuration loading system
4. Build mock simulator with demo data

### Phase 2: Core Components (Week 2)  
1. Implement list panel with bubbles list component
2. Build viewport panel with high-performance rendering
3. Create status panel with spinner and progress
4. Add keyboard navigation and focus management

### Phase 3: Responsive Layout (Week 3)
1. Implement breakpoint-based layout calculation
2. Build three layout modes (small/medium/large)
3. Add smooth resize handling
4. Test across different terminal sizes

### Phase 4: Polish & Optimization (Week 4)
1. Add theme and styling system
2. Implement performance optimizations
3. Add error handling and recovery
4. Create comprehensive documentation

### Critical Success Factors
- **Single tea.Program**: Never create multiple renderers
- **WindowSizeMsg First**: Always wait for dimensions before rendering
- **Stable Line Count**: Update content in-place, never change height
- **Config-Driven**: All values from ui.yaml, no hardcoded constants
- **Performance Focus**: Use alt-screen, high-performance viewport, lazy rendering

This architecture provides a solid foundation for a responsive, performant TUI that meets all the specified constraints while following Charmbracelet best practices.