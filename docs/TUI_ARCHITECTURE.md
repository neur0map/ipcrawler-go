# IPCrawler TUI Architecture Design

## Executive Summary

This document outlines the unified architecture for IPCrawler's Terminal User Interface (TUI) built on the Charmbracelet ecosystem. The design addresses the current issues of duplicate TUI paths, hardcoded dimensions, overlapping components, and missing live data integration.

## Current Issues Analysis

### 1. Duplicate TUI Paths ❌
- **Old Path**: `/internal/tui/` using `tui.Monitor` with `tea.NewProgram()` 
- **New Path**: `/internal/ui/` using `ui.RunTUI()` with separate `tea.NewProgram()`
- **Problem**: Two competing TUI implementations causing conflicts

### 2. Hardcoded Dimensions ❌
- Layout breakpoints: 120, 80 pixels hardcoded in layout.go
- Magic numbers in component sizing throughout modern_model.go
- Border widths, spacing constants embedded in calculation logic

### 3. Layout Overlapping ❌
- Overview screen has misaligned boxes
- Some tools don't render at all
- Missing proper resize handling and bounds checking

### 4. Missing Live Data Integration ❌
- TUI shows mock/demo data instead of real workflow events
- Event bus not properly connected to UI updates
- No real-time streaming of tool outputs

## Solution Architecture

### Core Principles

1. **Single TUI Stack**: Exactly one `tea.Program` and one render loop
2. **Configuration-Driven**: Zero hardcoded dimensions, strings, or icons
3. **Responsive Design**: Dynamic layout based on terminal size
4. **Live Data Only**: Real-time integration with workflow execution
5. **Accessibility**: High-contrast, color-adaptive, non-TTY fallback

### Architecture Stack

```
┌─────────────────────────────────────────────────────────────┐
│                        main.go                              │
│                   (Single Entry Point)                     │
└─────────────────────────────┬───────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────┐
│                    /internal/ui/                            │
│                 (Unified TUI System)                        │
├─────────────────────────────────────────────────────────────┤
│  ui.go          │ Single tea.Program instance              │
│  model/         │ Bubble Tea Models & Message handling     │
│  components/    │ Reusable Bubbles components              │
│  screens/       │ Screen-specific layouts                  │
│  layout/        │ Responsive layout calculations           │
│  theme/         │ Monochrome theme & accessibility         │
│  keymap/        │ Keyboard interaction patterns            │
└─────────────────────────────┬───────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────┐
│                    /configs/ui.yaml                         │
│              (Configuration-Driven Design)                  │
├─────────────────────────────────────────────────────────────┤
│  • Layout breakpoints and responsive rules                 │
│  • Component spacing, padding, and margins                 │
│  • Status symbols, colors, and accessibility options      │
│  • Keyboard shortcuts and interaction patterns             │
└─────────────────────────────────────────────────────────────┘
```

### Component Architecture

#### 1. Main Application Model (`/internal/ui/model/app.go`)
- **Single Source of Truth**: One tea.Model implementation
- **State Management**: Workflow status, tool execution, logs, system stats
- **Resize Handling**: WindowSizeMsg processing with dynamic layout updates
- **Event Processing**: Real-time message handling from workflow executor

#### 2. Layout System (`/internal/ui/layout/layout.go`)
```go
// Responsive breakpoints from config
type LayoutMode int
const (
    LargeLayout  LayoutMode = iota  // ≥120 cols: [Nav] | [Main] | [Status]
    MediumLayout                    // 80-119 cols: [Nav] | [Main+Status]  
    SmallLayout                     // <80 cols: Single panel with tabs
)
```

#### 3. Component System (`/internal/ui/components/`)
- **Navigation Panel**: Bubbles list for workflows with live status
- **Main Content**: Bubbles table for tool executions + viewport for logs
- **Status Panel**: Progress indicators, system stats, current focus
- **Help System**: Glamour-rendered markdown help overlay

#### 4. Theme System (`/internal/ui/theme/theme.go`)
```go
// Monochrome, high-contrast theme
var MonochromeTheme = Theme{
    Primary:   lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#ffffff"},
    Secondary: lipgloss.AdaptiveColor{Light: "#cccccc", Dark: "#cccccc"},
    Border:    lipgloss.AdaptiveColor{Light: "#444444", Dark: "#444444"},
    // ... accessibility-focused palette
}
```

### Data Flow Architecture

```
Workflow Executor ──→ Event Bus ──→ TUI Messages ──→ Model Updates ──→ View Rendering
        │                                │                    │              │
        │                                │                    │              │
    ┌───▼───┐                      ┌─────▼─────┐         ┌────▼────┐   ┌─────▼─────┐
    │ Tool  │                      │ Message   │         │ Bubble  │   │ Lipgloss  │
    │ Start │                      │ Types:    │         │ Tea     │   │ Layout    │
    │ Tool  │                      │ • Workflow│         │ Update  │   │ Rendering │
    │ End   │                      │ • Tool    │         │ Cycle   │   │           │
    │ Log   │                      │ • Log     │         │         │   │           │
    └───────┘                      │ • Error   │         └─────────┘   └───────────┘
                                   │ • Tick    │
                                   └───────────┘
```

### Configuration Schema (`/configs/ui.yaml`)

