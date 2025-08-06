package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Message types and colors
const (
	Success = "success"
	Error   = "error"
	Warning = "warning"
	Info    = "info"
)

// Messages holds all the application messages
type Messages struct{}

// NewMessages creates a new Messages instance
func NewMessages() *Messages {
	return &Messages{}
}

// General System Messages
func (m *Messages) SystemHealthOK() {
	badge := CreateBadge("HEALTHY", "success")
	message := SuccessText.Render("System Status: All systems operational")
	fmt.Println(lipgloss.JoinHorizontal(lipgloss.Left, badge, " ", message))
}

func (m *Messages) SystemVersion(version string) {
	badge := CreateBadge("v"+version, "info")
	message := InfoText.Render("IPCrawler Network Security Scanner")
	fmt.Println(lipgloss.JoinHorizontal(lipgloss.Left, badge, " ", message))
}

func (m *Messages) RunningWithRootPrivileges() {
	fmt.Println(WarningStyle.Render(WarningPrefix + " Running with root privileges"))
}

func (m *Messages) AllSystemsOperational() {
	badge := CreateBadge("READY", "success")
	message := SuccessText.Render("All scanning tools available and operational")
	fmt.Println(lipgloss.JoinHorizontal(lipgloss.Left, badge, " ", message))
}

func (m *Messages) MissingTools(tools []string) {
	badge := CreateBadge("ERROR", "error")
	message := ErrorText.Render("Missing required tools: " + strings.Join(tools, ", "))
	fmt.Println(lipgloss.JoinHorizontal(lipgloss.Left, badge, " ", message))

	infoBadge := CreateBadge("FIX", "info")
	fixMessage := InfoText.Render("Run: ") + CodeStyle.Render("make install-tools")
	fmt.Println(lipgloss.JoinHorizontal(lipgloss.Left, infoBadge, " ", fixMessage))
}

// Workflow and Execution Messages
func (m *Messages) LoadedWorkflows(count int) {
	fmt.Printf("Loaded %d workflows:\n", count)
}

func (m *Messages) WorkflowInfo(key, name string) {
	fmt.Printf("  - %s: %s\n", key, name)
}

func (m *Messages) WorkflowExecutionLevels(levels [][]string) {
	fmt.Printf("Workflow execution levels: %v\n", levels)
}

func (m *Messages) ExecutingLevel(levelIndex, count int, workflows []string) {
	fmt.Printf("Executing level %d with %d workflows: %v\n", levelIndex, count, workflows)
}

func (m *Messages) StartingWorkflow(name, description string) {
	fmt.Printf(InfoStyle.Render(InfoPrefix+" Starting workflow: %s (%s)")+"\n", name, description)
}

func (m *Messages) ExecutingCommand(tool string, args []string) {
	fmt.Printf(InfoStyle.Render("  Executing: %s %s")+"\n", tool, strings.Join(args, " "))
}

func (m *Messages) ExecutingCommandDebug(cmd string) {
	fmt.Printf(InfoStyle.Render(InfoPrefix+" Executing command: %s")+"\n", cmd)
}

func (m *Messages) ExecutingCommandFast(cmd string) {
	fmt.Printf(InfoStyle.Render(InfoPrefix+" Executing command (fast mode): %s")+"\n", cmd)
}

func (m *Messages) RunningSudo(args []string) {
	fmt.Printf(InfoStyle.Render(InfoPrefix+" Running with sudo: sudo %s")+"\n", strings.Join(args, " "))
}

// Scan Results Messages
func (m *Messages) ScanCancelled() {
	badge := CreateBadge("CANCELLED", "warning")
	message := WarningText.Render("🛑 Scan cancelled by user")
	fmt.Println(lipgloss.JoinHorizontal(lipgloss.Left, badge, " ", message))
}

func (m *Messages) ScanCompleted(target string) {
	fmt.Printf(SuccessStyle.Render(SuccessPrefix+" ✅ Scan completed for %s")+"\n", target)
}

func (m *Messages) ResultsSaved() {
	fmt.Printf(InfoStyle.Render(InfoPrefix+" 📁 Results saved in current directory") + "\n")
}

func (m *Messages) DiscoveredPorts(count int, ports string) {
	fmt.Printf(SuccessStyle.Render("  ✓ Discovered %d open ports: %s")+"\n", count, ports)
}

func (m *Messages) NoDiscoveredPorts() {
	fmt.Printf(WarningStyle.Render("  ⚠️ No discovered ports found, using common ports for deep scan") + "\n")
}

