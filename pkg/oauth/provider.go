package oauth

import (
	"context"
	"time"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

// Provider manages OAuth token lifecycle for a single MCP server
// Each provider runs in its own goroutine with dynamic timing based on token expiry
type Provider struct {
	name         string
	isRefreshing bool
	stopChan     chan struct{}
	eventChan    chan Event
	credHelper   *CredentialHelper
	reloadFn     func(ctx context.Context, serverName string) error
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
		// Get current token status to calculate dynamic timer
		status, err := p.credHelper.GetTokenStatus(ctx, p.name)

		var timer *time.Timer
		if err != nil {
			// Failed to check status - retry in 1 minute
			logf("- Failed to get token status for %s: %v", p.name, err)
			timer = time.NewTimer(1 * time.Minute)
		} else if p.isRefreshing {
			// Refresh already in progress - wait for SSE event with timeout
			logf("- Refresh in progress for %s, waiting for event", p.name)
			timer = time.NewTimer(10 * time.Second) // Timeout if event never comes
		} else if status.Valid && !status.NeedsRefresh {
			// Token valid - calculate when to check next
			timeUntilExpiry := time.Until(status.ExpiresAt)
			timeUntilRefresh := timeUntilExpiry - 5*time.Minute

			// Ensure minimum 1-minute interval
			if timeUntilRefresh < 1*time.Minute {
				timeUntilRefresh = 1 * time.Minute
			}

			timer = time.NewTimer(timeUntilRefresh)
			logf("- Token valid for %s, next check in %v", p.name, timeUntilRefresh.Round(time.Second))
		} else {
			// Token needs refresh - trigger immediately
			logf("- Token needs refresh for %s (expires: %s)", p.name, status.ExpiresAt.Format(time.RFC3339))
			timer = time.NewTimer(0)
		}

		select {
		case <-timer.C:
			// Time to refresh - trigger GetOAuthApp and wait for event
			logf("- Triggering token refresh for %s", p.name)
			p.isRefreshing = true

			go func() {
				authClient := desktop.NewAuthClient()
				app, err := authClient.GetOAuthApp(context.Background(), p.name)
				if err != nil {
					logf("- GetOAuthApp failed for %s: %v", p.name, err)
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

			// Wait for SSE event in nested select (don't loop back yet)
			select {
			case event := <-p.eventChan:
				// SSE event arrived - reload server
				logf("- Provider %s received event: %s", p.name, event.Type)
				if err := p.reloadFn(ctx, p.name); err != nil {
					logf("- Failed to reload %s after %s: %v", p.name, event.Type, err)
				}
				p.isRefreshing = false

			case <-time.After(10 * time.Second):
				// Timeout - event never came
				logf("- Timeout waiting for SSE event for %s, will retry", p.name)
				p.isRefreshing = false

			case <-p.stopChan:
				// Provider stopped (unauthorized or removed)
				return

			case <-ctx.Done():
				// Gateway shutdown
				return
			}

		case event := <-p.eventChan:
			// Received unsolicited SSE event (login-success, logout, etc.)
			timer.Stop()
			logf("- Provider %s received event: %s", p.name, event.Type)
			if err := p.reloadFn(ctx, p.name); err != nil {
				logf("- Failed to reload %s after %s: %v", p.name, event.Type, err)
			}

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

// Stop signals the provider to shutdown gracefully
func (p *Provider) Stop() {
	close(p.stopChan)
}

// SendEvent sends an SSE event to this provider's event channel
func (p *Provider) SendEvent(event Event) {
	p.eventChan <- event
}
