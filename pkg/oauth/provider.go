package oauth

import (
	"context"
	"time"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

const maxRefreshRetries = 3 // Max attempts to refresh when DD doesn't update expiry

// Provider manages OAuth token lifecycle for a single MCP server
// Each provider runs in its own goroutine with dynamic timing based on token expiry
type Provider struct {
	name              string
	lastRefreshExpiry time.Time // Last expiry we attempted to refresh (workaround for DD bug)
	refreshRetryCount int       // Number of retries for same expiry
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
			// Check if we already tried to refresh this exact expiry (DD bug workaround)
			if !p.lastRefreshExpiry.IsZero() && status.ExpiresAt.Equal(p.lastRefreshExpiry) {
				// Token expiry unchanged after refresh - DD didn't persist new expiry
				p.refreshRetryCount++

				if p.refreshRetryCount >= maxRefreshRetries {
					// Tried max times, expiry never changed - give up
					logf("! Token expiry unchanged after %d refresh attempts for %s", maxRefreshRetries, p.name)
					logf("! This is a Docker Desktop bug - provider stopping")
					logf("! Provider will recreate on next EventTokenRefresh or EventLoginSuccess")
					return
				}

				// Retry with exponential backoff (2min, 4min, 6min)
				backoff := time.Duration(p.refreshRetryCount) * 2 * time.Minute
				logf("! Token expiry unchanged for %s, retry %d/%d in %v", p.name, p.refreshRetryCount, maxRefreshRetries, backoff)
				time.Sleep(backoff)
				continue
			}

			// Expiry different from last refresh (or first refresh) - reset retry count
			if p.refreshRetryCount > 0 {
				logf("- Token expiry updated for %s, resetting retry count", p.name)
				p.refreshRetryCount = 0
			}

			// Token expired or expiring soon - trigger refresh and wait for SSE event
			logf("- Token needs refresh for %s (expires: %s)", p.name, status.ExpiresAt.Format(time.RFC3339))
			logf("- Triggering token refresh for %s", p.name)

			// Track this expiry to detect DD bug on next iteration
			p.lastRefreshExpiry = status.ExpiresAt

			go func() {
				authClient := desktop.NewAuthClient()
				app, err := authClient.GetOAuthApp(context.Background(), p.name)
				if err != nil {
					// Refresh failed - stop provider
					logf("- GetOAuthApp failed for %s: %v", p.name, err)
					p.Stop()
					return
				}

				if !app.Authorized {
					// User hasn't authorized - stop provider
					logf("! Provider %s not authorized. Run 'docker mcp oauth authorize %s'", p.name, p.name)
					p.Stop()
					return
				}
				// Authorized: SSE event will arrive on p.eventChan
			}()

			// Park waiting for SSE event - don't loop back until it arrives
			select {
			case event := <-p.eventChan:
				// SSE event arrived - reload server
				logf("- Provider %s received event: %s", p.name, event.Type)
				if err := p.reloadFn(ctx, p.name); err != nil {
					logf("- Failed to reload %s after %s: %v", p.name, event.Type, err)
				}
				// Loop back - token should be valid now

			case <-time.After(10 * time.Second):
				// SSE event never came - stop provider
				logf("! Timeout waiting for SSE event for %s - stopping provider", p.name)
				return

			case <-p.stopChan:
				// Provider stopped (unauthorized or removed)
				return

			case <-ctx.Done():
				// Gateway shutdown
				return
			}

		} else {
			// Token valid - park until next refresh needed
			timeUntilExpiry := time.Until(status.ExpiresAt)
			timeUntilRefresh := timeUntilExpiry - 5*time.Minute

			// Ensure minimum 1-minute interval
			if timeUntilRefresh < 1*time.Minute {
				timeUntilRefresh = 1 * time.Minute
			}

			timer := time.NewTimer(timeUntilRefresh)
			logf("- Token valid for %s, next check in %v", p.name, timeUntilRefresh.Round(time.Second))

			// Park waiting for timer or unsolicited event
			select {
			case <-timer.C:
				// Time to check/refresh - loop back

			case event := <-p.eventChan:
				// Event arrived while waiting - reload and recalculate timer
				timer.Stop()
				logf("- Provider %s received event: %s", p.name, event.Type)
				if err := p.reloadFn(ctx, p.name); err != nil {
					logf("- Failed to reload %s after %s: %v", p.name, event.Type, err)
				}
				// Loop back to recalculate timer based on current token state

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
	close(p.stopChan)
}

// SendEvent sends an SSE event to this provider's event channel
func (p *Provider) SendEvent(event Event) {
	p.eventChan <- event
}
