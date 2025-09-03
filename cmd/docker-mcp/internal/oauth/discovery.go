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
// 
// MCP AUTHORIZATION SPEC COMPLIANCE:
// - Implements MCP Authorization Specification Section 4.1 "Authorization Server Discovery"
// - Follows RFC 9728 "OAuth 2.0 Protected Resource Metadata" 
// - Follows RFC 8414 "OAuth 2.0 Authorization Server Metadata"
// - Includes fallback for non-MCP-compliant servers (compatibility extension)
//
// FLOW:
// 1. Make request to MCP server to trigger 401 response
// 2. Parse WWW-Authenticate header for resource_metadata URL (RFC 9728 Section 5.1)
// 3. Fetch Protected Resource Metadata (RFC 9728 Section 3)
// 4. Extract authorization server URL(s) 
// 5. Fetch Authorization Server Metadata (RFC 8414 Section 3)
func DiscoverOAuthRequirements(ctx context.Context, serverURL string) (*OAuthDiscovery, error) {
	// Create HTTP client with reasonable timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// STEP 1: Make initial MCP request to trigger 401 Unauthorized
	// MCP Spec Section 4.1: "MCP request without token" should trigger 401
	// Use POST with initialize request as per spec diagrams (line 107, 162)
	mcpPayload := `{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"mcp-gateway","version":"1.0.0"}},"id":1}`
	req, err := http.NewRequestWithContext(ctx, "POST", serverURL, strings.NewReader(mcpPayload))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Set headers for MCP protocol request
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "docker-mcp-gateway/1.0.0")
	req.Header.Set("Accept", "application/json")
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connecting to server %s: %w", serverURL, err)
	}
	defer resp.Body.Close()

	// If not 401, OAuth is not required (Authorization is OPTIONAL per MCP spec Section 2.1)
	if resp.StatusCode != http.StatusUnauthorized {
		return &OAuthDiscovery{
			RequiresOAuth: false,
		}, nil
	}

	// STEP 2: Parse WWW-Authenticate header for resource metadata URL
	// MCP Spec Section 4.1: "MCP servers MUST use the HTTP header WWW-Authenticate when returning a 401 Unauthorized 
	// to indicate the location of the resource server metadata URL as described in RFC9728 Section 5.1"
	wwwAuth := resp.Header.Get("WWW-Authenticate")
	if wwwAuth == "" {
		return nil, fmt.Errorf("server returned 401 but no WWW-Authenticate header")
	}

	challenges, err := ParseWWWAuthenticate(wwwAuth)
	if err != nil {
		return nil, fmt.Errorf("parsing WWW-Authenticate header: %w", err)
	}

	// STEP 3: Extract resource_metadata URL from Bearer challenge
	// RFC 9728 Section 5.1: WWW-Authenticate response SHOULD include resource_metadata parameter
	resourceMetadataURL := FindResourceMetadataURL(challenges)
	if resourceMetadataURL == "" {
		// COMPATIBILITY EXTENSION: Try convention-based discovery for non-MCP-compliant servers
		// This is NOT in the MCP spec but enables compatibility with servers like Notion
		// that don't include resource_metadata in WWW-Authenticate headers
		resourceMetadataURL = tryConventionBasedDiscovery(serverURL)
		if resourceMetadataURL == "" {
			// ADDITIONAL FALLBACK: Try direct authorization server discovery
			// Some servers like Atlassian have /.well-known/oauth-authorization-server
			// but not /.well-known/oauth-protected-resource
			discovery, err := tryDirectAuthServerDiscovery(ctx, client, serverURL)
			if err == nil && discovery != nil {
				fmt.Printf("Server is not fully MCP-compliant, using direct authorization server discovery\n")
				return discovery, nil
			}
			return nil, fmt.Errorf("server is not MCP-compliant: no resource_metadata URL found in WWW-Authenticate header, convention-based discovery failed, and direct auth server discovery failed")
		}
		fmt.Printf("Server is not fully MCP-compliant, using convention-based discovery\n")
	}

	// STEP 4: Fetch OAuth Protected Resource Metadata
	// MCP Spec Section 3.1: "MCP servers MUST implement OAuth 2.0 Protected Resource Metadata (RFC9728)"
	// RFC 9728 Section 3: Defines the structure and required fields
	resourceMetadata, err := fetchOAuthProtectedResourceMetadata(ctx, client, resourceMetadataURL)
	if err != nil {
		return nil, fmt.Errorf("fetching protected resource metadata: %w", err)
	}

	// STEP 5: Fetch Authorization Server Metadata  
	// MCP Spec Section 3.1: "Authorization servers MUST provide OAuth 2.0 Authorization Server Metadata (RFC8414)"
	// MCP Spec Section 4.2: "MCP clients MUST use the OAuth 2.0 Authorization Server Metadata"
	authServerMetadata, err := fetchAuthorizationServerMetadata(ctx, client, resourceMetadata.AuthorizationServer)
	if err != nil {
		return nil, fmt.Errorf("fetching authorization server metadata: %w", err)
	}

	// STEP 6: Build discovery result with all discovered OAuth configuration
	discovery := &OAuthDiscovery{
		RequiresOAuth: true,
		
		// From Protected Resource Metadata (RFC 9728)
		ResourceURL:         resourceMetadata.Resource,
		ResourceServer:      resourceMetadata.Resource,
		AuthorizationServer: resourceMetadata.AuthorizationServer,
		Scopes:              resourceMetadata.Scopes,
		
		// From Authorization Server Metadata (RFC 8414)
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
		
		// PKCE support detection (OAuth 2.1 MUST requirement)
		SupportsPKCE:        containsString(authServerMetadata.CodeChallengeMethodsSupported, "S256"),
		CodeChallengeMethod: authServerMetadata.CodeChallengeMethodsSupported,
	}

	// Extract additional scopes from WWW-Authenticate if not available in resource metadata
	if len(discovery.Scopes) == 0 {
		discovery.Scopes = FindRequiredScopes(challenges)
	}

	return discovery, nil
}

