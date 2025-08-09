//go:build !windows
// +build !windows

package term

import (
	"os"
	"syscall"
	"unsafe"
)

// Unix-specific terminal detection and sizing

const (
	ioctlReadTermios = 0x5401 // TCGETS
)

// winsize represents the terminal window size structure
type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// getTerminalSizeUnix gets terminal size on Unix-like systems
func getTerminalSizeUnix() (width, height int) {
	// Try stdout first, then stderr, then stdin
	fds := []int{int(os.Stdout.Fd()), int(os.Stderr.Fd()), int(os.Stdin.Fd())}
	
	for _, fd := range fds {
		if w, h := getTerminalSizeFd(fd); w > 0 && h > 0 {
			return w, h
		}
	}
	
	// Try environment variables as fallback
	return getTerminalSizeFromEnv()
}

// getTerminalSizeFd gets terminal size for a specific file descriptor
func getTerminalSizeFd(fd int) (width, height int) {
	var ws winsize
	
	// Use TIOCGWINSZ ioctl to get window size
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&ws)))
	
	if err != 0 {
		return 0, 0
	}
	
	return int(ws.Col), int(ws.Row)
}

// getTerminalSizeFromEnv attempts to get terminal size from environment variables
func getTerminalSizeFromEnv() (width, height int) {
	// Default values
	width, height = 80, 24
	
	// Try COLUMNS and LINES environment variables
	if cols := os.Getenv("COLUMNS"); cols != "" {
		if w := parseInt(cols); w > 0 {
			width = w
		}
	}
	
	if lines := os.Getenv("LINES"); lines != "" {
		if h := parseInt(lines); h > 0 {
			height = h
		}
	}
	
	return width, height
}

// parseInt parses a string to int, returning 0 on error
func parseInt(s string) int {
	result := 0
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			result = result*10 + int(ch-'0')
		} else {
			return 0 // Invalid character
		}
	}
	return result
}