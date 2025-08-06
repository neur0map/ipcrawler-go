package ui

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// Tables handles all table rendering
type Tables struct{}

// NewTables creates a new Tables instance
func NewTables() *Tables {
	return &Tables{}
}

// PortInfo represents port information for display
type PortInfo struct {
	Number  int
	State   string
	Service string
	Version string
}

// VulnerabilityInfo represents vulnerability information for display
type VulnerabilityInfo struct {
	TemplateID string
	Name       string
	Severity   string
}

// RenderPortDiscoveryTable renders a table showing discovered ports
func (t *Tables) RenderPortDiscoveryTable(ports []PortInfo) {
	if len(ports) == 0 {
		return
	}

	rows := [][]string{}
	for _, port := range ports {
		protocol := port.Service
		if protocol == "" {
			protocol = "tcp" // Default protocol for naabu
		}
		rows = append(rows, []string{
			strconv.Itoa(port.Number),
			protocol,
			"open",
		})
	}

	tbl := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(TableBorderStyle).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == table.HeaderRow:
				return TableHeaderStyle
			case row%2 == 0:
				return TableEvenRowStyle
			default:
				return TableOddRowStyle
			}
		}).
		Headers(NetworkIcon+" Port", ToolIcon+" Protocol", CheckIcon+" State").
		Rows(rows...)

	fmt.Println(tbl)
}

// RenderServiceDetectionTable renders a table showing detected services
func (t *Tables) RenderServiceDetectionTable(ports []PortInfo) {
	if len(ports) == 0 {
		return
	}

	rows := [][]string{}
	for _, port := range ports {
		if port.State == "open" {
			service := port.Service
			if service == "" {
				service = "unknown"
			}

			version := port.Version
			if version == "" {
				version = "-"
			}

			rows = append(rows, []string{
				strconv.Itoa(port.Number),
				service,
				version,
			})
		}
	}

	if len(rows) > 0 {
		tbl := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(TableBorderStyle).
			StyleFunc(func(row, col int) lipgloss.Style {
				switch {
				case row == table.HeaderRow:
					return TableHeaderStyle
				case row%2 == 0:
					return TableEvenRowStyle
				default:
					return TableOddRowStyle
				}
			}).
			Headers(NetworkIcon+" Port", ToolIcon+" Service", InfoIcon+" Version").
			Rows(rows...)

		fmt.Println(tbl)
	}
}

// RenderVulnerabilityTable renders a table showing vulnerabilities
func (t *Tables) RenderVulnerabilityTable(vulnerabilities []VulnerabilityInfo) {
	if len(vulnerabilities) == 0 {
		return
	}

	rows := [][]string{}
	for _, vuln := range vulnerabilities {
		name := vuln.Name
		if len(name) > 50 {
			name = name[:47] + "..."
		}
		templateID := vuln.TemplateID
		if len(templateID) > 25 {
			templateID = templateID[:22] + "..."
		}

		rows = append(rows, []string{
			vuln.Severity,
			name,
			templateID,
		})
	}

	if len(rows) > 0 {
		tbl := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(TableBorderStyle).
			StyleFunc(func(row, col int) lipgloss.Style {
				switch {
				case row == table.HeaderRow:
					return TableHeaderStyle
				case col == 0 && row != table.HeaderRow:
					// Color severity column based on severity level
					severity := rows[row-1][0]
					baseStyle := TableEvenRowStyle
					if row%2 != 0 {
						baseStyle = TableOddRowStyle
					}
					switch severity {
					case "critical":
						return baseStyle.Foreground(ErrorColor).Bold(true)
					case "high":
						return baseStyle.Foreground(ErrorColor).Bold(true)
					case "medium":
						return baseStyle.Foreground(TextPrimary)
					case "low":
						return baseStyle.Foreground(TextMuted)
					}
					return baseStyle
				case row%2 == 0:
					return TableEvenRowStyle
				default:
					return TableOddRowStyle
				}
			}).
			Headers(SecurityIcon+" Severity", ErrorIcon+" Vulnerability", FolderIcon+" Template ID").
			Rows(rows...)

		fmt.Println(tbl)
	}
}

// RenderWorkflowTable renders a table showing workflow information
func (t *Tables) RenderWorkflowTable(workflows [][]string) {
	if len(workflows) == 0 {
		return
	}

	if len(workflows) < 2 {
		return // Need at least header + 1 row
	}

	headers := workflows[0]
	rows := workflows[1:]

	tbl := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(TableBorderStyle).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == table.HeaderRow:
				return TableHeaderStyle
			case row%2 == 0:
				return TableEvenRowStyle
			default:
				return TableOddRowStyle
			}
		}).
		Headers(headers...).
		Rows(rows...)

	fmt.Println(tbl)
}

// RenderInfoTable renders a general information table without headers
func (t *Tables) RenderInfoTable(data [][]string) {
	if len(data) == 0 {
		return
	}

	tbl := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(TableBorderStyle).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row%2 == 0 {
				return TableEvenRowStyle
			}
			return TableOddRowStyle
		}).
		Rows(data...)

	fmt.Println(tbl)
}

// RenderGenericTable renders a generic table with headers
func (t *Tables) RenderGenericTable(data [][]string) {
	if len(data) == 0 {
		return
	}

	if len(data) < 2 {
		return // Need at least header + 1 row
	}

	headers := data[0]
	rows := data[1:]

	tbl := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(TableBorderStyle).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == table.HeaderRow:
				return TableHeaderStyle
			case row%2 == 0:
				return TableEvenRowStyle
			default:
				return TableOddRowStyle
			}
		}).
		Headers(headers...).
		Rows(rows...)

	fmt.Println(tbl)
}