package secrets

import (
	"regexp"
	"strings"
)

// TemplateToSecretKey converts template format to secret key format
// {{dockerhub.username}} -> dockerhub.username
// {{github.token}} -> github.token
// {{openai.api_key}} -> openai.api_key
func TemplateToSecretKey(template string) string {
	// Remove {{ }} wrapper
	key := strings.TrimPrefix(template, "{{")
	key = strings.TrimSuffix(key, "}}")

	// Kubernetes secret keys allow: alphanumeric, -, _, .
	// Replace any invalid characters with triple underscores for easy debugging
	validKeyRegex := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	key = validKeyRegex.ReplaceAllString(key, "___")

	return key
}

// SecretKeyToTemplate converts secret key back to template format (for debugging)
// dockerhub.username -> {{dockerhub.username}}
func SecretKeyToTemplate(key string) string {
	// Reverse any triple underscore replacements if needed
	// (Not fully reversible, but useful for debugging)
	return "{{" + key + "}}"
}

// ExtractTemplateContent extracts the content between {{ and }}
// {{dockerhub.username}} -> dockerhub.username
// Returns empty string if not a valid template
func ExtractTemplateContent(template string) string {
	if !strings.HasPrefix(template, "{{") || !strings.HasSuffix(template, "}}") {
		return ""
	}
	return strings.TrimPrefix(strings.TrimSuffix(template, "}}"), "{{")
}

// IsTemplate checks if a string is a template format
// {{dockerhub.username}} -> true
// dockerhub.username -> false
func IsTemplate(s string) bool {
	return strings.HasPrefix(s, "{{") && strings.HasSuffix(s, "}}")
}
