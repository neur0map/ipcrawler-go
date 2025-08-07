# NSLookup Tool

DNS lookup tool for domain name resolution and record queries.

## Purpose
- DNS record resolution
- Multiple record type support (A, NS, MX, TXT, etc.)
- Cross-platform DNS queries

## Usage
- Query A records: `nslookup domain.com`
- Query specific type: `nslookup -type=MX domain.com`
- Debug mode: `nslookup -debug domain.com`

## Output Format
Plain text output with DNS resolution results.

## Common Record Types
- A: IPv4 addresses
- AAAA: IPv6 addresses
- NS: Name servers
- MX: Mail exchange servers
- TXT: Text records
- CNAME: Canonical names