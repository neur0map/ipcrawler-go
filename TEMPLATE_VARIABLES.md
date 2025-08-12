# IPCrawler Template Variables Reference

> **The Complete Guide to Template Variables and Commands**

This document serves as the definitive reference for all template variables (`{{variable}}` patterns) available throughout IPCrawler's configuration files, tool definitions, and execution contexts.

## Table of Contents

- [Overview](#overview)
- [Core Variables](#core-variables)
- [Directory Variables](#directory-variables)
- [File Naming Variables](#file-naming-variables)
- [Execution Metadata](#execution-metadata)
- [Output Mode Variables](#output-mode-variables)
- [Usage Examples](#usage-examples)
- [Best Practices](#best-practices)
- [Reference Tables](#reference-tables)

---

## Overview

Template variables in IPCrawler use the `{{variable_name}}` syntax and are resolved at runtime based on the execution context. These variables enable dynamic configuration of:

- Tool command arguments
- Output file paths and names
- Directory structures  
- Log destinations
- Workspace organization

### Variable Resolution Order

1. **Execution Context** - Variables set during tool execution
2. **Configuration Defaults** - Values from config files
3. **Generated Values** - Computed values (timestamps, sanitized names)
4. **Fallback Values** - Safe defaults when other sources fail

---

## Core Variables

### `{{target}}`
**Description**: The primary target being scanned (IP address, hostname, or CIDR range)

**Examples**:
```yaml
# Tool argument
- "{{target}}"

# In tool config
args:
  quick_scan:
    - "-target"
    - "{{target}}"
```

**Generated Values**:
- `192.168.1.1` (single IP)
- `example.com` (hostname)
- `192.168.1.0/24` (CIDR range)
- `192.168.1.1,10.0.0.1` (multiple targets)

### `{{workspace}}`
**Description**: The base workspace directory for the current target

**Pattern**: `./workspace/{sanitized_target}/`

**Examples**:
```yaml
# Config usage
directory: "{{workspace}}/logs/info/"

# Resolves to
directory: "./workspace/192_168_1_1/logs/info/"
```

### `{{output_dir}}`
**Description**: Alias for `{{workspace}}` - maintained for backward compatibility

**Usage**: Identical to `{{workspace}}`

---

## Directory Variables

These variables point to specific subdirectories within the target workspace:

### `{{logs_dir}}`
**Description**: Directory for application and tool logs
**Default**: `{{workspace}}/logs/`

### `{{scans_dir}}` 
**Description**: Directory for scan results and tool output
**Default**: `{{workspace}}/scans/`

### `{{reports_dir}}`
**Description**: Directory for generated reports and summaries  
**Default**: `{{workspace}}/reports/`

### `{{raw_dir}}`
**Description**: Directory for raw tool output and temporary files
**Default**: `{{workspace}}/raw/`

**Directory Structure Example**:
```
workspace/
└── 192_168_1_1/
    ├── logs/
    │   ├── info/
    │   ├── error/
    │   ├── warning/
    │   └── debug/
    ├── scans/          # {{scans_dir}}
    ├── reports/        # {{reports_dir}}
    └── raw/           # {{raw_dir}}
```

---

## File Naming Variables

### `{{output_file}}`
**Description**: Generated filename for tool output (behavior depends on `scan_output_mode`)

**Naming Patterns**:
- **overwrite mode**: `{tool}_{target}` (e.g., `nmap_192_168_1_1`)
- **timestamp mode**: `{tool}_{target}_{timestamp}` (e.g., `nmap_192_168_1_1_20240112_143052`)
- **both mode**: `{tool}_{target}_{timestamp}` (e.g., `nmap_192_168_1_1_20240112_143052`)

### `{{output_file_latest}}`
**Description**: Latest version filename (only available when `scan_output_mode: "both"`)

**Pattern**: `{tool}_{target}_latest` (e.g., `nmap_192_168_1_1_latest`)

### `{{output_path}}`
**Description**: Full file path combining directory and filename

**Example**: `{{scans_dir}}/{{output_file}}` → `./workspace/192_168_1_1/scans/nmap_192_168_1_1_20240112_143052.xml`

### `{{output_path_latest}}`
**Description**: Full path to latest version file (when available)

**Example**: `{{scans_dir}}/{{output_file_latest}}` → `./workspace/192_168_1_1/scans/nmap_192_168_1_1_latest.xml`

---

## Execution Metadata

### `{{timestamp}}`
**Description**: Execution timestamp in `YYYYMMDD_HHMMSS` format

**Example**: `20240112_143052`

**Usage**:
```yaml
# In filenames
output_file: "scan_{{timestamp}}.json"

# In content
header: "Scan started at {{timestamp}}"
```

### `{{session_id}}`
**Description**: Unique session identifier for this execution

**Pattern**: `session_{unix_timestamp}`

**Example**: `session_1705072232`

### `{{tool_name}}`
**Description**: Name of the currently executing tool

**Examples**: `nmap`, `naabu`, `gobuster`

### `{{mode}}`
**Description**: Execution mode/profile for the tool

**Examples**: `aggressive`, `quick_scan`, `stealth`, `all_ports`

---

## Output Mode Variables

**New in v1.1+**: Enhanced file management with configurable output modes

### Configuration
```yaml
# configs/output.yaml
output:
  scan_output_mode: "both"  # "overwrite" | "timestamp" | "both"
  create_latest_links: true
```

### Mode Behaviors

#### `"overwrite"` Mode
- **Files**: `{{output_file}}` → `nmap_192_168_1_1.xml`
- **Behavior**: Each scan overwrites the previous file
- **Use Case**: When you only need the latest results

#### `"timestamp"` Mode  
- **Files**: `{{output_file}}` → `nmap_192_168_1_1_20240112_143052.xml`
- **Behavior**: Each scan creates a new timestamped file
- **Use Case**: When you need complete scan history

#### `"both"` Mode (Default)
- **Files**: 
  - `{{output_file}}` → `nmap_192_168_1_1_20240112_143052.xml`
  - `{{output_file_latest}}` → `nmap_192_168_1_1_latest.xml` (symlink)
- **Behavior**: Creates timestamped files + latest symlinks
- **Use Case**: Best of both worlds - history + easy access to current

---

## Usage Examples

### Tool Configuration (nmap)
```yaml
# tools/nmap/config.yaml
tool: "nmap"

args:
  aggressive:
    - "-sV"
    - "-O" 
    - "-A"
    - "-oA"
    - "{{scans_dir}}/{{output_file}}"
    - "{{target}}"
```

### Output Configuration
```yaml
# configs/output.yaml
output:
  workspace_base: "./workspace"
  
  info:
    directory: "{{workspace}}/logs/info/"
    level: "info"
    
  scans:
    directory: "{{workspace}}/scans/"
```

### Workflow Definition
```yaml
# workflows/reconnaissance/port-scanning.yaml
name: "Port Scanning"
steps:
  - name: "Fast Discovery"
    tool: "naabu"
    output: "{{scans_dir}}/ports_{{timestamp}}.json"
  
  - name: "Service Enumeration"  
    tool: "nmap"
    input: "{{scans_dir}}/ports_{{timestamp}}.json"
    output: "{{scans_dir}}/services_{{timestamp}}.xml"
```

### Runtime Resolution Example
```bash
# Configuration:
target: "192.168.1.100"
tool: "nmap"
mode: "aggressive"
scan_output_mode: "both"

# Variables resolve to:
{{target}} → "192.168.1.100"
{{workspace}} → "./workspace/192_168_1_100"
{{scans_dir}} → "./workspace/192_168_1_100/scans"
{{output_file}} → "nmap_192_168_1_100_20240112_143052"
{{output_file_latest}} → "nmap_192_168_1_100_latest"
{{output_path}} → "./workspace/192_168_1_100/scans/nmap_192_168_1_100_20240112_143052.xml"

# Final command:
nmap -sV -O -A -oA ./workspace/192_168_1_100/scans/nmap_192_168_1_100_20240112_143052 192.168.1.100
```

---

## Best Practices

### 1. **Use Appropriate Directory Variables**
```yaml
# ✅ Good - Use specific directory variables
output: "{{scans_dir}}/{{output_file}}"
logs: "{{logs_dir}}/debug.log"

# ❌ Avoid - Hardcoded paths
output: "./workspace/scans/output.xml"
```

### 2. **Leverage Output Modes**
```yaml
# ✅ Good - Let IPCrawler handle file naming
output: "{{scans_dir}}/{{output_file}}"

# ❌ Avoid - Manual timestamp handling
output: "{{scans_dir}}/scan_$(date +%Y%m%d_%H%M%S).xml"
```

### 3. **Target Sanitization**
The `{{target}}` variable is automatically sanitized for use in filenames:
- `192.168.1.1` → `192_168_1_1`
- `example.com` → `example_com`  
- `192.168.1.0/24` → `192_168_1_0_24`

### 4. **Variable Availability**
Not all variables are available in all contexts:

| Variable | Tool Args | Config Files | Workflows | Runtime |
|----------|-----------|--------------|-----------|---------|
| `{{target}}` | ✅ | ❌ | ✅ | ✅ |
| `{{workspace}}` | ✅ | ✅ | ✅ | ✅ |
| `{{output_file}}` | ✅ | ❌ | ✅ | ✅ |
| `{{timestamp}}` | ✅ | ❌ | ✅ | ✅ |

---

## Reference Tables

### Complete Variable List

| Variable | Type | Description | Example Value |
|----------|------|-------------|---------------|
| `{{target}}` | Core | Target being scanned | `192.168.1.1` |
| `{{workspace}}` | Core | Base workspace directory | `./workspace/192_168_1_1` |
| `{{output_dir}}` | Core | Alias for workspace | `./workspace/192_168_1_1` |
| `{{logs_dir}}` | Directory | Logs directory path | `./workspace/192_168_1_1/logs` |
| `{{scans_dir}}` | Directory | Scans directory path | `./workspace/192_168_1_1/scans` |
| `{{reports_dir}}` | Directory | Reports directory path | `./workspace/192_168_1_1/reports` |
| `{{raw_dir}}` | Directory | Raw output directory path | `./workspace/192_168_1_1/raw` |
| `{{output_file}}` | File | Generated output filename | `nmap_192_168_1_1_20240112_143052` |
| `{{output_file_latest}}` | File | Latest version filename | `nmap_192_168_1_1_latest` |
| `{{output_path}}` | File | Full output file path | `./workspace/.../nmap_...xml` |
| `{{output_path_latest}}` | File | Full latest file path | `./workspace/.../nmap_..._latest.xml` |
| `{{timestamp}}` | Metadata | Execution timestamp | `20240112_143052` |
| `{{session_id}}` | Metadata | Session identifier | `session_1705072232` |
| `{{tool_name}}` | Metadata | Current tool name | `nmap` |
| `{{mode}}` | Metadata | Execution mode | `aggressive` |

### Filename Patterns by Output Mode

| Mode | `{{output_file}}` Pattern | `{{output_file_latest}}` | Symlink Created |
|------|---------------------------|--------------------------|-----------------|
| `overwrite` | `{tool}_{target}` | Not available | No |
| `timestamp` | `{tool}_{target}_{timestamp}` | Not available | No |
| `both` | `{tool}_{target}_{timestamp}` | `{tool}_{target}_latest` | Yes |

---

## Version History

- **v1.1** - Added output mode variables (`{{output_file_latest}}`, `{{output_path_latest}}`)
- **v1.0** - Initial template variable system

---

> **📝 Note**: This documentation is automatically generated from the IPCrawler source code. Variables and patterns may be extended in future versions. Always refer to the latest version of this file for accurate information.