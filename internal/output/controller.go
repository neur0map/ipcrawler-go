package output

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// OutputMode represents the CLI output mode
type OutputMode int

const (
	OutputModeNormal  OutputMode = iota // Only raw tool output
	OutputModeVerbose                   // Both logs and raw output
	OutputModeDebug                     // Only logs, no raw tool output
)

// ANSI color codes for terminal output
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

// OutputController manages console output based on the selected mode
type OutputController struct {
	mode        OutputMode
	outputMutex sync.Mutex // Global mutex for synchronized output
}

// NewOutputController creates a new output controller with the specified mode
func NewOutputController(mode OutputMode) *OutputController {
	return &OutputController{
		mode: mode,
	}
}

// PrintRaw outputs raw tool output to console based on the current mode
func (oc *OutputController) PrintRaw(content string) {
	switch oc.mode {
	case OutputModeNormal, OutputModeVerbose:
		fmt.Print(content)
	case OutputModeDebug:
		// In debug mode, don't show raw tool output
	}
}

// PrintRawLine outputs a single line of raw tool output
func (oc *OutputController) PrintRawLine(line string) {
	switch oc.mode {
	case OutputModeNormal, OutputModeVerbose:
		fmt.Println(line)
	case OutputModeDebug:
		// In debug mode, don't show raw tool output
	}
}

// PrintToolSeparator outputs a visual separator between tool outputs
func (oc *OutputController) PrintToolSeparator(toolName, mode string) {
	switch oc.mode {
	case OutputModeNormal, OutputModeVerbose:
		fmt.Printf("\n%s════════════════════════════════════════════════════════════════════════════════%s\n", colorCyan, colorReset)
		fmt.Printf("%s▶ %s [%s]%s\n", colorBold+colorGreen, toolName, mode, colorReset)
		fmt.Printf("%s════════════════════════════════════════════════════════════════════════════════%s\n\n", colorCyan, colorReset)
	case OutputModeDebug:
		// In debug mode, don't show separators
	}
}

// PrintToolEnd outputs a visual end marker for tool output
func (oc *OutputController) PrintToolEnd() {
	switch oc.mode {
	case OutputModeNormal, OutputModeVerbose:
		fmt.Printf("\n%s────────────────────────────────────────────────────────────────────────────────%s\n", colorGray, colorReset)
	case OutputModeDebug:
		// In debug mode, don't show end markers
	}
}

// PrintRawSection outputs a formatted section of raw tool output
func (oc *OutputController) PrintRawSection(toolName, mode, output string) {
	switch oc.mode {
	case OutputModeNormal, OutputModeVerbose:
		fmt.Printf("\n=== RAW OUTPUT: %s %s ===\n", toolName, mode)
		fmt.Print(output)
		fmt.Printf("=== END OUTPUT ===\n\n")
	case OutputModeDebug:
		// In debug mode, don't show raw tool output
	}
}

// PrintLog outputs log messages based on the current mode
func (oc *OutputController) PrintLog(level, msg string, args ...interface{}) {
	switch oc.mode {
	case OutputModeNormal:
		// In normal mode, don't show log messages
	case OutputModeVerbose, OutputModeDebug:
		// Show logs in verbose and debug modes
		if len(args) > 0 {
			fmt.Printf("[%s] "+msg+"\n", append([]interface{}{level}, args...)...)
		} else {
			fmt.Printf("[%s] %s\n", level, msg)
		}
	}
}

// PrintError outputs error messages (shown differently based on mode)
func (oc *OutputController) PrintError(line string) {
	switch oc.mode {
	case OutputModeNormal:
		// In normal mode, show stderr as plain text (it's tool output)
		fmt.Fprintln(os.Stderr, line)
	case OutputModeVerbose, OutputModeDebug:
		// In verbose/debug modes, show stderr with yellow color to indicate it's stderr
		fmt.Fprintf(os.Stderr, "%s%s%s\n", colorYellow, line, colorReset)
	}
}

// PrintWarning outputs warning messages based on the current mode
func (oc *OutputController) PrintWarning(msg string, args ...interface{}) {
	switch oc.mode {
	case OutputModeNormal:
		// In normal mode, don't show warning messages
	case OutputModeVerbose, OutputModeDebug:
		// Show warnings in verbose and debug modes
		if len(args) > 0 {
			fmt.Printf("Warning: "+msg+"\n", args...)
		} else {
			fmt.Printf("Warning: %s\n", msg)
		}
	}
}

