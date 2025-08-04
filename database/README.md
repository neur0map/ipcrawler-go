# IPCrawler Database

This directory contains JSON databases for security assessments, replacing hardcoded values in Go files with maintainable, reusable data structures.

## Structure

```
database/
├── ports/
│   └── high_risk_ports.json      # High-risk port definitions
├── services/
│   └── common_services.json      # Common service information  
├── vulnerabilities/
│   └── risk_levels.json          # Risk assessment criteria
└── wordlists/                    # Future: wordlists for directory scanning
```

## Usage

### Loading Database

```go
import "ipcrawler/core/database"

// Get global database instance (singleton with caching)
db := database.GetGlobalDatabase()

// Check if port is high-risk
isHighRisk, portInfo, err := db.IsHighRiskPort(22)
if err != nil {
    log.Printf("Database error: %v", err)
} else if isHighRisk {
    log.Printf("High-risk port detected: %s", portInfo.Description)
}

// Get service information
serviceInfo, err := db.GetServiceInfo("SSH", 22)
if err != nil {
    log.Printf("Database error: %v", err)
} else if serviceInfo != nil {
    log.Printf("Service: %s on port %d", serviceInfo.Name, serviceInfo.Port)
}

// Calculate comprehensive risk score
score, level, err := db.CalculateRiskScore(22, "ssh", false, true, true)
if err != nil {
    log.Printf("Risk calculation error: %v", err)
} else {
    log.Printf("Risk score: %.1f (%s)", score, level)
}
```

### Adding New Data

1. **High-Risk Ports**: Edit `ports/high_risk_ports.json`
   - Add new ports to appropriate categories
   - Include risk level, description, and common attacks

2. **Services**: Edit `services/common_services.json`
   - Add service information by category
   - Include security checks and default credentials

3. **Risk Levels**: Edit `vulnerabilities/risk_levels.json`
   - Modify risk calculation factors
   - Update port risk mappings

## Performance

- **JSON parsing**: ~50-100μs for typical files
- **Caching**: Data loaded once, cached in memory
- **Thread-safe**: Uses sync.RWMutex for concurrent access
- **Memory efficient**: Only loads requested databases

## Benefits Over Hardcoded Values

1. **Maintainability**: Easy to update without recompiling
2. **Reusability**: Other tools can use the same database
3. **Version Control**: Track changes to security definitions
4. **Collaboration**: Security teams can contribute to databases
5. **Customization**: Organizations can maintain custom definitions
6. **Performance**: Fast JSON parsing with in-memory caching

## File Format Standards

All JSON files follow this structure:

```json
{
  "metadata": {
    "name": "Database Name",
    "description": "Purpose and scope",
    "version": "1.0.0",
    "last_updated": "2025-08-04",
    "source": "OWASP, NIST, etc."
  },
  "data": {
    // Actual database content
  }
}
```

## Contributing

When updating database files:

1. Increment version number in metadata
2. Update `last_updated` field
3. Test with `go test ./core/database/...`
4. Document changes in commit message

## Future Enhancements

- [ ] Wordlist databases for directory scanning
- [ ] CVE mappings for vulnerabilities
- [ ] Custom user-defined databases
- [ ] Database validation schemas
- [ ] Auto-update mechanisms