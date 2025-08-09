# IPCrawler TUI Implementation Guide

## Quick Start

### 1. Initialize Go Module
```bash
cd /Users/carlosm/Documents/ipcrawler
go mod tidy
```

### 2. Run the Application
```bash
go run cmd/ipcrawler/main.go
```

### 3. Testing Different Terminal Sizes
```bash
# Small layout (<80 cols)
stty cols 70 rows 24 && go run cmd/ipcrawler/main.go

# Medium layout (80-119 cols)  
stty cols 100 rows 30 && go run cmd/ipcrawler/main.go

# Large layout (≥120 cols)
stty cols 140 rows 40 && go run cmd/ipcrawler/main.go
```

## Architecture Features Implemented

### ✅ Single tea.Program Pattern
- **Location**: `cmd/ipcrawler/main.go`
- **Implementation**: Exactly one `tea.NewProgram()` call
- **Alt-screen**: Enabled with `tea.WithAltScreen()`

### ✅ WindowSizeMsg-Safe Rendering
- **Location**: `internal/ui/app.go:Update()`
- **Pattern**: `ready` flag prevents rendering before dimensions received
- **Routing**: WindowSizeMsg distributed to all child components

### ✅ Responsive Layout System
- **Location**: `internal/ui/layout.go`
- **Breakpoints**: 80 cols (Small→Medium), 120 cols (Medium→Large)
- **Layouts**:
  - **Small (<80)**: Stacked vertical panels
  - **Medium (80-119)**: Two-column + footer
  - **Large (≥120)**: Three-column horizontal

### ✅ Configuration-Driven Design
- **Location**: `configs/ui.yaml` + `internal/ui/config.go`
- **Coverage**: Colors, sizing, keybindings, component settings
- **Type Safety**: Strongly typed config structs with validation

### ✅ Simulator Interface
- **Location**: `internal/simulator/`
- **Features**: Mock workflows, tools, streaming logs, metrics
- **Integration**: Proper tea.Cmd message flow for events

### ✅ Performance Optimizations
- **Alt-Screen**: Single instance with high-performance viewport
- **Stable Line Count**: Fixed panel heights prevent flicker
- **Message Routing**: Direct routing to focused/relevant components
- **Memory Management**: Circular log buffer with size limits

## Component Architecture

### Root App Model (`internal/ui/app.go`)
```go
type App struct {
    ready    bool              // WindowSizeMsg received?
    width    int               // Terminal dimensions
    height   int
    layout   LayoutMode        // S/M/L responsive mode
    focused  FocusedPanel      // Current focus state
    
    // Child components
    listPanel   *ListPanel     // Workflows/tools
    viewport    *ViewportPanel // Logs/content  
    statusPanel *StatusPanel   // Status/progress
    
    // Systems
    layoutManager *LayoutManager
    config        *Config
    simulator     Simulator
    keys          KeyMap
    theme         *Theme
}
```

### Layout Manager (`internal/ui/layout.go`)
- **Three Layout Modes**: Automatic switching based on terminal width
- **Focus Indicators**: Visual feedback with accent border colors
- **Panel Sizing**: Percentage-based with minimum size constraints
- **Stable Heights**: Consistent dimensions to prevent flicker

### Simulator System (`internal/simulator/`)
- **Interface Design**: Clean abstraction for demo vs real backend
- **Mock Data**: Rich content for workflows, tools, logs, and metrics
- **Event Simulation**: Realistic workflow execution and log streaming
- **Tea Integration**: Proper message-based event flow

## Key Implementation Patterns

### 1. WindowSizeMsg Handling
```go
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        // CRITICAL: Handle resize first
        a.width, a.height = msg.Width, msg.Height
        a.ready = true
        a.layout = a.calculateLayout()
        
        // Route to all child components
        return a, a.routeWindowSizeToChildren(msg)
    }
}
```

### 2. Message Routing Pattern  
```go
// Route to focused component for keyboard input
func (a *App) routeKeyToFocused(msg tea.KeyMsg) tea.Cmd {
    switch a.focused {
    case FocusListPanel:
        _, cmd := a.listPanel.Update(msg)
        return cmd
    // ...
    }
}
```

