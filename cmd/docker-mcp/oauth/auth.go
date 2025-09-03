package oauth

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/oauth"
)

func Authorize(ctx context.Context, app string, scopes string) error {
	// Check if this is a remote MCP server
	if strings.HasSuffix(app, "-remote") {
		return authorizeRemoteMCPServer(ctx, app, scopes)
	}

	client := desktop.NewAuthClient()

	authResponse, err := client.PostOAuthApp(ctx, app, scopes, false)
	if err != nil {
		return err
	}

	fmt.Printf("Opening your browser for authentication. If it doesn't open automatically, please visit: %s\n", authResponse.BrowserURL)

	return nil
}

func authorizeRemoteMCPServer(ctx context.Context, serverName string, scopes string) error {
	client := desktop.NewAuthClient()

	// Check if DCR client exists (should exist after server enable)
	dcrClient, err := client.GetDCRClient(ctx, serverName)
	if err != nil {
		// Fallback: DCR client doesn't exist, suggest server enable
		fmt.Printf("⚠️ OAuth not set up for %s.\n", serverName)
		fmt.Printf("Run 'docker mcp server enable %s' to set up OAuth automatically.\n", serverName)
		return fmt.Errorf("DCR client not found for %s: %w", serverName, err)
	}

	fmt.Printf("🔐 Starting OAuth authorization for %s...\n", serverName)
	fmt.Printf("   Using existing client: %s\n", dcrClient.ClientID)

	// Generate PKCE parameters for this authorization flow
	fmt.Printf("🔧 Generating PKCE parameters...\n")
	codeVerifier := oauth.GenerateCodeVerifier()
	state := oauth.GenerateState()

	// Store PKCE parameters with server context
	pkceParams := desktop.StorePKCERequest{
		State:        state,
		CodeVerifier: codeVerifier,
		ServerName:   serverName,
	}
	
	// Get catalog to find resource URL for proper token scoping (RFC 8707)
	cat, err := catalog.GetWithOptions(ctx, true, nil)
	if err != nil {
		return fmt.Errorf("failed to get catalog: %w", err)
	}

	server, found := cat.Servers[serverName]
	if found {
		if server.Remote.URL != "" {
			pkceParams.ResourceURL = server.Remote.URL
		} else if server.SSEEndpoint != "" {
			pkceParams.ResourceURL = server.SSEEndpoint
		}
	}

	// Store PKCE parameters in Docker Desktop
	if err := client.StorePKCE(ctx, pkceParams); err != nil {
		return fmt.Errorf("failed to store PKCE parameters: %w", err)
	}

	// Build authorization URL using stored DCR client
	authURL, err := buildAuthorizationURLFromDCRClient(*dcrClient, state, codeVerifier, pkceParams.ResourceURL, strings.Fields(scopes))
	if err != nil {
		return fmt.Errorf("failed to build authorization URL: %w", err)
	}

	// Open browser for OAuth flow
	fmt.Printf("🌐 Opening browser for OAuth authentication...\n")
	if err := oauth.OpenBrowser(authURL); err != nil {
		fmt.Printf("Failed to open browser automatically. Please visit: %s\n", authURL)
	} else {
		fmt.Printf("If the browser doesn't open, visit: %s\n", authURL)
	}

	fmt.Printf("✅ Once authenticated, %s will be ready for use\n", serverName)

	return nil
}

// buildAuthorizationURLFromDCRClient builds OAuth authorization URL using stored DCR client
func buildAuthorizationURLFromDCRClient(dcrClient desktop.DCRClient, state, codeVerifier, resourceURL string, scopes []string) (string, error) {
	if dcrClient.AuthorizationEndpoint == "" {
		return "", fmt.Errorf("DCR client missing authorization endpoint")
	}

	if dcrClient.ClientID == "" {
		return "", fmt.Errorf("DCR client missing client ID")
	}

	// Generate PKCE challenge
	challenge := oauth.GenerateS256Challenge(codeVerifier)

	// Build OAuth parameters using url.Values for proper encoding
	params := url.Values{}
	params.Set("client_id", dcrClient.ClientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", "https://mcp.docker.com/oauth/callback") // mcp-oauth callback
	params.Set("state", state)

	// PKCE parameters (OAuth 2.1 MUST requirement for public clients)
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256") // Strongest available method

	// Resource parameter (RFC 8707 for token audience binding)
	if resourceURL != "" {
		params.Set("resource", resourceURL)
	}

	// Add scopes if provided
	if len(scopes) > 0 {
		params.Set("scope", strings.Join(scopes, " "))
	}

	// Build complete authorization URL
	authURL := dcrClient.AuthorizationEndpoint + "?" + params.Encode()

	return authURL, nil
}
