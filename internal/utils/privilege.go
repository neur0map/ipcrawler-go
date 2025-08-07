package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// IsRunningAsRoot checks if the current process is running with root privileges
func IsRunningAsRoot() bool {
	return os.Geteuid() == 0
}

// IsSudoAvailable checks if the sudo command is available on the system
func IsSudoAvailable() bool {
	_, err := exec.LookPath("sudo")
	return err == nil
}

// RequestPrivilegeEscalation handles the privilege escalation process
// It checks current privileges and restarts with sudo if needed and available
func RequestPrivilegeEscalation() error {
	// Already running as root
	if IsRunningAsRoot() {
		return nil
	}

	// Check if sudo is available
	if !IsSudoAvailable() {
		return fmt.Errorf("sudo command not found - elevated privileges not available")
	}

	// Restart the process with sudo
	return RestartWithSudo()
}

// RestartWithSudo replaces the current process with a sudo version
// This approach starts fresh with elevated privileges from the beginning
func RestartWithSudo() error {
	// Get the full path to the current executable
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	
	// Resolve symlinks to get the actual executable path
	realExecPath, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		// If we can't resolve symlinks, fall back to the original path
		realExecPath = execPath
	}

	// Prepare arguments: real executable path + all original arguments + sudo restart flag
	args := make([]string, 0, len(os.Args)+1)
	args = append(args, realExecPath) // Use resolved executable path instead of symlink
	args = append(args, os.Args[1:]...) // Add all original arguments
	args = append(args, "--sudo-restart") // Add flag to indicate this is a sudo restart

	// Debug information (could be logged)
	fmt.Printf("  📍 Executable path: %s\n", execPath)
	fmt.Printf("  🔍 Resolved path: %s\n", realExecPath)
	fmt.Printf("  🔧 Sudo command: sudo %s %v\n", realExecPath, os.Args[1:])

	// For Go applications, we need to use syscall.Exec or os/exec
	// Since Go doesn't have direct os.Execvp, we'll use exec.Command with replacement
	cmd := exec.Command("sudo", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// Run the command and wait for it to complete (this replaces the current process)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to restart with sudo: %w", err)
	}

	// If we reach here, the sudo command completed successfully
	// Exit with the same code as the sudo process
	os.Exit(0)
	return nil
}

// IsSudoRestart checks if the application was started via sudo restart
func IsSudoRestart() bool {
	for _, arg := range os.Args {
		if arg == "--sudo-restart" {
			return true
		}
	}
	return false
}

// RemoveSudoRestartFlag removes the --sudo-restart flag from os.Args
func RemoveSudoRestartFlag() {
	var newArgs []string
	for _, arg := range os.Args {
		if arg != "--sudo-restart" {
			newArgs = append(newArgs, arg)
		}
	}
	os.Args = newArgs
}

// CheckPrivilegeRequirements determines if elevated privileges are needed
// based on the tools and arguments being used
func CheckPrivilegeRequirements(tools []string, args [][]string) bool {
	for i, tool := range tools {
		if requiresPrivileges(tool, args[i]) {
			return true
		}
	}
	return false
}

// requiresPrivileges checks if a specific tool with given arguments needs root privileges
// This is now completely generic - no hardcoded tool knowledge
func requiresPrivileges(tool string, args []string) bool {
	// No hardcoded tool assumptions - all tools are handled generically
	// The CLI should determine sudo needs based on YAML configuration (args_sudo vs args_normal)
	// This function is kept for backward compatibility but returns false for generic behavior
	return false
}

// GetPrivilegeStatus returns a description of the current privilege status
func GetPrivilegeStatus() string {
	if IsRunningAsRoot() {
		return "Running with elevated privileges (root)"
	}
	
	if IsSudoAvailable() {
		return "Running with normal privileges (sudo available)"
	}
	
	return "Running with normal privileges (sudo not available)"
}

// IsValidTarget checks if a target is a valid hostname or IP address
func IsValidTarget(target string) bool {
	// Basic validation - check if it's not empty and doesn't contain invalid characters
	if target == "" || len(target) > 255 {
		return false
	}
	
	// Check for obviously invalid characters
	for _, char := range target {
		if char < 32 || char > 126 {
			return false
		}
	}
	
	// Allow hostnames, IP addresses, and common formats
	// This is a permissive check - more detailed validation can be added later
	return true
}

// LookPath is an alias for exec.LookPath for compatibility
func LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// PrivilegeOption represents privilege escalation options
type PrivilegeOption struct {
	UseSudo             bool
	RequestEscalation   bool
}

// HandlePrivilegeDecision handles the privilege escalation decision process
func HandlePrivilegeDecision(needsSudo bool, interactive bool) *PrivilegeOption {
	option := &PrivilegeOption{
		UseSudo:           needsSudo,
		RequestEscalation: needsSudo && !IsRunningAsRoot(),
	}
	
	return option
}