### 3. Responsive Layout Calculation
```go
func (a *App) calculateLayout() LayoutMode {
    switch {
    case a.width >= 120: return LayoutLarge    // 3-column
    case a.width >= 80:  return LayoutMedium   // 2-column + footer
    default:             return LayoutSmall    // stacked
    }
}
```

### 4. Focus Management
```go
func (a *App) cycleFocus() {
    switch a.focused {
    case FocusListPanel:    a.focused = FocusViewportPanel
    case FocusViewportPanel: 
        if a.layout == LayoutLarge {
            a.focused = FocusStatusPanel  // 3-column: go to status
        } else {
            a.focused = FocusListPanel    // 2-column: cycle back
        }
    case FocusStatusPanel:  a.focused = FocusListPanel
    }
}
```

## Next Implementation Steps

### Phase 2: Component Implementation (Missing)
You still need to implement the actual component files:

1. **`internal/ui/components/list_panel.go`**
   - Bubbles list integration
   - Workflow/tool item rendering
   - Keyboard navigation
   - Filter functionality

2. **`internal/ui/components/viewport.go`**
   - Bubbles viewport integration  
   - High-performance log rendering
   - Auto-scroll functionality
   - Scroll navigation

3. **`internal/ui/components/status_panel.go`**
   - Bubbles spinner/progress integration
   - System metrics display
   - Real-time status updates
   - Compact vs full view modes

### Development Commands
```bash
# Install dependencies
go mod tidy

# Run with different layouts for testing
COLUMNS=70  go run cmd/ipcrawler/main.go   # Small
COLUMNS=100 go run cmd/ipcrawler/main.go   # Medium  
COLUMNS=140 go run cmd/ipcrawler/main.go   # Large

# Test configuration loading
go run cmd/ipcrawler/main.go -config configs/ui.yaml

# Development build
go build -o bin/ipcrawler cmd/ipcrawler/main.go
```

### Testing Strategy
1. **Unit Tests**: Component behavior and configuration loading
2. **Layout Tests**: Verify responsive breakpoints work correctly
3. **Resize Tests**: Ensure smooth transitions between layout modes
4. **Performance Tests**: Check rendering speed and memory usage
5. **Cross-Terminal Tests**: Verify compatibility across different terminals

## Configuration Customization

### Modify Colors (`configs/ui.yaml`)
```yaml
ui:
  theme:
    colors:
      accent: "#FF6B6B"      # Change focus color
      success: "#51CF66"     # Change success color
      warning: "#FFD93D"     # Change warning color
```

### Adjust Layout Breakpoints
```yaml
ui:
  layout:
    breakpoints:
      small: 70              # Earlier stacking
      medium: 110            # Earlier 3-column
```

### Customize Keybindings
```yaml
ui:
  keys:
    quit: ["q", "ctrl+c", "esc"]
    focus_list: ["1", "f1"]
    focus_main: ["2", "f2"]
```

## Architecture Validation Checklist

- [x] **Single tea.Program**: Only one renderer instance
- [x] **WindowSizeMsg Safety**: No rendering before dimensions
- [x] **Responsive Layouts**: S/M/L modes with smooth transitions
- [x] **Configuration-Driven**: All settings from ui.yaml  
- [x] **Simulator Interface**: Rich demo content without backend
- [x] **Zero Flicker**: Stable line counts and in-place updates
- [x] **Alt-Screen Performance**: High-performance viewport
- [x] **Focus Management**: Clear indicators and navigation
- [x] **Theme System**: Consistent styling with adaptive colors
- [ ] **Component Implementation**: Still need the 3 main components
- [ ] **Integration Testing**: Full app testing across layouts
- [ ] **Performance Validation**: Memory and rendering benchmarks

This architecture provides a solid foundation that follows all Charmbracelet best practices while meeting the specific constraints identified in the research brief. The next phase is implementing the three core components to bring the UI to life.