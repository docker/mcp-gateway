package oauth

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"golang.org/x/oauth2"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/desktop"
	pkgoauth "github.com/docker/mcp-gateway/pkg/oauth"
	"github.com/docker/mcp-gateway/pkg/oauth/dcr"
)

// Authorize performs OAuth authorization for a server, routing to the
// appropriate flow based on the per-server mode (Desktop, CE, or Community).
func Authorize(ctx context.Context, app string, scopes string) error {
	isCommunity, err := lookupIsCommunity(ctx, app)
	if err != nil {
		// Server not in catalog -- fall back to legacy global routing
		// so existing servers without catalog entries still work.
		if pkgoauth.IsCEMode() {
			return authorizeCEMode(ctx, app, scopes)
		}
		return authorizeDesktopMode(ctx, app, scopes)
	}

	switch pkgoauth.DetermineMode(ctx, isCommunity) {
	case pkgoauth.ModeCE:
		return authorizeCEMode(ctx, app, scopes)
	case pkgoauth.ModeCommunity:
		return authorizeCommunityMode(ctx, app, scopes)
	default: // ModeDesktop
		return authorizeDesktopMode(ctx, app, scopes)
	}
}

// lookupIsCommunity checks the catalog to determine if a server is a community server.
func lookupIsCommunity(ctx context.Context, serverName string) (bool, error) {
	cat, err := catalog.GetWithOptions(ctx, true, nil)
	if err != nil {
		return false, err
	}
	server, found := cat.Servers[serverName]
	if !found {
		return false, fmt.Errorf("server %s not found in catalog", serverName)
	}
	return server.IsCommunity(), nil
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

	// Create OAuth manager with read-write credential helper
	credHelper := pkgoauth.NewReadWriteCredentialHelper()
	manager := pkgoauth.NewManager(credHelper)

	// Step 1: Ensure DCR client is registered
	fmt.Printf("Checking DCR registration...\n")
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

	// Step 3: Build authorization URL with callback URL in state
	fmt.Printf("Generating authorization URL...\n")

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

// authorizeCommunityMode handles OAuth for community servers in Desktop mode.
// Uses the Gateway OAuth flow (localhost callback, PKCE) with docker pass storage.
func authorizeCommunityMode(ctx context.Context, serverName string, scopes string) error {
	fmt.Printf("Starting OAuth authorization for %s (community)...\n", serverName)

	// Validate docker pass is available (required for community mode)
	if err := desktop.CheckHasDockerPass(ctx); err != nil {
		return fmt.Errorf("docker pass required for community server OAuth: %w", err)
	}

	// Step 1: Ensure DCR client is registered in docker pass
	fmt.Printf("Checking DCR registration...\n")
	dcrClient, err := pkgoauth.GetDCRClientFromDockerPass(ctx, serverName)
	if err != nil || dcrClient.ClientID == "" {
		// No DCR client in docker pass -- perform discovery and registration
		dcrClient, err = dcr.DiscoverAndRegister(ctx, serverName, scopes, pkgoauth.DefaultRedirectURI)
		if err != nil {
			return fmt.Errorf("DCR registration failed: %w", err)
		}
		if err := pkgoauth.SaveDCRClientToDockerPass(ctx, serverName, dcrClient); err != nil {
			return fmt.Errorf("failed to save DCR client: %w", err)
		}
	}

	// Step 2: Create callback server
	callbackServer, err := pkgoauth.NewCallbackServer()
	if err != nil {
		return fmt.Errorf("failed to create callback server: %w", err)
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

	// Step 3: Build authorization URL with PKCE
	fmt.Printf("Generating authorization URL...\n")

	provider := pkgoauth.NewDCRProvider(dcrClient, pkgoauth.DefaultRedirectURI)
	verifier := provider.GeneratePKCE()

	stateManager := pkgoauth.NewStateManager()
	baseState := stateManager.Generate(serverName, verifier)

	// Encode callback port in state for mcp-oauth proxy routing
	callbackURL := callbackServer.URL()
	parsedCallback, err := url.Parse(callbackURL)
	if err != nil {
		return fmt.Errorf("invalid callback URL: %w", err)
	}
	port := parsedCallback.Port()
	if port == "" {
		return fmt.Errorf("callback URL missing port")
	}
	state := fmt.Sprintf("mcp-gateway:%s:%s", port, baseState)

	config := provider.Config()

	scopesList := []string{}
	if scopes != "" {
		scopesList = []string{scopes}
	}
	if len(scopesList) > 0 {
		config.Scopes = scopesList
	}

	opts := []oauth2.AuthCodeOption{
		oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(verifier),
	}
	if provider.ResourceURL() != "" {
		opts = append(opts, oauth2.SetAuthURLParam("resource", provider.ResourceURL()))
	}

	authURL := config.AuthCodeURL(state, opts...)

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

	// Validate the returned state to prevent CSRF attacks.
	// The mcp-oauth proxy strips the "mcp-gateway:PORT:" prefix and passes
	// the bare UUID to our localhost callback, so callbackState is the UUID
	// that stateManager.Generate() returned.
	validatedServer, validatedVerifier, err := stateManager.Validate(callbackState)
	if err != nil {
		return fmt.Errorf("OAuth state validation failed: %w", err)
	}
	if validatedServer != serverName {
		return fmt.Errorf("OAuth state mismatch: expected server %q, got %q", serverName, validatedServer)
	}

	// Step 6: Exchange code for token
	fmt.Printf("Exchanging authorization code for access token...\n")

	exchangeOpts := []oauth2.AuthCodeOption{
		oauth2.VerifierOption(validatedVerifier),
	}
	if provider.ResourceURL() != "" {
		exchangeOpts = append(exchangeOpts, oauth2.SetAuthURLParam("resource", provider.ResourceURL()))
	}

	token, err := config.Exchange(ctx, code, exchangeOpts...)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	// Step 7: Store token in docker pass
	if err := pkgoauth.SaveTokenToDockerPass(ctx, serverName, token); err != nil {
		return fmt.Errorf("failed to store token: %w", err)
	}

	fmt.Printf("Authorization successful! Token stored securely.\n")
	fmt.Printf("You can now use: docker mcp server start %s\n", serverName)

	return nil
}
