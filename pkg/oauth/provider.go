package oauth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/oauth/dcr"
)

// DCRProvider represents a dynamically registered OAuth provider
// Implements public client + PKCE for security
type DCRProvider struct {
	name        string
	config      *oauth2.Config
	resourceURL string // For RFC 8707 token audience binding
}

// NewDCRProvider creates a new DCR provider from a registered DCR client
func NewDCRProvider(dcrClient dcr.Client, redirectURL string) *DCRProvider {
	config := &oauth2.Config{
		ClientID:     dcrClient.ClientID,
		ClientSecret: "", // Public client - no secret
		RedirectURL:  redirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  dcrClient.AuthorizationEndpoint,
			TokenURL: dcrClient.TokenEndpoint,
		},
		Scopes: dcrClient.RequiredScopes,
	}

	return &DCRProvider{
		name:        dcrClient.ServerName,
		config:      config,
		resourceURL: dcrClient.ResourceURL,
	}
}

// Name returns the provider name
func (p *DCRProvider) Name() string {
	return p.name
}

// Config returns the OAuth2 configuration
func (p *DCRProvider) Config() *oauth2.Config {
	return p.config
}

// ResourceURL returns the resource URL for RFC 8707 token audience binding
func (p *DCRProvider) ResourceURL() string {
	return p.resourceURL
}

// GeneratePKCE generates a new PKCE code verifier
// The challenge is automatically computed by oauth2 library when using S256ChallengeOption
func (p *DCRProvider) GeneratePKCE() string {
	return oauth2.GenerateVerifier()
}

// Provider manages OAuth token lifecycle for a single MCP server (CE mode only).
// Polls token expiry, triggers refresh when needed, and reloads the server connection.
// In Desktop mode, Secrets Engine handles token refresh and SSE events trigger reloads.
type Provider struct {
	name              string
	lastRefreshExpiry time.Time
	refreshRetryCount int
	stopOnce          sync.Once
	stopChan          chan struct{}
	credHelper        *CredentialHelper
	reloadFn          func(ctx context.Context, serverName string) error
}

const maxRefreshRetries = 7 // Max attempts to refresh when expiry hasn't changed

// NewProvider creates a new OAuth provider for token refresh polling
func NewProvider(name string, reloadFn func(context.Context, string) error) *Provider {
	return &Provider{
		name:       name,
		stopChan:   make(chan struct{}),
		credHelper: NewOAuthCredentialHelper(),
		reloadFn:   reloadFn,
	}
}

// Run starts the provider's background polling loop.
// Checks token expiry, triggers refresh when needed, and reloads server connections.
func (p *Provider) Run(ctx context.Context) {
	log.Logf("- Started OAuth provider loop for %s", p.name)
	defer log.Logf("- Stopped OAuth provider loop for %s", p.name)

	for {
		// Check current token status
		status, err := p.credHelper.GetTokenStatus(ctx, p.name)
		if err != nil {
			log.Logf("! Unable to get token status for %s: %v", p.name, err)
			log.Logf("! Run 'docker mcp oauth authorize %s' if not yet authorized", p.name)
			return
		}

		// Calculate wait duration based on token status
		var waitDuration time.Duration

		if status.NeedsRefresh {
			// Token needs refresh - check if expiry unchanged from last attempt
			expiryUnchanged := !p.lastRefreshExpiry.IsZero() && status.ExpiresAt.Equal(p.lastRefreshExpiry)

			if expiryUnchanged {
				p.refreshRetryCount++
			} else {
				if p.refreshRetryCount > 0 {
					log.Logf("- Token expiry updated for %s, resetting refresh count", p.name)
				}
				p.refreshRetryCount = 1
			}

			if p.refreshRetryCount > maxRefreshRetries {
				log.Logf("! Token expiry unchanged after %d refresh attempts for %s", maxRefreshRetries, p.name)
				return
			}

			// Exponential backoff: 30s, 1min, 2min, 4min, 8min...
			waitDuration = time.Duration(30*(1<<(p.refreshRetryCount-1))) * time.Second
			log.Logf("- Triggering token refresh for %s, attempt %d/%d, waiting %v",
				p.name, p.refreshRetryCount, maxRefreshRetries, waitDuration)

			p.lastRefreshExpiry = status.ExpiresAt

			// Refresh token and reload server connection
			go func() {
				if err := p.refreshTokenCE(); err != nil {
					log.Logf("! Token refresh failed for %s: %v", p.name, err)
					return
				}
				// Reload server to pick up the new token
				if err := p.reloadFn(ctx, p.name); err != nil {
					log.Logf("! Failed to reload %s after token refresh: %v", p.name, err)
				}
			}()

		} else {
			// Token still valid
			timeUntilExpiry := time.Until(status.ExpiresAt)
			waitDuration = max(0, timeUntilExpiry-10*time.Second)
			log.Logf("- Token valid for %s, next check in %v", p.name, waitDuration.Round(time.Second))
		}

		// Wait until next check, interruptible by stop signal
		if waitDuration > 0 {
			timer := time.NewTimer(waitDuration)
			select {
			case <-timer.C:
				// Wait complete, continue to next iteration
			case <-p.stopChan:
				timer.Stop()
				return
			case <-ctx.Done():
				timer.Stop()
				return
			}
		}
	}
}

// Stop signals the provider to shutdown gracefully
func (p *Provider) Stop() {
	p.stopOnce.Do(func() {
		close(p.stopChan)
	})
}

// refreshTokenCE refreshes an OAuth token in CE mode
// Uses the same oauth2 library refresh mechanism as Desktop
func (p *Provider) refreshTokenCE() error {
	// Create read-write credential helper for save operations
	rwHelper := NewReadWriteCredentialHelper()

	// Get DCR client from credential helper
	dcrMgr := dcr.NewManager(rwHelper, "")
	dcrClient, err := dcrMgr.GetDCRClient(p.name)
	if err != nil {
		return fmt.Errorf("failed to get DCR client: %w", err)
	}

	// Get current token and create token store
	tokenStore := NewTokenStore(rwHelper)
	token, err := tokenStore.Retrieve(dcrClient)
	if err != nil {
		return fmt.Errorf("failed to retrieve token: %w", err)
	}

	// Refresh token using oauth2 library
	provider := NewDCRProvider(dcrClient, DefaultRedirectURI)
	config := provider.Config()

	// TokenSource automatically refreshes using refresh_token
	refreshedToken, err := config.TokenSource(context.Background(), token).Token()
	if err != nil {
		return fmt.Errorf("token refresh failed: %w", err)
	}

	// Save refreshed token
	if err := tokenStore.Save(dcrClient, refreshedToken); err != nil {
		return fmt.Errorf("failed to save refreshed token: %w", err)
	}

	log.Logf("- Successfully refreshed token for %s", p.name)
	return nil
}
