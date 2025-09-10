package oauth

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/oauth"
)

func Authorize(ctx context.Context, app string, scopes string) error {
	// First check if DCR client exists (indicates this is a DCR provider)
	client := desktop.NewAuthClient()
	if _, err := client.GetDCRClient(ctx, app); err == nil {
		// This is a DCR provider - handle it with the MCP OAuth flow
		return authorizeRemoteMCPServer(ctx, app, scopes)
	}

	// Not a DCR provider - handle traditional OAuth flow for built-in providers
	authResponse, err := client.PostOAuthApp(ctx, app, scopes, false)
	if err != nil {
		return err
	}

	// Check if the response contains a valid browser URL
	if authResponse.BrowserURL == "" {
		return fmt.Errorf("OAuth provider does not exist")
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

	// Get catalog to find resource URL for proper token scoping (RFC 8707)
	cat, err := catalog.GetWithOptions(ctx, true, nil)
	if err != nil {
		return fmt.Errorf("failed to get catalog: %w", err)
	}

	server, found := cat.Servers[serverName]
	resourceURL := ""
	if found {
		if server.Remote.URL != "" {
			resourceURL = server.Remote.URL
		} else if server.SSEEndpoint != "" {
			resourceURL = server.SSEEndpoint
		}
	}

	// Create minimal OAuth discovery from DCR client for consistent PKCE handling
	discovery := &oauth.OAuthDiscovery{
		AuthorizationEndpoint: dcrClient.AuthorizationEndpoint,
		TokenEndpoint:         dcrClient.TokenEndpoint,
		ResourceURL:           resourceURL,
	}

	// Generate PKCE parameters and build authorization URL
	fmt.Printf("🔧 Generating PKCE parameters...\n")
	authURL, pkceFlow, err := oauth.BuildAuthorizationURL(discovery, dcrClient.ClientID, strings.Fields(scopes), serverName)
	if err != nil {
		return fmt.Errorf("failed to build authorization URL: %w", err)
	}

	// Store PKCE parameters in Docker Desktop
	pkceParams := desktop.StorePKCERequest{
		State:        pkceFlow.State,
		CodeVerifier: pkceFlow.CodeVerifier,
		ResourceURL:  pkceFlow.ResourceURL,
		ServerName:   pkceFlow.ServerName,
	}
	
	if err := client.StorePKCE(ctx, pkceParams); err != nil {
		return fmt.Errorf("failed to store PKCE parameters: %w", err)
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

