package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DiscoverOAuthRequirements probes an MCP server to discover OAuth requirements
// Following MCP Authorization specification and RFC 9728/RFC 8414
func DiscoverOAuthRequirements(ctx context.Context, serverURL string) (*OAuthDiscovery, error) {
	// Create HTTP client with reasonable timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Step 1: Try initial connection to MCP server to check for 401
	req, err := http.NewRequestWithContext(ctx, "GET", serverURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Set MCP-specific headers to identify as MCP client
	req.Header.Set("User-Agent", "docker-mcp-gateway/1.0.0")
	req.Header.Set("Accept", "application/json")
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connecting to server %s: %w", serverURL, err)
	}
	defer resp.Body.Close()

	// If not 401, server doesn't require OAuth
	if resp.StatusCode != http.StatusUnauthorized {
		return &OAuthDiscovery{
			RequiresOAuth: false,
		}, nil
	}

	// Step 2: Parse WWW-Authenticate header
	wwwAuth := resp.Header.Get("WWW-Authenticate")
	if wwwAuth == "" {
		return nil, fmt.Errorf("server returned 401 but no WWW-Authenticate header")
	}

	challenges, err := ParseWWWAuthenticate(wwwAuth)
	if err != nil {
		return nil, fmt.Errorf("parsing WWW-Authenticate header: %w", err)
	}

	// Step 3: Find resource_metadata URL from Bearer challenge
	resourceMetadataURL := FindResourceMetadataURL(challenges)
	if resourceMetadataURL == "" {
		// Fallback: Try convention-based discovery for non-MCP-compliant servers
		resourceMetadataURL = tryConventionBasedDiscovery(serverURL)
		if resourceMetadataURL == "" {
			return nil, fmt.Errorf("server is not MCP-compliant: no resource_metadata URL found in WWW-Authenticate header and convention-based discovery failed")
		}
		fmt.Printf("Server is not fully MCP-compliant, using convention-based discovery\n")
	}

	// Step 4: Fetch OAuth Protected Resource Metadata (RFC 9728)
	resourceMetadata, err := fetchOAuthProtectedResourceMetadata(ctx, client, resourceMetadataURL)
	if err != nil {
		return nil, fmt.Errorf("fetching protected resource metadata: %w", err)
	}

	// Step 5: Fetch Authorization Server Metadata (RFC 8414)
	authServerMetadata, err := fetchAuthorizationServerMetadata(ctx, client, resourceMetadata.AuthorizationServer)
	if err != nil {
		return nil, fmt.Errorf("fetching authorization server metadata: %w", err)
	}

	// Step 6: Build discovery result
	discovery := &OAuthDiscovery{
		RequiresOAuth: true,
		
		// From Protected Resource Metadata
		ResourceURL:         resourceMetadata.Resource,
		ResourceServer:      resourceMetadata.Resource,
		AuthorizationServer: resourceMetadata.AuthorizationServer,
		Scopes:              resourceMetadata.Scopes,
		
		// From Authorization Server Metadata
		Issuer:                authServerMetadata.Issuer,
		AuthorizationEndpoint: authServerMetadata.AuthorizationEndpoint,
		TokenEndpoint:         authServerMetadata.TokenEndpoint,
		RegistrationEndpoint:  authServerMetadata.RegistrationEndpoint,
		JWKSUri:              authServerMetadata.JWKSUri,
		ScopesSupported:      authServerMetadata.ScopesSupported,
		ResponseTypesSupported: authServerMetadata.ResponseTypesSupported,
		ResponseModesSupported: authServerMetadata.ResponseModesSupported,
		GrantTypesSupported:   authServerMetadata.GrantTypesSupported,
		TokenEndpointAuthMethodsSupported: authServerMetadata.TokenEndpointAuthMethodsSupported,
		
		// PKCE support
		SupportsPKCE:        containsString(authServerMetadata.CodeChallengeMethodsSupported, "S256"),
		CodeChallengeMethod: authServerMetadata.CodeChallengeMethodsSupported,
	}

	// Extract additional scopes from WWW-Authenticate if not in resource metadata
	if len(discovery.Scopes) == 0 {
		discovery.Scopes = FindRequiredScopes(challenges)
	}

	return discovery, nil
}

