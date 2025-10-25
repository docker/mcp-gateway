package oauth

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/docker/docker-credential-helpers/credentials"
	"golang.org/x/oauth2"

	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/oauth/dcr"
)

// DefaultRedirectURI can be overridden via DOCKER_MCP_OAUTH_REDIRECT_URI env var
// For local testing, set to: http://localhost:8080/oauth/callback
const DefaultRedirectURI = "https://mcp.docker.com/oauth/callback"

// Manager orchestrates OAuth flows for DCR-based providers
type Manager struct {
	dcrManager   *dcr.Manager
	tokenStore   *TokenStore
	stateManager *StateManager
	redirectURI  string
}

// NewManager creates a new OAuth manager for CE mode
func NewManager(credHelper credentials.Helper) *Manager {
	redirectURI := DefaultRedirectURI

	// Allow override for local testing
	if envURI := os.Getenv("DOCKER_MCP_OAUTH_REDIRECT_URI"); envURI != "" {
		redirectURI = envURI
		log.Logf("- Using custom redirect URI: %s", redirectURI)
	}

	return &Manager{
		dcrManager:   dcr.NewManager(credHelper, redirectURI),
		tokenStore:   NewTokenStore(credHelper),
		stateManager: NewStateManager(),
		redirectURI:  redirectURI,
	}
}

// SetRedirectURI sets a custom redirect URI (for testing or custom deployments)
func (m *Manager) SetRedirectURI(uri string) {
	m.redirectURI = uri
}

// EnsureDCRClient ensures a DCR client is registered for the server
// If no client exists or it's incomplete, performs discovery and registration
func (m *Manager) EnsureDCRClient(ctx context.Context, serverName string, scopes string) error {
	// Check if DCR client already exists
	client, err := m.dcrManager.GetDCRClient(serverName)
	if err == nil && client.ClientID != "" {
		// Already registered
		log.Logf("- DCR client already registered for %s (clientID: %s)", serverName, client.ClientID)
		return nil
	}

	// Need to perform DCR
	log.Logf("- No DCR client found for %s, performing registration...", serverName)
	return m.dcrManager.PerformDiscoveryAndRegistration(ctx, serverName, scopes)
}

// BuildAuthorizationURL generates the OAuth authorization URL with PKCE
// If callbackURL is provided, extracts port and embeds in state for mcp-oauth proxy routing
// Returns: authURL, baseState, verifier, error
func (m *Manager) BuildAuthorizationURL(ctx context.Context, serverName string, scopes []string, callbackURL string) (string, string, string, error) {
	// Get DCR client
	dcrClient, err := m.dcrManager.GetDCRClient(serverName)
	if err != nil {
		return "", "", "", fmt.Errorf("DCR client not found for %s: %w", serverName, err)
	}

	if dcrClient.ClientID == "" {
		return "", "", "", fmt.Errorf("DCR client for %s has no clientID - registration incomplete", serverName)
	}

	// Create provider
	provider := NewDCRProvider(dcrClient, m.redirectURI)

	// Generate PKCE verifier
	verifier := provider.GeneratePKCE()

	// Generate base state UUID
	baseState := m.stateManager.Generate(serverName, verifier)

	// If callback URL provided, extract port and format state for mcp-oauth
	// Format: mcp-gateway:PORT:UUID
	var state string
	if callbackURL != "" {
		// Parse callback URL to extract port
		parsedCallback, err := url.Parse(callbackURL)
		if err != nil {
			return "", "", "", fmt.Errorf("invalid callback URL: %w", err)
		}

		port := parsedCallback.Port()
		if port == "" {
			return "", "", "", fmt.Errorf("callback URL missing port")
		}

		state = fmt.Sprintf("mcp-gateway:%s:%s", port, baseState)
		log.Logf("- State format for proxy: mcp-gateway:%s:UUID", port)
	} else {
		state = baseState
	}

	// Build authorization URL
	config := provider.Config()

	// Override scopes if provided
	if len(scopes) > 0 {
		config.Scopes = scopes
	}

	opts := []oauth2.AuthCodeOption{
		oauth2.AccessTypeOffline,             // Request refresh token
		oauth2.S256ChallengeOption(verifier), // PKCE challenge
	}

	// Add resource parameter for RFC 8707 token audience binding
	if provider.ResourceURL() != "" {
		opts = append(opts, oauth2.SetAuthURLParam("resource", provider.ResourceURL()))
		log.Logf("- Adding resource parameter: %s", provider.ResourceURL())
	}

	authURL := config.AuthCodeURL(state, opts...)

	log.Logf("- Generated authorization URL for %s with PKCE", serverName)
	return authURL, baseState, verifier, nil
}

// ExchangeCode exchanges an authorization code for an access token
func (m *Manager) ExchangeCode(ctx context.Context, code string, state string) error {
	// Validate state and retrieve verifier
	serverName, verifier, err := m.stateManager.Validate(state)
	if err != nil {
		return fmt.Errorf("invalid state parameter: %w", err)
	}

	log.Logf("- Exchanging authorization code for %s", serverName)

	// Get DCR client
	dcrClient, err := m.dcrManager.GetDCRClient(serverName)
	if err != nil {
		return fmt.Errorf("DCR client not found for %s: %w", serverName, err)
	}

	// Create provider
	provider := NewDCRProvider(dcrClient, m.redirectURI)
	config := provider.Config()

	// Exchange code for token
	opts := []oauth2.AuthCodeOption{
		oauth2.VerifierOption(verifier), // PKCE verifier
	}

	// Add resource parameter for token request (RFC 8707)
	if provider.ResourceURL() != "" {
		opts = append(opts, oauth2.SetAuthURLParam("resource", provider.ResourceURL()))
	}

	token, err := config.Exchange(ctx, code, opts...)
	if err != nil {
		return fmt.Errorf("token exchange failed for %s: %w", serverName, err)
	}

	log.Logf("- Token exchanged for %s (access: %v, refresh: %v)",
		serverName, token.AccessToken != "", token.RefreshToken != "")

	// Store token
	if err := m.tokenStore.Save(dcrClient, token); err != nil {
		return fmt.Errorf("failed to store token for %s: %w", serverName, err)
	}

	return nil
}

// RevokeToken revokes an OAuth token for a server
func (m *Manager) RevokeToken(ctx context.Context, serverName string) error {
	dcrClient, err := m.dcrManager.GetDCRClient(serverName)
	if err != nil {
		return fmt.Errorf("DCR client not found for %s: %w", serverName, err)
	}

	return m.tokenStore.Delete(dcrClient)
}

// DeleteDCRClient removes a DCR client registration
func (m *Manager) DeleteDCRClient(serverName string) error {
	return m.dcrManager.DeleteDCRClient(serverName)
}
