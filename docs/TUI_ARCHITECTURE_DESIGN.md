# IPCrawler TUI Architecture Design

## 1. System Architecture Overview

### 1.1 Core Architecture Principles

**Single Program Runtime**: Exactly one `tea.NewProgram` instance manages the entire TUI lifecycle.

**Component Hierarchy**:
```
tea.Program
└── AppModel (root tea.Model)
    ├── Layout (responsive calculations)
    ├── Theme (configuration-driven styling)
    ├── Components (Bubbles components)
    │   ├── WorkflowList (left panel)
    │   ├── ToolTable (center-top panel)
    │   ├── LogViewport (center-bottom panel)
    │   ├── StatusPanel (right panel)
    │   ├── ProgressBar (status indicators)
    │   ├── Spinner (loading states)
    │   └── Help (markdown help)
    └── EventBus (live data integration)
```

### 1.2 Message Flow Architecture

**Bubble Tea Message Types**:
```go
// Core Bubble Tea messages
type tea.WindowSizeMsg    // Terminal resize events
type tea.KeyMsg          // Keyboard input
type tea.MouseMsg        // Mouse events (if enabled)

// Custom application messages
type WorkflowUpdateMsg   // Workflow status changes
type ToolExecutionMsg    // Tool execution events  
type LogMsg             // Log entries
type SystemStatsMsg     // Performance metrics
type ErrorMsg           // Error notifications
type DataRefreshMsg     // Manual refresh requests
```

**Event Routing Strategy**:
```go
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // 1. Handle global messages (resize, quit)
    // 2. Route to focused component
    // 3. Update dependent components
    // 4. Return batch commands
}
```

## 2. Responsive Layout System

### 2.1 Layout Modes and Breakpoints

**Three Layout Modes**:

```go
type LayoutMode int
const (
    LargeLayout  LayoutMode = iota  // ≥120 cols: [Nav|Main|Status]
    MediumLayout                    // 80-119 cols: [Nav|Main+Status]  
    SmallLayout                     // 40-79 cols: Single panel + tabs
)
```

**Responsive Calculations**:
```go
type LayoutCalculator struct {
    terminalWidth  int
    terminalHeight int
    mode          LayoutMode
    config        *UIConfig  // From configs/ui.yaml
}

func (l *LayoutCalculator) CalculateComponentSizes() ComponentSizes {
    contentWidth := l.terminalWidth - l.borderAndPaddingWidth()
    contentHeight := l.terminalHeight - l.headerAndFooterHeight()
    
    switch l.mode {
    case LargeLayout:
        return l.calculateThreePanelLayout(contentWidth, contentHeight)
    case MediumLayout:
        return l.calculateTwoPanelLayout(contentWidth, contentHeight)
    case SmallLayout:
        return l.calculateSinglePanelLayout(contentWidth, contentHeight)
    }
}
```

### 2.2 Panel Layout Strategy

**Large Layout (≥120 columns)**:
```
┌─────────────────────────────────────────────────────────────┐
│ IPCrawler TUI - Target: example.com             [spinner] │
├──────────────┬─────────────────────────┬──────────────────┤
│ Workflows    │ Tool Executions         │ Status           │
│              │                         │                  │
│ ○ dns_lookup │ ┌─Tool─┬─Status─┬─Dur─┐  │ Running: 2       │
│ ● port_scan  │ │naabu │running │30s │  │ Done: 1          │
│ ○ vhost_disc │ │dig   │done    │5s  │  │ Failed: 0        │
│              │ └─────┴────────┴────┘  │                  │
│              │                         │ Memory: 45%      │
│              ├─────────────────────────┤ CPU: 12%         │
│              │ Output & Logs           │                  │
│              │                         │ Focus: main      │
│              │ [15:30:42] INFO: Start  │                  │
│              │ [15:30:43] Tool: naabu  │                  │
│              │ [15:30:44] Progress 50% │                  │
└──────────────┴─────────────────────────┴──────────────────┘
```

