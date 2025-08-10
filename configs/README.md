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
- **os_detection**: Enable OS detection heuristics
- **execution.tools_root**: Absolute/relative root where tools must reside
- **execution.args_validation**: Validate arguments before execution
- **execution.exec_validation**: Validate executables before execution

### output.yaml
Output and logging configuration:
- **timestamp/time_format**: Timestamp emission and format
- **info/error/warning/debug**: Directories, log levels, and filenames per sink
- **raw**: Location for raw tool output

### tools.yaml
Global tool execution policy:
- **tool_execution.max_concurrent_executions**: How many tools can be in-flight
- **tool_execution.max_parallel_executions**: How many run simultaneously
- **default_timeout_seconds**: Fallback timeout for tools
- **retry_attempts**: Default retry count
- **argv_policy**:
  - **max_args / max_arg_bytes / max_argv_bytes**: Argument limits
  - **deny_shell_metachars**: Reject shell metacharacters in args
  - **allowed_char_classes**: Allowed character classes in args
- **execution**:
  - **tools_path**: Root path where tool binaries must resolve
  - **args_validation**: Enable argument validation
  - **exec_validation**: Enable executable validation

## Usage

All configuration files are automatically loaded when IPCrawler starts. If a config file is not found, default values are used.

### Modifying Settings

1. Edit the desired YAML file
2. Save your changes
3. Restart IPCrawler for changes to take effect

## Examples

### Changing UI Colors
```yaml
ui:
  theme:
    colors:
      primary: "#00FF00"
      accent: "#FF00FF"
```

### Enabling High Performance Mode
```yaml
ui:
  performance:
    framerate_cap: 120
    lazy_rendering: false
  components:
    viewport:
      high_performance: true
      scroll_speed: 5
```

### Adjust tool parallelism and safety (tools.yaml)
```yaml
tool_execution:
  max_concurrent_executions: 3
  max_parallel_executions: 2

default_timeout_seconds: 1800
retry_attempts: 1

argv_policy:
  max_args: 64
  max_arg_bytes: 512
  max_argv_bytes: 16384
  deny_shell_metachars: true
  allowed_char_classes: [alnum, "-", "_", ".", ":", "/", "="]

execution:
  tools_path: "./tools"
  args_validation: true
  exec_validation: true
```

## Priority

Configuration priority (highest to lowest):
1. Command-line flags (if implemented)
2. Environment variables
3. Config files
4. Default values in code