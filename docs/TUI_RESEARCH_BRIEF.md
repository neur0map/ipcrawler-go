# IPCrawler TUI Research Brief: Charmbracelet Ecosystem Architecture

## Executive Summary

This brief outlines the architectural decisions for rebuilding the IPCrawler TUI exclusively using the Charmbracelet ecosystem components. Based on comprehensive analysis of official documentation and terminal UI best practices, this document provides the technical foundation for implementing a responsive, accessible, and performant terminal interface.

## 1. Charmbracelet Ecosystem Component Roles

### 1.1 Bubble Tea - The Runtime Foundation

**Role**: Bubble Tea serves as the core runtime and state management system following The Elm Architecture pattern.

**Key Responsibilities**:
- **Single Source of Truth**: One `tea.Program` instance managing the entire application lifecycle
- **Message-Driven Updates**: All state changes flow through the `Update(tea.Msg) (tea.Model, tea.Cmd)` method
- **WindowSizeMsg Handling**: Critical for responsive layout - dimensions received via `tea.WindowSizeMsg`
- **Event Loop Management**: Handles keyboard input, terminal resizing, and command execution

**Core Pattern**:
```go
type Model struct {
    ready     bool              // Guard against rendering before WindowSizeMsg
    width     int
    height    int
    // ... other state
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.ready = true
        return m.handleResize(msg)
    // ... other messages
    }
}
```

### 1.2 Bubbles - Interactive Components

**Role**: Bubbles provides pre-built, composable UI components that integrate seamlessly with Bubble Tea.

**Selected Components for IPCrawler**:
- **List**: Workflow navigation (left panel) with filtering and selection
- **Table**: Tool execution display (center panel) with sortable columns  
- **Viewport**: Live logs and output streams (center-bottom panel) with scrolling
- **Progress**: Workflow execution progress indicators
- **Spinner**: Loading states for active operations
- **Key**: Keyboard shortcut management and help system

**Integration Pattern**:
```go
type ComponentState struct {
    workflowList   list.Model
    toolTable     table.Model
    outputViewport viewport.Model
    progressBar   progress.Model
    spinner       spinner.Model
}

// Each component updates independently but coordinates through main model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Route messages to appropriate components based on focus
    switch m.focused {
    case FocusWorkflowList:
        m.components.workflowList, cmd = m.components.workflowList.Update(msg)
    // ... other routing
    }
}
```

### 1.3 Lipgloss - Layout and Styling Engine

**Role**: Lipgloss handles all layout calculations, styling, and responsive behavior through its powerful joining and placement functions.

**Core Layout Functions**:
- **JoinHorizontal/JoinVertical**: Primary layout composition with alignment control
- **Place/PlaceHorizontal/PlaceVertical**: Precise positioning within defined spaces
- **Width/Height/MaxWidth/MaxHeight**: Responsive sizing constraints
- **Style Inheritance**: Consistent theming across components

**Responsive Layout Strategy**:
```go
// Three-panel layout for large screens (≥120 cols)
content := lipgloss.JoinHorizontal(lipgloss.Top,
    lipgloss.NewStyle().Width(navWidth).Render(navPanel),
    lipgloss.NewStyle().Width(mainWidth).Render(mainPanel), 
    lipgloss.NewStyle().Width(statusWidth).Render(statusPanel))

// Two-panel layout for medium screens (80-119 cols)
content := lipgloss.JoinHorizontal(lipgloss.Top,
    lipgloss.NewStyle().Width(navWidth).Render(navPanel),
    lipgloss.NewStyle().Width(combinedWidth).Render(
        lipgloss.JoinVertical(lipgloss.Left, mainPanel, statusPanel)))

// Single-panel layout for small screens (40-79 cols)
content := renderCurrentView() // Tab-based switching
```

### 1.4 Glamour - Help System

**Role**: Glamour renders markdown help documentation with terminal-appropriate styling.

**Usage**: Help panel with keyboard shortcuts, workflow explanations, and system status.

### 1.5 Log + Termenv - Output and Color Management

