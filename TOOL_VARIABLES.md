# IPCrawler Tool-Specific Variables

> **Complete Guide to Tool-Specific Template Variables**

This document provides comprehensive documentation for tool-specific template variables used in IPCrawler tool configurations. These variables extend the core template system with tool-specific functionality.

## Table of Contents

- [Overview](#overview)
- [Naabu Variables](#naabu-variables)
- [Variable Types](#variable-types)
- [Usage Examples](#usage-examples)
- [Best Practices](#best-practices)

---

## Overview

Tool-specific variables complement the core template variables documented in `TEMPLATE_VARIABLES.md`. They provide:

- **Tool Customization**: Specific parameters for individual tools
- **Dynamic Configuration**: Runtime variable resolution
- **Type Safety**: Defined variable types and validation
- **Default Values**: Sensible fallbacks for common use cases

### Variable Resolution Priority

1. **User Input** - Values provided during workflow execution
2. **Tool Defaults** - Values defined in tool configuration
3. **System Defaults** - Fallback values from template resolver

---

## Naabu Variables

### Core Template Variables Used
- `{{target}}` - Target host/IP/CIDR to scan
- `{{scans_dir}}` - Directory for scan output files
- `{{output_file}}` - Generated output filename

### Tool-Specific Variables

#### `{{ports}}`
**Description**: Comma-separated list of ports to scan

**Type**: `string`

**Default**: `"80,443"`

**Usage**:
```yaml
# Custom port scanning
custom_ports:
  args:
    - "-p"
    - "{{ports}}"
```

**Examples**:
- `"22,80,443,8080,8443"` - Common web and SSH ports
- `"21-25,53,80,110,143,443,993,995"` - Email and web services
- `"u:53,u:161"` - UDP ports (DNS, SNMP)

#### `{{rate}}`
**Description**: Scan rate in packets per second

**Type**: `integer`

**Default**: `"1000"`

**Usage**:
```yaml
# High-speed scanning
top1000_scan:
  args:
    - "-rate"
    - "{{rate}}"
```

**Examples**:
- `"500"` - Conservative rate for stable networks
- `"1000"` - Standard rate for most environments
- `"2000"` - High-speed rate for fast networks
- `"100"` - Stealth rate to avoid detection

---

## Variable Types

### String Variables
- **Format**: Text values, possibly comma-separated
- **Examples**: `{{ports}}`, `{{target}}`
- **Validation**: Pattern matching, character restrictions

### Integer Variables
- **Format**: Numeric values
- **Examples**: `{{rate}}`
- **Validation**: Range checking, type conversion

### Boolean Variables
- **Format**: `true`/`false` values
- **Usage**: Feature flags, mode switches
- **Validation**: Boolean conversion

---

## Usage Examples

### 1. Naabu Fast Scan
```yaml
# Configuration
mode: "fast_scan"

# Command generated:
# naabu -host {{target}} -top-ports 100 -rate 1000 -json -o {{scans_dir}}/{{output_file}}.json -silent
```

### 2. Custom Port Scan
```yaml
# User input
ports: "22,80,443,8080,8443"

# Configuration
mode: "custom_ports"

# Command generated:
# naabu -host {{target}} -p 22,80,443,8080,8443 -json -o {{scans_dir}}/{{output_file}}.json -silent
```

### 3. High-Speed Scan
```yaml
# User input
rate: "2000"

# Configuration
mode: "top1000_scan"

# Command generated:
# naabu -host {{target}} -top-ports 1000 -rate 2000 -json -o {{scans_dir}}/{{output_file}}.json -silent
```

### 4. Stealth Scan
```yaml
# Configuration
mode: "stealth_scan"

# Command generated:
# naabu -host {{target}} -scan-type s -top-ports 100 -rate 100 -timeout 5000 -json -o {{scans_dir}}/{{output_file}}.json -silent
```

---

## Best Practices

### 1. **Use Appropriate Scan Modes**
```yaml
# ✅ Good - Use specific modes for different scenarios
fast_scan: # For quick discovery
connect_scan: # For firewalled environments  
syn_scan: # For comprehensive scanning (requires root)

# ❌ Avoid - Hardcoding scan parameters
args: ["-host", "target.com", "-p", "80,443"]
```

### 2. **Leverage Variable Defaults**
```yaml
# ✅ Good - Use defaults when possible
{{ports}} # Resolves to "80,443" if not specified

# ✅ Good - Override when needed
ports: "22,80,443,8080,8443"
```

### 3. **Choose Appropriate Scan Rates**
```yaml
# ✅ Good - Match rate to environment
rate: "100"   # Stealth/slow networks
rate: "1000"  # Standard environments
rate: "2000"  # Fast/internal networks

# ❌ Avoid - Excessive rates that cause false negatives
rate: "10000" # Too fast, likely inaccurate
```

### 4. **Privilege-Aware Mode Selection**
```yaml
# ✅ Good - Choose based on available privileges
requires_sudo: false # connect_scan, fast_scan, custom_ports
requires_sudo: true  # syn_scan, comprehensive_scan, host_discovery

# ✅ Good - Fallback modes
# If sudo not available, fall back to connect_scan
```

---

## Tool Configuration Structure

### Mode Definition
```yaml
mode_name:
  description: "Human-readable description"
  requires_sudo: true|false
  args:
    - "-flag"
    - "{{variable}}"
    - "static_value"
```

### Variable Definition
```yaml
variables:
  variable_name:
    description: "Variable purpose and usage"
    default: "default_value"
    example: "example_value"
    type: "string|integer|boolean"
```

---

## Future Tool Variables

### Planned Extensions
- **Nmap Variables**: `{{scripts}}`, `{{timing}}`, `{{scan_type}}`
- **Gobuster Variables**: `{{wordlist}}`, `{{extensions}}`, `{{threads}}`
- **Nuclei Variables**: `{{templates}}`, `{{severity}}`, `{{tags}}`

### Variable Naming Convention
- Use lowercase with underscores: `{{scan_type}}`
- Be descriptive: `{{custom_ports}}` not `{{cp}}`
- Match tool flag names when possible: `{{rate}}` for `-rate`

---

## Integration with Core Variables

### Combined Usage
```yaml
# Example combining core and tool-specific variables
args:
  - "-host"
  - "{{target}}"           # Core variable
  - "-p"  
  - "{{ports}}"            # Tool-specific variable
  - "-rate"
  - "{{rate}}"             # Tool-specific variable
  - "-o"
  - "{{scans_dir}}/{{output_file}}.json"  # Core variables
```

### Template Resolution Order
1. **Core variables** resolved first (target, directories, files)
2. **Tool-specific variables** resolved second (ports, rates, etc.)
3. **Static values** used as-is

---

## Version History

- **v1.0** - Initial tool-specific variable system
- **v1.1** - Added naabu variables (`{{ports}}`, `{{rate}}`)

---

> **📝 Note**: Always refer to individual tool configuration files for the most current variable definitions and usage examples.