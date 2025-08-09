# IPCrawler TUI Redesign: Charmbracelet-First Architecture

## Executive Summary

This document outlines the complete redesign of IPCrawler's TUI using **exclusively** the Charmbracelet ecosystem. The new design fixes all identified issues in the current implementation while delivering a modern, responsive, and stable terminal interface.

## Current Implementation Analysis

### Major Issues Identified

1. **WindowSizeMsg Mishandling**: Current code computes layout before receiving initial WindowSizeMsg, causing broken first renders
2. **Multiple TUI Systems**: Overlapping rendering systems and duplicate entrypoints 
3. **Complex Abstractions**: Over-engineered architecture with unnecessary layers
4. **Layout Overlaps**: Improper use of Lipgloss causing content to exceed boundaries
5. **Hardcoded Dimensions**: Magic numbers despite configuration attempts
6. **Non-Standard Patterns**: Reinventing Bubble Tea patterns instead of following MVU

### Root Cause

The current implementation tries to abstract away Bubble Tea instead of embracing it, leading to complex workarounds and unstable behavior.

## New Architecture Design

### Core Principles

1. **Single TUI Stack**: Exactly one `tea.Program` with one render loop
2. **Charmbracelet-First**: Use the ecosystem as intended, not as an implementation detail  
3. **WindowSizeMsg-Gated**: No layout computation until first WindowSizeMsg received
4. **Zero Hardcoding**: All dimensions from `/configs/ui.yaml` and runtime calculations
5. **Stable Lines**: In-place updates with no scroll creep or flicker
6. **Live Data Only**: Bind to real event streams, no mock data in production

### Stack Components

| Component | Purpose | Package |
|-----------|---------|---------|
| **Runtime** | MVU state machine & event loop | `charmbracelet/bubbletea` |
| **Components** | Interactive UI elements | `charmbracelet/bubbles` |
| **Layout** | Responsive styling & borders | `charmbracelet/lipgloss` |
| **Help** | Markdown help/about panels | `charmbracelet/glamour` |
| **Logging** | Structured logs & color detection | `charmbracelet/log` |

## Architecture Overview

```
┌─ main.go ─────────────────────────────────────┐
│ • Single tea.NewProgram() call                │
│ • Bind to live event streams                  │
│ • Non-TTY fallback (plain logs)               │
└───────────────────────────────────────────────┘
                      │
┌─ /internal/ui/model/app.go ─────────────────┐
│ type AppModel struct {                      │
│   ready    bool     // WindowSizeMsg guard │
│   width    int      // Current dimensions  │
│   height   int                             │
│   // Embedded Bubbles components           │
│   list     list.Model                      │
│   table    table.Model                     │
│   viewport viewport.Model                  │
│   spinner  spinner.Model                   │
│   help     help.Model                      │
│ }                                           │
└─────────────────────────────────────────────┘
                      │
┌─ Layout System ─────────────────────────────┐
│ Lipgloss-based responsive layout:           │
│ • Large:  [Nav] | [Main] | [Status]        │ 
│ • Medium: [Nav] | [Main] (status footer)   │
│ • Small:  Stacked screens with tabs        │
└─────────────────────────────────────────────┘
```

## Directory Structure

```
/internal/ui/
├── model/
│   ├── app.go           # Main Bubble Tea model
│   ├── messages.go      # Custom message types
│   └── update.go        # Update method implementations
├── components/
│   ├── workflows.go     # Workflow list (Bubbles list)
│   ├── tools.go         # Tool table (Bubbles table) 
│   ├── logs.go          # Log viewport (Bubbles viewport)
│   └── status.go        # Status panel (custom)
├── layout/
│   ├── responsive.go    # Breakpoint logic
│   └── render.go        # Lipgloss layout functions
├── theme/
│   ├── styles.go        # Lipgloss style definitions
│   └── config.go        # Theme configuration loader
└── help/
    └── content.go       # Glamour markdown help

/configs/
└── ui.yaml              # All visual tokens & breakpoints

/internal/term/
├── detect.go            # Terminal capability detection
└── fallback.go          # Non-TTY plain output
```

## Implementation Details

### 1. Initial Render Guard Pattern

```go
type AppModel struct {
    ready  bool
    width  int  
    height int
    // ... other fields
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.ready = true  // Enable rendering
        m.updateComponentSizes()
        return m, nil
    // ... other messages only processed if m.ready
    }
}

func (m AppModel) View() string {
    if !m.ready {
        return "\n  Initializing..."
    }
    return m.renderLayout()
}
```

### 2. Responsive Layout with Lipgloss

```go
func (m AppModel) renderLayout() string {
    mode := m.getLayoutMode() // based on m.width
    
    switch mode {
    case LargeLayout:
        nav := m.renderNav()
        main := m.renderMain() 
        status := m.renderStatus()
        
        return lipgloss.JoinHorizontal(
            lipgloss.Top,
            lipgloss.NewStyle().Width(m.navWidth()).Render(nav),
            lipgloss.NewStyle().Width(m.mainWidth()).Render(main),
            lipgloss.NewStyle().Width(m.statusWidth()).Render(status),
        )
    case MediumLayout:
        // Two-panel layout
    case SmallLayout:  
        // Single-panel with tabs
    }
}
```

### 3. Component Integration