// fetchOAuthProtectedResourceMetadata fetches metadata from /.well-known/oauth-protected-resource
// 
// RFC 9728 COMPLIANCE:
// - Implements RFC 9728 Section 3 "Protected Resource Metadata"
// - Validates required fields: resource, authorization_server(s)
// - Handles both singular and plural authorization server formats
func fetchOAuthProtectedResourceMetadata(ctx context.Context, client *http.Client, metadataURL string) (*OAuthProtectedResourceMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", metadataURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	
	// RFC 9728 Section 3.1: Response MUST be application/json
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
	
	// RFC 9728 Section 3.2: Validate required fields
	if metadata.Resource == "" {
		return nil, fmt.Errorf("resource field missing in protected resource metadata")
	}
	
	// COMPATIBILITY: Handle both authorization_server (singular) and authorization_servers (plural) formats
	// RFC 9728 defines authorization_servers as array, but some servers use singular form
	if metadata.AuthorizationServer == "" {
		if len(metadata.AuthorizationServers) == 0 {
			return nil, fmt.Errorf("authorization_server or authorization_servers field missing in protected resource metadata")
		}
		// MCP Spec Section 4.1: "The responsibility for selecting which authorization server to use lies with the MCP client"
		// TODO: Implement selection logic for multiple authorization servers (Phase 2)
		metadata.AuthorizationServer = metadata.AuthorizationServers[0]
	}
	
	return &metadata, nil
}

