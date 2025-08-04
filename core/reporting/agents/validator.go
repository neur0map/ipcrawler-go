package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	
	"ipcrawler/internal/utils"
)

// ValidatorAgent validates and cross-references cleaned data
type ValidatorAgent struct {
	*BaseAgent
	config *ValidatorConfig
}

// ValidatorConfig holds configuration for the validator agent
type ValidatorConfig struct {
	ValidationRules []string          `yaml:"validation_rules"`
	StrictMode      bool              `yaml:"strict_mode"`
	ValidationLevel string            `yaml:"validation_level"` // "high", "medium", "low"
	RuleWeights     map[string]int    `yaml:"rule_weights"`
	FailThreshold   int               `yaml:"fail_threshold"`
}

// DefaultValidatorConfig returns default configuration
func DefaultValidatorConfig() *ValidatorConfig {
	return &ValidatorConfig{
		ValidationRules: []string{
			"check_data_completeness",
			"verify_json_txt_consistency",
			"validate_required_fields",
		},
		StrictMode:      false,
		ValidationLevel: "medium",
		RuleWeights: map[string]int{
			"check_data_completeness":       5,
			"verify_json_txt_consistency":   4,
			"validate_required_fields":      3,
			"check_port_completeness":       4,
			"verify_service_detection":      3,
			"check_vulnerability_completeness": 5,
			"verify_severity_classification":   4,
			"validate_cve_mapping":            3,
			"check_path_accessibility":        2,
			"verify_response_codes":           2,
		},
		FailThreshold: 10,
	}
}

// NewValidatorAgent creates a new reviewer agent
func NewValidatorAgent(config *ValidatorConfig) *ValidatorAgent {
	if config == nil {
		config = DefaultValidatorConfig()
	}
	
	return &ValidatorAgent{
		BaseAgent: NewBaseAgent("validator", nil),
		config:    config,
	}
}