```go
// Embed Bubbles components directly
type AppModel struct {
    workflows list.Model
    tools     table.Model
    logs      viewport.Model
    progress  progress.Model
    spinner   spinner.Model
    help      help.Model
}

func (m AppModel) updateComponentSizes() {
    // Use GetFrameSize for accurate calculations
    navStyle := m.getNavStyle()
    v, h := navStyle.GetFrameSize()
    
    m.workflows.SetSize(
        m.navWidth() - h,
        m.navHeight() - v,
    )
    
    // Similar for other components...
}
```

### 4. Configuration-Driven Styling

```yaml
# /configs/ui.yaml (simplified)
layout:
  breakpoints:
    large: 120
    medium: 80  
    small: 40
  panels:
    nav_ratio: 0.25
    main_ratio: 0.50
    status_ratio: 0.25
theme:
  mode: monochrome
  colors:
    primary: "#ffffff"
    border: "#444444"
    accent: "#888888"
```

### 5. Event Integration

```go
// In main.go - bind to live data
func setupTUI(target string) {
    model := NewAppModel(target)
    program := tea.NewProgram(model)
    
    // Connect to live event streams
    go func() {
        for event := range workflowEvents {
            program.Send(WorkflowUpdateMsg{
                ID: event.WorkflowID,
                Status: event.Status,
                // ...
            })
        }
    }()
    
    // Run the program
    program.Run()
}
```

## Key Features

### Responsive Breakpoints

| Width | Layout | Description |
|-------|--------|-------------|
| ≥120  | Large  | Three panels: [Nav] \| [Main] \| [Status] |
| 80-119| Medium | Two panels: [Nav] \| [Main] + status footer |
| 40-79 | Small  | Single panel with tab navigation |
| <40   | Error  | "Terminal too small" message |

### Component Behavior

| Component | Purpose | Bubble Tea Component |
|-----------|---------|---------------------|
| **Workflow List** | Show running workflows | `bubbles/list` |
| **Tool Table** | Recent tool executions | `bubbles/table` |
| **Log Viewport** | Streaming output | `bubbles/viewport` |
| **Progress** | Workflow progress | `bubbles/progress` |
| **Status** | System info | Custom (simple text) |
| **Help** | Markdown help | `glamour` renderer |

### Key Bindings

| Key | Action | Context |
|-----|--------|---------|
| `q`, `Ctrl+C` | Quit | Global |
| `?` | Toggle help | Global |
| `Tab` | Next panel | Navigation |
| `1`, `2`, `3` | Direct panel focus | Large layout |
| `Space` | Next view | Small layout |
| `↑`/`↓`, `j`/`k` | Navigate lists/tables | Focused component |

### Non-TTY Fallback

```go
// In main.go
if !term.IsTTY() || term.IsCI() {
    // Plain text output mode
    logger.SetPlainOutput()
    runPlainMode(target)
    return
}

// Otherwise use TUI
runTUIMode(target)
```

## Implementation Plan

### Phase 1: Core Runtime (Week 1)
- [ ] Single `tea.Program` with WindowSizeMsg guard
- [ ] Basic three-panel layout with Lipgloss
- [ ] Configuration loading and theme system
- [ ] Non-TTY fallback mode

### Phase 2: Component Integration (Week 1) 
- [ ] Embed Bubbles list for workflows
- [ ] Embed Bubbles table for tools
- [ ] Embed Bubbles viewport for logs
- [ ] Component size calculation with GetFrameSize

### Phase 3: Live Data & Polish (Week 1)
- [ ] Connect to real workflow event streams
- [ ] Add Glamour help system
- [ ] Spinner and progress integration
- [ ] Responsive layout testing

### Phase 4: Testing & CI (Week 1)
- [ ] Golden frame tests for layout stability
- [ ] Virtual TTY testing at multiple sizes
- [ ] Performance testing under load
- [ ] CI integration and asciinema recording

## Success Criteria

1. **Single TUI Stack**: Exactly one `tea.Program` in the entire codebase
2. **Zero Overlaps**: No content exceeds terminal boundaries at any size
3. **Stable Lines**: Status updates in-place without adding lines
4. **Live Data**: Real workflow events render without mock data
5. **Responsive**: Smooth transitions between layout modes
6. **Accessible**: High contrast, clean fallbacks, screen reader compatible
7. **Performance**: <16ms render times, handles 1k+ events/min

## Testing Strategy

### Layout Stability Tests
```bash
# Test at common terminal sizes
./test-layout.sh 80x24 100x30 120x40 160x48

# Verify no overlaps
diff frame1.txt frame2.txt  # Should show in-place updates only

# Performance under load  
./stress-test.sh 1000events 60seconds
```

### Golden Frame Testing
- Capture rendered frames at key states
- Verify consistent layout across resize events
- Test all three layout modes thoroughly

## Migration Strategy

1. **Create New**: Build new TUI alongside existing (no modification of current code)
2. **Switch Flag**: Add `--new-tui` flag to test new implementation
3. **A/B Test**: Run both implementations in parallel during development
4. **Replace**: Once stable, replace old implementation completely
5. **Cleanup**: Remove old TUI code and abstractions

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| **WindowSizeMsg Issues** | Implement proper guard pattern from research |
| **Layout Overlaps** | Use GetFrameSize and Lipgloss joins exclusively |
| **Performance** | Limit component updates, use proper Bubble Tea patterns |
| **Terminal Compatibility** | Extensive testing across terminal types |
| **Event Binding** | Test with real workflow data from day one |

This design provides a solid foundation for a stable, responsive TUI that fully leverages the Charmbracelet ecosystem while meeting all project requirements.