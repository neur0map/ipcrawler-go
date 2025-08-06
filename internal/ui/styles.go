package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Modern Monochrome Styles (aliases from modern_styles.go)
var (
	// Status styles - monochrome with only red for errors
	SuccessStyle = ModernSuccessStyle
	InfoStyle    = ModernInfoStyle
	WarningStyle = ModernWarningStyle
	ErrorStyle   = ModernErrorStyle

	// Text styles - all monochrome
	BoldStyle   = lipgloss.NewStyle().Bold(true)
	CyanStyle   = InfoText      // Now monochrome
	YellowStyle = WarningText   // Now monochrome  
	GreenStyle  = SuccessText   // Now monochrome
	GrayStyle   = SecondaryText // Muted gray

	// Header and banner styles (clean monochrome)
	HeaderStyle  = ModernBannerStyle
	SectionStyle = ModernSectionHeaderStyle
	
	// Table styling (clean monochrome)
	TableHeaderStyle  = ModernTableHeaderStyle
	TableCellStyle    = ModernTableCellStyle
	TableBorderStyle  = ModernTableBorderStyle
	TableOddRowStyle  = ModernTableOddRowStyle
	TableEvenRowStyle = ModernTableEvenRowStyle
)

// Message prefixes (modern icons)
var (
	SuccessPrefix = CheckIcon
	InfoPrefix    = InfoIcon
	WarningPrefix = WarningIcon
	ErrorPrefix   = ErrorIcon
)

// Progress and spinner styles (modern versions)
var (
	SpinnerStyle = ModernSpinnerStyle
	ProgressStyle = ModernProgressStyle
)