package scanners

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/neur0map/ipcrawler/internal/registry"
)

// AutoDetector handles automatic detection of variables in files and code
type AutoDetector struct {
	manager       registry.RegistryManager
	variableRegex *regexp.Regexp
}

// NewAutoDetector creates a new auto-detection system
func NewAutoDetector(manager registry.RegistryManager) *AutoDetector {
	// Regex to match {{variable}} patterns
	variableRegex := regexp.MustCompile(`\{\{([^}]+)\}\}`)

	return &AutoDetector{
		manager:       manager,
		variableRegex: variableRegex,
	}
}

// ScanFile scans a single file for variable patterns
func (ad *AutoDetector) ScanFile(filePath string) (*registry.ScanResult, error) {
	result := &registry.ScanResult{
		FilePath:  filePath,
		Variables: []registry.DetectedVariable{},
		Errors:    []string{},
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to read file: %v", err))
		return result, nil
	}

	// Determine source type based on file extension
	source := ad.determineSourceType(filePath)

	// Scan content
	detectedVars, err := ad.ScanString(string(content), source)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to scan content: %v", err))
		return result, nil
	}

	result.Variables = detectedVars
	return result, nil
}

// ScanDirectory recursively scans a directory for variable patterns
func (ad *AutoDetector) ScanDirectory(dirPath string) ([]*registry.ScanResult, error) {
	var results []*registry.ScanResult

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files that can't be accessed
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only scan relevant file types
		if !ad.shouldScanFile(path) {
			return nil
		}

		result, scanErr := ad.ScanFile(path)
		if scanErr != nil {
			// Log error but continue scanning
			if result == nil {
				result = &registry.ScanResult{
					FilePath: path,
					Errors:   []string{scanErr.Error()},
				}
			}
		}

		if result != nil {
			results = append(results, result)
		}

		return nil
	})

	return results, err
}

// ScanString scans a string for variable patterns
func (ad *AutoDetector) ScanString(content string, source registry.VariableSource) ([]registry.DetectedVariable, error) {
	var detectedVars []registry.DetectedVariable

	lines := strings.Split(content, "\n")
	for lineNum, line := range lines {
		matches := ad.variableRegex.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				varName := fmt.Sprintf("{{%s}}", match[1])

				detectedVar := registry.DetectedVariable{
					Name:       varName,
					LineNumber: lineNum + 1,
					Context:    strings.TrimSpace(line),
					Source:     source,
				}

				detectedVars = append(detectedVars, detectedVar)
			}
		}
	}

	return detectedVars, nil
}

// AutoRegisterFromScanResults processes scan results and auto-registers new variables
func (ad *AutoDetector) AutoRegisterFromScanResults(results []*registry.ScanResult) error {
	for _, result := range results {
		for _, detectedVar := range result.Variables {
			// Create detection context
			context := registry.DetectionContext{
				FilePath:   result.FilePath,
				LineNumber: detectedVar.LineNumber,
				Context:    detectedVar.Context,
				Source:     detectedVar.Source,
				Tool:       ad.extractToolFromPath(result.FilePath),
				Timestamp:  time.Now(),
			}

			// Auto-register the variable
			if err := ad.manager.AutoRegisterVariable(detectedVar.Name, context); err != nil {
				// Log error but continue with other variables
				fmt.Printf("Warning: Failed to auto-register variable %s: %v\n", detectedVar.Name, err)
			}
		}
	}

	return nil
}

// ScanProjectForVariables scans the entire project for variables
func (ad *AutoDetector) ScanProjectForVariables(projectRoot string) error {
	// Define directories to scan
	scanDirs := []string{
		filepath.Join(projectRoot, "tools"),
		filepath.Join(projectRoot, "workflows"),
		filepath.Join(projectRoot, "configs"),
		filepath.Join(projectRoot, "internal"),
		filepath.Join(projectRoot, "cmd"),
	}

	for _, dir := range scanDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue // Skip directories that don't exist
		}

		fmt.Printf("Scanning directory: %s\n", dir)
		results, err := ad.ScanDirectory(dir)
		if err != nil {
			fmt.Printf("Warning: Error scanning directory %s: %v\n", dir, err)
			continue
		}

		// Auto-register found variables
		if err := ad.AutoRegisterFromScanResults(results); err != nil {
			fmt.Printf("Warning: Error auto-registering variables from %s: %v\n", dir, err)
		}

		fmt.Printf("Found %d files with variables in %s\n", len(results), dir)
	}

	return nil
}

// WatchForChanges monitors files for changes and auto-detects new variables
func (ad *AutoDetector) WatchForChanges(projectRoot string) error {
	// This is a simplified implementation
	// In a real implementation, you might use a file watcher library
	fmt.Printf("File watching not implemented yet. Use ScanProjectForVariables() periodically.\n")
	return nil
}

// GetRegistryStatistics returns current registry statistics
func (ad *AutoDetector) GetRegistryStatistics() registry.RegistryStatistics {
	return ad.manager.GetStatistics()
}

