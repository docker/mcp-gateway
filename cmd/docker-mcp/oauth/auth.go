package oauth

import (
	"context"
	"fmt"
	"os"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/desktop"
	"github.com/docker/mcp-gateway/pkg/oauth"
)

func Authorize(ctx context.Context, app string, scopes string) error {
	client := desktop.NewAuthClient()
	// Check if DCR client exists
	dcrClient, err := client.GetDCRClient(ctx, app)
	if err != nil {
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

	// This is a DCR provider - check if it needs setup (atomic DCR)
	fmt.Fprintf(os.Stderr, "[MCP-DCR] Found DCR client for %s, state: %s\n", app, dcrClient.State)
	if dcrClient.State == "unregistered" {
		// Unregistered DCR provider - needs atomic discovery + DCR + auth
		fmt.Printf("first-time oauth setup for %s\n", app)
		return performAtomicDCRAndAuthorize(ctx, app, scopes)
	}

	// DCR client exists and is ready - proceed with normal authorization
	return authorizeRemoteMCPServer(ctx, app, scopes, dcrClient)
}

// performAtomicDCRAndAuthorize performs discovery, DCR, and authorization atomically
func performAtomicDCRAndAuthorize(ctx context.Context, serverName string, scopes string) error {
	// Get catalog to find server configuration
	cat, err := catalog.GetWithOptions(ctx, true, nil)
	if err != nil {
		return fmt.Errorf("failed to get catalog: %w", err)
	}

	server, found := cat.Servers[serverName]
	if !found {
		return fmt.Errorf("server %s not found in catalog", serverName)
	}

	// Get server URL
	serverURL := server.Remote.URL
	if serverURL == "" {
		serverURL = server.SSEEndpoint
		if serverURL == "" {
			return fmt.Errorf("server %s has no remote URL configured", serverName)
		}
	}

	fmt.Fprintf(os.Stderr, "[MCP-DCR] Starting OAuth discovery for server: %s\n", serverName)
	fmt.Printf("discovering oauth requirements for %s\n", serverName)

	// STEP 1: OAuth Discovery (catalog-based, bypass 401 probe)
	discovery, err := oauth.DiscoverOAuthRequirements(ctx, serverURL)
	if err != nil {
		return fmt.Errorf("OAuth discovery failed: %w", err)
	}

	// STEP 2: Dynamic Client Registration
	fmt.Fprintf(os.Stderr, "[MCP-DCR] Starting DCR registration for server: %s\n", serverName)
	fmt.Printf("registering oauth client for %s\n", serverName)
	credentials, err := oauth.PerformDCR(ctx, discovery, serverName)
	if err != nil {
		return fmt.Errorf("DCR registration failed: %w", err)
	}

	// Extract provider name from OAuth config
	var providerName string
	if server.OAuth != nil && len(server.OAuth.Providers) > 0 {
		providerName = server.OAuth.Providers[0].Provider // Use first provider
	} else {
		return fmt.Errorf("no OAuth providers configured for server %s", serverName)
	}

	// STEP 3: Store DCR client in Docker Desktop (updates the pending provider)
	client := desktop.NewAuthClient()
	dcrRequest := desktop.RegisterDCRRequest{
		ClientID:              credentials.ClientID,
		ProviderName:          providerName,
		AuthorizationEndpoint: credentials.AuthorizationEndpoint,
		TokenEndpoint:         credentials.TokenEndpoint,
		ResourceURL:           credentials.ServerURL,
	}

	if err := client.RegisterDCRClient(ctx, serverName, dcrRequest); err != nil {
		return fmt.Errorf("failed to store DCR client: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[MCP-DCR] DCR registration successful, clientID: %s, authEndpoint: %s\n", credentials.ClientID, credentials.AuthorizationEndpoint)
	fmt.Printf("oauth client registered successfully\n")
	fmt.Printf("   Client ID: %s\n", credentials.ClientID)

	// STEP 4: Continue with authorization
	dcrClient := &desktop.DCRClient{
		ServerName:            serverName,
		ProviderName:          providerName,
		ClientID:              credentials.ClientID,
		AuthorizationEndpoint: credentials.AuthorizationEndpoint,
		TokenEndpoint:         credentials.TokenEndpoint,
	}

	return authorizeRemoteMCPServer(ctx, serverName, scopes, dcrClient)
}

func authorizeRemoteMCPServer(ctx context.Context, serverName string, scopes string, dcrClient *desktop.DCRClient) error {
	client := desktop.NewAuthClient()

	fmt.Printf("starting oauth authorization for %s\n", serverName)
	fmt.Printf("   Using client: %s\n", dcrClient.ClientID)

	// Start OAuth flow via Docker Desktop (handles PKCE generation and browser opening)
	fmt.Printf("starting oauth authorization flow\n")
	authResponse, err := client.PostOAuthApp(ctx, serverName, scopes, false)
	if err != nil {
		return fmt.Errorf("failed to start OAuth flow: %w", err)
	}

	// Provide user feedback based on auth response
	if authResponse.BrowserURL != "" {
		fmt.Printf("browser opened for oauth authentication\n")
		fmt.Printf("If the browser doesn't open, visit: %s\n", authResponse.BrowserURL)
	} else {
		fmt.Printf("oauth flow started successfully\n")
	}

	return nil
}
