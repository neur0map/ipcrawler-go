# Dig Integration

Dig (Domain Information Groper) is a flexible DNS lookup tool.

## Configuration

- **Output Format**: JSON (when available)
- **Command**: `dig`
- **Default Args**: `+json` for JSON output

## Flag Sets

- `short`: Use `+short` for brief answers only
- `trace`: Use `+trace` for full query path
- `verbose`: Use `+verbose` for detailed output

## Example Usage

```yaml
- id: resolve_a
  tool: dig
  override_args:
    - "{{target}}"
    - "A"
    - "+short"
    - "+json"
  output: "out/{{target}}/dns_a.json"
```

## Output Format

Returns JSON with:
- `Status`: Query status code
- `Question[]`: Questions asked
- `Answer[]`: DNS answers with name, type, TTL, data
- `Authority[]`: Authority section
- `Additional[]`: Additional section
- `time_taken`: Query execution time

## Notes

- JSON output requires newer versions of dig
- Fallback to text parsing may be needed
- Supports all DNS record types (A, AAAA, MX, TXT, etc.)
- Use `+short` for simple IP resolution