package template

import (
	"regexp"
	"strings"
)

var templateRegex = regexp.MustCompile(`\{\{([^}]+)\}\}`)

func ApplyTemplate(template string, data map[string]string) string {
	return templateRegex.ReplaceAllStringFunc(template, func(match string) string {
		key := strings.TrimSpace(match[2 : len(match)-2])
		
		if strings.Contains(key, ".") {
			parts := strings.Split(key, ".")
			if len(parts) == 2 && parts[0] != "" {
				stepKey := parts[0] + "_" + parts[1]
				if val, ok := data[stepKey]; ok {
					return val
				}
			}
		}
		
		if val, ok := data[key]; ok {
			return val
		}
		return match
	})
}

func ApplyTemplateToSlice(templates []string, data map[string]string) []string {
	result := make([]string, len(templates))
	for i, tmpl := range templates {
		result[i] = ApplyTemplate(tmpl, data)
	}
	return result
}

func ApplyTemplateToInterface(value interface{}, data map[string]string) interface{} {
	switch v := value.(type) {
	case string:
		return ApplyTemplate(v, data)
	case []string:
		return ApplyTemplateToSlice(v, data)
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = ApplyTemplateToInterface(item, data)
		}
		return result
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			result[key] = ApplyTemplateToInterface(val, data)
		}
		return result
	default:
		return value
	}
}

func ExtractTemplateKeys(template string) []string {
	matches := templateRegex.FindAllStringSubmatch(template, -1)
	keys := make([]string, 0, len(matches))
	seen := make(map[string]bool)
	
	for _, match := range matches {
		if len(match) > 1 {
			key := strings.TrimSpace(match[1])
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
		}
	}
	return keys
}