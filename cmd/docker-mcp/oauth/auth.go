package oauth

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/mcp-gateway/pkg/desktop"
	pkgoauth "github.com/docker/mcp-gateway/pkg/oauth"
)

func Authorize(ctx context.Context, app string, scopes string) error {
	// Check if running in CE mode
	if pkgoauth.IsCEMode() {
		return authorizeCEMode(ctx, app, scopes)
	}

	// Desktop mode - existing implementation
	return authorizeDesktopMode(ctx, app, scopes)
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
	fmt.Printf("🔐 Starting OAuth authorization for %s (CE mode)\n", serverName)

	// Create OAuth manager with read-write credential helper
	credHelper := pkgoauth.NewReadWriteCredentialHelper()
	manager := pkgoauth.NewManager(credHelper)

	// Step 1: Ensure DCR client is registered
	fmt.Printf("📋 Checking DCR registration...\n")
	if err := manager.EnsureDCRClient(ctx, serverName, scopes); err != nil {
		return fmt.Errorf("DCR registration failed: %w", err)
	}

	// Step 2: Create callback server
	callbackServer, err := pkgoauth.NewCallbackServer()
	if err != nil {
		return fmt.Errorf("failed to create callback server: %w", err)
	}

	// Start callback server in background
	go func() {
		if err := callbackServer.Start(); err != nil {
			fmt.Printf("! Callback server error: %v\n", err)
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		callbackServer.Shutdown(shutdownCtx)
	}()

	// Step 3: Build authorization URL with callback URL in state
	fmt.Printf("🔗 Generating authorization URL...\n")

	scopesList := []string{}
	if scopes != "" {
		scopesList = []string{scopes}
	}

	// Pass callback URL - will be embedded in state for mcp-oauth proxy routing
	callbackURL := callbackServer.URL()
	authURL, baseState, _, err := manager.BuildAuthorizationURL(ctx, serverName, scopesList, callbackURL)
	if err != nil {
		return fmt.Errorf("failed to generate authorization URL: %w", err)
	}

	// Store base state for later validation
	_ = baseState // We'll validate using the state from callback

	// Step 4: Open browser
	fmt.Printf("🌐 Opening browser for authorization...\n")
	fmt.Printf("   If it doesn't open automatically, visit: %s\n", authURL)

	if err := pkgoauth.OpenBrowser(authURL); err != nil {
		fmt.Printf("! Failed to open browser automatically: %v\n", err)
		fmt.Printf("! Please open the URL manually\n")
	}

	// Step 5: Wait for callback
	fmt.Printf("⏳ Waiting for authorization callback on http://localhost:%d/callback...\n", callbackServer.Port())

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	code, callbackState, err := callbackServer.Wait(timeoutCtx)
	if err != nil {
		return fmt.Errorf("failed to receive callback: %w", err)
	}

	// Step 6: Exchange code for token
	fmt.Printf("🔄 Exchanging authorization code for access token...\n")
	if err := manager.ExchangeCode(ctx, code, callbackState); err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	fmt.Printf("✅ Authorization successful! Token stored securely.\n")
	fmt.Printf("   You can now use: docker mcp server start %s\n", serverName)

	return nil
}

