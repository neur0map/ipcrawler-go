package utils

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
)

// FixSudoPermissions fixes ownership of directories when running with sudo
func FixSudoPermissions(reportPath string) error {
	// Check if we're running with sudo
	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser == "" {
		// Not running with sudo, no need to fix permissions
		return nil
	}
	
	// Get the original user's UID and GID
	originalUser, err := user.Lookup(sudoUser)
	if err != nil {
		return fmt.Errorf("failed to lookup original user %s: %w", sudoUser, err)
	}
	
	uid, err := strconv.Atoi(originalUser.Uid)
	if err != nil {
		return fmt.Errorf("failed to parse UID %s: %w", originalUser.Uid, err)
	}
	
	gid, err := strconv.Atoi(originalUser.Gid)
	if err != nil {
		return fmt.Errorf("failed to parse GID %s: %w", originalUser.Gid, err)
	}
	
	// Recursively change ownership of the report directory
	return filepath.Walk(reportPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Change ownership to the original user
		if err := syscall.Chown(path, uid, gid); err != nil {
			return fmt.Errorf("failed to change ownership of %s: %w", path, err)
		}
		
		return nil
	})
}

// GetSudoUserInfo returns information about the original user when running with sudo
func GetSudoUserInfo() (uid, gid int, username string, err error) {
	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser == "" {
		return 0, 0, "", fmt.Errorf("not running with sudo")
	}
	
	originalUser, err := user.Lookup(sudoUser)
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to lookup original user %s: %w", sudoUser, err)
	}
	
	uid, err = strconv.Atoi(originalUser.Uid)
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to parse UID %s: %w", originalUser.Uid, err)
	}
	
	gid, err = strconv.Atoi(originalUser.Gid)
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to parse GID %s: %w", originalUser.Gid, err)
	}
	
	return uid, gid, sudoUser, nil
}

// FixFilePermissions fixes permissions for a specific file when running with sudo
func FixFilePermissions(filePath string) error {
	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser == "" {
		// Not running with sudo, no need to fix permissions
		return nil
	}
	
	uid, gid, _, err := GetSudoUserInfo()
	if err != nil {
		return err
	}
	
	// Change ownership of the file
	if err := syscall.Chown(filePath, uid, gid); err != nil {
		return fmt.Errorf("failed to change ownership of %s: %w", filePath, err)
	}
	
	return nil
}

// WriteFileWithPermissions writes a file and fixes permissions when running with sudo
func WriteFileWithPermissions(filePath string, data []byte, perm os.FileMode) error {
	// Write the file
	if err := os.WriteFile(filePath, data, perm); err != nil {
		return err
	}
	
	// Fix permissions if running with sudo
	if err := FixFilePermissions(filePath); err != nil {
		// Log warning but don't fail - the file is still written
		fmt.Printf("Warning: Could not fix permissions for %s: %v\n", filePath, err)
	}
	
	return nil
}