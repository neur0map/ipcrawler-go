package core

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/pterm/pterm"
)

// ScanPreview contains information about the upcoming scan
type ScanPreview struct {
	Target        string
	Template      string
	Workflows     []*Workflow
	ReportDir     string
	EstimatedTime string
}

// PrivilegeOption represents the user's choice for privilege escalation
type PrivilegeOption struct {
	UseSudo     bool
	UserChoice  string
}

// ShowScanPreview displays comprehensive scan information and gets user confirmation
func ShowScanPreview(preview *ScanPreview) (*PrivilegeOption, error) {
	// Display scan overview header
	pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle()).
		WithMargin(1).
		Println("IPCrawler Security Scanner - Scan Preview")
	
	// Show scan details
	pterm.DefaultSection.Println("Scan Configuration")
	
	infoData := pterm.TableData{
		{"Target", preview.Target},
		{"Template", preview.Template},
		{"Workflows", fmt.Sprintf("%d workflows", len(preview.Workflows))},
		{"Report Directory", preview.ReportDir},
		{"Estimated Time", preview.EstimatedTime},
	}
	
	pterm.DefaultTable.WithHasHeader(false).WithData(infoData).Render()
	pterm.Println()

	// Show workflow details
	pterm.DefaultSection.Println("Workflows to Execute")
	
	workflowData := [][]string{{"Workflow", "Description", "Tools"}}
	for _, workflow := range preview.Workflows {
		tools := extractToolsFromWorkflow(workflow)
		workflowData = append(workflowData, []string{
			workflow.Name,
			workflow.Description,
			strings.Join(tools, ", "),
		})
	}
	
	pterm.DefaultTable.WithHasHeader(true).WithData(workflowData).Render()
	pterm.Println()

	// Show sudo benefits and prompt
	return promptForSudoChoice(preview)
}

// promptForSudoChoice explains sudo benefits and gets user choice
func promptForSudoChoice(preview *ScanPreview) (*PrivilegeOption, error) {
	pterm.DefaultSection.Println("Privilege Escalation Options")
	
	// Create comparison table
	comparisonData := [][]string{
		{"Feature", "Without Sudo", "With Sudo"},
		{"Safety", "✓ Safe mode", "⚠ Requires elevated privileges"},
		{"Scan Types", "TCP connect scans", "✓ Stealth SYN scans"},
		{"OS Detection", "Limited", "✓ Advanced OS fingerprinting"},
		{"Port Scanning", "Basic detection", "✓ Comprehensive port analysis"},
		{"Speed", "Standard", "✓ Faster stealth scanning"},
		{"Accuracy", "Good", "✓ Higher accuracy results"},
		{"System Impact", "✓ No system changes", "May require root access"},
	}
	
	pterm.DefaultTable.WithHasHeader(true).WithData(comparisonData).Render()
	pterm.Println()

	// Show specific benefits for this scan
	showScanSpecificBenefits(preview)
	
	// Important notice about process restart
	pterm.DefaultSection.Println("Important Notice")
	pterm.Info.Println("If you choose sudo mode:")
	pterm.Printf("  • %s\n", 
		pterm.Yellow("The application will restart with elevated privileges"))
	pterm.Printf("  • %s\n", 
		pterm.Yellow("You will be prompted for your password"))
	pterm.Printf("  • %s\n", 
		pterm.Yellow("All current settings and arguments will be preserved"))
	pterm.Printf("  • %s\n", 
		pterm.Cyan("This ensures proper privilege separation and security"))
	pterm.Println()
	
	// Prompt for user choice
	pterm.Info.Println("Choose your scanning mode:")
	pterm.Printf("  • %s: Maximum accuracy and advanced features (requires restart)\n", 
		pterm.Green("sudo mode"))
	pterm.Printf("  • %s: Safe scanning with basic features (continue immediately)\n", 
		pterm.Yellow("normal mode"))
	pterm.Println()

	for {
		pterm.Print("Run with elevated privileges? (y/N): ")
		
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read user input: %w", err)
		}

		choice := strings.TrimSpace(strings.ToLower(input))
		
		switch choice {
		case "y", "yes":
			pterm.Success.Println("✓ Sudo mode selected - Application will restart with elevated privileges")
			return &PrivilegeOption{UseSudo: true, UserChoice: "yes"}, nil
		case "n", "no", "":
			pterm.Info.Println("✓ Normal mode selected - Safe scanning enabled")
			return &PrivilegeOption{UseSudo: false, UserChoice: "no"}, nil
		default:
			pterm.Warning.Printf("Please enter 'y' for yes or 'n' for no (or press Enter for no)\n")
			continue
		}
	}
}

// showScanSpecificBenefits shows benefits specific to the current scan
func showScanSpecificBenefits(preview *ScanPreview) {
	sudoTools := []string{}
	normalTools := []string{}
	
	for _, workflow := range preview.Workflows {
		tools := extractToolsFromWorkflow(workflow)
		for _, tool := range tools {
			switch strings.ToLower(tool) {
			case "nmap":
				sudoTools = append(sudoTools, "nmap (stealth scans, OS detection)")
			case "masscan":
				sudoTools = append(sudoTools, "masscan (high-speed scanning)")
			default:
				normalTools = append(normalTools, tool)
			}
		}
	}

	if len(sudoTools) > 0 {
		pterm.DefaultSection.Printf("Tools that benefit from sudo privileges:")
		for _, tool := range sudoTools {
			pterm.Printf("  • %s\n", pterm.Green(tool))
		}
		pterm.Println()
	}

	if len(normalTools) > 0 {
		pterm.Printf("Tools that work without sudo: %s\n", 
			pterm.Cyan(strings.Join(normalTools, ", ")))
		pterm.Println()
	}
}

// extractToolsFromWorkflow extracts tool names from workflow steps
func extractToolsFromWorkflow(workflow *Workflow) []string {
	var tools []string
	toolSet := make(map[string]bool)

	for _, step := range workflow.Steps {
		if step.Tool != "" && !toolSet[step.Tool] {
			tools = append(tools, step.Tool)
			toolSet[step.Tool] = true
		}
	}

	return tools
}

// EstimateScanTime provides a rough estimate of scan duration
func EstimateScanTime(workflows []*Workflow) string {
	totalMinutes := 0
	
	for _, workflow := range workflows {
		// Basic time estimates per workflow type
		switch {
		case strings.Contains(strings.ToLower(workflow.Name), "port"):
			totalMinutes += 2 // Port discovery: ~2 minutes
		case strings.Contains(strings.ToLower(workflow.Name), "deep"):
			totalMinutes += 5 // Deep scan: ~5 minutes
		case strings.Contains(strings.ToLower(workflow.Name), "vuln"):
			totalMinutes += 10 // Vulnerability scan: ~10 minutes
		default:
			totalMinutes += 3 // Default: ~3 minutes
		}
	}

	if totalMinutes <= 5 {
		return "2-5 minutes"
	} else if totalMinutes <= 15 {
		return "5-15 minutes"
	} else {
		return "15+ minutes"
	}
}