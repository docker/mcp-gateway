package oauth

import (
	"context"
	"sync"
	"time"

	"github.com/docker/mcp-gateway/pkg/desktop"
	"github.com/docker/mcp-gateway/pkg/log"
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

		// Calculate wait duration and whether to trigger refresh
		var waitDuration time.Duration
		var shouldTriggerRefresh bool

		if status.NeedsRefresh {
			// Token needs refresh - check if expiry unchanged from last attempt
			expiryUnchanged := !p.lastRefreshExpiry.IsZero() && status.ExpiresAt.Equal(p.lastRefreshExpiry)

			if expiryUnchanged {
				// Expiry unchanged - increment retry count
				p.refreshRetryCount++
			} else {
				// Expiry changed or first attempt for this expiry - reset count
				if p.refreshRetryCount > 0 {
					log.Logf("- Token expiry updated for %s, resetting refresh count", p.name)
				}
				p.refreshRetryCount = 1
			}

			// Check if exceeded max attempts
			if p.refreshRetryCount > maxRefreshRetries {
				log.Logf("! Token expiry unchanged after %d refresh attempts for %s", maxRefreshRetries, p.name)
				return
			}

			// Exponential backoff for all refresh attempts: 30s, 1min, 2min, 4min, 8min...
			waitDuration = time.Duration(30*(1<<(p.refreshRetryCount-1))) * time.Second
			log.Logf("- Triggering token refresh for %s, attempt %d/%d, waiting %v",
				p.name, p.refreshRetryCount, maxRefreshRetries, waitDuration)

			p.lastRefreshExpiry = status.ExpiresAt
			shouldTriggerRefresh = true

		} else {
			// Token valid - wait until 10s before expiry
			timeUntilExpiry := time.Until(status.ExpiresAt)
			waitDuration = max(0, timeUntilExpiry-10*time.Second)

			log.Logf("- Token valid for %s, next check in %v", p.name, waitDuration.Round(time.Second))
			shouldTriggerRefresh = false
		}

		// Trigger refresh if needed (before waiting)
		if shouldTriggerRefresh {
			go func() {
				authClient := desktop.NewAuthClient()
				app, err := authClient.GetOAuthApp(context.Background(), p.name)
				if err != nil {
					log.Logf("! GetOAuthApp failed for %s: %v", p.name, err)
					return
				}

				if !app.Authorized {
					log.Logf("! GetOAuthApp returned Authorized=false for %s", p.name)
					return
				}
				// Authorized: SSE event will be handled below
			}()
		}

		// Common wait pattern - interruptible by SSE events
		if waitDuration > 0 {
			timer := time.NewTimer(waitDuration)

			select {
			case <-timer.C:
				// Wait complete - loop back to check token

			case event := <-p.eventChan:
				// SSE event arrived - handle it
				timer.Stop()
				log.Logf("- Provider %s received event: %s", p.name, event.Type)

				if err := p.reloadFn(ctx, p.name); err != nil {
					log.Logf("- Failed to reload %s after %s: %v", p.name, event.Type, err)
				}

				// Reset refresh state on successful event
				if event.Type == EventLoginSuccess || event.Type == EventTokenRefresh {
					p.refreshRetryCount = 0
					p.lastRefreshExpiry = time.Time{}
				}
				// Loop back to check fresh token

			case <-p.stopChan:
				timer.Stop()
				return

			case <-ctx.Done():
				timer.Stop()
				return
			}
		}
		// If waitDuration = 0, loops back immediately
	}
}

// Stop signals the provider to shutdown gracefully
// Safe to call multiple times - stopChan only closed once
func (p *Provider) Stop() {
	p.stopOnce.Do(func() {
		close(p.stopChan)
	})
}

// SendEvent sends an SSE event to this provider's event channel
func (p *Provider) SendEvent(event Event) {
	p.eventChan <- event
}
