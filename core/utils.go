package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	
	"ipcrawler/internal/utils"
	"github.com/pterm/pterm"
)

// CreateReportDirectory creates the report directory structure for a target
func CreateReportDirectory(baseDir, target string) (string, error) {
	// Sanitize target name for directory
	sanitizedTarget := strings.ReplaceAll(target, ".", "_")
	sanitizedTarget = strings.ReplaceAll(sanitizedTarget, ":", "_")
	sanitizedTarget = strings.ReplaceAll(sanitizedTarget, "/", "_")
	
	timestamp := time.Now().Format("20060102_150405")
	reportPath := filepath.Join(baseDir, sanitizedTarget, fmt.Sprintf("timestamp_%s", timestamp))
	
	// Create directory structure
	dirs := []string{
		reportPath,
		filepath.Join(reportPath, "raw"),
		filepath.Join(reportPath, "processed"),
		filepath.Join(reportPath, "summary"),
		filepath.Join(reportPath, "logs"),
	}
	
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	
	// Fix permissions if running with sudo
	if err := utils.FixSudoPermissions(reportPath); err != nil {
		// Log warning but don't fail - the directories still work for root
		pterm.Warning.Printf("Could not fix permissions for %s: %v\n", reportPath, err)
	}
	
	return reportPath, nil
}

// ExecuteCommand executes a security tool command with the given arguments
func ExecuteCommand(tool string, args []string, debugMode bool) error {
	// Create command
	cmd := exec.Command(tool, args...)
	
	// Set up stdout and stderr based on debug mode
	if debugMode {
		// In debug mode, show all output
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		// In normal mode, suppress output (but still capture for potential error reporting)
		cmd.Stdout = nil
		cmd.Stderr = nil
	}
	
	// Run the command
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute %s: %w", tool, err)
	}
	
	return nil
}

// ExecuteCommandWithOutput executes a command and returns its output
func ExecuteCommandWithOutput(tool string, args []string) ([]byte, error) {
	cmd := exec.Command(tool, args...)
	
	// Run the command and capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("failed to execute %s: %w", tool, err)
	}
	
	return output, nil
}

