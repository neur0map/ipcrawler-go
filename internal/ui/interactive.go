package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	
	"github.com/charmbracelet/lipgloss"
)

// Interactive handles all user interaction and prompts
type Interactive struct {
	messages *Messages
	banners  *Banners
	tables   *Tables
}

// NewInteractive creates a new Interactive instance
func NewInteractive() *Interactive {
	return &Interactive{
		messages: NewMessages(),
		banners:  NewBanners(),
		tables:   NewTables(),
	}
}

// PrivilegeOption represents privilege escalation options
type PrivilegeOption struct {
	UseSudo           bool
	RequestEscalation bool
}

// ScanPreview contains information for scan preview
type ScanPreview struct {
	Target           string
	Template         string
	ReportDir        string
	Workflows        map[string]interface{}
	PrivilegedTools  []string
	NormalTools      []string
}

// ShowScanPreview displays a compact scan preview and gets user consent for privilege escalation
func (i *Interactive) ShowScanPreview(preview *ScanPreview) (*PrivilegeOption, error) {
	// Compact header
	fmt.Println(PrimaryText.Render("🔒 IPCrawler Scan Preview"))
	fmt.Println()

	// Create compact info display using horizontal layout
	targetInfo := "🎯 " + preview.Target
	templateInfo := "🔧 " + preview.Template  
	workflowCount := "📊 " + fmt.Sprintf("%d workflows", len(preview.Workflows))
	
	// Combine info in one line
	infoLine := lipgloss.JoinHorizontal(lipgloss.Left, 
		PrimaryText.Render(targetInfo), "  │  ",
		SecondaryText.Render(templateInfo), "  │  ", 
		SecondaryText.Render(workflowCount))
	
	fmt.Println(infoLine)
	fmt.Println()

	// Compact privilege comparison using side-by-side columns
	if len(preview.PrivilegedTools) > 0 {
		fmt.Println(PrimaryText.Render("🔐 Choose Scanning Mode:"))
		fmt.Println()

		// Create two compact columns for comparison
		normalColumn := lipgloss.NewStyle().
			Width(40).
			Padding(1, 2).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Render(lipgloss.JoinVertical(lipgloss.Left,
				InfoText.Render("✓ NORMAL MODE"),
				SecondaryText.Render("Tools: "+strings.Join(preview.NormalTools, ", ")),
				SecondaryText.Render("• TCP connect scans"),
				SecondaryText.Render("• Safe, no password needed"),
			))

		privilegedColumn := lipgloss.NewStyle().
			Width(40).
			Padding(1, 2).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Render(lipgloss.JoinVertical(lipgloss.Left,
				WarningText.Render("⚠ PRIVILEGED MODE"),
				SecondaryText.Render("Tools: "+strings.Join(append(preview.NormalTools, preview.PrivilegedTools...), ", ")),
				SecondaryText.Render("• Advanced SYN scans"),
				ErrorText.Render("• Requires password"),
			))

		// Display columns side by side
		columns := lipgloss.JoinHorizontal(lipgloss.Top, normalColumn, "  ", privilegedColumn)
		fmt.Println(columns)
		fmt.Println()

		// Simple user choice prompt
		for {
			promptText := PrimaryText.Render("Run with elevated privileges? (y/N): ")
			fmt.Print(promptText)
			
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("failed to read user input: %w", err)
			}
			
			choice := strings.ToLower(strings.TrimSpace(input))
			switch choice {
			case "y", "yes":
				fmt.Println(SuccessText.Render("✓ Privileged mode selected"))
				return &PrivilegeOption{UseSudo: true, RequestEscalation: true}, nil
			case "n", "no", "":
				fmt.Println(InfoText.Render("✓ Normal mode selected"))
				return &PrivilegeOption{UseSudo: false, RequestEscalation: false}, nil
			default:
				fmt.Println(ErrorText.Render("Please enter 'y' for yes or 'n' for no"))
				continue
			}
		}
	}

	// No privileged tools, return normal mode
	return &PrivilegeOption{UseSudo: false, RequestEscalation: false}, nil
}

// ShowPrivilegeInfo displays information about privilege requirements
func (i *Interactive) ShowPrivilegeInfo(privilegedTools, normalTools []string) {
	if len(privilegedTools) > 0 {
		i.banners.ShowSectionHeader("Tools that benefit from sudo privileges:")
		for _, tool := range privilegedTools {
			fmt.Printf("  • %s\n", GreenStyle.Render(tool))
		}
		fmt.Println()
	}

	if len(normalTools) > 0 {
		fmt.Printf("Tools that work without sudo: %s\n",
			CyanStyle.Render(strings.Join(normalTools, ", ")))
		fmt.Println()
	}
}