// PrintInfo outputs info messages based on the current mode
func (oc *OutputController) PrintInfo(msg string, args ...interface{}) {
	switch oc.mode {
	case OutputModeNormal:
		// In normal mode, show info messages about tool output status (but no IPCrawler logs)
		if len(args) > 0 {
			fmt.Printf(msg+"\n", args...)
		} else {
			fmt.Printf("%s\n", msg)
		}
	case OutputModeVerbose, OutputModeDebug:
		// Show info in verbose and debug modes with [INFO] prefix
		if len(args) > 0 {
			fmt.Printf("[INFO] "+msg+"\n", args...)
		} else {
			fmt.Printf("[INFO] %s\n", msg)
		}
	}
}

// ShouldShowRaw returns true if raw output should be displayed
func (oc *OutputController) ShouldShowRaw() bool {
	return oc.mode == OutputModeNormal || oc.mode == OutputModeVerbose
}

// ShouldShowLogs returns true if log messages should be displayed
func (oc *OutputController) ShouldShowLogs() bool {
	return oc.mode == OutputModeVerbose || oc.mode == OutputModeDebug
}

// PrintWorkflowTree displays a tree view of discovered workflow files
func (oc *OutputController) PrintWorkflowTree(workflowsPath string, workflows map[string]interface{}) {
	// Always show workflow tree regardless of mode
	fmt.Printf("\n%s+==============================================================================+%s\n", colorCyan, colorReset)
	fmt.Printf("%s|                              WORKFLOW TREE                                 	 |%s\n", colorCyan, colorReset)
	fmt.Printf("%s+==============================================================================+%s\n", colorCyan, colorReset)

	// Build workflow tree structure from file paths
	tree, fileCount := oc.buildWorkflowTree(workflowsPath, workflows)

	// Print the tree
	fmt.Printf("\n%s[+] workflows/%s %s(%d workflow files discovered)%s\n",
		colorBold+colorBlue, colorReset, colorGray, fileCount, colorReset)
	oc.printTreeLevel(tree, "", true)
	fmt.Printf("\n%s================================================================================%s\n", colorGray, colorReset)
}

// buildWorkflowTree creates a tree structure from workflow file paths
func (oc *OutputController) buildWorkflowTree(workflowsPath string, workflows map[string]interface{}) (map[string]interface{}, int) {
	tree := make(map[string]interface{})
	fileCount := 0

	// Walk the workflows directory to build the tree
	filepath.WalkDir(workflowsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == workflowsPath {
			return nil
		}

		// Get relative path from workflows root
		relPath, err := filepath.Rel(workflowsPath, path)
		if err != nil {
			return nil
		}

		// Skip descriptions.yaml
		if d.Name() == "descriptions.yaml" {
			return nil
		}

		// Split path into components
		parts := strings.Split(relPath, string(filepath.Separator))

		// Build nested map structure
		current := tree
		for i, part := range parts {
			if i == len(parts)-1 {
				// This is a file
				if strings.HasSuffix(part, ".yaml") {
					current[part] = "file"
					fileCount++
				}
			} else {
				// This is a directory
				if current[part] == nil {
					current[part] = make(map[string]interface{})
				}
				current = current[part].(map[string]interface{})
			}
		}

		return nil
	})

	return tree, fileCount
}

// printTreeLevel recursively prints tree levels with proper formatting
func (oc *OutputController) printTreeLevel(node map[string]interface{}, prefix string, isRoot bool) {
	// Sort keys for consistent output
	var keys []string
	var dirs []string
	var files []string

	for key, value := range node {
		keys = append(keys, key)
		if value == "file" {
			files = append(files, key)
		} else {
			dirs = append(dirs, key)
		}
	}

	sort.Strings(dirs)
	sort.Strings(files)

	// Print directories first, then files
	allItems := append(dirs, files...)

	for i, item := range allItems {
		isLast := i == len(allItems)-1
		value := node[item]

		var connector, childPrefix string
		if isLast {
			connector = "`-- "
			childPrefix = prefix + "    "
		} else {
			connector = "|-- "
			childPrefix = prefix + "|   "
		}

		if value == "file" {
			// Print file with yaml icon and green color
			fmt.Printf("%s%s%s[F] %s%s%s\n", prefix, connector, colorGreen, item, colorReset, oc.getFileDescription(item))
		} else {
			// Print directory with folder icon and blue color
			fmt.Printf("%s%s%s[D] %s/%s\n", prefix, connector, colorBold+colorBlue, item, colorReset)
			// Recursively print subdirectories
			if subNode, ok := value.(map[string]interface{}); ok {
				oc.printTreeLevel(subNode, childPrefix, false)
			}
		}
	}
}

