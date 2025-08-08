package workflow

import (
	"net"
	"regexp"
	"strings"
)

// TargetType represents the type of target (IP, domain, etc.)
type TargetType int

const (
	TargetTypeUnknown TargetType = iota
	TargetTypeIPv4
	TargetTypeIPv6
	TargetTypeDomain
	TargetTypeHostname
)

// TargetInfo contains information about the target
type TargetInfo struct {
	Original string
	Type     TargetType
	IsLocal  bool
	IsPrivate bool
}

// AnalyzeTarget analyzes the target and returns information about it
func AnalyzeTarget(target string) *TargetInfo {
	info := &TargetInfo{
		Original: target,
		Type:     TargetTypeUnknown,
	}
	
	// Check if it's an IP address
	if ip := net.ParseIP(target); ip != nil {
		if ip.To4() != nil {
			info.Type = TargetTypeIPv4
		} else {
			info.Type = TargetTypeIPv6
		}
		
		// Check if it's a local or private IP
		info.IsLocal = ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
		info.IsPrivate = ip.IsPrivate()
		return info
	}
	
	// Check if it's a domain name
	if isDomainName(target) {
		info.Type = TargetTypeDomain
		
		// Check for local domains
		if strings.HasSuffix(target, ".local") || 
		   strings.HasSuffix(target, ".localhost") ||
		   target == "localhost" {
			info.IsLocal = true
		}
		return info
	}
	
	// Default to hostname if it contains valid characters
	if isValidHostname(target) {
		info.Type = TargetTypeHostname
		return info
	}
	
	return info
}

// isDomainName checks if the string is a valid domain name
func isDomainName(s string) bool {
	// Basic domain validation
	if len(s) == 0 || len(s) > 253 {
		return false
	}
	
	// Must contain at least one dot for a domain
	if !strings.Contains(s, ".") && s != "localhost" {
		return false
	}
	
	// Check each label
	labels := strings.Split(s, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return false
		}
		
		// Label must start and end with alphanumeric
		if !isAlphaNum(label[0]) || !isAlphaNum(label[len(label)-1]) {
			return false
		}
		
		// Check middle characters
		for i := 1; i < len(label)-1; i++ {
			if !isAlphaNum(label[i]) && label[i] != '-' {
				return false
			}
		}
	}
	
	return true
}

// isValidHostname checks if the string is a valid hostname
func isValidHostname(s string) bool {
	if len(s) == 0 || len(s) > 253 {
		return false
	}
	
	// Hostname regex pattern
	hostnameRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
	return hostnameRegex.MatchString(s)
}

// isAlphaNum checks if a byte is alphanumeric
func isAlphaNum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// ShouldSkipStep determines if a step should be skipped based on target type
func ShouldSkipStep(step Step, targetInfo *TargetInfo) (bool, string) {
	// DNS-specific checks
	if step.Tool == "nslookup" || step.Tool == "dig" {
		// MX and TXT records don't make sense for IP addresses
		if targetInfo.Type == TargetTypeIPv4 || targetInfo.Type == TargetTypeIPv6 {
			if step.UseFlags == "mx_lookup" || step.UseFlags == "txt_lookup" {
				return true, "DNS record type not applicable for IP addresses"
			}
			
			// SPF records are also domain-specific
			if strings.Contains(strings.ToLower(step.ID), "spf") {
				return true, "SPF records not applicable for IP addresses"
			}
		}
	}
	
	// Reverse DNS might fail for private IPs
	if step.UseFlags == "ptr_lookup" && targetInfo.IsPrivate {
		// Don't skip, but be aware it might fail
		return false, ""
	}
	
	// Check for step conditions if defined
	if conditions, ok := step.ExtraData["conditions"].(map[string]interface{}); ok {
		if targetType, ok := conditions["target_type"].(string); ok {
			switch targetType {
			case "domain_only":
				if targetInfo.Type != TargetTypeDomain && targetInfo.Type != TargetTypeHostname {
					return true, "Step requires domain target"
				}
			case "ip_only":
				if targetInfo.Type != TargetTypeIPv4 && targetInfo.Type != TargetTypeIPv6 {
					return true, "Step requires IP address target"
				}
			}
		}
	}
	
	return false, ""
}

// GetTargetTypeString returns a string representation of the target type
func (t TargetType) String() string {
	switch t {
	case TargetTypeIPv4:
		return "IPv4"
	case TargetTypeIPv6:
		return "IPv6"
	case TargetTypeDomain:
		return "Domain"
	case TargetTypeHostname:
		return "Hostname"
	default:
		return "Unknown"
	}
}