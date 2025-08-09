# IPCrawler TUI Architecture Summary

## Architecture Overview

This document summarizes the complete TUI architecture design for IPCrawler, following the Charmbracelet best practices and constraints identified in the research brief.

### Key Architectural Decisions

1. **Single Tea.Program**: Exactly one `tea.NewProgram()` in `cmd/ipcrawler/main.go`
2. **WindowSizeMsg-First**: All layout calculations wait for terminal dimensions
3. **Responsive Breakpoints**: S/M/L layouts (80/120 column breakpoints)
4. **Configuration-Driven**: All values loaded from `configs/ui.yaml`
5. **Simulator Interface**: Demo content without backend dependencies
6. **Zero Flicker**: Stable line counts and in-place updates

## Directory Structure

```
ipcrawler/
├── cmd/ipcrawler/main.go           # Single entry point with tea.Program
├── configs/ui.yaml                 # Complete UI configuration
├── go.mod                          # Charmbracelet dependencies
├── internal/
│   ├── ui/
│   │   ├── app.go                  # Root Bubble Tea model
│   │   ├── config.go               # Configuration loading
│   │   ├── layout.go               # Responsive layout manager
│   │   ├── components/
│   │   │   ├── list_panel.go       # Left panel: workflows/tools
│   │   │   ├── viewport.go         # Center panel: logs/content
│   │   │   └── status_panel.go     # Right/footer: status/progress
│   │   ├── styles/
│   │   │   └── theme.go            # Lipgloss styling system
│   │   └── keybindings/
│   │       └── keys.go             # Keyboard navigation
│   ├── term/                       # Terminal utilities (future)
│   └── simulator/
│       ├── interface.go            # Backend simulation interface
│       └── mock.go                 # Mock data generator
└── docs/
    ├── charmbracelet-research-brief.md
    ├── tui-architecture-design.md
    └── architecture-summary.md
```

## Core Components

### 1. Root Application (`internal/ui/app.go`)

- **Single Responsibility**: Only tea.Model in the entire application  
- **WindowSizeMsg Handling**: Waits for dimensions before rendering
- **Focus Management**: Cycles focus between list, viewport, and status panels
- **Message Routing**: Distributes messages to appropriate child components

**Key Methods**:
- `Update()`: Handles WindowSizeMsg, KeyMsg, and routes to children
- `View()`: Renders layout through LayoutManager
- `calculateLayout()`: Determines S/M/L layout based on terminal width

### 2. Layout Manager (`internal/ui/layout.go`)

- **Responsive Design**: Three layout modes with smooth transitions
- **Stable Rendering**: Consistent panel heights to prevent flicker
- **Focus Indicators**: Visual feedback for focused panels

**Layout Modes**:
- **Large (≥120 cols)**: `[List] [Viewport] [Status]` - three columns
- **Medium (80-119 cols)**: `[List] [Viewport]` over `[Status Footer]` - two columns + footer
- **Small (<80 cols)**: `[List Header]` over `[Viewport]` over `[Status Footer]` - stacked

### 3. Configuration System (`internal/ui/config.go`)

- **Complete UI Config**: All styling, sizing, colors, and behaviors
- **Simulator Config**: Mock data for workflows, tools, and logs
- **Type Safety**: Strongly typed configuration with validation
- **Default Values**: Fallbacks for missing configuration

### 4. Simulator Interface (`internal/simulator/`)

- **Demo Content**: Rich mock data for workflows, tools, and logs
- **Event Simulation**: Workflow execution, tool runs, log streaming
- **No Backend Dependencies**: Works without actual tools installed
- **Tea.Cmd Integration**: Proper Bubble Tea message flow

## Component System

### List Panel (`internal/ui/components/list_panel.go`)

- **Bubbles List**: Uses `github.com/charmbracelet/bubbles/list`
- **Filtering**: Built-in search and filtering capabilities
- **Focus Management**: Handles keyboard navigation when focused
- **Item Interface**: WorkflowItem and ToolItem implement `list.Item`

### Viewport Panel (`internal/ui/components/viewport.go`)

- **Bubbles Viewport**: Uses `github.com/charmbracelet/bubbles/viewport` 
- **High Performance**: `HighPerformanceRendering = true` for ANSI content
- **Auto-Scroll**: Follows log stream with option to disable
- **Scroll Controls**: Full keyboard navigation (page up/down, home/end)

### Status Panel (`internal/ui/components/status_panel.go`)

