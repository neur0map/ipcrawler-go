# Charmbracelet TUI Research Brief
**Fixing IPCrawler TUI using Bubble Tea Ecosystem**

## Executive Summary

This research brief outlines the findings from investigating the Charmbracelet ecosystem for building responsive, stable terminal user interfaces (TUIs) in Go. The objective is to fix the broken IPCrawler TUI using **exclusively** Bubble Tea runtime, Bubbles components, Lipgloss styling, Glamour markdown rendering, and Log/termenv for structured logging.

## Architecture Overview: Bubble Tea vs Bubbles

### Bubble Tea (Core Runtime)
**Purpose**: The foundational TUI framework providing the runtime and state machine.

- **Architecture**: Model-View-Update (MVU) pattern borrowed from Elm
- **Core Components**: 
  - `tea.Model` - Application state container
  - `tea.Msg` - Message types for events (key presses, window resize, custom)
  - `tea.Cmd` - Commands for I/O operations and side effects
  - `tea.Program` - The runtime that orchestrates the MVU cycle

**Key Insight**: Bubble Tea is the **runtime engine**, not a component library. It handles the event loop, message dispatching, and render cycles.

### Bubbles (Component Library)
**Purpose**: Ready-made interactive components built on top of Bubble Tea.

**Available Components**:
- `list` - Interactive lists with filtering and selection
- `table` - Tabular data display with styling
- `viewport` - Scrollable content areas with smooth scrolling
- `progress` - Progress bars with customizable styles
- `spinner` - Loading indicators with multiple animation types
- `textarea` - Multi-line text input with editing capabilities
- `key` - Key binding management and help text

**Integration Pattern**: Bubbles components implement the `tea.Model` interface, allowing them to be embedded in larger applications and composed together.

## Initial Render vs WindowSizeMsg Strategy

### The Critical First Render Problem
**Issue**: Computing layout before knowing terminal dimensions results in broken initial frames.

**Solution**: Initial Render Guard Pattern
```go
type model struct {
    ready  bool
    width  int
    height int
    // ... other fields
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.ready = true // Only set ready after first WindowSizeMsg
        return m, nil
    }
    // ... handle other messages only if m.ready
}

func (m model) View() string {
    if !m.ready {
        return "\n  Initializing..."
    }
    // ... compute layout using m.width and m.height
}
```

### WindowSizeMsg Handling Best Practices
1. **Stash Dimensions**: Always store width/height in model state
2. **Dynamic Layout**: Use stored dimensions for all layout calculations
3. **Responsive Breakpoints**: Define small/medium/large terminal layouts
4. **No Magic Numbers**: All dimensions derived from current size + theme tokens

## Responsive Layout Rules

### Lipgloss Layout Principles
**Core Concept**: Declarative styling similar to CSS, but for terminals.

**Layout Methods**:
- `lipgloss.JoinHorizontal()` - Horizontal composition with alignment
- `lipgloss.JoinVertical()` - Vertical composition with alignment  
- `lipgloss.PlaceHorizontal()` - Position within a defined width
- `lipgloss.PlaceVertical()` - Position within a defined height

### Frame Size Calculation
**Critical Method**: `GetFrameSize()` accounts for margins, borders, padding
```go
v, h := docStyle.GetFrameSize()
component.SetSize(msg.Width-h, msg.Height-v)
```

### Responsive Breakpoint Strategy
```yaml
breakpoints:
  small: width < 80
  medium: 80 <= width < 120  
  large: width >= 120

layouts:
  small: stacked_screens_with_tabs
  medium: left_nav_main_content_footer
  large: left_nav_main_content_right_status
```

### Overlap Prevention Rules
1. **Dynamic Width Calculation**: Always derive from current terminal size
2. **Content Truncation**: Use `lipgloss.Width()` and `MaxWidth()` for safe bounds
3. **Stable Line Count**: In-place updates must not add lines (avoid scroll creep)
4. **Non-TTY Fallback**: Clean text output without ANSI codes

## Component Integration Patterns

### Viewport for Scrollable Content
- **Purpose**: Handle content larger than available screen space
- **Features**: Mouse wheel support, smooth scrolling, high-performance rendering
- **Use Case**: Streaming logs, detailed output, help content

