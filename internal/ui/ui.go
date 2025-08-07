package ui

import (
	"fmt"
)

// Task status constants
const (
	TaskPending   = "pending"
	TaskRunning   = "running"
	TaskCompleted = "completed"
	TaskFailed    = "failed"
)

// Notification type constants
const (
	NotificationSuccess = "success"
	NotificationError   = "error"
	NotificationWarning = "warning"
	NotificationInfo    = "info"
)

// Messages provides simple message output
type Messages struct{}

// NewMessages creates a new Messages instance
func NewMessages() *Messages {
	return &Messages{}
}

func (m *Messages) SystemHealthOK() {
	fmt.Println("✅ System Status: All systems operational")
}

func (m *Messages) SystemVersion(version string) {
	fmt.Printf("🔧 IPCrawler v%s - Network Security Scanner\n", version)
}

func (m *Messages) RunningWithRootPrivileges() {
	fmt.Println("⚠️ Running with root privileges")
}

func (m *Messages) AllSystemsOperational() {
	fmt.Println("✅ All scanning tools available and operational")
}

func (m *Messages) AvailableTemplates() {
	fmt.Println("📄 Available templates:")
}

func (m *Messages) DefaultTemplate(template string) {
	fmt.Printf("  ✓ %s (default)\n", template)
}

func (m *Messages) Template(template string) {
	fmt.Printf("  • %s\n", template)
}

func (m *Messages) ScanCompleted(target string) {
	fmt.Printf("✅ Scan completed for %s\n", target)
}

func (m *Messages) ResultsSaved() {
	fmt.Println("💾 Results saved to workspace")
}

func (m *Messages) ScanCancelled() {
	fmt.Println("❌ Scan cancelled by user")
}

func (m *Messages) DisableOutput() {
	// Disable output - placeholder
}

func (m *Messages) Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

// Banners provides banner display functionality
type Banners struct{}

func NewBanners() *Banners {
	return &Banners{}
}

func (b *Banners) ShowApplicationBanner(version, target, template string) {
	fmt.Printf(`
🚀 IPCrawler v%s
===================
🎯 Target: %s
📋 Template: %s

`, version, target, template)
}

// Interactive provides interactive functionality (placeholder)
type Interactive struct{}

func NewInteractive() *Interactive {
	return &Interactive{}
}

// SimpleLiveDashboard provides minimal dashboard (placeholder)
type SimpleLiveDashboard struct{}

func (s *SimpleLiveDashboard) Enable()  {}
func (s *SimpleLiveDashboard) Disable() {}
func (s *SimpleLiveDashboard) Start()   {}

func (s *SimpleLiveDashboard) AddTask(id, name string, status interface{}) {}
func (s *SimpleLiveDashboard) AddNotification(notifType interface{}, message string) {
	switch notifType.(type) {
	default:
		fmt.Printf("📢 %s\n", message)
	}
}

var GlobalSimpleDashboard = &SimpleLiveDashboard{}

// UI provides a centralized interface for all user interface operations
type UI struct {
	Messages        *Messages
	Banners         *Banners
	Interactive     *Interactive
	SimpleDashboard *SimpleLiveDashboard
}

// New creates a new UI instance
func New() *UI {
	return &UI{
		Messages:        NewMessages(),
		Banners:         NewBanners(),
		Interactive:     NewInteractive(),
		SimpleDashboard: GlobalSimpleDashboard,
	}
}

func (ui *UI) EnableModernUI()  {}
func (ui *UI) DisableModernUI() {}

// Global UI instance
var Global *UI

func init() {
	Global = New()
}