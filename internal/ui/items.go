package ui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

// WorkflowItem represents a workflow in the list
type WorkflowItem struct {
	id          string
	description string
	status      string
	progress    float64
}

// FilterValue implements list.Item interface
func (i WorkflowItem) FilterValue() string {
	return i.description
}

// Title returns the item title
func (i WorkflowItem) Title() string {
	return i.description
}

// Description returns the item description  
func (i WorkflowItem) Description() string {
	status := i.status
	if i.progress > 0 && i.progress < 1 {
		status = fmt.Sprintf("%s (%.1f%%)", status, i.progress*100)
	}
	return fmt.Sprintf("ID: %s | Status: %s", i.id, status)
}

// WorkflowDelegate is a custom delegate for workflow list items
type WorkflowDelegate struct {
	list.DefaultDelegate
}

// NewWorkflowDelegate creates a new workflow delegate
func NewWorkflowDelegate() WorkflowDelegate {
	d := WorkflowDelegate{DefaultDelegate: list.NewDefaultDelegate()}
	
	// Customize the delegate styling
	d.ShowDescription = true
	d.SetHeight(2)
	d.SetSpacing(1)
	
	return d
}

// Render renders the workflow item
func (d WorkflowDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(WorkflowItem)
	if !ok {
		return
	}
	
	// Get status symbol
	statusSymbol := d.getStatusSymbol(item.status)
	
	// Style for selected vs normal items
	var titleStyle, descStyle lipgloss.Style
	
	if index == m.Index() {
		// Selected item styling
		titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#666666")).
			Bold(true).
			Padding(0, 1)
		descStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CCCCCC")).
			Background(lipgloss.Color("#444444")).
			Padding(0, 1)
	} else {
		// Normal item styling
		titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))
		descStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))
	}
	
	// Render title with status symbol
	title := fmt.Sprintf("%s %s", statusSymbol, item.description)
	titleStr := titleStyle.Render(title)
	
	// Render description
	descStr := descStyle.Render(item.Description())
	
	// Write the rendered content
	fmt.Fprint(w, titleStr)
	fmt.Fprint(w, "\n")
	fmt.Fprint(w, descStr)
}

// getStatusSymbol returns the appropriate symbol for a status
func (d WorkflowDelegate) getStatusSymbol(status string) string {
	switch status {
	case "running":
		return "●"
	case "completed":
		return "✓"
	case "failed":
		return "✗"
	case "pending":
		return "○"
	case "paused":
		return "⏸"
	default:
		return "○"
	}
}

// getStatusColor returns the appropriate color for a status
func (d WorkflowDelegate) getStatusColor(status string) lipgloss.Color {
	switch status {
	case "running":
		return lipgloss.Color("#FFFF00") // Yellow
	case "completed":
		return lipgloss.Color("#00FF00") // Green  
	case "failed":
		return lipgloss.Color("#FF0000") // Red
	case "pending":
		return lipgloss.Color("#888888") // Gray
	case "paused":
		return lipgloss.Color("#FFA500") // Orange
	default:
		return lipgloss.Color("#888888") // Gray
	}
}