package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// DefaultRegistryManager implements the RegistryManager interface
type DefaultRegistryManager struct {
	database *RegistryDatabase
	dbPath   string
	mutex    sync.RWMutex
	autoSave bool
}

// NewRegistryManager creates a new registry manager
func NewRegistryManager(dbPath string) (*DefaultRegistryManager, error) {
	manager := &DefaultRegistryManager{
		dbPath:   dbPath,
		autoSave: true,
		database: &RegistryDatabase{
			Version:     "1.0.0",
			LastUpdated: time.Now(),
			Variables:   make(map[string]*VariableRecord),
			Statistics: RegistryStatistics{
				VariablesByType:     make(map[VariableType]int),
				VariablesByCategory: make(map[VariableCategory]int),
				VariablesBySource:   make(map[VariableSource]int),
			},
		},
	}

	// Try to load existing database
	if err := manager.LoadDatabase(); err != nil {
		// If file doesn't exist, that's ok - we'll create it
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load registry database: %w", err)
		}
	}

	return manager, nil
}

// AddVariable adds a new variable to the registry
func (rm *DefaultRegistryManager) AddVariable(record *VariableRecord) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if record.Name == "" {
		return fmt.Errorf("variable name cannot be empty")
	}

	// Set defaults
	if record.FirstDetected.IsZero() {
		record.FirstDetected = time.Now()
	}
	record.LastSeen = time.Now()

	// Add to database
	rm.database.Variables[record.Name] = record
	rm.database.LastUpdated = time.Now()

	// Update statistics
	rm.updateStatistics()

	if rm.autoSave {
		return rm.saveDatabase()
	}

	return nil
}

// GetVariable retrieves a variable by name
func (rm *DefaultRegistryManager) GetVariable(name string) (*VariableRecord, error) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	record, exists := rm.database.Variables[name]
	if !exists {
		return nil, fmt.Errorf("variable '%s' not found", name)
	}

	// Create a copy to avoid external modification
	recordCopy := *record
	return &recordCopy, nil
}

// UpdateVariable updates an existing variable
func (rm *DefaultRegistryManager) UpdateVariable(name string, record *VariableRecord) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if _, exists := rm.database.Variables[name]; !exists {
		return fmt.Errorf("variable '%s' not found", name)
	}

	record.LastSeen = time.Now()
	rm.database.Variables[name] = record
	rm.database.LastUpdated = time.Now()

	// Update statistics
	rm.updateStatistics()

	if rm.autoSave {
		return rm.saveDatabase()
	}

	return nil
}

// DeleteVariable removes a variable from the registry
func (rm *DefaultRegistryManager) DeleteVariable(name string) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if _, exists := rm.database.Variables[name]; !exists {
		return fmt.Errorf("variable '%s' not found", name)
	}

	delete(rm.database.Variables, name)
	rm.database.LastUpdated = time.Now()

	// Update statistics
	rm.updateStatistics()

	if rm.autoSave {
		return rm.saveDatabase()
	}

	return nil
}

// ListVariables returns all variables in the registry
func (rm *DefaultRegistryManager) ListVariables() []*VariableRecord {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	var variables []*VariableRecord
	for _, record := range rm.database.Variables {
		recordCopy := *record
		variables = append(variables, &recordCopy)
	}

	// Sort by name
	sort.Slice(variables, func(i, j int) bool {
		return variables[i].Name < variables[j].Name
	})

	return variables
}

// GetVariablesByType returns variables of a specific type
func (rm *DefaultRegistryManager) GetVariablesByType(varType VariableType) []*VariableRecord {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	var variables []*VariableRecord
	for _, record := range rm.database.Variables {
		if record.Type == varType {
			recordCopy := *record
			variables = append(variables, &recordCopy)
		}
	}

	return variables
}

// GetVariablesByCategory returns variables of a specific category
func (rm *DefaultRegistryManager) GetVariablesByCategory(category VariableCategory) []*VariableRecord {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	var variables []*VariableRecord
	for _, record := range rm.database.Variables {
		if record.Category == category {
			recordCopy := *record
			variables = append(variables, &recordCopy)
		}
	}

	return variables
}

// GetVariablesByTool returns variables associated with a specific tool
func (rm *DefaultRegistryManager) GetVariablesByTool(toolName string) []*VariableRecord {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	var variables []*VariableRecord
	for _, record := range rm.database.Variables {
		if record.ToolName == toolName {
			recordCopy := *record
			variables = append(variables, &recordCopy)
		}
	}

	return variables
}

