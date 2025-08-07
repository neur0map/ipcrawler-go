# IPCrawler-Go

A scalable, config-driven wrapper for external CLI tools like naabu, nmap, and others. IPCrawler uses YAML-based tool definitions and workflow DAG execution without hardcoding tool names or logic in Go files.

## Features

- **Config-driven architecture**: No hardcoded tool names - everything loaded from `tools/*/config.yaml`
- **{{target}} templating**: Dynamic target substitution in commands, arguments, and output paths
- **Workflow orchestration**: DAG-based step execution with dependency resolution
- **Parallel execution**: Global throttling with configurable concurrency limits
- **Tool discovery**: Automatic registration from filesystem structure
- **Organized workflows**: Categorized workflow folders (basic/, dns/, etc.)
- **Comprehensive documentation**: Each tool includes example outputs and usage docs

## Project Structure

```
./
├── build/ipcrawler/               # ipcrawler binary
├── cmd/ipcrawler/                 # CLI entrypoint
├── tools/                         # Per-tool folders
│   ├── naabu/
│   │   ├── config.yaml           # ToolConfig (command, output, mappings)
│   │   ├── example_output.json   # Sample raw output (optional)
│   │   └── docs.md               # Optional tool-specific notes
│   ├── nmap/
│   │   ├── config.yaml
│   │   ├── example_output.xml
│   │   └── docs.md
│   ├── ffuf/
│   │   ├── config.yaml
│   │   └── example_output.json
│   └── dig/
│       ├── config.yaml
│       ├── docs.md
│       └── example_output.json
├── workflows/
│   ├── basic/
│   │   ├── simple_scan.yaml
│   │   └── full_fingerprint.yaml
│   └── dns/
│       ├── dns_lookup.yaml
│       └── dns_enum.yaml
├── internal/
│   ├── registry/                  # Loads all tools/*/config.yaml
│   ├── tool/                      # Tool interface + generic adapter
│   ├── parser/                    # JSON/XML parser engine
│   ├── template/                  # {{target}} templating engine
│   ├── config/                    # Global config loader
│   └── workflow/                  # Workflow loader and DAG executor
├── reports/                       # User-facing final outputs (txt, md, json)
├── out/                          # Internal intermediate files
├── global.yaml                   # Global settings for workflows and tools
├── go.mod
├── makefile
└── README.md
```

## Installation

```bash
# Build the binary
make build

# Or install with all dependencies
make install
```

## Usage

```bash
# List available tools and workflows
./build/ipcrawler list

# Run all workflows for a target
./build/ipcrawler example.com

# Run specific workflow
./build/ipcrawler example.com --workflow simple_scan

# Override concurrency limit
./build/ipcrawler example.com --parallel 5
```

## Configuration

### Global Config (`global.yaml`)

```yaml
# Directories to scan for workflow YAML files (supports nested folders)
workflow_folders:
  - "workflows/basic"
  - "workflows/dns"
  - "custom-workflows"
max_concurrent_workflows: 3
default_output_dir: "out"
default_report_dir: "reports"
```

### Tool Config (`tools/{name}/config.yaml`)

```yaml
name: naabu
command: naabu
output: json                       # json, xml, or text
args:
  default: ["-silent", "-json"]
  flags:                          # Reusable flag sets
    fast: ["-p", "80,443,8080,8443"]
    full: ["-p-"]
mappings:                         # Output parsing (future feature)
  - type: port
    query: "[]"
    fields:
      ip: "ip"
      port: "port"
```

### Workflow Config (`workflows/{name}.yaml`)

```yaml
id: portscan_fingerprint
description: Fast port scan + fingerprinting
parallel: true                    # Can run concurrently with other workflows
steps:
  - id: scan_fast
    tool: naabu
    use_flags: fast               # Use predefined flag set
    override_args:                # Additional/override arguments
      - "-host"
      - "{{target}}"             # Template substitution
    output: "out/{{target}}/naabu_fast.json"
    
  - id: merge_results
    type: merge_files             # Built-in step type
    inputs:
      - "{{scan_fast.output}}"   # Reference other step outputs
    output: "out/{{target}}/merged.json"
    depends_on: [scan_fast]       # Dependency resolution
```

## Key Components

### 1. Template Engine (`internal/template/`)
- Replaces `{{target}}` and `{{step.output}}` placeholders
- Recursive application to strings, slices, and nested structures
- Supports step output references: `{{scan_fast.output}}`

### 2. Workflow Executor (`internal/workflow/`)
- Global concurrency control with semaphores
- Parallel workflow throttling (max N concurrent workflows)
- DAG-based step execution with dependency resolution
- Circular dependency detection and validation

### 3. Tool Registry (`internal/registry/`)
- Dynamic tool discovery from `tools/*/config.yaml`
- No hardcoded tool names - everything loaded at runtime
- Generic adapter pattern for consistent tool execution

### 4. Config System (`internal/config/`)
- Global configuration from `.ipcrawlerrc.yaml`
- Searches current directory and `~/.ipcrawlerrc.yaml`
- Graceful fallback to sensible defaults

## Architecture Principles

1. **No Hardcoding**: Tool names, workflow files, and output paths are all configuration-driven
2. **Template-First**: All strings support `{{target}}` substitution
3. **DAG Execution**: Dependency resolution with parallel execution where possible
4. **Global Throttling**: Workflow-level concurrency limits, not step-level
5. **Extensible**: Add new tools by creating `tools/{name}/config.yaml`

## Example Execution

```bash
$ ./build/ipcrawler test.com

IPCrawler starting for target: test.com

Configuration:
  Workflow folders: [workflows custom-workflows]
  Max concurrent workflows: 3
  Output directory: out
  Report directory: reports

Loading tools...
Registered tool: ffuf
Registered tool: naabu  
Registered tool: nmap
Available tools: [ffuf naabu nmap]

Loading workflows...
Found 2 workflow(s) to execute:
  - portscan_fingerprint: Fast port scan + fingerprinting [parallel]
  - simple_scan: Basic port scan [parallel]

Starting workflow execution...
===================================================
Starting parallel workflow: simple_scan (1/3 running)
Starting parallel workflow: portscan_fingerprint (2/3 running)
Executing workflow: simple_scan - Basic port scan with naabu
  Executing step: portscan
    Running tool: naabu -> out/test.com/ports.json
Completed workflow: simple_scan (1 completed)
===================================================

All workflows completed successfully!
Results saved to: out/test.com/
Reports available in: reports/test.com/
```

## Adding New Tools

1. Create `tools/{toolname}/config.yaml`:
```yaml
name: my_tool
command: my_tool
output: json
args:
  default: ["--output", "json"]
  flags:
    fast: ["--quick"]
```

2. Tool is automatically registered and available in workflows:
```yaml
steps:
  - id: my_step
    tool: my_tool
    use_flags: fast
    output: "out/{{target}}/my_tool.json"
```

## Advanced Features

- **Workflow Dependencies**: Steps can depend on other steps via `depends_on`
- **Built-in Step Types**: `merge_files`, future: `filter`, `transform`
- **Flag Inheritance**: Reusable argument sets via `use_flags`
- **Output References**: Use `{{step_id.output}}` to reference other step outputs
- **Global Throttling**: Prevent resource exhaustion with `max_concurrent_workflows`

## Testing

The system includes comprehensive validation:
- Circular dependency detection
- Step ID uniqueness validation
- Tool existence verification
- Output path templating verification
- Workflow syntax validation