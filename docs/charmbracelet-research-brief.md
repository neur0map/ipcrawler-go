# Charmbracelet TUI Framework Research Brief

## Executive Summary

This research brief provides a comprehensive analysis of the Charmbracelet ecosystem for building modern terminal user interfaces (TUIs) in Go. The research covers five core libraries: Bubble Tea (runtime), Bubbles (components), Lipgloss (styling), Glamour (markdown), and Log/termenv (logging/colors), with specific focus on responsive design patterns, performance optimization, and architectural best practices for building resize-safe, flicker-free terminal applications.

## 1. Bubble Tea: MVU Runtime & Architecture

### Model-View-Update (MVU) Pattern

Bubble Tea implements the Model-View-Update pattern inspired by The Elm Architecture:

- **Model**: Holds application state as simple Go structs
- **Update**: Handles messages (events) and returns updated model + optional commands  
- **View**: Pure function that renders current model state as a string

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Handle keyboard input
    case tea.WindowSizeMsg:
        // Critical for responsive layouts
        m.width, m.height = msg.Width, msg.Height
        m.ready = true
    }
    return m, nil
}
```

### WindowSizeMsg and Initial Render Guards

**Critical Finding**: Applications must implement an initial-render guard to prevent layout computation until the first `WindowSizeMsg` arrives. This message is sent:
- Shortly after program startup  
- On every terminal resize event
- Contains current terminal dimensions

**Best Practice**: Store dimensions in model and set a `ready` flag:

```go
type model struct {
    ready  bool
    width  int  
    height int
}

func (m model) View() string {
    if !m.ready {
        return "Initializing..." // Don't compute layout yet
    }
    // Safe to use m.width/m.height for layout
}
```

**Source**: Bubble Tea documentation, `tea.WindowSizeMsg` handling patterns

### Single Program Architecture

**Hard Constraint**: Exactly one `tea.NewProgram` per application. Multiple instances cause:
- Conflicting terminal control
- Race conditions in rendering  
- Unpredictable cursor/screen state

**Implementation**:
```go
func main() {
    p := tea.NewProgram(initialModel(), tea.WithAltScreen())
    if _, err := p.Run(); err != nil {
        fmt.Printf("Error: %v\n", err)
        os.Exit(1)
    }
}
```

## 2. Bubbles: Interactive Components

### Core Component Architecture

Bubbles provides pre-built interactive components that implement the Bubble Tea interface:

**List Component**:
- Requires items implementing `list.Item` interface with `FilterValue() string`
- `DefaultItem` interface adds `Title()` and `Description()` methods
- Customizable delegates for styling and behavior

**Viewport Component**:  
- High-performance scrolling for large content
- `HighPerformanceRendering` mode for ANSI-heavy content
- Critical for main content areas with streaming data

**Key Binding System**:
```go
type KeyMap struct {
    Up   key.Binding
    Down key.Binding  
}

var DefaultKeyMap = KeyMap{
    Up: key.NewBinding(
        key.WithKeys("k", "up"),
        key.WithHelp("↑/k", "move up"),
    ),
}
```

**Source**: Charmbracelet Bubbles documentation, component interfaces

## 3. Lipgloss: Styling & Responsive Layout

### Responsive Layout System

Lipgloss provides CSS-like styling with powerful layout primitives:

**Join Operations**:
```go
// Horizontal layout: Left Nav | Main Content | Right Status  
lipgloss.JoinHorizontal(lipgloss.Top, leftNav, mainContent, rightStatus)

// Vertical stacking for mobile/small screens
lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
```

**Breakpoint Strategy**: Use terminal dimensions to choose layout:
- **Large (≥120 cols)**: Three-column layout
- **Medium (80-119 cols)**: Two-column with footer status
- **Small (<80 cols)**: Stacked single-column with tabs

### Dynamic Sizing and Constraints

**Width/Height Calculation**:
```go
// Responsive column widths
leftWidth := int(0.25 * float64(termWidth))   // 25% for nav
rightWidth := int(0.20 * float64(termWidth))  // 20% for status  
mainWidth := termWidth - leftWidth - rightWidth - 4 // Minus borders

style := lipgloss.NewStyle().
    Width(mainWidth).
    Height(termHeight - 4) // Minus header/footer
```

**Measurement Functions**: `lipgloss.Width()`, `lipgloss.Height()`, `lipgloss.Size()` for actual rendered dimensions.

### Border and Spacing System

```go
// Consistent border system
var panelStyle = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("240")).
    Padding(1).
    Margin(0, 1)