```yaml
# Layout Configuration
layout:
  breakpoints:
    large: 120    # Three-panel layout
    medium: 80    # Two-panel layout
    small: 40     # Minimum width
  
  spacing:
    none: 0
    small: 1
    medium: 2
    large: 4
  
  panels:
    navigation:
      min_width: 20
      preferred_width_ratio: 0.25
    main:
      min_width: 40  
      preferred_width_ratio: 0.50
    status:
      min_width: 15
      preferred_width_ratio: 0.25

# Theme Configuration  
theme:
  mode: "monochrome"  # monochrome | adaptive
  high_contrast: true
  accessibility:
    color_disabled_fallback: true
    screen_reader_compatible: true

# Status Symbols
symbols:
  running: "●"
  completed: "✓" 
  failed: "✗"
  pending: "○"
  spinner: ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"]

# Keyboard Shortcuts
keymap:
  global:
    quit: ["q", "ctrl+c"]
    help: ["?"]
    refresh: ["ctrl+r"]
  navigation:
    up: ["up", "k"]
    down: ["down", "j"] 
    left: ["left", "h"]
    right: ["right", "l"]
    next_panel: ["tab"]
    prev_panel: ["shift+tab"]
  actions:
    select: ["enter", " "]
    cancel: ["esc"]
```

### Message System

```go
// Core message types for real-time updates
type WorkflowUpdateMsg struct {
    WorkflowID   string
    Status       string        // running, completed, failed
    Progress     float64       // 0.0 to 1.0
    Description  string
    StartTime    time.Time
    Duration     time.Duration
    Error        error
}

type ToolExecutionMsg struct {
    ToolName     string
    WorkflowID   string
    Status       string
    Progress     float64
    Duration     time.Duration
    Output       string        // Live tool output
    Error        error
    Args         []string
}

type SystemStatsMsg struct {
    CPUUsage     float64
    MemoryUsage  float64
    ActiveTools  int
    Timestamp    time.Time
}
```

### Responsive Layout Logic

```go
// Dynamic layout calculation
func (l *Layout) Update(width, height int) {
    l.mode = DetermineLayout(width, height)
    l.dimensions = calculateDimensions(l.mode, width, height)
    
    // Ensure no overlap - critical requirement
    l.validateBounds()
    l.adjustForTerminalLimits(width, height)
}

// Anti-overlap protection
func (l *Layout) EnsureNoOverlap(content string) string {
    actualWidth := lipgloss.Width(content)
    actualHeight := lipgloss.Height(content)
    
    if actualWidth > l.dimensions.Terminal.Width {
        content = l.truncateWidth(content)
    }
    if actualHeight > l.dimensions.Terminal.Height {
        content = l.truncateHeight(content)
    }
    
    return content
}
```

### Accessibility Features

1. **Color Detection**: Use termenv to detect terminal capabilities
2. **Non-TTY Fallback**: Clean log output with zero ANSI codes
3. **High Contrast**: Monochrome theme optimized for accessibility
4. **Keyboard Navigation**: Full keyboard control, no mouse dependency
5. **Screen Reader Support**: Structured output for assistive technology

### Testing Strategy

```makefile
# UI Test Targets
test-ui: test-ui-layout test-ui-interaction test-ui-data

test-ui-layout:
    # Test responsive layout at different terminal sizes
    COLUMNS=80 LINES=24 go test ./internal/ui/layout/...
    COLUMNS=120 LINES=40 go test ./internal/ui/layout/...
    
test-ui-interaction:
    # Test keyboard navigation and component interaction
    go test ./internal/ui/keymap/...
    go test ./internal/ui/components/...

test-ui-data:
    # Test live data integration and message handling
    go test ./internal/ui/model/...
```

### Migration Plan

#### Phase 1: Foundation ✅ (Research Complete)
- [x] Research Charmbracelet ecosystem patterns
- [x] Analyze current TUI implementation issues
- [x] Design unified architecture

#### Phase 2: Implementation
- [ ] Remove `/internal/tui/` (old TUI path)
- [ ] Enhance `/internal/ui/` as single TUI entry point
- [ ] Implement configuration-driven layout system
- [ ] Add proper resize handling and overlap prevention

#### Phase 3: Integration  
- [ ] Connect live data from workflow executor
- [ ] Implement real-time event streaming
- [ ] Add comprehensive error handling

#### Phase 4: Validation
- [ ] UI tests for layout stability
- [ ] Accessibility compliance testing
- [ ] Performance validation under load

### Success Criteria

- ✅ **Single TUI Entry Point**: Exactly one `tea.NewProgram()` call
- ✅ **Zero Hardcoded Values**: All dimensions from `/configs/ui.yaml`
- ✅ **No Overlaps**: Components fit within terminal bounds at all sizes
- ✅ **Live Data**: Real workflow events drive UI updates
- ✅ **Stable Line Count**: In-place updates without line growth
- ✅ **Accessibility**: High contrast, non-TTY fallback, keyboard navigation

### Error Handling & Resilience

1. **Terminal Resize Storms**: Debounce resize events, cache layout calculations
2. **Data Overload**: Implement log rotation, limit message queues
3. **Network Issues**: Graceful degradation when data streams fail
4. **Component Failures**: Fallback rendering for individual component errors

This architecture provides a solid foundation for a modern, accessible, and maintainable TUI that meets all specified requirements while following Charmbracelet ecosystem best practices.