### List for Navigation
- **Features**: Built-in filtering, selection state, key bindings
- **Customization**: Custom delegates for item rendering
- **Integration**: Embed in sidebar for workflow/tool selection

### Key Bindings with Bubbles Key Component
```go
type KeyMap struct {
    Up   key.Binding
    Down key.Binding
    Help key.Binding
    Quit key.Binding
}

var DefaultKeyMap = KeyMap{
    Up:   key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "move up")),
    Down: key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "move down")),
    Help: key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help")),
    Quit: key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}
```

## Styling and Theming

### Color Profile Detection
- **Automatic**: Lipgloss detects terminal capabilities (ANSI16, ANSI256, TrueColor)
- **Adaptive Colors**: Different colors for light/dark backgrounds
- **Graceful Degradation**: Falls back to supported color profiles

### Theme Configuration
**Approach**: Externalize all visual tokens to YAML configuration
```yaml
theme:
  colors:
    primary: "#6f03fc"
    secondary: "#f0f0f0"
    accent: "#ff6b6b"
  spacing:
    padding: 1
    margin: 2
  borders:
    style: "rounded"
    width: 1
```

## Performance and Stability Considerations

### Render Performance
- **Frame-Rate Based**: Bubble Tea includes built-in frame limiting
- **Bypass Renderer**: Viewport can bypass normal renderer for heavy ANSI content
- **Mouse Support**: Built-in mouse wheel and click handling

### Memory Management
- **Value vs Pointer Receivers**: Use value receivers for functional style
- **Component Lifecycle**: Proper initialization and cleanup of embedded components

## Accessibility and Usability

### Keyboard Navigation
- **Standard Bindings**: Arrow keys, vi-style (hjkl), Enter, Space, Escape
- **Help System**: Built-in help with `?` key using Glamour markdown
- **Focus Management**: Clear visual indicators for focused elements

### High Contrast Support
- **WCAG Compliance**: Minimum 4.5:1 contrast ratio for normal text
- **Terminal Detection**: Respect terminal's dark/light background
- **Fallback Styles**: Graceful degradation for limited color terminals

## Integration with IPCrawler Requirements

### Configuration-Driven Design
**Alignment**: External YAML configs match project's zero-hardcoding policy
- All dimensions, colors, and text in `/configs/ui.yaml`
- Theme tokens in `/internal/ui/theme`
- No magic numbers in code

### Live Data Binding
**Strategy**: Connect TUI to existing event streams
- Use existing workflow execution channels
- Bind to JSONL log streams
- Integrate with tool output watchers

### Testing Strategy
**Approach**: Virtual TTY testing for layout stability
- Golden frame tests for visual regression
- Automated resize testing (80x24, 100x30, 120x40, 160x48)
- Non-TTY output validation

## Citations and Resources

1. **Bubble Tea Documentation**: github.com/charmbracelet/bubbletea - MVU architecture, WindowSizeMsg handling
2. **Bubbles Components**: github.com/charmbracelet/bubbles - List, table, viewport, progress components
3. **Lipgloss Styling**: github.com/charmbracelet/lipgloss - Responsive layouts, frame calculations
4. **Glamour Markdown**: github.com/charmbracelet/glamour - Terminal markdown rendering
5. **TUI Best Practices 2025**: leg100.github.io/posts/building-bubbletea-programs - Layout management, GetFrameSize usage
6. **Accessibility Guidelines**: WCAG 2.2 recommendations for contrast, spacing, and responsive design
7. **Context7 MCP Documentation**: Comprehensive API references and code examples for all Charmbracelet components

## Implementation Roadmap

1. **Foundation**: Single tea.Program with WindowSizeMsg-gated rendering
2. **Components**: Embed Bubbles list, viewport, progress, spinner as needed
3. **Layout**: Lipgloss-based responsive layout with external breakpoint config
4. **Integration**: Connect to existing IPCrawler event streams and watchers
5. **Testing**: Golden tests for layout stability and interaction patterns

This research establishes the foundation for a robust, responsive TUI that meets all project requirements while following Charmbracelet ecosystem best practices.