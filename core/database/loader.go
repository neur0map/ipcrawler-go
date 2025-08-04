package database

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync"
)

// DatabaseManager handles loading and caching of database files
type DatabaseManager struct {
	basePath       string
	highRiskPorts  *HighRiskPortsDB
	commonServices *CommonServicesDB
	riskLevels     *RiskLevelsDB
	mutex          sync.RWMutex
}

// NewDatabaseManager creates a new database manager
func NewDatabaseManager(basePath string) *DatabaseManager {
	if basePath == "" {
		basePath = "database"
	}
	
	return &DatabaseManager{
		basePath: basePath,
	}
}

// GetHighRiskPorts returns the high risk ports database
func (dm *DatabaseManager) GetHighRiskPorts() (*HighRiskPortsDB, error) {
	dm.mutex.RLock()
	if dm.highRiskPorts != nil {
		defer dm.mutex.RUnlock()
		return dm.highRiskPorts, nil
	}
	dm.mutex.RUnlock()

	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	// Double-check locking pattern
	if dm.highRiskPorts != nil {
		return dm.highRiskPorts, nil
	}

	filePath := filepath.Join(dm.basePath, "ports", "high_risk_ports.json")
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read high risk ports database: %w", err)
	}

	var db HighRiskPortsDB
	if err := json.Unmarshal(data, &db); err != nil {
		return nil, fmt.Errorf("failed to parse high risk ports database: %w", err)
	}

	dm.highRiskPorts = &db
	return dm.highRiskPorts, nil
}

// GetCommonServices returns the common services database
func (dm *DatabaseManager) GetCommonServices() (*CommonServicesDB, error) {
	dm.mutex.RLock()
	if dm.commonServices != nil {
		defer dm.mutex.RUnlock()
		return dm.commonServices, nil
	}
	dm.mutex.RUnlock()

	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	if dm.commonServices != nil {
		return dm.commonServices, nil
	}

	filePath := filepath.Join(dm.basePath, "services", "common_services.json")
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read common services database: %w", err)
	}

	var db CommonServicesDB
	if err := json.Unmarshal(data, &db); err != nil {
		return nil, fmt.Errorf("failed to parse common services database: %w", err)
	}

	dm.commonServices = &db
	return dm.commonServices, nil
}

// GetRiskLevels returns the risk levels database
func (dm *DatabaseManager) GetRiskLevels() (*RiskLevelsDB, error) {
	dm.mutex.RLock()
	if dm.riskLevels != nil {
		defer dm.mutex.RUnlock()
		return dm.riskLevels, nil
	}
	dm.mutex.RUnlock()

	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	if dm.riskLevels != nil {
		return dm.riskLevels, nil
	}

	filePath := filepath.Join(dm.basePath, "vulnerabilities", "risk_levels.json")
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read risk levels database: %w", err)
	}

	var db RiskLevelsDB
	if err := json.Unmarshal(data, &db); err != nil {
		return nil, fmt.Errorf("failed to parse risk levels database: %w", err)
	}

	dm.riskLevels = &db
	return dm.riskLevels, nil
}

// IsHighRiskPort checks if a port is considered high risk
func (dm *DatabaseManager) IsHighRiskPort(port int) (bool, *PortInfo, error) {
	db, err := dm.GetHighRiskPorts()
	if err != nil {
		return false, nil, err
	}

	return db.IsHighRiskPort(port)
}

// GetServiceInfo returns information about a service
func (dm *DatabaseManager) GetServiceInfo(serviceName string, port int) (*ServiceInfo, error) {
	db, err := dm.GetCommonServices()
	if err != nil {
		return nil, err
	}

	return db.GetServiceInfo(serviceName, port), nil
}

// CalculateRiskScore calculates risk score for a port/service combination
func (dm *DatabaseManager) CalculateRiskScore(port int, serviceName string, hasAuth bool, isEncrypted bool, isInternetFacing bool) (float64, string, error) {
	riskDB, err := dm.GetRiskLevels()
	if err != nil {
		return 0, "", err
	}

	// Check if port is in high-risk categories
	baseScore := 0.0
	for _, criticalPort := range riskDB.PortRiskMappings.CriticalPorts {
		if port == criticalPort {
			baseScore = 9.0
			break
		}
	}
	
	if baseScore == 0.0 {
		for _, highRiskPort := range riskDB.PortRiskMappings.HighRiskPorts {
			if port == highRiskPort {
				baseScore = 7.0
				break
			}
		}
	}
	
	if baseScore == 0.0 {
		for _, mediumRiskPort := range riskDB.PortRiskMappings.MediumRiskPorts {
			if port == mediumRiskPort {
				baseScore = 4.0
				break
			}
		}
	}
	
	if baseScore == 0.0 {
		baseScore = 1.0 // Default low risk
	}

	// Apply risk factors
	factors := riskDB.ServiceRiskFactors
	
	// Authentication factor
	if !hasAuth {
		baseScore += factors.Authentication.NoAuthRequired
	}
	
	// Encryption factor
	if !isEncrypted {
		baseScore += factors.Encryption.NoEncryption
	}
	
	// Exposure factor
	if isInternetFacing {
		baseScore += factors.Exposure.InternetFacing
	}

	// Cap at 10.0
	if baseScore > 10.0 {
		baseScore = 10.0
	}

	// Determine risk level
	riskLevel := "low"
	for level, info := range riskDB.RiskLevels {
		if baseScore >= parseScoreRange(info.ScoreRange) {
			riskLevel = level
		}
	}

	return baseScore, riskLevel, nil
}

// parseScoreRange parses score range strings like "7.0-8.9"
func parseScoreRange(scoreRange string) float64 {
	// Simple parsing - in production you'd use proper parsing
	if scoreRange == "9.0-10.0" {
		return 9.0
	} else if scoreRange == "7.0-8.9" {
		return 7.0
	} else if scoreRange == "4.0-6.9" {
		return 4.0
	} else if scoreRange == "0.1-3.9" {
		return 0.1
	}
	return 0.0
}

// Global database manager instance
var globalDB *DatabaseManager
var once sync.Once

// GetGlobalDatabase returns the global database manager instance
func GetGlobalDatabase() *DatabaseManager {
	once.Do(func() {
		globalDB = NewDatabaseManager("database")
	})
	return globalDB
}