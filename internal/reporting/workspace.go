package reporting

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultWorkspaceManager implements workspace operations
func (w *DefaultWorkspaceManager) CreateWorkspace(target string) (string, error) {
	// Generate unique workspace name with timestamp
	timestamp := time.Now().Format("20060102-150405")
	workspaceName := fmt.Sprintf("%s-%s", target, timestamp)
	
	// Create base workspace directory
	workspaceDir := filepath.Join(w.BaseDir, workspaceName)
	
	// Create required subdirectories
	subdirs := []string{"reports", "json", "logs"}
	for _, subdir := range subdirs {
		dir := filepath.Join(workspaceDir, subdir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	
	return workspaceDir, nil
}

// GetReportDir returns the reports subdirectory path
func (w *DefaultWorkspaceManager) GetReportDir(workspace string) string {
	return filepath.Join(workspace, "reports")
}

// GetJSONDir returns the JSON outputs subdirectory path
func (w *DefaultWorkspaceManager) GetJSONDir(workspace string) string {
	return filepath.Join(workspace, "json")
}

// GetLogsDir returns the logs subdirectory path
func (w *DefaultWorkspaceManager) GetLogsDir(workspace string) string {
	return filepath.Join(workspace, "logs")
}

// CleanupWorkspace removes temporary files
func (w *DefaultWorkspaceManager) CleanupWorkspace(workspace string) error {
	// Only remove logs directory, keep reports and json
	logsDir := w.GetLogsDir(workspace)
	if _, err := os.Stat(logsDir); err == nil {
		return os.RemoveAll(logsDir)
	}
	return nil
}