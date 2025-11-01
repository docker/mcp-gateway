package oauth

import (
	"fmt"
	"regexp"
	"strings"
)

// ParseWWWAuthenticate parses a WWW-Authenticate header value
//
// RFC 6750 COMPLIANCE - OAuth 2.0 Bearer Token Usage:
// - Section 3: Defines WWW-Authenticate Header Field format
// - Section 3.1: Specifies Bearer challenge syntax
// - Supports both RFC 6750 token68 and auth-param formats
//
// MCP SPEC SUPPORT:
// - MCP Authorization Specification Section 4.1: WWW-Authenticate header parsing
// - RFC 9728 Section 5.1: Handles resource_metadata parameter
//
// ROBUST PARSING:
// - Handles quoted and unquoted parameter values
// - Supports multiple authentication schemes in single header
// - Gracefully handles malformed headers (best-effort parsing)
//
// Example inputs:
//
//	Bearer realm="example.com", scope="read write", resource_metadata="https://example.com/.well-known/oauth-protected-resource"
//	Bearer realm=example.com scope="read write"
//	Basic realm="example.com", Bearer realm="api.example.com" scope="read"
func ParseWWWAuthenticate(headerValue string) ([]WWWAuthenticateChallenge, error) {
	if headerValue == "" {
		return nil, fmt.Errorf("empty WWW-Authenticate header")
	}

	var challenges []WWWAuthenticateChallenge

	// Use regex to find auth schemes and their parameters
	// This handles multiple schemes in one header: Basic realm="...", Bearer realm="..." scope="..."
	schemeRegex := regexp.MustCompile(`(?i)([a-z][a-z0-9\-_]*)\s+([^,]*(?:,\s*[^=\s]+\s*=\s*[^,]*)*)[,\s]*`)
	matches := schemeRegex.FindAllStringSubmatch(headerValue, -1)

	if len(matches) == 0 {
		// Fallback: try to parse as single scheme if regex fails
		return parseSingleScheme(headerValue)
	}

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		scheme := match[1]
		paramString := match[2]

		parameters := parseAuthParameters(paramString)

		challenges = append(challenges, WWWAuthenticateChallenge{
			Scheme:     scheme,
			Parameters: parameters,
		})
	}

	if len(challenges) == 0 {
		return nil, fmt.Errorf("no valid authentication challenges found in WWW-Authenticate header: %s", headerValue)
	}

	return challenges, nil
}

// parseSingleScheme attempts to parse a header with a single authentication scheme
func parseSingleScheme(headerValue string) ([]WWWAuthenticateChallenge, error) {
	parts := strings.SplitN(strings.TrimSpace(headerValue), " ", 2)
	if len(parts) < 1 {
		return nil, fmt.Errorf("invalid WWW-Authenticate header format")
	}

	scheme := parts[0]
	var paramString string
	if len(parts) > 1 {
		paramString = parts[1]
	}

	parameters := parseAuthParameters(paramString)

	return []WWWAuthenticateChallenge{
		{
			Scheme:     scheme,
			Parameters: parameters,
		},
	}, nil
}

// parseAuthParameters parses authentication parameters from a parameter string
//
// Handles multiple formats:
// - Quoted values: param="value"
// - Unquoted values: param=value
// - Mixed: param1="quoted value", param2=unquoted
//
// Examples:
//
//	realm="example.com", scope="read write"
//	realm=example.com scope="read write"
//	realm="example.com", resource_metadata="https://example.com/.well-known/oauth-protected-resource"
func parseAuthParameters(paramString string) map[string]string {
	parameters := make(map[string]string)

	if paramString == "" {
		return parameters
	}

	// Use regex to parse key=value pairs, handling quoted and unquoted values
	paramRegex := regexp.MustCompile(`([a-zA-Z0-9_-]+)\s*=\s*(?:"([^"]*)"|([^,\s]+))`)
	matches := paramRegex.FindAllStringSubmatch(paramString, -1)

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		key := match[1]
		quotedValue := match[2]   // Value inside quotes
		unquotedValue := match[3] // Value without quotes

		// Use quoted value if present, otherwise unquoted
		var value string
		if quotedValue != "" {
			value = quotedValue
		} else {
			value = unquotedValue
		}

		parameters[key] = value
	}

	return parameters
}

// FindResourceMetadataURL searches for the resource_metadata URL in WWW-Authenticate challenges
//
// RFC 9728 COMPLIANCE:
// - Section 5.1: Defines resource_metadata parameter in WWW-Authenticate response
// - Returns the first resource_metadata URL found across all challenges
func FindResourceMetadataURL(challenges []WWWAuthenticateChallenge) string {
	for _, challenge := range challenges {
		if challenge.Parameters == nil {
			continue
		}
		if resourceMetadataURL, exists := challenge.Parameters["resource_metadata"]; exists && resourceMetadataURL != "" {
			return resourceMetadataURL
		}
	}
	return ""
}

// FindRequiredScopes extracts required OAuth scopes from WWW-Authenticate challenges
//
// RFC 6750 COMPLIANCE:
// - Section 3: Defines scope parameter in Bearer challenges
// - Returns space-separated scopes as a slice
//
// Searches all Bearer challenges and combines scopes into a unique set
func FindRequiredScopes(challenges []WWWAuthenticateChallenge) []string {
	scopesSet := make(map[string]bool)

	for _, challenge := range challenges {
		// Only process Bearer challenges for OAuth scopes
		if !strings.EqualFold(challenge.Scheme, "Bearer") {
			continue
		}

		if challenge.Parameters == nil {
			continue
		}

		if scopeParam, exists := challenge.Parameters["scope"]; exists && scopeParam != "" {
			// Split space-separated scopes per OAuth 2.0 spec (RFC 6749 Section 3.3)
			scopes := strings.Fields(scopeParam)
			for _, scope := range scopes {
				if scope != "" {
					scopesSet[scope] = true
				}
			}
		}
	}

	// Convert set to slice
	var scopes []string
	for scope := range scopesSet {
		scopes = append(scopes, scope)
	}

	return scopes
}
