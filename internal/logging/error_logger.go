package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// MaxLogSize defines maximum log file size before rotation (10MB)
	MaxLogSize = 10 * 1024 * 1024
	// LogFileName is the default error log file name
	LogFileName = "error.log"
)

// ErrorContext holds all contextual information about an error
type ErrorContext struct {
	Timestamp   time.Time         `json:"timestamp"`
	Tool        string            `json:"tool,omitempty"`
	Command     []string          `json:"command,omitempty"`
	Error       string            `json:"error"`
	ErrorType   string            `json:"error_type,omitempty"`
	Template    string            `json:"template,omitempty"`
	Target      string            `json:"target,omitempty"`
	ExitCode    int               `json:"exit_code,omitempty"`
	StdErr      []string          `json:"stderr,omitempty"`
	Duration    time.Duration     `json:"duration,omitempty"`
	Context     map[string]string `json:"context,omitempty"`
}

// ErrorLogger provides thread-safe error logging to file
type ErrorLogger struct {
	mu       sync.Mutex
	logFile  *os.File
	logPath  string
	maxSize  int64
	disabled bool
}

// GlobalErrorLogger is the singleton error logger instance
var GlobalErrorLogger *ErrorLogger

// Initialize creates and initializes the global error logger
func Initialize(logDir string) error {
	if GlobalErrorLogger != nil {
		return nil // Already initialized
	}

	logger, err := NewErrorLogger(logDir)
	if err != nil {
		return fmt.Errorf("failed to initialize error logger: %w", err)
	}

	GlobalErrorLogger = logger
	return nil
}

// NewErrorLogger creates a new error logger instance
func NewErrorLogger(logDir string) (*ErrorLogger, error) {
	// Create log directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	logPath := filepath.Join(logDir, LogFileName)
	
	// Open or create log file in append mode
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &ErrorLogger{
		logFile: logFile,
		logPath: logPath,
		maxSize: MaxLogSize,
	}, nil
}

// LogError logs an error with full context
func (el *ErrorLogger) LogError(ctx ErrorContext) error {
	if el.disabled {
		return nil
	}

	el.mu.Lock()
	defer el.mu.Unlock()

	// Set timestamp if not provided
	if ctx.Timestamp.IsZero() {
		ctx.Timestamp = time.Now()
	}

	// Check for log rotation
	if err := el.rotateIfNeeded(); err != nil {
		// Don't fail on rotation error, just print warning
		fmt.Fprintf(os.Stderr, "Warning: Failed to rotate log: %v\n", err)
	}

	// Encode error context to JSON
	data, err := json.Marshal(ctx)
	if err != nil {
		return fmt.Errorf("failed to marshal error context: %w", err)
	}

	// Write to log file with newline
	if _, err := el.logFile.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write to log file: %w", err)
	}

	// Sync to ensure data is written
	return el.logFile.Sync()
}

// LogCommandError logs a command execution error
func (el *ErrorLogger) LogCommandError(tool string, args []string, err error, exitCode int, stderr []string) {
	ctx := ErrorContext{
		Tool:      tool,
		Command:   args,
		Error:     err.Error(),
		ErrorType: "command_execution",
		ExitCode:  exitCode,
		StdErr:    stderr,
	}
	
	if logErr := el.LogError(ctx); logErr != nil {
		fmt.Fprintf(os.Stderr, "Failed to log error: %v\n", logErr)
	}
}

// LogTemplateError logs a template execution error
func (el *ErrorLogger) LogTemplateError(template, target string, err error, duration time.Duration) {
	ctx := ErrorContext{
		Template:  template,
		Target:    target,
		Error:     err.Error(),
		ErrorType: "template_execution",
		Duration:  duration,
	}
	
	if logErr := el.LogError(ctx); logErr != nil {
		fmt.Fprintf(os.Stderr, "Failed to log error: %v\n", logErr)
	}
}

// LogGenericError logs a generic error with optional context
func (el *ErrorLogger) LogGenericError(errorType string, err error, context map[string]string) {
	ctx := ErrorContext{
		Error:     err.Error(),
		ErrorType: errorType,
		Context:   context,
	}
	
	if logErr := el.LogError(ctx); logErr != nil {
		fmt.Fprintf(os.Stderr, "Failed to log error: %v\n", logErr)
	}
}

// rotateIfNeeded checks log file size and rotates if necessary
func (el *ErrorLogger) rotateIfNeeded() error {
	fileInfo, err := el.logFile.Stat()
	if err != nil {
		return err
	}

	if fileInfo.Size() < el.maxSize {
		return nil // No rotation needed
	}

	// Close current file
	el.logFile.Close()

	// Generate archive name with timestamp
	timestamp := time.Now().Format("20060102_150405")
	archivePath := fmt.Sprintf("%s.%s", el.logPath, timestamp)

	// Rename current log to archive
	if err := os.Rename(el.logPath, archivePath); err != nil {
		// Try to reopen original file if rename fails
		el.logFile, _ = os.OpenFile(el.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		return err
	}

	// Create new log file
	el.logFile, err = os.OpenFile(el.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to create new log file after rotation: %w", err)
	}

	return nil
}

// Close closes the error logger
func (el *ErrorLogger) Close() error {
	el.mu.Lock()
	defer el.mu.Unlock()

	if el.logFile != nil {
		return el.logFile.Close()
	}
	return nil
}

// Disable temporarily disables error logging
func (el *ErrorLogger) Disable() {
	el.mu.Lock()
	defer el.mu.Unlock()
	el.disabled = true
}

// Enable re-enables error logging
func (el *ErrorLogger) Enable() {
	el.mu.Lock()
	defer el.mu.Unlock()
	el.disabled = false
}

// Helper functions for quick logging when GlobalErrorLogger is initialized

// LogCommandErrorGlobal logs a command error using the global logger
func LogCommandErrorGlobal(tool string, args []string, err error, exitCode int, stderr []string) {
	if GlobalErrorLogger != nil {
		GlobalErrorLogger.LogCommandError(tool, args, err, exitCode, stderr)
	}
}

// LogTemplateErrorGlobal logs a template error using the global logger
func LogTemplateErrorGlobal(template, target string, err error, duration time.Duration) {
	if GlobalErrorLogger != nil {
		GlobalErrorLogger.LogTemplateError(template, target, err, duration)
	}
}

// LogGenericErrorGlobal logs a generic error using the global logger
func LogGenericErrorGlobal(errorType string, err error, context map[string]string) {
	if GlobalErrorLogger != nil {
		GlobalErrorLogger.LogGenericError(errorType, err, context)
	}
}