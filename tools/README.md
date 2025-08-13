# Tools Directory

This directory contains tool configuration files but **NOT** the actual tool binaries.

## Tool Installation

Tools must be installed separately on the system before IPCrawler can use them. By default, IPCrawler will find tools in your system PATH. You can optionally configure a specific `tools_path` to restrict tool execution to a particular directory.

### Naabu Installation

```bash
# Install naabu using go
go install -v github.com/projectdiscovery/naabu/v2/cmd/naabu@latest

# Verify installation
naabu -version

# The tool should now be available in your PATH
which naabu
```

### Tool Configuration

Each tool has its own subdirectory with a `config.yaml` file that defines:
- Execution modes and their arguments
- Output formats and file naming
- Tool-specific variables and templates

Example structure:
```
tools/
├── README.md
├── naabu/
│   └── config.yaml
├── nmap/
│   └── config.yaml
└── reusable.yaml
```

### Security Notes

- Tools are executed with security validation enabled
- By default, tools can be executed from system PATH (configurable)
- Set `tools_root` in security.yaml to restrict execution to specific directories
- Arguments are validated against security policies  
- Path traversal and shell injection attempts are blocked

### Adding New Tools

1. Create a new directory under `tools/` with the tool name
2. Add a `config.yaml` file with tool configuration
3. Ensure the tool binary is installed in your system PATH (or configured tools directory)
4. Test execution through IPCrawler

See existing tool configurations for examples.