// fetchAuthorizationServerMetadata fetches metadata from /.well-known/oauth-authorization-server
//
// RFC 8414 COMPLIANCE:
// - Implements RFC 8414 Section 3 "Authorization Server Metadata"  
// - Validates required fields: issuer, authorization_endpoint, token_endpoint
// - Validates issuer URL matches authorization server URL (RFC 8414 Section 3.2)
func fetchAuthorizationServerMetadata(ctx context.Context, client *http.Client, authServerURL string) (*OAuthAuthorizationServerMetadata, error) {
	// RFC 8414 Section 3: Construct well-known URL
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
	
	// RFC 8414 Section 3.1: Response MUST be application/json
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
	
	// RFC 8414 Section 3.2: Validate required fields
	if metadata.Issuer == "" {
		return nil, fmt.Errorf("issuer field missing in authorization server metadata")
	}
	if metadata.AuthorizationEndpoint == "" {
		return nil, fmt.Errorf("authorization_endpoint field missing in authorization server metadata")
	}
	if metadata.TokenEndpoint == "" {
		return nil, fmt.Errorf("token_endpoint field missing in authorization server metadata")
	}
	
	// RFC 8414 Section 3.2: Validate issuer URL matches authorization server URL
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

// tryDirectAuthServerDiscovery attempts to discover OAuth configuration by directly
// accessing the authorization server metadata when resource metadata is not available.
// This handles servers like Atlassian that have /.well-known/oauth-authorization-server
// but not /.well-known/oauth-protected-resource
func tryDirectAuthServerDiscovery(ctx context.Context, client *http.Client, serverURL string) (*OAuthDiscovery, error) {
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}

	// Try direct authorization server metadata at server base URL
	authServerMetadataURL := fmt.Sprintf("%s://%s/.well-known/oauth-authorization-server", parsedURL.Scheme, parsedURL.Host)
	
	// Test if the URL is accessible
	if !testURLAccessible(authServerMetadataURL) {
		return nil, fmt.Errorf("authorization server metadata not found at %s", authServerMetadataURL)
	}

	// Fetch authorization server metadata directly without validation
	// We can't use fetchAuthorizationServerMetadata because it validates issuer matches server URL
	// but for direct discovery the issuer might be different (e.g., Cloudflare Workers)
	authServerMetadata, err := fetchDirectAuthorizationServerMetadata(ctx, client, authServerMetadataURL)
	if err != nil {
		return nil, fmt.Errorf("fetching direct authorization server metadata: %w", err)
	}

	// Build discovery result without resource metadata
	// We use the server URL as both resource URL and the authorization server URL
	discovery := &OAuthDiscovery{
		RequiresOAuth:         true,
		ResourceURL:           serverURL, // Use the MCP server URL as the resource
		AuthorizationServer:   authServerMetadata.Issuer, // Use issuer from metadata
		AuthorizationEndpoint: authServerMetadata.AuthorizationEndpoint,
		TokenEndpoint:         authServerMetadata.TokenEndpoint,
		RegistrationEndpoint:  authServerMetadata.RegistrationEndpoint,
		Scopes:                []string{}, // No scopes discovered, will use default or catalog-configured
	}

	return discovery, nil
}

// fetchDirectAuthorizationServerMetadata fetches authorization server metadata without issuer validation
// This is used for direct discovery where the metadata URL might be on a different host than the issuer
func fetchDirectAuthorizationServerMetadata(ctx context.Context, client *http.Client, metadataURL string) (*OAuthAuthorizationServerMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", metadataURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "docker-mcp-gateway/1.0.0")
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching metadata from %s: %w", metadataURL, err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("authorization server metadata request returned %d", resp.StatusCode)
	}
	
	var metadata OAuthAuthorizationServerMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("parsing authorization server metadata: %w", err)
	}
	
	// Basic validation without issuer URL matching
	if metadata.Issuer == "" {
		return nil, fmt.Errorf("issuer field missing in authorization server metadata")
	}
	if metadata.AuthorizationEndpoint == "" {
		return nil, fmt.Errorf("authorization_endpoint field missing in authorization server metadata")
	}
	if metadata.TokenEndpoint == "" {
		return nil, fmt.Errorf("token_endpoint field missing in authorization server metadata")
	}
	
	return &metadata, nil
}