// ValidateAllVariables validates the registry and returns issues
func (ad *AutoDetector) ValidateAllVariables() []string {
	return ad.manager.ValidateRegistry()
}

// Private helper methods

func (ad *AutoDetector) determineSourceType(filePath string) registry.VariableSource {
	ext := strings.ToLower(filepath.Ext(filePath))
	dir := filepath.Dir(filePath)

	// Determine source based on file location and extension
	if strings.Contains(dir, "tools") && ext == ".yaml" {
		return registry.ConfigFileSource
	}
	if strings.Contains(dir, "workflows") && ext == ".yaml" {
		return registry.WorkflowFileSource
	}
	if strings.Contains(dir, "configs") && ext == ".yaml" {
		return registry.ConfigFileSource
	}
	if ext == ".go" {
		return registry.ExecutionContextSource
	}

	return registry.ConfigFileSource // Default
}

func (ad *AutoDetector) shouldScanFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))

	// Scan these file types
	scanExtensions := map[string]bool{
		".yaml": true,
		".yml":  true,
		".go":   true,
		".json": true,
		".md":   true,
	}

	// Skip certain directories
	skipDirs := map[string]bool{
		".git":         true,
		"node_modules": true,
		"vendor":       true,
		"bin":          true,
		"dist":         true,
		"build":        true,
	}

	// Check if any part of the path contains skip directories
	pathParts := strings.Split(filePath, string(filepath.Separator))
	for _, part := range pathParts {
		if skipDirs[part] {
			return false
		}
	}

	return scanExtensions[ext]
}

func (ad *AutoDetector) extractToolFromPath(filePath string) string {
	// Extract tool name from file path
	// e.g., "tools/nmap/config.yaml" -> "nmap"

	pathParts := strings.Split(filePath, string(filepath.Separator))
	for i, part := range pathParts {
		if part == "tools" && i+1 < len(pathParts) {
			return pathParts[i+1]
		}
	}

	// Check for tool names in the path
	toolNames := []string{"nmap", "naabu", "gobuster", "nuclei", "ffuf", "httpx", "subfinder"}
	for _, toolName := range toolNames {
		if strings.Contains(strings.ToLower(filePath), toolName) {
			return toolName
		}
	}

	return ""
}

// ConfigScanner specializes in scanning configuration files
type ConfigScanner struct {
	*AutoDetector
}

// NewConfigScanner creates a scanner specialized for config files
func NewConfigScanner(manager registry.RegistryManager) *ConfigScanner {
	return &ConfigScanner{
		AutoDetector: NewAutoDetector(manager),
	}
}

// ScanConfigFiles scans all configuration files in the project
func (cs *ConfigScanner) ScanConfigFiles(projectRoot string) error {
	configDirs := []string{
		filepath.Join(projectRoot, "tools"),
		filepath.Join(projectRoot, "workflows"),
		filepath.Join(projectRoot, "configs"),
	}

	for _, dir := range configDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			if info.IsDir() {
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".yaml" || ext == ".yml" {
				result, scanErr := cs.ScanFile(path)
				if scanErr != nil {
					fmt.Printf("Warning: Error scanning config file %s: %v\n", path, scanErr)
					return nil
				}

				if len(result.Variables) > 0 {
					fmt.Printf("Found %d variables in config file: %s\n", len(result.Variables), path)
					cs.AutoRegisterFromScanResults([]*registry.ScanResult{result})
				}
			}

			return nil
		})

		if err != nil {
			fmt.Printf("Warning: Error walking config directory %s: %v\n", dir, err)
		}
	}

	return nil
}

// CodeScanner specializes in scanning Go code for variables
type CodeScanner struct {
	*AutoDetector
}

// NewCodeScanner creates a scanner specialized for Go code
func NewCodeScanner(manager registry.RegistryManager) *CodeScanner {
	return &CodeScanner{
		AutoDetector: NewAutoDetector(manager),
	}
}

// ScanGoCode scans Go source files for variable usage
func (gcs *CodeScanner) ScanGoCode(projectRoot string) error {
	codeDirs := []string{
		filepath.Join(projectRoot, "internal"),
		filepath.Join(projectRoot, "cmd"),
	}

	for _, dir := range codeDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			if info.IsDir() {
				return nil
			}

			if strings.HasSuffix(path, ".go") {
				result, scanErr := gcs.ScanFile(path)
				if scanErr != nil {
					fmt.Printf("Warning: Error scanning Go file %s: %v\n", path, scanErr)
					return nil
				}

				if len(result.Variables) > 0 {
					fmt.Printf("Found %d variable references in Go file: %s\n", len(result.Variables), path)
					gcs.AutoRegisterFromScanResults([]*registry.ScanResult{result})
				}
			}

			return nil
		})

		if err != nil {
			fmt.Printf("Warning: Error walking code directory %s: %v\n", dir, err)
		}
	}

	return nil
}