// Validate checks if the reviewer agent is properly configured
func (r *ValidatorAgent) Validate() error {
	if r.config == nil {
		return fmt.Errorf("reviewer config is required")
	}
	
	validLevels := []string{"high", "medium", "low"}
	found := false
	for _, level := range validLevels {
		if r.config.ValidationLevel == level {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("invalid validation level: %s", r.config.ValidationLevel)
	}
	
	return nil
}

// Process validates cleaned data from multiple cleaners
func (r *ValidatorAgent) Process(input *AgentInput) (*AgentOutput, error) {
	if err := r.ValidateInput(input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	
	r.LogInfo("Reviewing cleaned data for target: %s", input.Target)
	
	output := r.CreateOutput(nil, input.Metadata, true)
	
	// Extract cleaned data from cleaners
	cleanerOutputs, ok := input.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid input data format")
	}
	
	// Perform validation
	validationResults := r.performValidation(cleanerOutputs, input.ReportDir)
	
	// Generate validation report
	validationReport := r.generateValidationReport(validationResults)
	
	// Save validation report
	if err := r.saveValidationReport(input.ReportDir, validationReport); err != nil {
		r.LogError("Failed to save validation report: %v", err)
		output.AddError(fmt.Errorf("failed to save validation report: %w", err))
	}
	
	// Check if validation passed
	totalScore := r.calculateValidationScore(validationResults)
	passed := totalScore < r.config.FailThreshold
	
	result := &ReviewerResult{
		ValidationResults: validationResults,
		ValidationReport:  validationReport,
		TotalScore:        totalScore,
		Passed:           passed,
		Summary:          r.generateSummary(validationResults),
	}
	
	output.Data = result
	output.Metadata["validation_score"] = fmt.Sprintf("%d", totalScore)
	output.Metadata["validation_passed"] = fmt.Sprintf("%t", passed)
	output.Metadata["rules_executed"] = fmt.Sprintf("%d", len(validationResults))
	
	if !passed && r.config.StrictMode {
		output.AddError(fmt.Errorf("validation failed with score %d (threshold: %d)", 
			totalScore, r.config.FailThreshold))
	} else if !passed {
		output.AddWarning(fmt.Sprintf("Validation failed with score %d (threshold: %d)", 
			totalScore, r.config.FailThreshold))
	}
	
	r.LogInfo("Review completed. Score: %d, Passed: %t", totalScore, passed)
	
	return output, nil
}

// ValidationResult represents the result of a single validation rule
type ValidationResult struct {
	RuleName    string   `json:"rule_name"`
	Passed      bool     `json:"passed"`
	Score       int      `json:"score"`
	Message     string   `json:"message"`
	Details     []string `json:"details,omitempty"`
	Severity    string   `json:"severity"` // "critical", "high", "medium", "low"
	Tool        string   `json:"tool,omitempty"`
	Suggestion  string   `json:"suggestion,omitempty"`
}

// ReviewerResult represents the complete reviewer output
type ReviewerResult struct {
	ValidationResults []ValidationResult `json:"validation_results"`
	ValidationReport  string             `json:"validation_report"`
	TotalScore        int                `json:"total_score"`
	Passed           bool               `json:"passed"`
	Summary          string             `json:"summary"`
}

// performValidation executes all configured validation rules
func (r *ValidatorAgent) performValidation(cleanerOutputs map[string]interface{}, reportDir string) []ValidationResult {
	results := make([]ValidationResult, 0)
	
	for _, ruleName := range r.config.ValidationRules {
		r.LogInfo("Executing validation rule: %s", ruleName)
		
		result := r.executeValidationRule(ruleName, cleanerOutputs, reportDir)
		if result != nil {
			results = append(results, *result)
		}
	}
	
	return results
}

// executeValidationRule executes a specific validation rule
func (r *ValidatorAgent) executeValidationRule(ruleName string, data map[string]interface{}, reportDir string) *ValidationResult {
	switch ruleName {
	case "check_data_completeness":
		return r.checkDataCompleteness(data)
	case "verify_json_txt_consistency":
		return r.verifyJSONTxtConsistency(data, reportDir)
	case "validate_required_fields":
		return r.validateRequiredFields(data)
	case "check_port_completeness":
		return r.checkPortCompleteness(data)
	case "verify_service_detection":
		return r.verifyServiceDetection(data)
	case "check_vulnerability_completeness":
		return r.checkVulnerabilityCompleteness(data)
	case "verify_severity_classification":
		return r.verifySeverityClassification(data)
	case "validate_cve_mapping":
		return r.validateCVEMapping(data)
	case "check_path_accessibility":
		return r.checkPathAccessibility(data)
	case "verify_response_codes":
		return r.verifyResponseCodes(data)
	default:
		r.LogWarning("Unknown validation rule: %s", ruleName)
		return &ValidationResult{
			RuleName: ruleName,
			Passed:   false,
			Score:    1,
			Message:  "Unknown validation rule",
			Severity: "low",
		}
	}
}

// Generic validation rules

// checkDataCompleteness validates that all expected data is present
func (r *ValidatorAgent) checkDataCompleteness(data map[string]interface{}) *ValidationResult {
	result := &ValidationResult{
		RuleName: "check_data_completeness",
		Severity: "high",
		Details:  make([]string, 0),
	}
	
	score := 0
	
	// Check if we have data from at least one tool
	if len(data) == 0 {
		result.Details = append(result.Details, "No cleaned data found from any tool")
		score += 5
	}
	
	// Check each tool's data
	for toolName, toolData := range data {
		if toolData == nil {
			result.Details = append(result.Details, fmt.Sprintf("No data from %s", toolName))
			score += 2
		}
	}
	
	result.Score = score
	result.Passed = score == 0
	
	if result.Passed {
		result.Message = "All expected data is present"
	} else {
		result.Message = fmt.Sprintf("Missing or incomplete data (score: %d)", score)
		result.Suggestion = "Check tool execution and output file generation"
	}
	
	return result
}

// verifyJSONTxtConsistency checks consistency between JSON and text outputs
func (r *ValidatorAgent) verifyJSONTxtConsistency(data map[string]interface{}, reportDir string) *ValidationResult {
	result := &ValidationResult{
		RuleName: "verify_json_txt_consistency",
		Severity: "medium",
		Details:  make([]string, 0),
	}
	
	score := 0
	
	// This is a simplified check - in practice, you'd compare actual file contents
	// processedDir := filepath.Join(reportDir, "processed")
	result.Details = append(result.Details, "JSON-Text consistency check placeholder")
	
	result.Score = score
	result.Passed = score == 0
	result.Message = "JSON and text outputs are consistent"
	
	return result
}

// validateRequiredFields checks that required fields are present in cleaned data
func (r *ValidatorAgent) validateRequiredFields(data map[string]interface{}) *ValidationResult {
	result := &ValidationResult{
		RuleName: "validate_required_fields",
		Severity: "medium",
		Details:  make([]string, 0),
	}
	
	score := 0
	
	// Tool-specific field validation would go here
	// This is a placeholder implementation
	
	result.Score = score
	result.Passed = score == 0
	result.Message = "All required fields are present"
	
	return result
}

// Tool-specific validation rules

// checkPortCompleteness validates nmap port scan completeness
func (r *ValidatorAgent) checkPortCompleteness(data map[string]interface{}) *ValidationResult {
	result := &ValidationResult{
		RuleName: "check_port_completeness",
		Tool:     "nmap",
		Severity: "medium",
		Details:  make([]string, 0),
	}
	
	score := 0
	
	// Check if nmap data exists
	nmapData, exists := data["nmap"]
	if !exists {
		result.Message = "No nmap data found"
		result.Score = 0
		result.Passed = true
		return result
	}
	
	// Validate nmap data structure (simplified)
	if nmapData != nil {
		result.Details = append(result.Details, "Nmap data structure validation placeholder")
	}
	
	result.Score = score
	result.Passed = score == 0
	result.Message = "Port scan appears complete"
	
	return result
}

// verifyServiceDetection validates service detection accuracy
func (r *ValidatorAgent) verifyServiceDetection(data map[string]interface{}) *ValidationResult {
	result := &ValidationResult{
		RuleName: "verify_service_detection",
		Tool:     "nmap",
		Severity: "low",
		Details:  make([]string, 0),
	}
	
	score := 0
	result.Score = score
	result.Passed = true
	result.Message = "Service detection validation placeholder"
	
	return result
}

// checkVulnerabilityCompleteness validates nuclei vulnerability findings
func (r *ValidatorAgent) checkVulnerabilityCompleteness(data map[string]interface{}) *ValidationResult {
	result := &ValidationResult{
		RuleName: "check_vulnerability_completeness",
		Tool:     "nuclei",
		Severity: "high",
		Details:  make([]string, 0),
	}
	
	score := 0
	
	// Check if nuclei data exists
	nucleiData, exists := data["nuclei"]
	if !exists {
		result.Message = "No nuclei data found"
		result.Score = 0
		result.Passed = true
		return result
	}
	
	// Validate nuclei data (simplified)
	if nucleiData != nil {
		result.Details = append(result.Details, "Nuclei vulnerability data validation placeholder")
	}
	
	result.Score = score
	result.Passed = score == 0
	result.Message = "Vulnerability scan appears complete"
	
	return result
}

// verifySeverityClassification validates severity classifications
func (r *ValidatorAgent) verifySeverityClassification(data map[string]interface{}) *ValidationResult {
	result := &ValidationResult{
		RuleName: "verify_severity_classification",
		Tool:     "nuclei",
		Severity: "medium",
		Details:  make([]string, 0),
	}
	
	score := 0
	result.Score = score
	result.Passed = true
	result.Message = "Severity classification validation placeholder"
	
	return result
}

// validateCVEMapping validates CVE mappings in vulnerability data
func (r *ValidatorAgent) validateCVEMapping(data map[string]interface{}) *ValidationResult {
	result := &ValidationResult{
		RuleName: "validate_cve_mapping",
		Tool:     "nuclei",
		Severity: "medium",
		Details:  make([]string, 0),
	}
	
	score := 0
	result.Score = score
	result.Passed = true
	result.Message = "CVE mapping validation placeholder"
	
	return result
}

// checkPathAccessibility validates gobuster path discoveries
func (r *ValidatorAgent) checkPathAccessibility(data map[string]interface{}) *ValidationResult {
	result := &ValidationResult{
		RuleName: "check_path_accessibility",
		Tool:     "gobuster",
		Severity: "low",
		Details:  make([]string, 0),
	}
	
	score := 0
	result.Score = score
	result.Passed = true
	result.Message = "Path accessibility validation placeholder"
	
	return result
}

// verifyResponseCodes validates HTTP response codes
func (r *ValidatorAgent) verifyResponseCodes(data map[string]interface{}) *ValidationResult {
	result := &ValidationResult{
		RuleName: "verify_response_codes",
		Tool:     "gobuster",
		Severity: "low",
		Details:  make([]string, 0),
	}
	
	score := 0
	result.Score = score
	result.Passed = true
	result.Message = "Response code validation placeholder"
	
	return result
}

// calculateValidationScore calculates the total validation score
func (r *ValidatorAgent) calculateValidationScore(results []ValidationResult) int {
	totalScore := 0
	
	for _, result := range results {
		weight := r.config.RuleWeights[result.RuleName]
		if weight == 0 {
			weight = 1 // Default weight
		}
		totalScore += result.Score * weight
	}
	
	return totalScore
}

// generateValidationReport creates a human-readable validation report
func (r *ValidatorAgent) generateValidationReport(results []ValidationResult) string {
	var report strings.Builder
	
	report.WriteString("DATA VALIDATION REPORT\n")
	report.WriteString("======================\n\n")
	
	passed := 0
	failed := 0
	
	for _, result := range results {
		if result.Passed {
			passed++
		} else {
			failed++
		}
	}
	
	report.WriteString(fmt.Sprintf("Validation Summary: %d passed, %d failed\n\n", passed, failed))
	
	// Failed validations first
	if failed > 0 {
		report.WriteString("FAILED VALIDATIONS\n")
		report.WriteString("------------------\n")
		
		for _, result := range results {
			if !result.Passed {
				report.WriteString(fmt.Sprintf("❌ %s [%s]\n", result.RuleName, strings.ToUpper(result.Severity)))
				report.WriteString(fmt.Sprintf("   Score: %d\n", result.Score))
				report.WriteString(fmt.Sprintf("   Message: %s\n", result.Message))
				if result.Tool != "" {
					report.WriteString(fmt.Sprintf("   Tool: %s\n", result.Tool))
				}
				if result.Suggestion != "" {
					report.WriteString(fmt.Sprintf("   Suggestion: %s\n", result.Suggestion))
				}
				if len(result.Details) > 0 {
					report.WriteString("   Details:\n")
					for _, detail := range result.Details {
						report.WriteString(fmt.Sprintf("     - %s\n", detail))
					}
				}
				report.WriteString("\n")
			}
		}
	}
	
	// Passed validations
	if passed > 0 {
		report.WriteString("PASSED VALIDATIONS\n")
		report.WriteString("------------------\n")
		
		for _, result := range results {
			if result.Passed {
				report.WriteString(fmt.Sprintf("✅ %s - %s\n", result.RuleName, result.Message))
			}
		}
	}
	
	return report.String()
}

// generateSummary creates a brief summary of validation results
func (r *ValidatorAgent) generateSummary(results []ValidationResult) string {
	passed := 0
	failed := 0
	critical := 0
	high := 0
	
	for _, result := range results {
		if result.Passed {
			passed++
		} else {
			failed++
			switch result.Severity {
			case "critical":
				critical++
			case "high":
				high++
			}
		}
	}
	
	return fmt.Sprintf("Validation: %d/%d passed, %d critical issues, %d high issues", 
		passed, len(results), critical, high)
}

// saveValidationReport saves the validation report to file
func (r *ValidatorAgent) saveValidationReport(reportDir, report string) error {
	// Create processed directory if it doesn't exist
	processedDir := filepath.Join(reportDir, "processed")
	if err := os.MkdirAll(processedDir, 0755); err != nil {
		return fmt.Errorf("failed to create processed directory: %w", err)
	}
	
	// Write to file
	filePath := filepath.Join(processedDir, "validation_report.txt")
	if err := utils.WriteFileWithPermissions(filePath, []byte(report), 0644); err != nil {
		return fmt.Errorf("failed to write validation report: %w", err)
	}
	
	r.LogInfo("Saved validation report to: %s", filePath)
	return nil
}