```

**Source**: Charmbracelet Lipgloss documentation, layout examples

## 4. Glamour & Log/Termenv: Content & Colors

### Markdown Rendering

Glamour provides styled markdown rendering:
```go
out, err := glamour.Render(markdown, "dark") // Built-in dark theme
```

**Available Styles**: `ascii`, `auto`, `dark`, `dracula`, `tokyo-night`, `light`, `notty`, `pink`

### Terminal Color Profiles

**Termenv Color Detection**:
- `termenv.TrueColor`: 16M colors (24-bit)
- `termenv.ANSI256`: 256 colors (8-bit)  
- `termenv.ANSI`: 16 colors (4-bit)
- `termenv.Ascii`: No color (1-bit)

**Non-TTY Handling**: Set `CLICOLOR_FORCE=1` for color in pipes/redirects, or gracefully fallback to plain text output.

**Source**: Termenv documentation, Glamour integration guides

## 5. Responsive Layout Rules & Performance

### Layout Breakpoint Implementation

```go
func (m model) computeLayout() (leftWidth, mainWidth, rightWidth int) {
    switch {
    case m.width >= 120: // Large
        return int(m.width * 0.25), int(m.width * 0.55), int(m.width * 0.20)
    case m.width >= 80:  // Medium  
        return int(m.width * 0.30), int(m.width * 0.70), 0
    default:             // Small
        return 0, m.width, 0 // Full-width stacked
    }
}
```

### Performance Optimization Strategies

**Alt-Screen Best Practices**:
- Use `tea.WithAltScreen()` program option, not in Init()
- Enables `viewport.HighPerformanceRendering` for scroll-heavy content
- Automatic cleanup on program exit

**Stable Line Count**: Critical for flicker-free updates
- Update content in-place: `"Starting workflows: (1/5) running"` → `"Starting workflows: (2/5) running"`  
- Never add lines during status updates
- Use viewport scrolling for dynamic content

**Framerate Control**: Bubble Tea includes built-in framerate limiting. For high-frequency updates, use `tea.Every()` to cap animation rates.

**Source**: Bubble Tea performance docs, TUI best practices research

## 6. Color and Contrast Policy

### Minimal Monochrome Design

**Base Palette**:
- **Primary**: Grayscale (#FAFAFA, #3C3C3C, #7D7D7D)
- **Accent**: Single subtle color (#7D56F4 purple or #04B575 green)  
- **Borders**: Light gray (#E5E5E5) for structure

**Adaptive Colors**:
```go
lipgloss.AdaptiveColor{Light: "236", Dark: "248"} // Auto light/dark
```

**Contrast Requirements**: Ensure 4.5:1 ratio for text, 3:1 for UI elements per WCAG guidelines.

## 7. Critical Implementation Guidelines

### Message Routing
Route `tea.WindowSizeMsg` to all child models that need dimensions for rendering calculations.

### Error Handling  
Handle `tea.KeyMsg` for quit keys (`q`, `ctrl+c`) in every Update method to prevent application lock-up.

### Testing Strategy
Use `teatest` library with golden files for regression testing UI layouts across different terminal sizes.

## Key Architectural Decisions

1. **Single Renderer**: One `tea.Program` handles all rendering
2. **Initial-Render Gating**: Wait for `WindowSizeMsg` before layout computation  
3. **Responsive Breakpoints**: S/M/L layouts based on terminal width
4. **Configuration-Driven**: All sizes, strings, colors from `/configs/ui.yaml`
5. **Simulator Interface**: Backend simulation without actual tool execution

---

## References

1. Charmbracelet Bubble Tea Documentation: https://github.com/charmbracelet/bubbletea
2. Charmbracelet Bubbles Components: https://github.com/charmbracelet/bubbles  
3. Charmbracelet Lipgloss Styling: https://github.com/charmbracelet/lipgloss
4. Charmbracelet Glamour: https://github.com/charmbracelet/glamour
5. Termenv Color Profiles: https://github.com/muesli/termenv
6. TUI Performance Best Practices: https://leg100.github.io/en/posts/building-bubbletea-programs/
7. Responsive TUI Layout Patterns: Modern TUI framework research (2024-2025)

*Research conducted: January 2025*  
*Target Implementation: Charmbracelet-only TUI with zero overlap, zero flicker*