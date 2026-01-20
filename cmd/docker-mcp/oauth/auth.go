package oauth

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/desktop"
	pkgoauth "github.com/docker/mcp-gateway/pkg/oauth"
)

func Authorize(ctx context.Context, app string, scopes string) error {
	// Check if running in CE mode AND if this is a remote server
	// CE mode with localhost redirect only works for remote MCP servers
	// Container-based servers (like github-official) need Docker Desktop OAuth
	if pkgoauth.IsCEMode() && isRemoteServer(ctx, app) {
		return authorizeCEMode(ctx, app, scopes)
	}

	// Desktop mode - existing implementation (for container servers or when CE mode is off)
	return authorizeDesktopMode(ctx, app, scopes)
}

// isRemoteServer checks if the server is a remote MCP server (not a container)
func isRemoteServer(ctx context.Context, serverName string) bool {
	cat, err := catalog.GetWithOptions(ctx, true, nil)
	if err != nil {
		return false
	}
	server, found := cat.Servers[serverName]
	if !found {
		return false
	}
	return server.Remote.URL != ""
}

// authorizeDesktopMode handles OAuth via Docker Desktop (existing behavior)
func authorizeDesktopMode(ctx context.Context, app string, scopes string) error {
	client := desktop.NewAuthClient()

	// Start OAuth flow - Docker Desktop handles DCR automatically if needed
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

// authorizeCEMode handles OAuth in standalone CE mode
func authorizeCEMode(ctx context.Context, serverName string, scopes string) error {
	fmt.Printf("Starting OAuth authorization for %s...\n", serverName)

	// Step 1: Create callback server FIRST to get the localhost URL
	// This allows us to register DCR with localhost redirect instead of mcp.docker.com proxy
	callbackServer, err := pkgoauth.NewCallbackServer()
	if err != nil {
		return fmt.Errorf("failed to create callback server: %w", err)
	}

	// Use localhost callback URL as redirect URI for DCR registration
	// This bypasses Docker's OAuth proxy which doesn't know about all providers
	callbackURL := callbackServer.URL()
	fmt.Printf("Using localhost redirect: %s\n", callbackURL)

	// Create OAuth manager with localhost redirect URI
	credHelper := pkgoauth.NewReadWriteCredentialHelper()
	manager := pkgoauth.NewManagerWithRedirectURI(credHelper, callbackURL)

	// Step 2: Ensure DCR client is registered with localhost redirect
	fmt.Printf("Checking DCR registration...\n")
	if err := manager.EnsureDCRClient(ctx, serverName, scopes); err != nil {
		return fmt.Errorf("DCR registration failed: %w", err)
	}

	// Start callback server in background
	go func() {
		if err := callbackServer.Start(); err != nil {
			fmt.Printf("Callback server error: %v\n", err)
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := callbackServer.Shutdown(shutdownCtx); err != nil {
			fmt.Printf("Warning: failed to shutdown callback server: %v\n", err)
		}
	}()

	// Step 3: Build authorization URL (no proxy state needed - direct localhost redirect)
	fmt.Printf("Generating authorization URL...\n")

	scopesList := []string{}
	if scopes != "" {
		scopesList = []string{scopes}
	}

	// Empty callbackURL since we're using direct localhost redirect (no proxy routing)
	authURL, baseState, _, err := manager.BuildAuthorizationURL(ctx, serverName, scopesList, "")
	if err != nil {
		return fmt.Errorf("failed to generate authorization URL: %w", err)
	}

	// Store base state for later validation
	_ = baseState // We'll validate using the state from callback

	// Step 4: Display authorization URL
	fmt.Printf("Please visit this URL to authorize:\n\n  %s\n\n", authURL)

	// Step 5: Wait for callback
	fmt.Printf("Waiting for authorization callback on http://localhost:%d/callback...\n", callbackServer.Port())

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	code, callbackState, err := callbackServer.Wait(timeoutCtx)
	if err != nil {
		return fmt.Errorf("failed to receive callback: %w", err)
	}

	// Step 6: Exchange code for token
	fmt.Printf("Exchanging authorization code for access token...\n")
	if err := manager.ExchangeCode(ctx, code, callbackState); err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	fmt.Printf("Authorization successful! Token stored securely.\n")
	fmt.Printf("You can now use: docker mcp server start %s\n", serverName)

	return nil
}
