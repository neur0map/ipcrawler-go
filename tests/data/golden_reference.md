# Golden Frame Test Reference

This document describes the golden frame testing methodology for the IPCrawler TUI.

## Overview

Golden frame tests validate that the TUI renders consistently across different terminal sizes and maintains visual stability over time.

## Test Terminal Sizes

The following terminal dimensions are tested to ensure comprehensive coverage:

### Standard Sizes
- **80x24**: Classic terminal size, triggers medium layout
- **100x30**: Wide medium layout  
- **120x40**: Large layout threshold
- **160x48**: Wide large layout

### Edge Cases  
- **60x20**: Small terminal, triggers stacked layout
- **40x15**: Very small terminal
- **200x60**: Very large terminal

## Layout Mode Mapping

| Terminal Width | Layout Mode | Panel Configuration |
|---------------|-------------|-------------------|
| < 80 cols     | Small       | Stacked panels vertically |
| 80-119 cols   | Medium      | Two-column with footer |
| ≥ 120 cols    | Large       | Three-column layout |

## Golden Frame Requirements

### 1. Line Stability
- **NO line growth**: The number of rendered lines must not increase over N updates
- **Consistent height**: Same terminal size should produce same line count
- **Bounded content**: Content must scroll or truncate to fit available space

### 2. Content Boundaries  
- **NO overlap**: Content must not exceed terminal boundaries
- **Responsive margins**: Appropriate spacing for each layout mode
- **Border consistency**: Panel borders must align properly

### 3. Update Stability
- **Flicker-free**: Updates should not cause visual artifacts
- **Smooth transitions**: Layout changes should be graceful
- **State preservation**: Focus and selection state maintained across updates

## Test Methodology

### Golden File Generation
```bash
# Generate golden files for all test sizes
make test-ui
```

Golden files are stored in `tests/golden/` with naming convention:
```
{test_name}_{width}x{height}.golden
```

### Validation Process
1. **Initialize** TUI with specific terminal size
2. **Capture** initial render output
3. **Apply** series of updates (navigation, content changes)
4. **Compare** final output with golden reference
5. **Verify** line count hasn't increased
6. **Check** for content overflow

### Update Sequence Testing
Standard update sequence applied to each terminal size:
- Initial render
- Tab navigation (5 cycles)
- Arrow key navigation (up/down)
- Panel focus changes (1/2/3 keys)
- Content updates (log streaming simulation)
- Final render capture

## Content Validation

### ANSI Code Handling
- **Non-TTY environments**: Zero ANSI escape sequences
- **TTY environments**: Proper ANSI code usage
- **Terminal capability**: Respects TERM environment variable

### Character Boundaries
- **Line width**: No line exceeds terminal width + small buffer (5 chars)
- **Unicode handling**: Proper width calculation for Unicode characters  
- **Control characters**: No raw control characters in output

## Performance Requirements

### Render Performance
- **Startup time**: < 500ms for initial render
- **Update latency**: < 16ms for 60fps equivalent
- **Memory bounds**: No significant memory growth over time

### Resize Handling
- **Rapid resize**: Handle 50+ resizes/second without issues
- **Layout transitions**: Smooth mode changes (small→medium→large)
- **State preservation**: Maintain focus and scroll position

## Error Conditions

### Graceful Degradation
- **Invalid sizes**: Handle negative or zero dimensions
- **Memory constraints**: Function with limited memory
- **Terminal limitations**: Adapt to terminal capabilities

### Recovery Mechanisms
- **Corrupted state**: Reset to known good state
- **Render failures**: Fallback to minimal rendering
- **Input errors**: Ignore invalid input gracefully

## CI/CD Integration

### Automated Testing
```bash
# Run all golden frame tests
make test-ui

# Test specific terminal size
COLUMNS=120 LINES=40 go test -run TestGoldenFrames ./tests/

# Generate new golden files
UPDATE_GOLDEN=1 make test-ui
```

### Headless Environments
- **TERM=dumb**: Plain text output validation
- **CI environments**: Non-interactive testing mode
- **Docker containers**: Minimal terminal capability handling

## Troubleshooting

### Common Issues

1. **Golden file mismatches**
   - Check for platform-specific differences
   - Verify ANSI code handling
   - Ensure consistent test environment

2. **Line count growth** 
   - Review content truncation logic
   - Check for unbounded loops
   - Verify viewport sizing

3. **Content overflow**
   - Validate boundary calculations
   - Test with various content lengths
   - Check Unicode character handling

### Debug Commands
```bash
# Manual inspection of specific size
COLUMNS=80 LINES=24 ./build/ipcrawler --debug ipcrawler.io

# Generate debug output
DEBUG=1 make test-ui

# Compare golden files
diff tests/golden/original.golden tests/golden/current.golden
```

## Maintenance

### Golden File Updates
Golden files should be updated when:
- Intentional UI changes are made
- New features are added that affect rendering
- Layout improvements are implemented

**Never** update golden files to fix failing tests without understanding the root cause.

### Version Compatibility
- Tag golden files with major version changes
- Maintain backward compatibility when possible
- Document breaking changes in release notes

---

*This reference document ensures consistent and reliable TUI testing across all environments and terminal configurations.*