**Medium Layout (80-119 columns)**:
```
┌───────────────────────────────────────────────────────────┐
│ IPCrawler TUI - Target: example.com         [spinner]    │
├──────────────┬────────────────────────────────────────────┤
│ Workflows    │ Tool Executions                            │
│              │ ┌─Tool─┬─Status─┬─Duration─┐                │
│ ○ dns_lookup │ │naabu │running │30s      │                │
│ ● port_scan  │ │dig   │done    │5s       │                │
│ ○ vhost_disc │ └─────┴────────┴─────────┘                │
│              ├────────────────────────────────────────────┤
│              │ Output & Logs                              │
│              │ [15:30:42] INFO: Starting workflows...     │
│              │ [15:30:43] Tool: naabu executing           │
│              ├────────────────────────────────────────────┤
│              │ Status: 2 running, 1 done | Memory: 45%   │
└──────────────┴────────────────────────────────────────────┘
```

**Small Layout (40-79 columns)**:
```
┌─────────────────────────────────────────┐
│ IPCrawler TUI - example.com  [spinner] │
├─────────────────────────────────────────┤
│ Current View: Workflows                 │
│                                         │
│ ○ dns_lookup                            │
│ ● port_scan                             │
│ ○ vhost_discovery                       │
│                                         │
│                                         │
│                                         │
│                                         │
├─────────────────────────────────────────┤
│ [workflows] table logs                  │
│ Tab: switch • ?: help • q: quit         │
└─────────────────────────────────────────┘
```

### 2.3 Overlap Prevention Strategy

**Safe Rendering Pattern**:
```go
func (m AppModel) renderPanel(content string, width, height int) string {
    // Ensure content never exceeds allocated space
    style := lipgloss.NewStyle().
        Width(width).
        Height(height).
        MaxWidth(width).
        MaxHeight(height)
    
    return style.Render(content)
}

func (m AppModel) joinPanels(panels ...string, widths ...int) string {
    var styledPanels []string
    for i, panel := range panels {
        styledPanels = append(styledPanels, 
            lipgloss.NewStyle().Width(widths[i]).Render(panel))
    }
    return lipgloss.JoinHorizontal(lipgloss.Top, styledPanels...)
}
```

## 3. Component Design

### 3.1 Workflow List Component (Left Panel)

**Responsibilities**:
- Display workflow status with icons
- Support filtering/search
- Focus indication and keyboard navigation
- Progress bars for running workflows

**Bubbles Integration**:
```go
type WorkflowListItem struct {
    ID          string
    Name        string
    Status      WorkflowStatus
    Progress    float64
    Description string
}

func (w WorkflowListItem) FilterValue() string { return w.Name }
func (w WorkflowListItem) Title() string { return w.Name }
func (w WorkflowListItem) Description() string { 
    icon := getStatusIcon(w.Status)
    if w.Status == "running" && w.Progress > 0 {
        return fmt.Sprintf("%s %s %.0f%%", icon, w.Description, w.Progress*100)
    }
    return fmt.Sprintf("%s %s", icon, w.Description)
}
```

### 3.2 Tool Execution Table (Center-Top Panel)

**Responsibilities**:
- Display recent tool executions
- Show real-time status and duration
- Sortable columns with auto-resize
- Focus state management

**Bubbles Integration**:
```go
func (m *AppModel) initializeToolTable() {
    columns := []table.Column{
        {Title: "Tool", Width: 0},     // Auto-calculated
        {Title: "Workflow", Width: 0}, // Auto-calculated
        {Title: "Status", Width: 10},  // Fixed
        {Title: "Duration", Width: 12}, // Fixed
        {Title: "Output", Width: 0},   // Remaining space
    }
    
    m.toolTable = table.New(
        table.WithColumns(columns),
        table.WithFocused(false),
        table.WithHeight(m.tableHeight),
    )
}
```

### 3.3 Log Viewport (Center-Bottom Panel)

