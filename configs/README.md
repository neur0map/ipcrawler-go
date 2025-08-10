# IPCrawler Configuration Files

This directory contains all configuration files for IPCrawler. These files control various aspects of the application's behavior and can be easily modified without recompiling.

## Configuration Files

### ui.yaml
Controls the Terminal User Interface (TUI) appearance and behavior:
- **Layout**: Panel sizes, breakpoints for responsive design
- **Theme**: Colors for UI elements (primary, accent, borders, etc.)
- **Components**: List behavior, viewport settings, status display
- **Keys**: Keyboard shortcuts and navigation
- **Performance**: Frame rate, rendering optimizations
- **Display**: Timestamps, progress bars, word wrap
- **Workflow**: Execution settings, parallelization

### security.yaml
Security and scanning configuration:
- **Scanning**: Thread limits, timeouts, rate limiting, user agents
- **Detection**: Severity levels, patterns, heuristics
- **Reporting**: Output formats, redaction settings

### output.yaml
Output and logging configuration:
- **Directory**: Where to save scan results
- **Formats**: Enable/disable output formats (JSON, CSV, HTML, PDF)
- **Logging**: Log levels, rotation, compression

## Usage

All configuration files are automatically loaded when IPCrawler starts. If a config file is not found, default values are used.

### Environment Variables

Environment variables can be used to override configuration values.

### Modifying Settings

1. Edit the desired YAML file
2. Save your changes
3. Restart IPCrawler for changes to take effect

### Example: Changing UI Colors

Edit `ui.yaml`:
```yaml
ui:
  theme:
    colors:
      primary: "#00FF00"    # Green primary color
      accent: "#FF00FF"     # Magenta accent
```

### Example: Enabling High Performance Mode

Edit `ui.yaml`:
```yaml
ui:
  performance:
    framerate_cap: 120       # Higher frame rate
    lazy_rendering: false    # Disable lazy rendering
  components:
    viewport:
      high_performance: true # Enable high-performance rendering
      scroll_speed: 5        # Faster scrolling
```



## Priority

Configuration priority (highest to lowest):
1. Command-line flags (if implemented)
2. Environment variables
3. Config files
4. Default values in code