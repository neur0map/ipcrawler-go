# Naabu Integration

Naabu is a fast port scanner written in Go with focus on reliability and simplicity.

## Configuration

- **Output Format**: JSON
- **Command**: `naabu`
- **Default Args**: `-silent`, `-json` for quiet JSON output

## Flag Sets

- `fast`: Scan common ports (80, 443, 8080, 8443)
- `top1000`: Scan top 1000 ports using `-p -`
- `full`: Full port scan using `-p-`

## Example Usage

```yaml
- id: portscan
  tool: naabu
  use_flags: fast
  override_args:
    - "-host"
    - "{{target}}"
  output: "out/{{target}}/ports.json"
```

## Output Format

Returns JSON array with objects containing:
- `ip`: Target IP address
- `port`: Open port number  
- `protocol`: Protocol (tcp/udp)

## Notes

- Requires root privileges for SYN scan
- Best used with `-silent` for clean JSON output
- Fast and reliable for basic port discovery