**Responsibilities**:
- Stream live logs with auto-scroll
- Support manual scrolling and search
- Structured log formatting
- Circular buffer management

**Bubbles Integration**:
```go
func (m *AppModel) updateLogViewport() {
    var logLines []string
    for _, log := range m.recentLogs(100) {
        timestamp := log.Timestamp.Format("15:04:05")
        level := strings.ToUpper(log.Level)
        line := fmt.Sprintf("[%s] %s: %s", timestamp, level, log.Message)
        logLines = append(logLines, line)
    }
    
    content := strings.Join(logLines, "\n")
    m.logViewport.SetContent(content)
    
    if m.autoScroll {
        m.logViewport.GotoBottom()
    }
}
```

### 3.4 Status Panel (Right Panel)

**Custom Component** (not using Bubbles):
```go
func (m AppModel) renderStatusContent() string {
    var lines []string
    
    // Workflow summary
    running, completed, failed := m.getWorkflowCounts()
    lines = append(lines, 
        fmt.Sprintf("Running: %d", running),
        fmt.Sprintf("Completed: %d", completed),
        fmt.Sprintf("Failed: %d", failed),
        "",
    )
    
    // System metrics
    if m.systemStats.Valid {
        lines = append(lines,
            "System:",
            fmt.Sprintf("CPU: %.1f%%", m.systemStats.CPUUsage),
            fmt.Sprintf("Memory: %.1f%%", m.systemStats.MemoryUsage),
            "",
        )
    }
    
    // Current focus
    lines = append(lines, fmt.Sprintf("Focus: %s", m.focused.String()))
    
    return strings.Join(lines, "\n")
}
```

## 4. State Management

### 4.1 Application State Model

```go
type AppModel struct {
    // Layout and rendering state
    ready         bool                 // WindowSizeMsg received guard
    terminalSize  tea.WindowSizeMsg   // Current terminal dimensions
    layout        *Layout             // Responsive layout calculator
    theme         *Theme              // Configuration-driven styling
    
    // Component state  
    components    ComponentState      // All Bubbles components
    focused       FocusedComponent    // Current focus target
    currentView   string              // Small layout view switcher
    
    // Data state
    workflows     []WorkflowStatus    // Workflow execution state
    tools         []ToolExecution     // Tool execution history  
    logs          []LogEntry          // Application logs
    systemStats   SystemStats         // Performance metrics
    
    // Configuration
    config        *UIConfig           // From configs/ui.yaml
    keyMap        KeyMap              // Keyboard shortcuts
    
    // Event handling
    eventBus      EventBus            // Live data integration
    cancelFunc    context.CancelFunc  // Graceful shutdown
}
```

### 4.2 Focus Management

**Focus States**:
```go
type FocusedComponent int
const (
    FocusWorkflowList FocusedComponent = iota
    FocusToolTable
    FocusLogViewport
    FocusStatusPanel
    FocusHelp
)

func (m *AppModel) cycleFocus() {
    switch m.layout.Mode() {
    case LargeLayout:
        // All components available
        m.cycleFocusLarge()
    case MediumLayout:
        // Skip status panel (integrated with main)
        m.cycleFocusMedium()
    case SmallLayout:
        // Tab-based view switching
        m.switchView()
    }
}
```

## 5. Event Integration

### 5.1 Live Data Integration

**Event Bus Pattern**:
```go
type EventBus interface {
    Subscribe(eventType string, handler func(tea.Msg)) error
    Publish(eventType string, data interface{}) error
    Close() error
}

func (m *AppModel) initializeEventHandlers() {
    m.eventBus.Subscribe("workflow.status", func(data tea.Msg) {
        if msg, ok := data.(WorkflowUpdateMsg); ok {
            // Update workflow state and trigger UI refresh
            m.handleWorkflowUpdate(msg)
        }
    })
    
    m.eventBus.Subscribe("tool.execution", func(data tea.Msg) {
        if msg, ok := data.(ToolExecutionMsg); ok {
            m.handleToolExecution(msg)
        }
    })
}
```

