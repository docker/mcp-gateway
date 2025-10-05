package oauth

import (
	"context"
	"sync"
	"time"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

const maxRefreshRetries = 7 // Max attempts to refresh when expiry hasn't changed

// Provider manages OAuth token lifecycle for a single MCP server
// Each provider runs in its own goroutine with dynamic timing based on token expiry
type Provider struct {
	name              string
	lastRefreshExpiry time.Time
	refreshRetryCount int
	stopOnce          sync.Once
	stopChan          chan struct{}
	eventChan         chan Event
	credHelper        *CredentialHelper
	reloadFn          func(ctx context.Context, serverName string) error
}

// NewProvider creates a new OAuth provider
func NewProvider(name string, reloadFn func(context.Context, string) error) *Provider {
	return &Provider{
		name:       name,
		stopChan:   make(chan struct{}),
		eventChan:  make(chan Event),
		credHelper: NewOAuthCredentialHelper(),
		reloadFn:   reloadFn,
	}
}

// Run starts the provider's background loop
// Loop dynamically adjusts timing based on token expiry
func (p *Provider) Run(ctx context.Context) {
	logf("- Started OAuth provider loop for %s", p.name)
	defer logf("- Stopped OAuth provider loop for %s", p.name)

	for {
		// Check current token status
		status, err := p.credHelper.GetTokenStatus(ctx, p.name)
		if err != nil {
			// Unable to get token status - stop provider
			// This means either never authorized or credential helper broken
			logf("! Unable to get token status for %s: %v", p.name, err)
			logf("! Run 'docker mcp oauth authorize %s' if not yet authorized", p.name)
			return
		}

		if status.NeedsRefresh {
			// Check if token expiry hasn't changed since last refresh
			if !p.lastRefreshExpiry.IsZero() && status.ExpiresAt.Equal(p.lastRefreshExpiry) {
				p.refreshRetryCount++

				if p.refreshRetryCount >= maxRefreshRetries {
					// Tried max times, expiry never changed - give up
					logf("! Token expiry unchanged after %d refresh attempts for %s", maxRefreshRetries, p.name)
					return
				}

				// Wait before retrying with exponential backoff: 30s, 1min, 2min, 4min, 8min...
				backoff := time.Duration(30*(1<<(p.refreshRetryCount-1))) * time.Second
				logf("! Token expiry unchanged for %s, retry %d/%d after %v", p.name, p.refreshRetryCount, maxRefreshRetries, backoff)
				time.Sleep(backoff)
				// Fall through to trigger refresh again
			}

			// Expiry different from last refresh (or first refresh) - reset retry count
			if p.refreshRetryCount > 0 && !status.ExpiresAt.Equal(p.lastRefreshExpiry) {
				logf("- Token expiry updated for %s, resetting retry count", p.name)
				p.refreshRetryCount = 0
			}

			// Token expired or expiring soon - trigger refresh
			logf("- Triggering Token refresh for %s (expires: %s)", p.name, status.ExpiresAt.Format(time.RFC3339))

			// Track this expiry to detect unchanged expiry on next iteration
			p.lastRefreshExpiry = status.ExpiresAt

			go func() {
				authClient := desktop.NewAuthClient()
				app, err := authClient.GetOAuthApp(context.Background(), p.name)
				if err != nil {
					// Refresh failed
					logf("! GetOAuthApp failed for %s: %v", p.name, err)
					return
				}

				if !app.Authorized {
					// Not authorized
					logf("! GetOAuthApp returned Authorized=false for %s", p.name)
					return
				}
				// Authorized: SSE event will be handled in next iteration
			}()

			// Brief sleep to let GetOAuthApp start, then loop back
			// SSE event will arrive asynchronously and trigger reload in next iteration
			time.Sleep(2 * time.Second)

		} else {
			// Token valid - park until next refresh needed
			timeUntilExpiry := time.Until(status.ExpiresAt)
			timeUntilRefresh := timeUntilExpiry - 10*time.Second

			// Ensure non-negative duration
			if timeUntilRefresh < 0 {
				timeUntilRefresh = 0 // Check immediately
			}

			timer := time.NewTimer(timeUntilRefresh)
			logf("- Token valid for %s, next check in %v", p.name, timeUntilRefresh.Round(time.Second))

			// Park waiting for timer or SSE event
			select {
			case <-timer.C:
				// Time to check/refresh - loop back

			case event := <-p.eventChan:
				// SSE event arrived while waiting - reload
				timer.Stop()
				logf("- Provider %s received event: %s", p.name, event.Type)
				if err := p.reloadFn(ctx, p.name); err != nil {
					logf("- Failed to reload %s after %s: %v", p.name, event.Type, err)
				}
				// Loop back to recalculate timer

			case <-p.stopChan:
				// Server disabled/removed - shutdown gracefully
				timer.Stop()
				return

			case <-ctx.Done():
				// Gateway shutdown
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

// SendEvent sends an SSE event to this provider's event channel
func (p *Provider) SendEvent(event Event) {
	p.eventChan <- event
}
