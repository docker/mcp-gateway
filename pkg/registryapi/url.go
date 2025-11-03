package registryapi

import (
	"fmt"
	"net/url"
	"strings"
)

// ServerURL represents a parsed MCP registry URL
type ServerURL struct {
	// BaseURL is the host plus anything before the API version (e.g., "https://registry.modelcontextprotocol.io")
	BaseURL string
	// APIVersion is the API version (e.g., "v0")
	APIVersion string
	// ServerName is the URL-encoded server name (e.g., "ai.aliengiraffe%2Fspotdb")
	ServerName string
	// Version (e.g., "latest", "0.1.0") - defaults to "latest" if not set
	Version string
	// RawURL is the original URL that was parsed
	RawURL string
}

// ParseServerURL parses an MCP registry URL into its components
func ParseServerURL(rawURL string) (*ServerURL, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Extract base URL (scheme + host)
	baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	// Parse the path components
	// Expected format: /v0/servers/{serverName}[/versions[/{version}]]
	// Note: serverName may be URL-encoded (e.g., ai.aliengiraffe%2Fspotdb)
	// Use EscapedPath() to preserve URL encoding like %2F
	path := strings.Trim(parsedURL.EscapedPath(), "/")

	// Find the pattern: {apiVersion}/servers/{serverName}
	// We need to be careful because serverName might contain encoded slashes
	parts := strings.Split(path, "/")

	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid registry URL: expected at least 3 path segments, got %d", len(parts))
	}

	// Find the API version (v0, v1, etc.) and "servers" keyword
	apiVersion := ""
	serversIdx := -1
	for i := range len(parts) - 1 {
		if strings.HasPrefix(parts[i], "v") && len(parts[i]) > 1 && parts[i+1] == "servers" {
			apiVersion = parts[i]
			serversIdx = i + 1
			break
		}
		// Add the path segment to the base URL if we aren't at the version yet
		baseURL = fmt.Sprintf("%s/%s", baseURL, parts[i])
	}

	if apiVersion == "" || serversIdx == -1 || serversIdx+1 >= len(parts) {
		return nil, fmt.Errorf("invalid registry URL: could not find API version and server name")
	}

	// The server name is at serversIdx + 1
	serverNameIdx := serversIdx + 1
	serverName := parts[serverNameIdx]

	result := &ServerURL{
		BaseURL:    baseURL,
		APIVersion: apiVersion,
		ServerName: serverName,
		RawURL:     rawURL,
	}

	// Check for versions endpoint and optional version
	// Versions would be at serverNameIdx + 1
	if serverNameIdx+1 < len(parts) {
		if parts[serverNameIdx+1] == "versions" {
			// Check if there's a specific version at serverNameIdx + 2
			if serverNameIdx+2 < len(parts) {
				result.Version = parts[serverNameIdx+2]
			}
		}
	}

	if result.Version == "" {
		result.Version = "latest"
	}

	return result, nil
}

// String returns the full URL
func (r *ServerURL) String() string {
	parts := []string{r.BaseURL, r.APIVersion, "servers", r.ServerName, "versions"}

	if r.Version != "" {
		parts = append(parts, r.Version)
	} else {
		parts = append(parts, "latest")
	}

	return strings.Join(parts, "/")
}

func (r *ServerURL) Raw() string {
	return r.RawURL
}

func (r *ServerURL) IsLatestVersion() bool {
	return r.Version == "latest" || r.Version == ""
}

// LatestVersionURL returns the URL for the latest version endpoint
func (r *ServerURL) LatestVersionURL() string {
	return fmt.Sprintf("%s/%s/servers/%s/versions/latest", r.BaseURL, r.APIVersion, r.ServerName)
}

// VersionsListURL returns the URL for the versions list endpoint
func (r *ServerURL) VersionsListURL() string {
	return fmt.Sprintf("%s/%s/servers/%s/versions", r.BaseURL, r.APIVersion, r.ServerName)
}

func (r *ServerURL) WithVersion(version string) *ServerURL {
	return &ServerURL{
		BaseURL:    r.BaseURL,
		APIVersion: r.APIVersion,
		ServerName: r.ServerName,
		Version:    version,
		RawURL:     r.RawURL,
	}
}