### 5.2 Status Line In-Place Updates

**Critical Requirement**: Status updates must not increase line count.

```go
func (m AppModel) renderStatusLine() string {
    // Fixed template prevents line growth
    template := "Workflows: (%d/%d) %s | Tools: %d active | Focus: %s"
    
    running, total := m.getWorkflowCounts()
    spinner := ""
    if running > 0 {
        spinner = m.spinner.View()
    } else {
        spinner = "✓"
    }
    activeTools := m.getActiveToolCount()
    
    status := fmt.Sprintf(template, running, total, spinner, activeTools, m.focused.String())
    
    // Ensure consistent width to prevent layout shifts
    return lipgloss.NewStyle().
        Width(m.terminalSize.Width).
        Render(status)
}
```

## 6. Keyboard Interaction Design

### 6.1 Global Shortcuts

```go
type KeyMap struct {
    // Global actions (always available)
    Quit      key.Binding  // q, ctrl+c
    Help      key.Binding  // ?
    Refresh   key.Binding  // ctrl+r
    
    // Navigation (context-aware)
    Up        key.Binding  // up, k
    Down      key.Binding  // down, j
    Left      key.Binding  // left, h
    Right     key.Binding  // right, l
    
    // Panel management
    NextPanel key.Binding  // tab
    PrevPanel key.Binding  // shift+tab
    
    // Direct focus (large layout only)
    FocusNav   key.Binding  // 1
    FocusMain  key.Binding  // 2  
    FocusStatus key.Binding  // 3
    
    // View switching (small layout only)
    NextView   key.Binding  // space
    PrevView   key.Binding  // shift+space
}
```

### 6.2 Context-Sensitive Help

**Help Content Structure**:
```go
func (m AppModel) getContextualHelp() help.KeyMap {
    switch m.focused {
    case FocusWorkflowList:
        return help.KeyMap{
            "navigate": "↑↓ or jk",
            "filter":   "/ to search",
            "select":   "enter",
        }
    case FocusToolTable:
        return help.KeyMap{
            "navigate": "↑↓ for rows",
            "sort":     "enter to sort column", 
            "scroll":   "page up/down",
        }
    case FocusLogViewport:
        return help.KeyMap{
            "scroll":     "↑↓ or jk",
            "page":       "page up/down",
            "top/bottom": "g/G",
        }
    }
}
```

## 7. Error Handling and Resilience

### 7.1 Graceful Degradation

**Terminal Size Constraints**:
```go
func (m AppModel) View() string {
    if !m.ready {
        return "Initializing IPCrawler TUI..."
    }
    
    // Handle insufficient terminal size
    if m.terminalSize.Width < 40 {
        return m.renderTooSmallMessage()
    }
    
    // Render appropriate layout
    return m.renderForCurrentLayout()
}

func (m AppModel) renderTooSmallMessage() string {
    return lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("red")).
        Render(fmt.Sprintf(
            "Terminal too small: %dx%d\nMinimum required: 40x10",
            m.terminalSize.Width, m.terminalSize.Height))
}
```

### 7.2 Non-TTY Fallback

**Automatic Mode Detection**:
```go
func RunTUI(target string) error {
    termInfo := term.GetTTYInfo()
    
    if !termInfo.IsTTY || termInfo.Width < 40 || termInfo.Height < 10 {
        return runPlainTextMode(target)
    }
    
    // Run full TUI
    appModel := NewAppModel(target)
    program := tea.NewProgram(
        appModel,
        tea.WithAltScreen(),
        tea.WithMouseCellMotion(),
    )
    
    _, err := program.Run()
    return err
}
```

## 8. Testing Strategy

### 8.1 Golden Frame Testing

