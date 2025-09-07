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
// - Gracefully handles servers with partial MCP compliance
//
// ROBUST DISCOVERY FLOW (Inspector-inspired):
// 1. Make request to MCP server to trigger 401 response
// 2. Default authorization server to MCP server domain
// 3. Try to parse WWW-Authenticate header for resource_metadata URL
// 4. If resource metadata available, try to fetch it (optional)
// 5. Always fetch Authorization Server Metadata (required)
// 6. Build discovery result with whatever information is available
func DiscoverOAuthRequirements(ctx context.Context, serverURL string) (*OAuthDiscovery, error) {
	// Create HTTP client with reasonable timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Parse server URL to extract base domain for defaults
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}

	// STEP 1: Make initial MCP request to trigger 401 Unauthorized
	// MCP Spec Section 4.1: "MCP request without token" should trigger 401
	// Use POST with initialize request as per spec diagrams
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

	// STEP 2: Parse WWW-Authenticate header (if present)
	// MCP Spec Section 4.1: "MCP servers MUST use the HTTP header WWW-Authenticate when returning a 401 Unauthorized"
	wwwAuth := resp.Header.Get("WWW-Authenticate")
	if wwwAuth == "" {
		return nil, fmt.Errorf("server returned 401 but no WWW-Authenticate header")
	}

	challenges, err := ParseWWWAuthenticate(wwwAuth)
	if err != nil {
		return nil, fmt.Errorf("parsing WWW-Authenticate header: %w", err)
	}

	// STEP 3: Initialize with intelligent defaults (Inspector pattern)
	// Default authorization server to MCP server's domain
	defaultAuthServerURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
	
	// Initialize discovery with defaults
	var resourceMetadata *OAuthProtectedResourceMetadata
	var resourceMetadataError error
	authServerURL := defaultAuthServerURL
	
	// STEP 4: Try to get resource metadata (OPTIONAL - don't fail if missing)
	// RFC 9728 Section 5.1: resource_metadata parameter in WWW-Authenticate
	resourceMetadataURL := FindResourceMetadataURL(challenges)
	if resourceMetadataURL != "" {
		// Resource metadata URL found - try to fetch it
		fmt.Printf("📋 Found resource_metadata URL in WWW-Authenticate: %s\n", resourceMetadataURL)
		resourceMetadata, resourceMetadataError = fetchOAuthProtectedResourceMetadata(ctx, client, resourceMetadataURL)
		if resourceMetadataError != nil {
			// Log warning but continue - resource metadata is supplementary
			fmt.Printf("⚠️  Failed to fetch resource metadata: %v (continuing with defaults)\n", resourceMetadataError)
		} else if resourceMetadata != nil && resourceMetadata.AuthorizationServer != "" {
			// Use authorization server from resource metadata if available
			authServerURL = resourceMetadata.AuthorizationServer
			fmt.Printf("✅ Using authorization server from resource metadata: %s\n", authServerURL)
		}
	} else {
		// No resource_metadata in WWW-Authenticate - try well-known endpoint
		fmt.Printf("📋 No resource_metadata in WWW-Authenticate, trying well-known endpoint\n")
		wellKnownURL := fmt.Sprintf("%s/.well-known/oauth-protected-resource", defaultAuthServerURL)
		resourceMetadata, resourceMetadataError = fetchOAuthProtectedResourceMetadata(ctx, client, wellKnownURL)
		if resourceMetadataError != nil {
			fmt.Printf("⚠️  Well-known resource metadata not available: %v\n", resourceMetadataError)
		} else if resourceMetadata != nil && resourceMetadata.AuthorizationServer != "" {
			authServerURL = resourceMetadata.AuthorizationServer
			fmt.Printf("✅ Found authorization server via well-known: %s\n", authServerURL)
		}
	}

	// STEP 5: Fetch Authorization Server Metadata (REQUIRED)
	// MCP Spec Section 3.1: "Authorization servers MUST provide OAuth 2.0 Authorization Server Metadata (RFC8414)"
	authServerMetadata, err := fetchAuthorizationServerMetadata(ctx, client, authServerURL)
	if err != nil {
		return nil, fmt.Errorf("fetching authorization server metadata from %s: %w", authServerURL, err)
	}
	fmt.Printf("✅ Successfully fetched authorization server metadata\n")

	// STEP 6: Build discovery result with all available information
	discovery := &OAuthDiscovery{
		RequiresOAuth: true,
		
		// Use resource metadata if available, otherwise use defaults
		ResourceURL:         defaultAuthServerURL,
		ResourceServer:      defaultAuthServerURL,
		AuthorizationServer: authServerURL,
		
		// From Authorization Server Metadata (RFC 8414) - always available
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

	// Override with resource metadata if successfully fetched
	if resourceMetadata != nil {
		if resourceMetadata.Resource != "" {
			discovery.ResourceURL = resourceMetadata.Resource
			discovery.ResourceServer = resourceMetadata.Resource
		}
		if len(resourceMetadata.Scopes) > 0 {
			discovery.Scopes = resourceMetadata.Scopes
		}
	}

	// Extract additional scopes from WWW-Authenticate if not available from metadata
	if len(discovery.Scopes) == 0 {
		discovery.Scopes = FindRequiredScopes(challenges)
	}

	return discovery, nil
}

// DiscoverOAuthFromCatalog performs OAuth discovery for servers pre-configured in catalog
// with OAuth requirements already known.
//
// This now simply delegates to the main DiscoverOAuthRequirements function,
// which has been refactored to handle all discovery patterns robustly.
func DiscoverOAuthFromCatalog(ctx context.Context, serverURL string) (*OAuthDiscovery, error) {
	// The main discovery function now handles all cases gracefully
	return DiscoverOAuthRequirements(ctx, serverURL)
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



// containsString checks if a slice contains a specific string
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}