// SearchVariables searches for variables by name, description, or tags
func (rm *DefaultRegistryManager) SearchVariables(query string) []*VariableRecord {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	query = strings.ToLower(query)
	var variables []*VariableRecord

	for _, record := range rm.database.Variables {
		match := false

		// Search in name
		if strings.Contains(strings.ToLower(record.Name), query) {
			match = true
		}

		// Search in description
		if strings.Contains(strings.ToLower(record.Description), query) {
			match = true
		}

		// Search in tags
		for _, tag := range record.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				match = true
				break
			}
		}

		// Search in tool name
		if strings.Contains(strings.ToLower(record.ToolName), query) {
			match = true
		}

		if match {
			recordCopy := *record
			variables = append(variables, &recordCopy)
		}
	}

	return variables
}

// AutoRegisterVariable automatically registers a variable from detection
func (rm *DefaultRegistryManager) AutoRegisterVariable(name string, context DetectionContext) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	// Check if variable already exists
	if existingRecord, exists := rm.database.Variables[name]; exists {
		// Update existing record
		existingRecord.LastSeen = time.Now()
		existingRecord.UsageCount++

		// Add usage location if not already present
		location := UsageLocation{
			Type:    getLocationTypeFromSource(context.Source),
			Path:    context.FilePath,
			Line:    context.LineNumber,
			Context: context.Context,
		}

		if !containsUsageLocation(existingRecord.UsedIn, location) {
			existingRecord.UsedIn = append(existingRecord.UsedIn, location)
		}

		rm.database.LastUpdated = time.Now()
		rm.updateStatistics()

		if rm.autoSave {
			return rm.saveDatabase()
		}
		return nil
	}

	// Create new variable record
	record := &VariableRecord{
		Name:          name,
		Type:          classifyVariableType(name, context),
		Category:      classifyVariableCategory(name, context),
		Description:   generateDescription(name, context),
		DataType:      inferDataType(name, context),
		Source:        context.Source,
		ToolName:      context.Tool,
		FirstDetected: context.Timestamp,
		LastSeen:      context.Timestamp,
		UsageCount:    1,
		AutoDetected:  true,
		UsedIn: []UsageLocation{
			{
				Type:    getLocationTypeFromSource(context.Source),
				Path:    context.FilePath,
				Line:    context.LineNumber,
				Context: context.Context,
			},
		},
	}

	rm.database.Variables[name] = record
	rm.database.LastUpdated = time.Now()
	rm.updateStatistics()

	if rm.autoSave {
		return rm.saveDatabase()
	}

	return nil
}

// UpdateUsage updates usage statistics for a variable
func (rm *DefaultRegistryManager) UpdateUsage(name string, location UsageLocation) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	record, exists := rm.database.Variables[name]
	if !exists {
		return fmt.Errorf("variable '%s' not found", name)
	}

	record.LastSeen = time.Now()
	record.UsageCount++

	// Add usage location if not already present
	if !containsUsageLocation(record.UsedIn, location) {
		record.UsedIn = append(record.UsedIn, location)
	}

	rm.database.LastUpdated = time.Now()
	rm.updateStatistics()

	if rm.autoSave {
		return rm.saveDatabase()
	}

	return nil
}

// GetStatistics returns current registry statistics
func (rm *DefaultRegistryManager) GetStatistics() RegistryStatistics {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	// Create a copy of statistics to avoid data races
	stats := RegistryStatistics{
		TotalVariables:      rm.database.Statistics.TotalVariables,
		AutoDetectedCount:   rm.database.Statistics.AutoDetectedCount,
		ManuallyAddedCount:  rm.database.Statistics.ManuallyAddedCount,
		DeprecatedCount:     rm.database.Statistics.DeprecatedCount,
		VariablesByType:     make(map[VariableType]int),
		VariablesByCategory: make(map[VariableCategory]int),
		VariablesBySource:   make(map[VariableSource]int),
		MostUsedVariables:   make([]VariableUsageRank, len(rm.database.Statistics.MostUsedVariables)),
		UnusedVariables:     make([]string, len(rm.database.Statistics.UnusedVariables)),
	}

	// Deep copy maps to avoid race conditions
	for k, v := range rm.database.Statistics.VariablesByType {
		stats.VariablesByType[k] = v
	}
	for k, v := range rm.database.Statistics.VariablesByCategory {
		stats.VariablesByCategory[k] = v
	}
	for k, v := range rm.database.Statistics.VariablesBySource {
		stats.VariablesBySource[k] = v
	}
	copy(stats.MostUsedVariables, rm.database.Statistics.MostUsedVariables)
	copy(stats.UnusedVariables, rm.database.Statistics.UnusedVariables)

	// Recalculate real-time statistics on the copy
	rm.calculateStatistics(&stats)

	return stats
}

