# IPCrawler - Claude Development Guide

A Go CLI tool for comprehensive network reconnaissance and vulnerability scanning.

> ⚠️ **SECURITY NOTICE**: This is a defensive security tool for authorized testing only. Always ensure proper authorization before scanning any targets.

## Quick Start Commands

### Build & Development
```bash
# Full installation (first time)
make install

# Build only
make

# Update to latest code
make update

# Clean builds
make clean

# Install/update tools only
make install-tools
```

### Running IPCrawler
```bash
# Basic scan
./build/ipcrawler example.com

# With debug output
./build/ipcrawler --debug example.com

# Check health/version
./build/ipcrawler --health
./build/ipcrawler --version
```

## Architecture Overview

### Core Components

1. **CLI Layer** (`cmd/cli.go`) - Main application logic, signal handling, privilege escalation
2. **Core Engine** (`core/`) - Workflow execution, reporting, configuration management
3. **Workflow System** (`workflows/`) - YAML-defined scanning templates
4. **Reporting Pipeline** (`core/reporting/agents/`) - Multi-stage report processing
5. **Privilege Management** (`internal/utils/privilege.go`) - Sudo handling and privilege escalation

### Key Design Patterns

- **Agent-based Architecture**: Modular processing agents (receiver, processor, validator, reporter)
- **Workflow-driven Execution**: YAML templates define scanning sequences
- **Privilege-aware Tools**: Different argument sets for sudo vs normal execution
- **Context-aware Cancellation**: Proper signal handling with cleanup
- **Deferred Directory Creation**: Report directories created after privilege decisions

## Project Structure

```
ipcrawler/
├── main.go                      # Entry point
├── cmd/cli.go                   # CLI logic, signal handling
├── core/
│   ├── config.go               # Configuration management
│   ├── workflow.go             # Workflow loading and execution
│   ├── utils.go                # Utility functions, directory management
│   ├── interactive.go          # User interaction prompts
│   └── reporting/
│       └── agents/             # Report processing pipeline
├── workflows/basic/            # Scanning templates
│   └── scanning/
│       ├── port-discovery.yaml # Fast port discovery with naabu
│       ├── deep-scan.yaml      # Detailed nmap service enumeration
├── internal/utils/             # Internal utilities
└── Makefile                    # Build automation
```

## Key Technical Concepts

### 1. Workflow System
- YAML-based scanning definitions in `workflows/basic/scanning/`
- Dual argument sets: `args_sudo` vs `args_normal` for privilege-aware execution
- Template variables: `{{target}}`, `{{report_dir}}` replaced at runtime
- Tool-specific configurations with validation and error handling

### 2. Privilege Escalation Flow
```go
// Check if tools need sudo
needsSudo := core.CheckPrivilegeRequirements(tools, args)

// Prompt user for decision
useSudo := interactive.PromptForSudo(needsSudo)

// Restart with sudo if needed (before directory creation)
if useSudo && !privilege.IsRunningAsRoot() {
    return privilege.RequestPrivilegeEscalation()
}

// Create report directory AFTER privilege decision
reportDir, err = core.CreateReportDirectory(config.ReportDir, target)
```

### 3. Signal Handling
- Context-based cancellation propagated to all child processes
- Process group management with `syscall.SysProcAttr{Setpgid: true}`
- Terminal cleanup with escape sequences
- UI artifact prevention with `pterm.DisableOutput()`

### 4. Modern Parallel Execution (errgroup)
```go
// Create errgroup with context for automatic error propagation
g, groupCtx := errgroup.WithContext(ctx)

// Set concurrency limit based on configuration
maxConcurrency := config.GetMaxConcurrency(len(workflowKeys))
g.SetLimit(maxConcurrency)

// Execute workflows with automatic cancellation on first error
g.Go(func() error {
    return executeWorkflow(groupCtx, workflowKey, ...)
})

// Wait for all workflows to complete or first error
if err := g.Wait(); err != nil {
    return err // All other workflows automatically cancelled
}
```

### 5. Report Processing Pipeline
1. **Receiver**: Validate and ingest raw tool output
2. **Universal Processor**: Parse and normalize data formats
3. **Validator**: Check data integrity and completeness
4. **Reporter**: Generate human-readable summaries

## Common Development Tasks

### Adding New Scanning Tools

