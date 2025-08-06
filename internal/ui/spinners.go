package ui

import (
	"fmt"
	
	"github.com/charmbracelet/lipgloss"
)

// Spinner manages loading indicators
type Spinner struct {
	currentText string
	isRunning   bool
}

// NewSpinner creates a new spinner instance
func NewSpinner() *Spinner {
	return &Spinner{}
}

// StartWorkflow starts a spinner for workflow execution
func (s *Spinner) StartWorkflow(workflowName string) *Spinner {
	text := fmt.Sprintf("Running %s...", workflowName)
	s.currentText = text
	s.isRunning = true
	
	// Display modern progress indicator
	badge := CreateBadge("RUNNING", "info")
	message := SpinnerStyle.Render(SpinnerIcon + " " + text)
	fmt.Println(lipgloss.JoinHorizontal(lipgloss.Left, badge, " ", message))
	return s
}

// UpdateText updates the spinner text
func (s *Spinner) UpdateText(text string) {
	if s.isRunning {
		s.currentText = text
		// In a real implementation, you might want to clear the previous line
		// and print the new text, but for simplicity, we'll just print it
	}
}

// Success marks the spinner as successful
func (s *Spinner) Success(text string) {
	if s.isRunning {
		badge := CreateBadge("SUCCESS", "success")
		message := SuccessText.Render(CheckIcon + " " + text)
		fmt.Println(lipgloss.JoinHorizontal(lipgloss.Left, badge, " ", message))
		s.isRunning = false
	}
}

// Fail marks the spinner as failed
func (s *Spinner) Fail(text string) {
	if s.isRunning {
		badge := CreateBadge("FAILED", "error")
		message := ErrorText.Render(CrossIcon + " " + text)
		fmt.Println(lipgloss.JoinHorizontal(lipgloss.Left, badge, " ", message))
		s.isRunning = false
	}
}

// Stop stops the spinner
func (s *Spinner) Stop() {
	if s.isRunning {
		s.isRunning = false
	}
}