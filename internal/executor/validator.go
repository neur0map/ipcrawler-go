package executor

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/neur0map/ipcrawler/internal/config"
)

// SecurityValidator handles security validation for tool execution
type SecurityValidator struct {
	config *config.Config
}

// NewSecurityValidator creates a new security validator
func NewSecurityValidator(cfg *config.Config) *SecurityValidator {
	return &SecurityValidator{
		config: cfg,
	}
}

// ValidateArguments validates command arguments against security policies
func (sv *SecurityValidator) ValidateArguments(args []string) error {
	if !sv.config.Security.Execution.ArgsValidation {
		return nil // Validation disabled
	}

	policy := sv.config.Tools.ArgvPolicy

	// Check maximum number of arguments
	if len(args) > policy.MaxArgs {
		return fmt.Errorf("too many arguments: %d > %d (max_args)", len(args), policy.MaxArgs)
	}

	// Calculate total argument size
	totalBytes := 0
	for _, arg := range args {
		argBytes := len(arg)

		// Check individual argument size
		if argBytes > policy.MaxArgBytes {
			return fmt.Errorf("argument too long: %d > %d bytes (max_arg_bytes): '%s'",
				argBytes, policy.MaxArgBytes, truncateString(arg, 50))
		}

		totalBytes += argBytes
	}

	// Check total arguments size
	if totalBytes > policy.MaxArgvBytes {
		return fmt.Errorf("total arguments too long: %d > %d bytes (max_argv_bytes)",
			totalBytes, policy.MaxArgvBytes)
	}

	// Check for shell metacharacters if enabled
	if policy.DenyShellMetachars {
		for _, arg := range args {
			if err := sv.checkShellMetacharacters(arg); err != nil {
				return fmt.Errorf("shell metacharacter found in argument '%s': %w",
					truncateString(arg, 50), err)
			}
		}
	}

	// Validate against allowed character classes
	if len(policy.AllowedCharClasses) > 0 {
		for _, arg := range args {
			if err := sv.validateCharacterClasses(arg, policy.AllowedCharClasses); err != nil {
				return fmt.Errorf("invalid characters in argument '%s': %w",
					truncateString(arg, 50), err)
			}
		}
	}

	// Check for path traversal attempts
	for _, arg := range args {
		if err := sv.checkPathTraversal(arg); err != nil {
			return fmt.Errorf("path traversal attempt in argument '%s': %w",
				truncateString(arg, 50), err)
		}
	}

	return nil
}

// ValidateExecutable validates that the executable path is allowed
func (sv *SecurityValidator) ValidateExecutable(executablePath string) error {
	if !sv.config.Security.Execution.ExecValidation {
		return nil // Validation disabled
	}

	// Get the tools root from security config
	toolsRoot := sv.config.Security.Execution.ToolsRoot
	if toolsRoot == "" {
		toolsRoot = sv.config.Tools.Execution.ToolsPath // Fallback to tools config
	}

	// If tools root is empty, allow any executable (system PATH)
	if toolsRoot == "" {
		return nil
	}

	// Convert to absolute paths for comparison
	absToolsRoot, err := filepath.Abs(toolsRoot)
	if err != nil {
		return fmt.Errorf("failed to resolve tools root path: %w", err)
	}

	absExecPath, err := filepath.Abs(executablePath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	// Check if executable is under tools root
	relPath, err := filepath.Rel(absToolsRoot, absExecPath)
	if err != nil {
		return fmt.Errorf("failed to compute relative path: %w", err)
	}

	// If relative path starts with "..", it's outside tools root
	if strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("executable outside tools root: %s not under %s",
			executablePath, toolsRoot)
	}

	// Check for additional path traversal attempts
	if strings.Contains(relPath, "..") {
		return fmt.Errorf("path traversal in executable path: %s", executablePath)
	}

	return nil
}

// checkShellMetacharacters checks for dangerous shell metacharacters
func (sv *SecurityValidator) checkShellMetacharacters(arg string) error {
	// Common shell metacharacters that could be dangerous
	dangerousChars := []string{
		";", "&", "|", "&&", "||", "$", "`", "$(", "${",
		">", ">>", "<", "<<", "*", "?", "[", "]", "!", "~",
		"'", "\"", "\\", "\n", "\r", "\t",
	}

	for _, char := range dangerousChars {
		if strings.Contains(arg, char) {
			return fmt.Errorf("dangerous shell metacharacter found: %s", char)
		}
	}

	return nil
}

// validateCharacterClasses validates argument against allowed character classes
func (sv *SecurityValidator) validateCharacterClasses(arg string, allowedClasses []string) error {
	// Build allowed character set
	allowedChars := make(map[rune]bool)

	for _, class := range allowedClasses {
		switch class {
		case "alnum":
			for r := 'a'; r <= 'z'; r++ {
				allowedChars[r] = true
			}
			for r := 'A'; r <= 'Z'; r++ {
				allowedChars[r] = true
			}
			for r := '0'; r <= '9'; r++ {
				allowedChars[r] = true
			}
		case "alpha":
			for r := 'a'; r <= 'z'; r++ {
				allowedChars[r] = true
			}
			for r := 'A'; r <= 'Z'; r++ {
				allowedChars[r] = true
			}
		case "digit":
			for r := '0'; r <= '9'; r++ {
				allowedChars[r] = true
			}
		default:
			// Treat as literal character(s)
			for _, r := range class {
				allowedChars[r] = true
			}
		}
	}

	// Check each character in the argument
	for _, r := range arg {
		if !allowedChars[r] {
			return fmt.Errorf("character '%c' not in allowed classes: %v", r, allowedClasses)
		}
	}

	return nil
}

// checkPathTraversal checks for path traversal attempts
func (sv *SecurityValidator) checkPathTraversal(arg string) error {
	// Check for obvious path traversal patterns
	traversalPatterns := []string{
		"../", "..\\", "/..", "\\..",
		"%2e%2e%2f", "%2e%2e\\", "%2e%2e/",
		"..%2f", "..%5c",
	}

	argLower := strings.ToLower(arg)
	for _, pattern := range traversalPatterns {
		if strings.Contains(argLower, pattern) {
			return fmt.Errorf("path traversal pattern detected: %s", pattern)
		}
	}

	// Check for encoded path traversal
	if matched, _ := regexp.MatchString(`%[0-9a-fA-F]{2}`, arg); matched {
		// Contains URL encoding - could be hiding path traversal
		if strings.Contains(argLower, "%2e") || strings.Contains(argLower, "%2f") ||
			strings.Contains(argLower, "%5c") {
			return fmt.Errorf("potential encoded path traversal detected")
		}
	}

	return nil
}

// truncateString truncates a string for safe display in error messages
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// IsASCIIPrintable checks if string contains only printable ASCII characters
func (sv *SecurityValidator) IsASCIIPrintable(s string) bool {
	for _, r := range s {
		if r < 32 || r > 126 {
			return false
		}
	}
	return true
}

// ValidateStringContent performs additional string content validation
func (sv *SecurityValidator) ValidateStringContent(s string, fieldName string) error {
	// Check for non-printable characters
	for i, r := range s {
		if !unicode.IsPrint(r) && !unicode.IsSpace(r) {
			return fmt.Errorf("non-printable character at position %d in %s: U+%04X",
				i, fieldName, r)
		}
	}

	// Check for extremely long strings that might cause issues
	if len(s) > 4096 {
		return fmt.Errorf("%s too long: %d characters (max 4096)", fieldName, len(s))
	}

	return nil
}
