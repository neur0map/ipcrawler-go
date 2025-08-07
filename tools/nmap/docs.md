# Nmap Integration

Nmap is the most popular network discovery and security auditing tool.

## Configuration

- **Output Format**: XML
- **Command**: `nmap`
- **Default Args**: `-oX -` for XML output to stdout

## Flag Sets

- `fast`: Fast scan with `-F` (top 100 ports)
- `intense`: Intensive scan with `-T4 -A -v` (aggressive timing, OS detection, version detection, script scanning)
- `stealth`: Stealth scan with `-sS -sV -T2` (SYN scan, version detection, slower timing)

## Example Usage

```yaml
- id: fingerprint
  tool: nmap
  use_flags: intense
  override_args:
    - "-iL"
    - "{{merged_ports.output}}"
  output: "out/{{target}}/nmap_fingerprint.xml"
  depends_on: [merged_ports]
```

## Output Format

Returns XML with detailed host information including:
- Host status and addresses
- Open ports with service detection
- Operating system fingerprinting (with -O)
- Service versions (with -sV)
- Script results (with -sC or -A)

## Notes

- Requires root privileges for some scan types
- XML output provides the most complete structured data
- Use `-iL` to read targets from file
- Timing templates: T0-T5 (paranoid to insane)