// getFileDescription returns a description for workflow files
func (oc *OutputController) getFileDescription(filename string) string {
	// Remove .yaml extension for lookup
	key := strings.TrimSuffix(filename, ".yaml")

	// Comprehensive descriptions for different workflow types
	descriptions := map[string]string{
		"dns-enumeration":   " %s(DNS reconnaissance and enumeration)%s",
		"port-scanning":     " %s(Port discovery and service detection)%s",
		"content-discovery": " %s(Web content and directory discovery)%s",
		"http-detection":    " %s(HTTP service and technology detection)%s",
		"basic-vuln-scan":   " %s(Basic vulnerability detection and analysis)%s",
		"test-nmap":         " %s(Basic nmap testing workflow)%s",
	}

	if desc, exists := descriptions[key]; exists {
		return fmt.Sprintf(desc, colorGray, colorReset)
	}

	// Generate description based on filename patterns
	if strings.Contains(key, "vuln") {
		return fmt.Sprintf(" %s(Vulnerability assessment workflow)%s", colorGray, colorReset)
	}
	if strings.Contains(key, "web") || strings.Contains(key, "http") {
		return fmt.Sprintf(" %s(Web application testing workflow)%s", colorGray, colorReset)
	}
	if strings.Contains(key, "dns") {
		return fmt.Sprintf(" %s(DNS analysis workflow)%s", colorGray, colorReset)
	}
	if strings.Contains(key, "port") || strings.Contains(key, "scan") {
		return fmt.Sprintf(" %s(Network scanning workflow)%s", colorGray, colorReset)
	}

	return fmt.Sprintf(" %s(Custom workflow)%s", colorGray, colorReset)
}

// PrintCompleteToolOutput displays the complete tool output atomically with synchronization
func (oc *OutputController) PrintCompleteToolOutput(toolName, mode, stdout, stderr string, hasError bool) {
	oc.outputMutex.Lock()
	defer oc.outputMutex.Unlock()

	// Print tool separator
	oc.printToolSeparatorUnsafe(toolName, mode)

	// Print stdout if available
	if stdout != "" {
		for _, line := range strings.Split(strings.TrimRight(stdout, "\n"), "\n") {
			oc.printRawLineUnsafe(line)
		}
	}

	// Print stderr if available
	if stderr != "" {
		for _, line := range strings.Split(strings.TrimRight(stderr, "\n"), "\n") {
			oc.printErrorUnsafe(line)
		}
	}

	// If no output at all, show a message
	if stdout == "" && stderr == "" {
		if hasError {
			oc.printInfoUnsafe("Tool failed with no output")
		} else {
			oc.printInfoUnsafe("Tool completed but produced no output")
		}
	}

	// Print tool end marker
	oc.printToolEndUnsafe()
}

// Thread-unsafe helper methods (must be called with mutex held)
func (oc *OutputController) printToolSeparatorUnsafe(toolName, mode string) {
	switch oc.mode {
	case OutputModeNormal, OutputModeVerbose:
		fmt.Printf("\n%s════════════════════════════════════════════════════════════════════════════════%s\n", colorCyan, colorReset)
		fmt.Printf("%s▶ %s [%s]%s\n", colorBold+colorGreen, toolName, mode, colorReset)
		fmt.Printf("%s════════════════════════════════════════════════════════════════════════════════%s\n\n", colorCyan, colorReset)
	case OutputModeDebug:
		// In debug mode, don't show separators
	}
}

func (oc *OutputController) printRawLineUnsafe(line string) {
	switch oc.mode {
	case OutputModeNormal, OutputModeVerbose:
		fmt.Println(line)
	case OutputModeDebug:
		// In debug mode, don't show raw tool output
	}
}

func (oc *OutputController) printErrorUnsafe(line string) {
	switch oc.mode {
	case OutputModeNormal:
		// In normal mode, show stderr as plain text (it's tool output)
		fmt.Fprintln(os.Stderr, line)
	case OutputModeVerbose, OutputModeDebug:
		// In verbose/debug modes, show stderr with yellow color to indicate it's stderr
		fmt.Fprintf(os.Stderr, "%s%s%s\n", colorYellow, line, colorReset)
	}
}

func (oc *OutputController) printInfoUnsafe(msg string) {
	switch oc.mode {
	case OutputModeNormal:
		// In normal mode, show info messages about tool output status
		fmt.Printf("%s\n", msg)
	case OutputModeVerbose, OutputModeDebug:
		// Show info in verbose and debug modes with [INFO] prefix
		fmt.Printf("[INFO] %s\n", msg)
	}
}

func (oc *OutputController) printToolEndUnsafe() {
	switch oc.mode {
	case OutputModeNormal, OutputModeVerbose:
		fmt.Printf("\n%s────────────────────────────────────────────────────────────────────────────────%s\n", colorGray, colorReset)
	case OutputModeDebug:
		// In debug mode, don't show end markers
	}
}