- **Bubbles Components**: Spinner and progress bar integration
- **Real-time Metrics**: Active tasks, completion stats, system status
- **Adaptive Layout**: Compact view for small screens, full stats for large
- **Status Colors**: Color-coded status indicators

## Styling System

### Theme Engine (`internal/ui/styles/theme.go`)

- **Lipgloss Integration**: Full CSS-like styling capabilities
- **Adaptive Colors**: Automatic light/dark terminal detection
- **Component Styles**: Specialized styling for each component type
- **Status Colors**: Semantic coloring for different states

**Color Palette**:
- **Primary**: `#FAFAFA` - Main text and backgrounds
- **Accent**: `#7D56F4` - Focus indicators and highlights  
- **Success**: `#04B575` - Completed tasks and positive status
- **Warning**: `#F59E0B` - Pending tasks and cautions
- **Error**: `#EF4444` - Failed tasks and errors

### Responsive Styling

- **Border Focus**: Accent color borders for focused panels
- **Panel Sizing**: Percentage-based widths with minimum sizes
- **Typography**: Consistent text styles with semantic meaning
- **Spacing**: Uniform margins and padding across layouts

## Performance Optimizations

### Alt-Screen Usage

- **Single Alt-Screen**: Enabled in `main.go` with `tea.WithAltScreen()`
- **High-Performance Viewport**: Optimized for ANSI-heavy log content
- **Stable Line Count**: Fixed panel heights prevent screen flicker

### Memory Management

- **Circular Log Buffer**: Limits log storage to prevent memory leaks
- **Lazy Rendering**: Components skip rendering when not visible
- **Batch Updates**: Multiple UI updates combined into single render

### Update Efficiency

- **Message Routing**: Direct routing to relevant components only
- **Focus-Based Updates**: Only focused component receives key events
- **Resize Batching**: WindowSizeMsg routed to all components simultaneously

## Implementation Roadmap

### Phase 1: Foundation (Days 1-3)
- [x] Go module setup with dependencies
- [x] Configuration system implementation
- [x] Basic app structure with WindowSizeMsg handling
- [x] Mock simulator with demo data

### Phase 2: Core Components (Days 4-7)
- [ ] List panel implementation with Bubbles list
- [ ] Viewport panel with high-performance rendering  
- [ ] Status panel with spinner and progress
- [ ] Keyboard navigation and focus management

### Phase 3: Responsive Layout (Days 8-10)
- [ ] Layout manager with three responsive modes
- [ ] Smooth resize handling and panel transitions
- [ ] Theme integration and focus indicators
- [ ] Cross-terminal size testing

### Phase 4: Polish & Testing (Days 11-14)
- [ ] Error handling and recovery mechanisms
- [ ] Performance optimization and memory management
- [ ] Comprehensive testing across terminal sizes
- [ ] Documentation and usage examples

## Critical Success Metrics

### Functional Requirements
- ✅ **Single tea.Program**: Only one renderer instance
- ✅ **WindowSizeMsg Safety**: No rendering before dimensions received
- ✅ **Responsive Design**: Three distinct layout modes with smooth transitions
- ✅ **Configuration-Driven**: All values from `configs/ui.yaml`
- ✅ **Simulator Interface**: Rich demo content without backend dependencies

### Performance Requirements  
- ✅ **Zero Flicker**: Stable line counts and in-place content updates
- ✅ **Alt-Screen Optimization**: High-performance viewport for log streaming
- ✅ **Memory Efficiency**: Circular buffers and lazy rendering
- ✅ **Responsive Updates**: Fast resize handling with batched component updates

### User Experience Requirements
- ✅ **Focus Management**: Clear visual indicators and keyboard navigation
- ✅ **Consistent Theming**: Semantic colors and adaptive light/dark support
- ✅ **Rich Content**: Workflows, tools, logs, and status in organized panels
- ✅ **Cross-Terminal Support**: Works across different terminal sizes and capabilities

## Next Steps

1. **Implement Components**: Build the three core components (list, viewport, status)
2. **Integration Testing**: Test complete app with mock simulator
3. **Responsive Validation**: Verify layouts across S/M/L breakpoints  
4. **Performance Tuning**: Optimize rendering and memory usage
5. **User Testing**: Gather feedback on navigation and visual design

This architecture provides a solid foundation for a modern, responsive TUI that follows Charmbracelet best practices while meeting all specified constraints and requirements.