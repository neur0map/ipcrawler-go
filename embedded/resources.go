package embedded

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Embedded file systems for resources
//go:embed workflows
var WorkflowsFS embed.FS

// TODO: Enable when configs/tools embedding is needed
// //go:embed configs  
// var ConfigsFS embed.FS
//
// //go:embed tools
// var ToolsFS embed.FS

// GetWorkflowFS returns the embedded workflows filesystem
func GetWorkflowFS() fs.FS {
	workflowsFS, err := fs.Sub(WorkflowsFS, "workflows")
	if err != nil {
		// Fallback to root if sub fails
		return WorkflowsFS
	}
	return workflowsFS
}

// TODO: Enable when configs embedding is needed
// GetConfigFS returns the embedded configs filesystem  
// func GetConfigFS() fs.FS {
// 	configFS, err := fs.Sub(ConfigsFS, "configs")
// 	if err != nil {
// 		return ConfigsFS
// 	}
// 	return configFS
// }

// TODO: Enable when tools embedding is needed
// GetToolsFS returns the embedded tools filesystem
// func GetToolsFS() fs.FS {
// 	toolsFS, err := fs.Sub(ToolsFS, "tools")
// 	if err != nil {
// 		return ToolsFS
// 	}
// 	return toolsFS
// }

// ListWorkflows returns a list of all available workflow files
func ListWorkflows() ([]string, error) {
	var workflows []string
	
	workflowFS := GetWorkflowFS()
	err := fs.WalkDir(workflowFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		if !d.IsDir() && strings.HasSuffix(path, ".yaml") {
			workflows = append(workflows, path)
		}
		
		return nil
	})
	
	return workflows, err
}

// ReadWorkflowFile reads a workflow file from embedded resources
func ReadWorkflowFile(path string) ([]byte, error) {
	workflowFS := GetWorkflowFS()
	return fs.ReadFile(workflowFS, path)
}

// TODO: Enable when configs embedding is needed
// ReadConfigFile reads a config file from embedded resources
// func ReadConfigFile(path string) ([]byte, error) {
// 	configFS := GetConfigFS()
// 	return fs.ReadFile(configFS, path)
// }

// TODO: Enable when tools embedding is needed  
// ReadToolFile reads a tool config file from embedded resources
// func ReadToolFile(path string) ([]byte, error) {
// 	toolsFS := GetToolsFS()
// 	return fs.ReadFile(toolsFS, path)
// }

// WorkflowExists checks if a workflow file exists in embedded resources
func WorkflowExists(path string) bool {
	workflowFS := GetWorkflowFS()
	_, err := fs.Stat(workflowFS, path)
	return err == nil
}

// GetAllWorkflowPaths returns all workflow file paths organized by category
func GetAllWorkflowPaths() (map[string][]string, error) {
	workflowsByCategory := make(map[string][]string)
	
	workflowFS := GetWorkflowFS()
	err := fs.WalkDir(workflowFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		if !d.IsDir() && strings.HasSuffix(path, ".yaml") {
			// Extract category from path (first directory)
			parts := strings.Split(path, "/")
			category := "uncategorized"
			if len(parts) > 1 {
				category = parts[0]
			}
			
			workflowsByCategory[category] = append(workflowsByCategory[category], path)
		}
		
		return nil
	})
	
	return workflowsByCategory, err
}

// ExtractEmbeddedResources extracts all embedded resources to a directory
// This is useful for development or when the filesystem is needed
func ExtractEmbeddedResources(targetDir string) error {
	// Extract workflows
	workflowsDir := filepath.Join(targetDir, "workflows")
	if err := extractFS(GetWorkflowFS(), workflowsDir); err != nil {
		return fmt.Errorf("failed to extract workflows: %v", err)
	}
	
	// TODO: Enable when configs/tools embedding is ready
	// // Extract configs
	// configsDir := filepath.Join(targetDir, "configs")
	// if err := extractFS(GetConfigFS(), configsDir); err != nil {
	// 	return fmt.Errorf("failed to extract configs: %v", err)
	// }
	//
	// // Extract tools  
	// toolsDir := filepath.Join(targetDir, "tools")
	// if err := extractFS(GetToolsFS(), toolsDir); err != nil {
	// 	return fmt.Errorf("failed to extract tools: %v", err)
	// }
	
	return nil
}

// extractFS extracts an embedded filesystem to a directory
func extractFS(embeddedFS fs.FS, targetDir string) error {
	return fs.WalkDir(embeddedFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		targetPath := filepath.Join(targetDir, path)
		
		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}
		
		data, err := fs.ReadFile(embeddedFS, path)
		if err != nil {
			return err
		}
		
		return os.WriteFile(targetPath, data, 0644)
	})
}