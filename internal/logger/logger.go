package logger

import (
	"fmt"
	"os"
)

// Logger interface for conditional output
type Logger interface {
	Printf(format string, args ...interface{})
	Println(args ...interface{})
}

// TUILogger sends output to TUI when available
type TUILogger struct {
	monitor TUIMonitor
	enabled bool
}

type TUIMonitor interface {
	SendLog(level, category, message string, data map[string]interface{})
}

// ConsoleLogger prints to stdout/stderr
type ConsoleLogger struct{}

var currentLogger Logger = &ConsoleLogger{}

// SetTUILogger sets the TUI logger when TUI is active
func SetTUILogger(monitor TUIMonitor) {
	currentLogger = &TUILogger{
		monitor: monitor,
		enabled: true,
	}
}

// SetConsoleLogger sets console logger when TUI is disabled
func SetConsoleLogger() {
	currentLogger = &ConsoleLogger{}
}

// Printf logs a formatted message
func Printf(format string, args ...interface{}) {
	currentLogger.Printf(format, args...)
}

// Println logs a message
func Println(args ...interface{}) {
	currentLogger.Println(args...)
}

// TUILogger implementation
func (l *TUILogger) Printf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	if l.monitor != nil && l.enabled {
		// Send to TUI log system
		l.monitor.SendLog("INFO", "system", message, nil)
	} else {
		fmt.Printf(format, args...)
	}
}

func (l *TUILogger) Println(args ...interface{}) {
	message := fmt.Sprint(args...)
	if l.monitor != nil && l.enabled {
		// Send to TUI log system
		l.monitor.SendLog("INFO", "system", message, nil)
	} else {
		fmt.Println(args...)
	}
}

// ConsoleLogger implementation
func (c *ConsoleLogger) Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func (c *ConsoleLogger) Println(args ...interface{}) {
	fmt.Println(args...)
}

// For compatibility with existing code
func Print(args ...interface{}) {
	message := fmt.Sprint(args...)
	currentLogger.Println(message)
}

// Error logging always goes to stderr and TUI
func Errorf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	
	// Always print to stderr for errors
	fmt.Fprintf(os.Stderr, format, args...)
	
	// Also send to TUI if available
	if tuiLogger, ok := currentLogger.(*TUILogger); ok && tuiLogger.monitor != nil {
		tuiLogger.monitor.SendLog("ERROR", "system", message, nil)
	}
}