// GetUnusedVariables returns variables that appear to be unused
func (rm *DefaultRegistryManager) GetUnusedVariables() []string {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	var unused []string
	for name, record := range rm.database.Variables {
		if record.UsageCount == 0 || len(record.UsedIn) == 0 {
			unused = append(unused, name)
		}
	}

	sort.Strings(unused)
	return unused
}

// ValidateRegistry validates the registry and returns a list of issues
func (rm *DefaultRegistryManager) ValidateRegistry() []string {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	var issues []string

	for name, record := range rm.database.Variables {
		// Check for empty required fields
		if record.Description == "" {
			issues = append(issues, fmt.Sprintf("Variable '%s' has no description", name))
		}

		if record.Type == "" {
			issues = append(issues, fmt.Sprintf("Variable '%s' has no type", name))
		}

		if record.Category == "" {
			issues = append(issues, fmt.Sprintf("Variable '%s' has no category", name))
		}

		// Check for deprecated variables
		if record.Deprecated && record.ReplacedBy == "" {
			issues = append(issues, fmt.Sprintf("Deprecated variable '%s' has no replacement specified", name))
		}

		// Check for missing example values
		if len(record.ExampleValues) == 0 && record.Type == MagicVariable {
			issues = append(issues, fmt.Sprintf("Magic variable '%s' has no example values", name))
		}
	}

	return issues
}

// SaveDatabase saves the registry to disk
func (rm *DefaultRegistryManager) SaveDatabase() error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()
	return rm.saveDatabase()
}

// LoadDatabase loads the registry from disk
func (rm *DefaultRegistryManager) LoadDatabase() error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	data, err := os.ReadFile(rm.dbPath)
	if err != nil {
		return err
	}

	var database RegistryDatabase
	if err := json.Unmarshal(data, &database); err != nil {
		return fmt.Errorf("failed to parse registry database: %w", err)
	}

	rm.database = &database
	rm.updateStatistics()

	return nil
}

// ExportDatabase exports the registry in the specified format
func (rm *DefaultRegistryManager) ExportDatabase(format string) ([]byte, error) {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	switch strings.ToLower(format) {
	case "json", "":
		return json.MarshalIndent(rm.database, "", "  ")
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// Private helper methods

func (rm *DefaultRegistryManager) saveDatabase() error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(rm.dbPath), 0755); err != nil {
		return fmt.Errorf("failed to create registry directory: %w", err)
	}

	// Update statistics before saving
	rm.updateStatistics()

	data, err := json.MarshalIndent(rm.database, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry database: %w", err)
	}

	if err := os.WriteFile(rm.dbPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write registry database: %w", err)
	}

	return nil
}

func (rm *DefaultRegistryManager) updateStatistics() {
	// Note: This should be called while holding a write lock
	rm.calculateStatistics(&rm.database.Statistics)
}