// fetchOAuthProtectedResourceMetadata fetches metadata from /.well-known/oauth-protected-resource
func fetchOAuthProtectedResourceMetadata(ctx context.Context, client *http.Client, metadataURL string) (*OAuthProtectedResourceMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", metadataURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	
	req.Header.Set("Accept", "application/json")
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching metadata from %s: %w", metadataURL, err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metadata endpoint returned status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	
	var metadata OAuthProtectedResourceMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return nil, fmt.Errorf("parsing JSON response: %w", err)
	}
	
	// Validate required fields
	if metadata.Resource == "" {
		return nil, fmt.Errorf("resource field missing in protected resource metadata")
	}
	
	// Handle both authorization_server (singular) and authorization_servers (plural) formats
	if metadata.AuthorizationServer == "" {
		if len(metadata.AuthorizationServers) == 0 {
			return nil, fmt.Errorf("authorization_server or authorization_servers field missing in protected resource metadata")
		}
		// Use the first authorization server from the array
		metadata.AuthorizationServer = metadata.AuthorizationServers[0]
	}
	
	return &metadata, nil
}

// fetchAuthorizationServerMetadata fetches metadata from /.well-known/oauth-authorization-server
func fetchAuthorizationServerMetadata(ctx context.Context, client *http.Client, authServerURL string) (*OAuthAuthorizationServerMetadata, error) {
	// Construct well-known URL
	var metadataURL string
	if strings.HasSuffix(authServerURL, "/") {
		metadataURL = authServerURL + ".well-known/oauth-authorization-server"
	} else {
		metadataURL = authServerURL + "/.well-known/oauth-authorization-server"
	}
	
	req, err := http.NewRequestWithContext(ctx, "GET", metadataURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	
	req.Header.Set("Accept", "application/json")
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching metadata from %s: %w", metadataURL, err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("authorization server metadata endpoint returned status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	
	var metadata OAuthAuthorizationServerMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return nil, fmt.Errorf("parsing JSON response: %w", err)
	}
	
	// Validate required fields (RFC 8414)
	if metadata.Issuer == "" {
		return nil, fmt.Errorf("issuer field missing in authorization server metadata")
	}
	if metadata.AuthorizationEndpoint == "" {
		return nil, fmt.Errorf("authorization_endpoint field missing in authorization server metadata")
	}
	if metadata.TokenEndpoint == "" {
		return nil, fmt.Errorf("token_endpoint field missing in authorization server metadata")
	}
	
	// Validate issuer URL matches authorization server URL (RFC 8414 requirement)
	issuerURL, err := url.Parse(metadata.Issuer)
	if err != nil {
		return nil, fmt.Errorf("invalid issuer URL: %w", err)
	}
	
	authURL, err := url.Parse(authServerURL)
	if err != nil {
		return nil, fmt.Errorf("invalid authorization server URL: %w", err)
	}
	
	if issuerURL.Scheme != authURL.Scheme || issuerURL.Host != authURL.Host {
		return nil, fmt.Errorf("issuer URL %s does not match authorization server URL %s", metadata.Issuer, authServerURL)
	}
	
	return &metadata, nil
}

// tryConventionBasedDiscovery attempts to find OAuth metadata using standard well-known locations
// when resource_metadata parameter is missing from WWW-Authenticate header (non-MCP-compliant servers)
func tryConventionBasedDiscovery(serverURL string) string {
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return ""
	}

	// Try convention-based well-known URLs
	candidates := []string{
		// Try at the server base URL (e.g., https://mcp.notion.com/.well-known/oauth-protected-resource)
		fmt.Sprintf("%s://%s/.well-known/oauth-protected-resource", parsedURL.Scheme, parsedURL.Host),
		// Try at the server path base (e.g., https://mcp.notion.com/mcp/.well-known/oauth-protected-resource)  
		strings.TrimSuffix(serverURL, "/") + "/.well-known/oauth-protected-resource",
	}

	for _, candidate := range candidates {
		// Test if the URL is accessible (don't fully fetch yet, just check if it exists)
		if testURLAccessible(candidate) {
			return candidate
		}
	}

	return ""
}

// testURLAccessible makes a HEAD request to check if a URL is accessible
func testURLAccessible(urlStr string) bool {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("HEAD", urlStr, nil)
	if err != nil {
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Accept 200 OK or 405 Method Not Allowed (some servers don't support HEAD)
	return resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusMethodNotAllowed
}

// containsString checks if a slice contains a specific string
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}