**Role**: Structured logging with automatic color profile detection and graceful non-TTY fallback.

**Integration**: 
- **TTY Detection**: Automatic switching between TUI and plain-text modes
- **Color Adaptation**: Respects user terminal capabilities (16/256/truecolor)
- **Accessibility**: No-color support for assistive technologies

## 2. Initial Render vs WindowSizeMsg Strategy

### 2.1 The Bootstrap Problem

**Challenge**: Bubble Tea applications must handle the "initial render guard" problem where the first `View()` call occurs before receiving `tea.WindowSizeMsg`, leading to mis-sized layouts.

**Solution - Graceful Bootstrap Sequence**:

```go
func (m Model) View() string {
    // Guard against premature rendering
    if !m.ready || m.width == 0 || m.height == 0 {
        return "Initializing IPCrawler TUI..."
    }
    
    // Check minimum size requirements
    if m.width < 40 || m.height < 10 {
        return m.renderTooSmallMessage()
    }
    
    // Safe to render full layout
    return m.renderMainContent()
}
```

### 2.2 WindowSizeMsg Handling Strategy

**Approach**: Treat every `WindowSizeMsg` as a potential layout mode change requiring component recalculation.

```go
func (m Model) handleResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
    oldMode := m.layout.GetMode()
    
    // Update dimensions and recalculate layout
    m.width = msg.Width
    m.height = msg.Height
    m.layout.Update(msg.Width, msg.Height)
    
    // Resize all components
    m.updateComponentSizes()
    
    // Handle layout mode transitions
    newMode := m.layout.GetMode()
    if oldMode != newMode {
        return m.handleLayoutModeChange(oldMode, newMode)
    }
    
    return m, nil
}
```

### 2.3 In-Place Updates Strategy

**Critical Requirement**: Status lines must update without adding new lines to prevent terminal flicker.

**Implementation**:
```go
// Status line maintains consistent structure
statusTemplate := "Workflows: (%d/%d) %s | Tools: %d active | Logs: %d entries"
status := fmt.Sprintf(statusTemplate, running, total, stateIcon, activeTools, logCount)

// Use fixed-width rendering to prevent line growth
return lipgloss.NewStyle().Width(m.width).Render(status)
```

## 3. Responsive Layout Rules

### 3.1 Breakpoint-Based Layout System

**Breakpoints** (from configs/ui.yaml):
- **Large**: ≥120 columns → Three-panel layout `[Nav 25%] [Main 50%] [Status 25%]`
- **Medium**: 80-119 columns → Two-panel layout `[Nav 30%] [Main+Status 70%]`  
- **Small**: 40-79 columns → Single-panel layout with tab switching
- **Minimum**: <40 columns → Error message (unusable)

### 3.2 Overlap Prevention Strategy

**Core Principle**: Use Lipgloss calculated widths with explicit constraints to prevent content overflow.

```go
type ComponentDimensions struct {
    Width  int
    Height int
    MaxWidth int
    MaxHeight int
}

func (l *Layout) CalculateComponentSizes() map[string]ComponentDimensions {
    available := l.width - l.totalBorderWidth() - l.totalPadding()
    
    switch l.mode {
    case LargeLayout:
        navWidth := max(20, int(float64(available) * 0.25))
        statusWidth := max(15, int(float64(available) * 0.25))
        mainWidth := available - navWidth - statusWidth
        // Ensure no component exceeds available space
        return map[string]ComponentDimensions{
            "nav":    {Width: navWidth, Height: l.contentHeight(), MaxWidth: navWidth},
            "main":   {Width: mainWidth, Height: l.contentHeight(), MaxWidth: mainWidth},
            "status": {Width: statusWidth, Height: l.contentHeight(), MaxWidth: statusWidth},
        }
    }
}
```

### 3.3 Content Wrapping and Truncation

**Text Handling**:
- **Long Lines**: Use Lipgloss `MaxWidth()` with `...` truncation indicators
- **Table Columns**: Dynamic width adjustment based on available space
- **Lists**: Vertical scrolling with ellipsis for overlong items

