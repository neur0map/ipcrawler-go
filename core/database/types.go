package database

import "time"

// Metadata represents common metadata for database files
type Metadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	LastUpdated string `json:"last_updated"`
	Source      string `json:"source,omitempty"`
	Standards   []string `json:"standards,omitempty"`
}

// HighRiskPortsDB represents the high risk ports database structure
type HighRiskPortsDB struct {
	Metadata   Metadata                       `json:"metadata"`
	Categories map[string]PortCategory        `json:"categories"`
}

// PortCategory represents a category of ports
type PortCategory struct {
	Description string     `json:"description"`
	Ports       []PortInfo `json:"ports"`
}

// PortInfo represents information about a specific port
type PortInfo struct {
	Port          int      `json:"port"`
	Protocol      string   `json:"protocol"`
	Service       string   `json:"service"`
	RiskLevel     string   `json:"risk_level"`
	Description   string   `json:"description"`
	CommonAttacks []string `json:"common_attacks"`
}

// CommonServicesDB represents the common services database structure
type CommonServicesDB struct {
	Metadata Metadata                    `json:"metadata"`
	Services map[string][]ServiceInfo    `json:"services"`
}

// ServiceInfo represents information about a service
type ServiceInfo struct {
	Name              string   `json:"name"`
	Port              int      `json:"port"`
	Protocol          string   `json:"protocol"`
	Description       string   `json:"description"`
	SecureVersion     string   `json:"secure_version,omitempty"`
	SecurityNotes     string   `json:"security_notes,omitempty"`
	CommonSoftware    []string `json:"common_software,omitempty"`
	DefaultCredentials []string `json:"default_credentials,omitempty"`
	SecurityChecks    []string `json:"security_checks,omitempty"`
	SecureAlternatives []string `json:"secure_alternatives,omitempty"`
}

// RiskLevelsDB represents the risk levels database structure
type RiskLevelsDB struct {
	Metadata           Metadata                      `json:"metadata"`
	RiskLevels         map[string]RiskLevel          `json:"risk_levels"`
	PortRiskMappings   PortRiskMappings              `json:"port_risk_mappings"`
	ServiceRiskFactors ServiceRiskFactors            `json:"service_risk_factors"`
}

// RiskLevel represents a risk level definition
type RiskLevel struct {
	ScoreRange      string   `json:"score_range"`
	Description     string   `json:"description"`
	Characteristics []string `json:"characteristics"`
	ExampleServices []string `json:"example_services,omitempty"`
}

// PortRiskMappings represents port to risk level mappings
type PortRiskMappings struct {
	CriticalPorts   []int `json:"critical_ports"`
	HighRiskPorts   []int `json:"high_risk_ports"`
	MediumRiskPorts []int `json:"medium_risk_ports"`
	LowRiskPorts    []int `json:"low_risk_ports"`
}

// ServiceRiskFactors represents risk calculation factors
type ServiceRiskFactors struct {
	Authentication AuthenticationFactors `json:"authentication"`
	Encryption     EncryptionFactors     `json:"encryption"`
	Exposure       ExposureFactors       `json:"exposure"`
}

// AuthenticationFactors represents authentication-related risk factors
type AuthenticationFactors struct {
	NoAuthRequired     float64 `json:"no_auth_required"`
	DefaultCredentials float64 `json:"default_credentials"`
	WeakAuth          float64 `json:"weak_auth"`
	StrongAuth        float64 `json:"strong_auth"`
}

// EncryptionFactors represents encryption-related risk factors
type EncryptionFactors struct {
	NoEncryption   float64 `json:"no_encryption"`
	WeakEncryption float64 `json:"weak_encryption"`
	StrongEncryption float64 `json:"strong_encryption"`
}

// ExposureFactors represents exposure-related risk factors
type ExposureFactors struct {
	InternetFacing  float64 `json:"internet_facing"`
	InternalNetwork float64 `json:"internal_network"`
	LocalhostOnly   float64 `json:"localhost_only"`
}

// IsHighRiskPort checks if a port is considered high risk and returns port info
func (db *HighRiskPortsDB) IsHighRiskPort(port int) (bool, *PortInfo, error) {
	for _, category := range db.Categories {
		for _, portInfo := range category.Ports {
			if portInfo.Port == port {
				isHighRisk := portInfo.RiskLevel == "high" || portInfo.RiskLevel == "critical"
				return isHighRisk, &portInfo, nil
			}
		}
	}
	return false, nil, nil
}

// GetServiceInfo returns service information for a given service name or port
func (db *CommonServicesDB) GetServiceInfo(serviceName string, port int) *ServiceInfo {
	for _, services := range db.Services {
		for _, service := range services {
			if service.Name == serviceName || service.Port == port {
				return &service
			}
		}
	}
	return nil
}

// GetAllHighRiskPorts returns a slice of all high and critical risk port numbers
func (db *HighRiskPortsDB) GetAllHighRiskPorts() []int {
	var ports []int
	for _, category := range db.Categories {
		for _, portInfo := range category.Ports {
			if portInfo.RiskLevel == "high" || portInfo.RiskLevel == "critical" {
				ports = append(ports, portInfo.Port)
			}
		}
	}
	return ports
}

// GetPortsByRiskLevel returns ports filtered by risk level
func (db *HighRiskPortsDB) GetPortsByRiskLevel(riskLevel string) []PortInfo {
	var ports []PortInfo
	for _, category := range db.Categories {
		for _, portInfo := range category.Ports {
			if portInfo.RiskLevel == riskLevel {
				ports = append(ports, portInfo)
			}
		}
	}
	return ports
}

// CacheInfo represents database cache information
type CacheInfo struct {
	LoadedAt    time.Time
	FileSize    int64
	LastModified time.Time
}