func (rm *DefaultRegistryManager) calculateStatistics(stats *RegistryStatistics) {
	// Reset counters
	stats.TotalVariables = len(rm.database.Variables)

	// Initialize maps if they're nil to avoid panic
	if stats.VariablesByType == nil {
		stats.VariablesByType = make(map[VariableType]int)
	} else {
		// Clear existing data
		for k := range stats.VariablesByType {
			delete(stats.VariablesByType, k)
		}
	}

	if stats.VariablesByCategory == nil {
		stats.VariablesByCategory = make(map[VariableCategory]int)
	} else {
		// Clear existing data
		for k := range stats.VariablesByCategory {
			delete(stats.VariablesByCategory, k)
		}
	}

	if stats.VariablesBySource == nil {
		stats.VariablesBySource = make(map[VariableSource]int)
	} else {
		// Clear existing data
		for k := range stats.VariablesBySource {
			delete(stats.VariablesBySource, k)
		}
	}

	stats.AutoDetectedCount = 0
	stats.ManuallyAddedCount = 0
	stats.DeprecatedCount = 0

	var usageRanks []VariableUsageRank

	for _, record := range rm.database.Variables {
		// Count by type
		stats.VariablesByType[record.Type]++

		// Count by category
		stats.VariablesByCategory[record.Category]++

		// Count by source
		stats.VariablesBySource[record.Source]++

		// Count auto-detected vs manual
		if record.AutoDetected {
			stats.AutoDetectedCount++
		} else {
			stats.ManuallyAddedCount++
		}

		// Count deprecated
		if record.Deprecated {
			stats.DeprecatedCount++
		}

		// Track usage
		usageRanks = append(usageRanks, VariableUsageRank{
			Name:       record.Name,
			UsageCount: record.UsageCount,
		})
	}

	// Sort by usage and take top 10
	sort.Slice(usageRanks, func(i, j int) bool {
		return usageRanks[i].UsageCount > usageRanks[j].UsageCount
	})

	if len(usageRanks) > 10 {
		usageRanks = usageRanks[:10]
	}
	stats.MostUsedVariables = usageRanks

	// Find unused variables (inline to avoid deadlock)
	var unused []string
	for name, record := range rm.database.Variables {
		if record.UsageCount == 0 || len(record.UsedIn) == 0 {
			unused = append(unused, name)
		}
	}
	sort.Strings(unused)
	stats.UnusedVariables = unused
}

// Helper functions for classification

func classifyVariableType(name string, context DetectionContext) VariableType {
	if strings.HasPrefix(name, "{{combined_") {
		return CombinedVariable
	}

	switch context.Source {
	case ToolParserSource:
		return MagicVariable
	case ResultCombinerSource:
		return CombinedVariable
	case ConfigFileSource:
		return ConfigVariable
	case WorkflowFileSource:
		return WorkflowVariable
	default:
		if context.Tool != "" {
			return MagicVariable
		}
		return TemplateVariable
	}
}

func classifyVariableCategory(name string, context DetectionContext) VariableCategory {
	name = strings.ToLower(name)

	if strings.Contains(name, "port") {
		return PortCategory
	}
	if strings.Contains(name, "service") {
		return ServiceCategory
	}
	if strings.Contains(name, "host") {
		return HostCategory
	}
	if strings.Contains(name, "dir") || strings.Contains(name, "path") {
		return DirectoryCategory
	}
	if strings.Contains(name, "file") {
		return FileCategory
	}
	if strings.Contains(name, "timestamp") || strings.Contains(name, "session") {
		return MetadataCategory
	}
	if strings.Contains(name, "coverage") || strings.Contains(name, "analysis") {
		return AnalysisCategory
	}

	// Core variables
	if strings.Contains(name, "target") || strings.Contains(name, "workspace") {
		return CoreCategory
	}

	return CoreCategory // Default
}

func generateDescription(name string, context DetectionContext) string {
	// Remove brackets for processing
	cleanName := strings.Trim(name, "{}")

	// Generate basic description based on name
	if strings.HasPrefix(cleanName, "combined_") {
		return fmt.Sprintf("Combined result from multiple %s scans", context.Tool)
	}

	if context.Tool != "" {
		return fmt.Sprintf("Generated by %s tool", context.Tool)
	}

	// Generate description based on name patterns
	if strings.Contains(cleanName, "port") {
		return "Port-related information"
	}
	if strings.Contains(cleanName, "service") {
		return "Service-related information"
	}
	if strings.Contains(cleanName, "host") {
		return "Host-related information"
	}

	return fmt.Sprintf("Auto-detected variable: %s", cleanName)
}

func inferDataType(name string, context DetectionContext) DataType {
	name = strings.ToLower(name)

	if strings.Contains(name, "count") {
		return IntegerType
	}
	if strings.Contains(name, "port") && (strings.Contains(name, "list") || strings.HasSuffix(name, "ports")) {
		return PortListType
	}
	if strings.Contains(name, "dir") || strings.Contains(name, "path") {
		return PathType
	}
	if strings.Contains(name, "target") {
		return IPType
	}

	return StringType // Default
}

func getLocationTypeFromSource(source VariableSource) string {
	switch source {
	case ToolParserSource:
		return "tool"
	case ConfigFileSource:
		return "config"
	case WorkflowFileSource:
		return "workflow"
	default:
		return "code"
	}
}

func containsUsageLocation(locations []UsageLocation, location UsageLocation) bool {
	for _, loc := range locations {
		if loc.Type == location.Type && loc.Path == location.Path && loc.Line == location.Line {
			return true
		}
	}
	return false
}
