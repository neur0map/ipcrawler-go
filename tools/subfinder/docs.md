# Subfinder - Subdomain Discovery Tool

## Overview
Subfinder is a passive subdomain discovery tool designed for legitimate reconnaissance activities. It uses certificate transparency logs, search engines, and various APIs to enumerate subdomains without requiring wordlists.

## Purpose
- **Passive reconnaissance** - No direct queries to target infrastructure
- **Certificate transparency** - Leverages CT logs (crt.sh, Censys, etc.)
- **Vhost discovery** - Finds virtual hosts like planning.htb from hackthebox
- **API enumeration** - Uses multiple passive sources

## Key Features
- Zero wordlist dependency
- Fast passive enumeration
- JSON output for pipeline integration
- Multiple source APIs
- Certificate transparency integration

## Flag Usage

### Basic Flags
- `passive_only`: Use only passive sources (recommended for stealth)
- `all_sources`: Query all available API sources
- `ct_only`: Use only certificate transparency sources

### Performance Flags
- `timeout_short`: 5-second timeout for quick scans
- `timeout_long`: 30-second timeout for thorough scans
- `fast_scan`: Optimized for speed with passive sources

### Advanced Flags
- `recursive`: Discover subdomains of discovered subdomains
- `comprehensive`: All sources with recursive discovery
- `verbose`: Enable detailed logging

## Security Considerations
- Passive tool - does not directly probe target infrastructure
- Uses public data sources only
- May reveal internal naming conventions
- Some sources may log your IP address

## Example Use Cases
1. **HTB/CTF Discovery**: Find planning.htb, dev.htb, etc.
2. **Bug Bounty**: Passive subdomain enumeration
3. **Penetration Testing**: Initial reconnaissance phase
4. **Asset Discovery**: Map organization's external footprint

## Integration Notes
- Designed for IPCrawler's configuration-driven architecture
- Follows zero hardcoding principles
- Outputs structured JSON for downstream processing
- Integrates with workflow dependency management