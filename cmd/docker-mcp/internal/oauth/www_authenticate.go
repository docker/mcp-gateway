package oauth

import (
	"fmt"
	"regexp"
	"strings"
)

// ParseWWWAuthenticate parses a WWW-Authenticate header value according to RFC 7235
// Example: Bearer realm="example", scope="read write", resource_metadata="https://api.example.com/.well-known/oauth-protected-resource"
func ParseWWWAuthenticate(headerValue string) ([]WWWAuthenticateChallenge, error) {
	if headerValue == "" {
		return nil, fmt.Errorf("empty WWW-Authenticate header")
	}

	var challenges []WWWAuthenticateChallenge
	
	// Split on commas that are not within quotes to separate multiple challenges
	parts := splitRespectingQuotes(headerValue, ',')
	
	var currentChallenge *WWWAuthenticateChallenge
	
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		
		// Check if this starts a new challenge (contains auth-scheme)
		if containsScheme(part) {
			// Save previous challenge if exists
			if currentChallenge != nil {
				challenges = append(challenges, *currentChallenge)
			}
			
			// Parse new challenge
			scheme, params, err := parseChallenge(part)
			if err != nil {
				return nil, fmt.Errorf("parsing challenge '%s': %w", part, err)
			}
			
			currentChallenge = &WWWAuthenticateChallenge{
				Scheme:     scheme,
				Parameters: params,
			}
		} else if currentChallenge != nil {
			// This is a continuation of parameters for the current challenge
			params, err := parseParameters(part)
			if err != nil {
				return nil, fmt.Errorf("parsing parameters '%s': %w", part, err)
			}
			
			// Merge parameters
			for k, v := range params {
				currentChallenge.Parameters[k] = v
			}
		}
	}
	
	// Add the last challenge
	if currentChallenge != nil {
		challenges = append(challenges, *currentChallenge)
	}
	
	if len(challenges) == 0 {
		return nil, fmt.Errorf("no valid challenges found")
	}
	
	return challenges, nil
}

// parseChallenge parses a single challenge starting with auth-scheme
func parseChallenge(challenge string) (string, map[string]string, error) {
	// Find the first space or comma to separate scheme from parameters
	spaceIdx := strings.Index(challenge, " ")
	commaIdx := strings.Index(challenge, ",")
	
	var schemeEnd int
	if spaceIdx == -1 && commaIdx == -1 {
		// Only scheme, no parameters
		return strings.TrimSpace(challenge), make(map[string]string), nil
	} else if spaceIdx == -1 {
		schemeEnd = commaIdx
	} else if commaIdx == -1 {
		schemeEnd = spaceIdx
	} else {
		schemeEnd = min(spaceIdx, commaIdx)
	}
	
	scheme := strings.TrimSpace(challenge[:schemeEnd])
	paramString := strings.TrimSpace(challenge[schemeEnd:])
	
	// Remove leading comma if present
	if strings.HasPrefix(paramString, ",") {
		paramString = strings.TrimSpace(paramString[1:])
	}
	
	params, err := parseParameters(paramString)
	if err != nil {
		return "", nil, err
	}
	
	return scheme, params, nil
}

// parseParameters parses auth-param = auth-param-name "=" ( token | quoted-string )
func parseParameters(paramString string) (map[string]string, error) {
	params := make(map[string]string)
	
	if paramString == "" {
		return params, nil
	}
	
	// Regular expression to match key=value pairs, handling quoted strings
	// This matches: key="quoted value" or key=token
	re := regexp.MustCompile(`([a-zA-Z0-9_-]+)\s*=\s*("([^"]*)"|([^,\s]+))`)
	
	matches := re.FindAllStringSubmatch(paramString, -1)
	
	for _, match := range matches {
		if len(match) >= 5 {
			key := match[1]
			var value string
			
			if match[3] != "" {
				// Quoted string value
				value = match[3]
			} else {
				// Token value
				value = match[4]
			}
			
			params[key] = value
		}
	}
	
	return params, nil
}

// containsScheme checks if the string starts with a scheme (word followed by space or params)
func containsScheme(s string) bool {
	// Look for a word at the beginning followed by space, comma, or end of string
	re := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*(\s|,|$)`)
	return re.MatchString(s)
}

// splitRespectingQuotes splits a string by delimiter while respecting quoted strings
func splitRespectingQuotes(s string, delimiter rune) []string {
	var result []string
	var current strings.Builder
	inQuotes := false
	
	for _, char := range s {
		if char == '"' {
			inQuotes = !inQuotes
			current.WriteRune(char)
		} else if char == delimiter && !inQuotes {
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(char)
		}
	}
	
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	
	return result
}

// FindResourceMetadataURL extracts the resource_metadata URL from WWW-Authenticate challenges
func FindResourceMetadataURL(challenges []WWWAuthenticateChallenge) string {
	for _, challenge := range challenges {
		// Look for Bearer challenges with resource_metadata parameter
		if strings.EqualFold(challenge.Scheme, "Bearer") {
			if resourceMetadata, exists := challenge.Parameters["resource_metadata"]; exists {
				return resourceMetadata
			}
		}
	}
	return ""
}

// FindBearerRealm extracts the realm from Bearer challenges  
func FindBearerRealm(challenges []WWWAuthenticateChallenge) string {
	for _, challenge := range challenges {
		if strings.EqualFold(challenge.Scheme, "Bearer") {
			if realm, exists := challenge.Parameters["realm"]; exists {
				return realm
			}
		}
	}
	return ""
}

// FindRequiredScopes extracts the required scopes from Bearer challenges
func FindRequiredScopes(challenges []WWWAuthenticateChallenge) []string {
	for _, challenge := range challenges {
		if strings.EqualFold(challenge.Scheme, "Bearer") {
			if scope, exists := challenge.Parameters["scope"]; exists {
				return strings.Fields(scope) // Split on whitespace
			}
		}
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}