**Implementation**:
```go
// Safe text rendering with overflow protection
func (m Model) renderSafeText(text string, maxWidth int) string {
    return lipgloss.NewStyle().
        MaxWidth(maxWidth).
        Render(text)
}
```

## 4. Technical Implementation Standards

### 4.1 Configuration-Driven Design

All UI parameters sourced from `/configs/ui.yaml`:
- Layout breakpoints and ratios
- Color schemes and accessibility settings  
- Keyboard shortcuts and interaction patterns
- Component behavior and limits

### 4.2 Accessibility Compliance

- **WCAG AAA**: 7:1 contrast ratio in monochrome theme
- **Screen Reader**: Structured output with semantic announcements
- **No-Color**: Graceful fallback for color-blind users and assistive tech
- **Keyboard Only**: Full functionality without mouse dependency

### 4.3 Performance Constraints

- **Responsive Resizing**: Layout recalculation must complete <100ms
- **Memory Efficiency**: Circular buffers for logs (1000 entries max)
- **Render Optimization**: Component-level updates without full redraws

## 5. Integration with Existing System

### 5.1 Minimal main.go Changes

Current integration point in main.go:151-177 requires only TUI instantiation:

```go
// In main.go - minimal change required
if !noTUI {
    monitor := ui.NewMonitor(target)
    if cancelCtx, err := monitor.Start(ctx); err == nil {
        ctx = cancelCtx
        executor.SetMonitor(monitor)
        logger.SetTUILogger(monitor)
    }
}
```

### 5.2 Data Flow Integration

**Live Data Sources**:
- Workflow status updates via channels
- Tool execution logs via structured events
- System metrics via periodic polling

**Event Bus Pattern**:
```go
type EventBus interface {
    Subscribe(eventType string, handler func(interface{}))
    Publish(eventType string, data interface{})
}

// TUI subscribes to workflow events
eventBus.Subscribe("workflow.status", m.handleWorkflowUpdate)
eventBus.Subscribe("tool.execution", m.handleToolExecution) 
eventBus.Subscribe("log.entry", m.handleLogEntry)
```

## 6. Quality Assurance Framework

### 6.1 Testing Strategy

**Golden Frame Testing**: Capture terminal output at different sizes and verify:
- No line count growth across updates
- No overlap at test resolutions (80x24, 100x30, 120x40, 160x48)
- Consistent focus indicators and keyboard navigation

**Static Analysis**: Verify exactly one `tea.NewProgram` instantiation per build.

### 6.2 Demo Mode

Separate demo path using `/pkg/sim/simulator.go` for:
- Showcase functionality without real workflow execution
- Rapid UI iteration and testing
- Documentation and presentation purposes

## 7. Sources and References

- **Bubble Tea Documentation**: https://github.com/charmbracelet/bubbletea/blob/main/README.md
- **Bubbles Component Library**: https://github.com/charmbracelet/bubbles/blob/master/README.md  
- **Lipgloss Layout System**: https://github.com/charmbracelet/lipgloss/blob/master/README.md
- **Glamour Markdown Rendering**: https://github.com/charmbracelet/glamour/blob/master/README.md
- **Terminal UI Best Practices**: Applied Go TUI Guidelines, GitHub awesome-tuis, Responsive TUI research
- **Accessibility Standards**: WCAG 2.1 AAA compliance guidelines
- **Performance Benchmarks**: Terminal rendering optimization studies

## 8. Implementation Roadmap

1. **Foundation**: Core tea.Model with WindowSizeMsg handling
2. **Layout System**: Responsive Lipgloss layout engine  
3. **Components**: Bubbles integration (list, table, viewport, progress, spinner)
4. **Theming**: Configuration-driven styling and accessibility
5. **Integration**: Event bus connection to existing workflow system
6. **Testing**: Golden frame tests and CI integration
7. **Documentation**: Usage guides and keyboard reference

---

**Document Version**: 1.0  
**Last Updated**: 2025-08-08  
**Research Lead**: Hive Mind Collective Intelligence System  
**Technical Scope**: TUI-only implementation (business logic unchanged)