1. **Create workflow YAML** in `workflows/basic/scanning/`:
```yaml
name: "New Tool Scan"
description: "Description of what the tool does"
report:
  enabled: true
  agents: ["receiver", "universal_processor", "validator", "reporter"]
steps:
  - tool: "newtool"
    args_sudo: ["--sudo-specific", "{{target}}"]
    args_normal: ["--normal-mode", "{{target}}"]
```

2. **Add privilege requirements** in `internal/utils/privilege.go`:
```go
func requiresPrivileges(tool string, args []string) bool {
    switch tool {
    case "newtool":
        return checkForPrivilegedFlags(args)
    }
}
```

3. **Add tool installation** in `Makefile`:
```makefile
GO_TOOLS := \
    github.com/example/newtool@latest
```

### Signal Handling Best Practices

```go
// Create cancellable context
ctx, cancel := context.WithCancel(context.Background())

// Set up signal handling
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

go func() {
    sig := <-sigChan
    pterm.DisableOutput()    // Stop UI updates immediately
    cancel()                 // Cancel all operations
    fmt.Fprintf(os.Stderr, "\n⚠️  Received signal: %v\n", sig)
}()

// Pass context to all operations
err := ExecuteWithContext(ctx, command)
```

### Report Directory Management

```go
// Generate path without creating directories (for preview)
previewPath := core.GenerateReportDirectoryPath(baseDir, target)

// Create directories only when ready
reportDir, err := core.CreateReportDirectory(baseDir, target)
```

## Recent Critical Fixes

### Issue 1: Duplicate Directory Creation
**Problem**: Report directories created before privilege escalation, causing duplicates when sudo restarts process.

**Solution**: Deferred directory creation using `GenerateReportDirectoryPath()` for previews, `CreateReportDirectory()` only after privilege decisions.

### Issue 2: Signal Handling Failures  
**Problem**: Ctrl+C not terminating properly, breaking terminal state.

**Solution**: Aggressive timeouts, process group management, terminal escape sequences for cleanup.

### Issue 3: UI Artifacts on Interrupt
**Problem**: Multiple signal handlers showing duplicate messages and UI corruption.

**Solution**: Simplified signal handling, `pterm.DisableOutput()` to stop UI updates, proper context cleanup.

### Issue 4: Parallel Workflow Improvements (2025-01-08)
**Problem**: Manual sync.WaitGroup management, basic error handling, potential system overload.

**Solution**: Implemented `golang.org/x/sync/errgroup` with:
- **Automatic error propagation** - First error cancels all workflows
- **Concurrency limiting** - `SetLimit()` prevents system overload  
- **Better context integration** - Works seamlessly with existing signal handling
- **Configurable limits** - User-configurable concurrency in config.yaml

## Development Guidelines

### Code Style
- Follow Go conventions and existing patterns
- Use context.Context for cancellation throughout
- Implement proper error handling with wrapped errors
- Add comprehensive logging for debugging

### Security Considerations
- Never commit secrets or API keys
- Validate all user inputs and file paths
- Use proper privilege escalation patterns
- Include only defensive security capabilities

### Testing Workflow
1. Build: `make`
2. Test basic functionality: `./build/ipcrawler --health`
3. Test privilege handling: `./build/ipcrawler example.com`
4. Test signal handling: Run scan, press Ctrl+C
5. Verify clean terminal state and no duplicate directories

### Error Recovery
- All tools should handle context cancellation gracefully
- Failed tools should not block pipeline execution
- Report generation should continue even with partial data
- Provide clear error messages with actionable solutions

## Configuration

### config.yaml Structure
```yaml
version: "0.1.1"
default_template: basic
templates:
  - basic

# Parallel execution configuration (NEW)
concurrency:
  max_concurrent_workflows: 4    # Maximum parallel workflows
  enable_auto_limit: true        # Adapt to workflow count
  use_errgroup: true            # Use errgroup for error handling
```

### Concurrency Configuration
- **max_concurrent_workflows**: Limits system resource usage
- **enable_auto_limit**: Automatically reduces concurrency for smaller workloads
- **use_errgroup**: Enables modern error propagation and cancellation

## Tool Dependencies

### Go Tools (installed via `go install`)
- `naabu` - Fast port discovery

### System Tools (via package manager)
- `nmap` - Network discovery and security auditing  


## Build System

The Makefile provides a complete installation experience:
- Cross-platform Go installation (1.24.5)
- Automatic tool dependency management
- Symlink-based installation (no PATH modifications needed)
- Clean build processes with outdated binary removal

---

**Version**: v0.1.1  
**Last Updated**: 2025-01-08  
**Target Go Version**: 1.24.5