func (m *Messages) UsingDiscoveredPorts(ports string) {
	fmt.Printf(InfoStyle.Render("  🔗 Using discovered ports for deep scan: %s")+"\n", ports)
}

func (m *Messages) ProvidedData(key, value string) {
	fmt.Printf(SuccessStyle.Render("  ✓ Provided %s: %s")+"\n", key, value)
}

// Port Discovery Results
func (m *Messages) FoundOpenPorts(count int) {
	fmt.Printf(InfoStyle.Render(InfoPrefix+" 🔍 Found %d open ports:")+"\n", count)
}

// Service Detection Results
func (m *Messages) ServicesDetected(services []string) {
	fmt.Printf(InfoStyle.Render(InfoPrefix+" 🔎 Services detected: %s")+"\n", strings.Join(services, ", "))
}

// Vulnerability Results
func (m *Messages) VulnerabilitiesFound(count int) {
	fmt.Printf(InfoStyle.Render(InfoPrefix+" 🔒 Found %d vulnerabilities")+"\n", count)
}

func (m *Messages) CriticalVulnerabilities(count int) {
	fmt.Printf(ErrorStyle.Render("   Critical: %d")+"\n", count)
}

func (m *Messages) HighVulnerabilities(count int) {
	fmt.Printf(WarningStyle.Render("   High: %d")+"\n", count)
}

func (m *Messages) MediumVulnerabilities(count int) {
	fmt.Printf(InfoStyle.Render("   Medium: %d")+"\n", count)
}

func (m *Messages) LowVulnerabilities(count int) {
	fmt.Printf(InfoStyle.Render("   Low: %d")+"\n", count)
}

// File and Directory Operations
func (m *Messages) CreatingOutputDirectory(dir string) {
	fmt.Printf(InfoStyle.Render(InfoPrefix+" Creating output directory: %s")+"\n", dir)
}

func (m *Messages) WaitingForToolOutputs() {
	fmt.Println(InfoStyle.Render(InfoPrefix + " 🔍 Waiting for tool outputs to complete..."))
}

func (m *Messages) NoOutputFilesExpected() {
	fmt.Println(InfoStyle.Render(InfoPrefix + " No output files expected, continuing immediately"))
}

func (m *Messages) WaitingForFiles(count int) {
	fmt.Printf(InfoStyle.Render(InfoPrefix+" Waiting for %d expected output files...")+"\n", count)
}

func (m *Messages) ExpectedFile(file string) {
	fmt.Printf(InfoStyle.Render("  - %s")+"\n", file)
}

func (m *Messages) AllOutputFilesReady(duration time.Duration) {
	fmt.Printf(SuccessStyle.Render(SuccessPrefix+" ✅ All output files ready after %v")+"\n", duration.Round(time.Millisecond))
}

// Privilege and Security
func (m *Messages) NoStdoutDetected() {
	fmt.Printf(WarningStyle.Render(WarningPrefix+" No stdout output detected, checking if tool wrote to stderr instead") + "\n")
}

// Templates - Clean styled boxes with consistent width
func (m *Messages) AvailableTemplates() {
	badge := CreateBadge("INFO", "info")
	message := InfoText.Render("Available templates:")
	fmt.Println(lipgloss.JoinHorizontal(lipgloss.Left, badge, " ", message))
}

func (m *Messages) DefaultTemplate(template string) {
	// Create a clean, consistent box for the template
	templateBox := lipgloss.NewStyle().
		Foreground(TextPrimary).
		Padding(0, 2).
		Margin(0, 0, 0, 2).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor).
		Width(30).
		Align(lipgloss.Left).
		Render("• " + template + " (default)")
	
	fmt.Println(templateBox)
}

func (m *Messages) Template(template string) {
	// Create a clean, consistent box for the template
	templateBox := lipgloss.NewStyle().
		Foreground(TextPrimary).
		Padding(0, 2).
		Margin(0, 0, 0, 2).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor).
		Width(30).
		Align(lipgloss.Left).
		Render("• " + template)
	
	fmt.Println(templateBox)
}

// Utility Functions
func (m *Messages) DisableOutput() {
	// With lipgloss, we can just redirect to devnull or stop printing
	// For now, we'll just do nothing as lipgloss doesn't have an equivalent
}

func (m *Messages) Print(text string) {
	fmt.Print(text)
}

func (m *Messages) Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func (m *Messages) Println(text string) {
	fmt.Println(text)
}

func (m *Messages) EmptyLine() {
	fmt.Println()
}