**Test Structure**:
```go
func TestLayoutRenderingNoLineGrowth(t *testing.T) {
    testSizes := []tea.WindowSizeMsg{
        {Width: 80, Height: 24},
        {Width: 100, Height: 30},
        {Width: 120, Height: 40},
        {Width: 160, Height: 48},
    }
    
    for _, size := range testSizes {
        t.Run(fmt.Sprintf("%dx%d", size.Width, size.Height), func(t *testing.T) {
            model := NewAppModel("test.com")
            model, _ = model.Update(size)
            
            // Render multiple times with updates
            view1 := model.View()
            lines1 := strings.Count(view1, "\n")
            
            // Simulate workflow updates
            model = simulateWorkflowUpdate(model)
            view2 := model.View()
            lines2 := strings.Count(view2, "\n")
            
            // Verify line count stability
            assert.Equal(t, lines1, lines2, "Line count must remain stable")
            
            // Verify no content overlap
            assertNoOverlap(t, view2, size)
        })
    }
}
```

### 8.2 Static Analysis

**Build-Time Verification**:
```bash
# Verify single tea.Program instantiation
grep -r "tea.NewProgram" --include="*.go" . | wc -l  # Must equal 1

# Verify no hardcoded dimensions  
grep -r "[0-9]\+x[0-9]\+" --include="*.go" internal/ui/ | wc -l  # Must equal 0

# Verify configuration sourcing
grep -r "configs/ui.yaml" --include="*.go" . | wc -l  # Must be > 0
```

## 9. Performance Optimization

### 9.1 Render Optimization

**Component-Level Updates**:
```go
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case WorkflowUpdateMsg:
        // Only update affected components
        m.updateWorkflowList()
        m.updateStatusPanel()
        // Don't trigger full re-render of unaffected components
        return m, nil
    }
}
```

### 9.2 Memory Management

**Circular Buffers**:
```go
func (m *AppModel) addLogEntry(entry LogEntry) {
    m.logs = append(m.logs, entry)
    
    // Maintain circular buffer
    if len(m.logs) > m.config.MaxLogEntries {
        m.logs = m.logs[1:]
    }
}
```

## 10. Directory Structure

```
/internal/ui/
├── theme/
│   └── theme.go              # Configuration-driven styling
├── layout/  
│   └── layout.go             # Responsive layout calculations
├── components/
│   ├── workflow_list.go      # Workflow navigation component
│   ├── tool_table.go         # Tool execution table component
│   ├── log_viewport.go       # Log streaming component
│   ├── status_panel.go       # Status information component
│   ├── progress.go           # Progress indicators
│   ├── spinner.go            # Loading animations
│   └── help.go               # Help system component
├── screens/
│   ├── overview.go           # Main TUI screen
│   ├── details.go            # Detailed view screen
│   └── summary.go            # Summary report screen
├── model/
│   ├── app.go                # Root tea.Model implementation
│   ├── messages.go           # Custom message types
│   ├── types.go              # State type definitions
│   └── events.go             # Event bus integration
└── ui.go                     # Public interface

/internal/term/
├── term.go                   # TTY detection and capabilities
├── term_unix.go              # Unix-specific terminal handling
└── fallback.go               # Non-TTY fallback behavior

/configs/
└── ui.yaml                   # Complete UI configuration
```

## 11. Implementation Priority

**Phase 1: Foundation**
1. Core tea.Model with WindowSizeMsg handling
2. Layout system with responsive calculations  
3. Theme system loading from configs/ui.yaml

**Phase 2: Components**
4. Bubbles component integration
5. Focus management and keyboard handling
6. Help system with Glamour

**Phase 3: Integration**
7. Event bus connection to existing workflow system
8. Live data streaming and updates
9. Non-TTY fallback implementation

**Phase 4: Quality Assurance**
10. Golden frame testing
11. Performance optimization
12. Accessibility compliance verification

---

**Architecture Version**: 1.0  
**Target Implementation**: Single tea.Program with responsive Charmbracelet ecosystem  
**Compliance**: All hard constraints addressed with implementation patterns