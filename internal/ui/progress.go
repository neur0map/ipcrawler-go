package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ProgressDisplay manages real-time scan progress and results
type ProgressDisplay struct {
	mutex       sync.Mutex
	target      string
	toolName    string
	startTime   time.Time
	isActive    bool
	discoveredPorts []PortInfo
	detectedServices []ServiceInfo
	currentStatus string
}

// ServiceInfo represents detected service information
type ServiceInfo struct {
	Port    int
	Service string
	Version string
}

// NewProgressDisplay creates a new progress display instance
func NewProgressDisplay(target, toolName string) *ProgressDisplay {
	return &ProgressDisplay{
		target:           target,
		toolName:         toolName,
		startTime:        time.Now(),
		discoveredPorts:  []PortInfo{},
		detectedServices: []ServiceInfo{},
	}
}

// Start begins the progress display
func (p *ProgressDisplay) Start() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	p.isActive = true
	p.startTime = time.Now()
	
	// Display initial status with generic message  
	badge := CreateBadge("RUNNING", "info")
	message := SpinnerStyle.Render(fmt.Sprintf("%s Running %s scan...", 
		SpinnerIcon, p.toolName))
	fmt.Println(lipgloss.JoinHorizontal(lipgloss.Left, badge, " ", message))
}

// AddDiscoveredPort adds a newly discovered port and displays it immediately
func (p *ProgressDisplay) AddDiscoveredPort(port PortInfo) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	if !p.isActive {
		return
	}
	
	p.discoveredPorts = append(p.discoveredPorts, port)
	
	// Display simple port discovery (no service details - that's for nmap)
	portStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("46")). // Green
		Bold(true)
	
	icon := "🔍"
	
	// Show just the open port for discovery phase
	message := fmt.Sprintf("%s Found open port: %s", 
		icon, 
		portStyle.Render(fmt.Sprintf("%d/tcp", port.Number)))
	
	fmt.Println("  " + message)
}

// AddDetectedService adds a newly detected service and displays it
func (p *ProgressDisplay) AddDetectedService(service ServiceInfo) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	if !p.isActive {
		return
	}
	
	p.detectedServices = append(p.detectedServices, service)
	
	// Display only the actual tool detection (no database override)
	serviceStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("33")). // Yellow
		Bold(true)
	
	versionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")) // Blue
	
	icon := "⚡"
	
	// Show actual detected service and version
	displayText := service.Service
	if service.Version != "" {
		displayText += " " + versionStyle.Render(service.Version)
	}
	
	message := fmt.Sprintf("%s Port %d: %s", 
		icon, service.Port, serviceStyle.Render(displayText))
	
	fmt.Println("  " + message)
}

// UpdateStatus updates the current scanning status
func (p *ProgressDisplay) UpdateStatus(status string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	if !p.isActive {
		return
	}
	
	p.currentStatus = status
}

// Complete marks the scan as completed and shows summary
func (p *ProgressDisplay) Complete() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	if !p.isActive {
		return
	}
	
	p.isActive = false
	duration := time.Since(p.startTime)
	
	// Display completion status
	badge := CreateBadge("SUCCESS", "success")
	
	var summaryParts []string
	if len(p.discoveredPorts) > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d ports", len(p.discoveredPorts)))
	}
	if len(p.detectedServices) > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d services", len(p.detectedServices)))
	}
	
	summary := ""
	if len(summaryParts) > 0 {
		summary = " (" + strings.Join(summaryParts, ", ") + ")"
	}
	
	message := SuccessText.Render(fmt.Sprintf("%s %s scan completed%s in %v", 
		CheckIcon, p.toolName, summary, duration.Round(time.Millisecond)))
	
	fmt.Println(lipgloss.JoinHorizontal(lipgloss.Left, badge, " ", message))
	fmt.Println()
}

// Fail marks the scan as failed
func (p *ProgressDisplay) Fail(errorMsg string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	if !p.isActive {
		return
	}
	
	p.isActive = false
	
	badge := CreateBadge("FAILED", "error")
	message := ErrorText.Render(fmt.Sprintf("%s %s scan failed: %s", 
		CrossIcon, p.toolName, errorMsg))
	
	fmt.Println(lipgloss.JoinHorizontal(lipgloss.Left, badge, " ", message))
	fmt.Println()
}

// GetDiscoveredPorts returns all discovered ports
func (p *ProgressDisplay) GetDiscoveredPorts() []PortInfo {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return append([]PortInfo{}, p.discoveredPorts...)
}

// GetDetectedServices returns all detected services
func (p *ProgressDisplay) GetDetectedServices() []ServiceInfo {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return append([]ServiceInfo{}, p.detectedServices...)
}

// ShowFinalSummary displays a comprehensive summary if there are results
func (p *ProgressDisplay) ShowFinalSummary() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	if len(p.discoveredPorts) == 0 && len(p.detectedServices) == 0 {
		return
	}
	
	// Create a summary table only if we have many ports or detailed service info
	if len(p.detectedServices) > 0 {
		// Show service detection table (which includes port info)
		fmt.Println(ModernSectionHeaderStyle.Render("📊 Service Detection Summary"))
		
		// Convert services to PortInfo format for table display
		var servicesPorts []PortInfo
		for _, service := range p.detectedServices {
			servicesPorts = append(servicesPorts, PortInfo{
				Number:  service.Port,
				State:   "open",
				Service: service.Service,
				Version: service.Version,
			})
		}
		Global.Tables.RenderServiceDetectionTable(servicesPorts)
		fmt.Println()
	} else if len(p.discoveredPorts) > 8 {
		// Only show port discovery table if we have many ports and no service details
		fmt.Println(ModernSectionHeaderStyle.Render("📊 Port Discovery Summary"))
		Global.Tables.RenderPortDiscoveryTable(p.discoveredPorts)
		fmt.Println()
	}
}