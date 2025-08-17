#!/bin/bash

# analyze-tui-deps.sh - Comprehensive TUI dependency analysis script
# This script identifies all TUI-related code and files for removal

echo "=== IPCrawler TUI Dependency Analysis ==="
echo "Scanning codebase for TUI references..."

# Define output file
ANALYSIS_FILE="tui-analysis-$(date +%Y%m%d_%H%M%S).log"

# Create backup directory
BACKUP_DIR="backup-$(date +%Y%m%d_%H%M%S)"
echo "Creating backup in: $BACKUP_DIR"
mkdir -p "$BACKUP_DIR"

# Backup critical files
echo "Backing up critical files..."
cp cmd/ipcrawler/main.go "$BACKUP_DIR/"
cp go.mod "$BACKUP_DIR/"
cp go.sum "$BACKUP_DIR/"
cp Makefile "$BACKUP_DIR/"
[ -d internal/tui ] && cp -r internal/tui "$BACKUP_DIR/"

# Start analysis
echo "=== TUI Dependency Analysis Report ===" > "$ANALYSIS_FILE"
echo "Generated: $(date)" >> "$ANALYSIS_FILE"
echo "" >> "$ANALYSIS_FILE"

# 1. Search for Charmbracelet imports
echo "1. CHARMBRACELET IMPORTS:" >> "$ANALYSIS_FILE"
echo "==========================" >> "$ANALYSIS_FILE"
rg "github\.com/charmbracelet" --type go -n >> "$ANALYSIS_FILE" 2>/dev/null || echo "No Charmbracelet imports found" >> "$ANALYSIS_FILE"
echo "" >> "$ANALYSIS_FILE"

# 2. Search for tea.NewProgram instances
echo "2. TEA.NEWPROGRAM INSTANCES:" >> "$ANALYSIS_FILE"
echo "============================" >> "$ANALYSIS_FILE"
rg "tea\.NewProgram" --type go -n -C 3 >> "$ANALYSIS_FILE" 2>/dev/null || echo "No tea.NewProgram instances found" >> "$ANALYSIS_FILE"
echo "" >> "$ANALYSIS_FILE"

# 3. Search for TUI-related function calls
echo "3. TUI FUNCTION CALLS:" >> "$ANALYSIS_FILE"
echo "======================" >> "$ANALYSIS_FILE"
rg "(runTUI|tea\.|lipgloss\.|bubbles\.)" --type go -n >> "$ANALYSIS_FILE" 2>/dev/null || echo "No TUI function calls found" >> "$ANALYSIS_FILE"
echo "" >> "$ANALYSIS_FILE"

# 4. Search for --no-tui references
echo "4. NO-TUI FLAG REFERENCES:" >> "$ANALYSIS_FILE"
echo "==========================" >> "$ANALYSIS_FILE"
rg "no-tui" -n >> "$ANALYSIS_FILE" 2>/dev/null || echo "No no-tui references found" >> "$ANALYSIS_FILE"
echo "" >> "$ANALYSIS_FILE"

# 5. Find TUI-related directories
echo "5. TUI DIRECTORIES:" >> "$ANALYSIS_FILE"
echo "==================" >> "$ANALYSIS_FILE"
find . -type d -name "*tui*" -not -path "./.git/*" >> "$ANALYSIS_FILE" 2>/dev/null || echo "No TUI directories found" >> "$ANALYSIS_FILE"
echo "" >> "$ANALYSIS_FILE"

# 6. Find TUI-related files
echo "6. TUI-RELATED FILES:" >> "$ANALYSIS_FILE"
echo "=====================" >> "$ANALYSIS_FILE"
find . -name "*tui*" -type f -not -path "./.git/*" >> "$ANALYSIS_FILE" 2>/dev/null || echo "No TUI files found" >> "$ANALYSIS_FILE"
echo "" >> "$ANALYSIS_FILE"

# 7. Go module dependencies to remove
echo "7. GO MODULE DEPENDENCIES TO REMOVE:" >> "$ANALYSIS_FILE"
echo "=====================================" >> "$ANALYSIS_FILE"
grep -E "(bubbletea|bubbles|lipgloss|go-isatty)" go.mod >> "$ANALYSIS_FILE" 2>/dev/null || echo "No TUI dependencies found in go.mod" >> "$ANALYSIS_FILE"
echo "" >> "$ANALYSIS_FILE"

# 8. Configuration files with TUI settings
echo "8. CONFIGURATION FILES WITH TUI SETTINGS:" >> "$ANALYSIS_FILE"
echo "==========================================" >> "$ANALYSIS_FILE"
find configs/ -name "*.yaml" -exec echo "--- {} ---" \; -exec grep -n -i "tui\|ui\|theme\|color" {} \; >> "$ANALYSIS_FILE" 2>/dev/null || echo "No TUI config settings found" >> "$ANALYSIS_FILE"

# Summary
echo "" >> "$ANALYSIS_FILE"
echo "=== SUMMARY ===" >> "$ANALYSIS_FILE"
echo "Files to modify:" >> "$ANALYSIS_FILE"
echo "  - cmd/ipcrawler/main.go (massive TUI code removal)" >> "$ANALYSIS_FILE"
echo "  - go.mod (remove TUI dependencies)" >> "$ANALYSIS_FILE"
echo "  - go.sum (cleanup after go.mod changes)" >> "$ANALYSIS_FILE"
echo "  - Makefile (remove TUI targets)" >> "$ANALYSIS_FILE"
echo "" >> "$ANALYSIS_FILE"
echo "Files/Directories to remove:" >> "$ANALYSIS_FILE"
echo "  - internal/tui/ (entire directory)" >> "$ANALYSIS_FILE"
echo "  - scripts/tui-launch-window.sh" >> "$ANALYSIS_FILE"
echo "" >> "$ANALYSIS_FILE"
echo "Files to keep but update:" >> "$ANALYSIS_FILE"
echo "  - configs/ui.yaml (minimal CLI settings only)" >> "$ANALYSIS_FILE"

# Output results
echo ""
echo "Analysis complete! Results saved to: $ANALYSIS_FILE"
echo "Backup created in: $BACKUP_DIR"
echo ""
echo "=== QUICK SUMMARY ==="
echo "TUI imports found: $(rg "github\.com/charmbracelet" --type go --count-matches 2>/dev/null || echo 0)"
echo "tea.NewProgram instances: $(rg "tea\.NewProgram" --type go --count-matches 2>/dev/null || echo 0)"
echo "TUI function calls: $(rg "(runTUI|tea\.|lipgloss\.|bubbles\.)" --type go --count-matches 2>/dev/null || echo 0)"
echo ""
echo "Next step: Review $ANALYSIS_FILE and run remove-tui-bulk.go"