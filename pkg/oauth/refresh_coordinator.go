package oauth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

// RefreshResult represents the result of a token refresh operation
type RefreshResult struct {
	Success bool
	Error   error
}

// RefreshState tracks the state of an ongoing refresh for a single provider
type RefreshState struct {
	mu                sync.Mutex
	refreshInProgress bool
	pendingRequests   []chan RefreshResult
}

// RefreshCoordinator manages OAuth token refresh coordination across multiple providers
type RefreshCoordinator struct {
	mu         sync.RWMutex
	providers  map[string]*RefreshState
	credHelper *CredentialHelper
}

// NewRefreshCoordinator creates a new RefreshCoordinator
func NewRefreshCoordinator() *RefreshCoordinator {
	return &RefreshCoordinator{
		providers:  make(map[string]*RefreshState),
		credHelper: NewOAuthCredentialHelper(),
	}
}

// getOrCreateState returns the RefreshState for a provider, creating if necessary
func (c *RefreshCoordinator) getOrCreateState(serverName string) *RefreshState {
	c.mu.Lock()
	defer c.mu.Unlock()

	if state, exists := c.providers[serverName]; exists {
		return state
	}

	state := &RefreshState{
		refreshInProgress: false,
		pendingRequests:   make([]chan RefreshResult, 0),
	}
	c.providers[serverName] = state
	return state
}

// EnsureValidToken checks token validity and coordinates refresh if needed
// Returns nil if token is valid or successfully refreshed, error otherwise
func (c *RefreshCoordinator) EnsureValidToken(ctx context.Context, serverName string) error {
	// Check token status
	status, err := c.credHelper.GetTokenStatus(ctx, serverName)
	if err != nil {
		logf("- Token status check failed for %s: %v", serverName, err)
		return fmt.Errorf("failed to check token status: %w", err)
	}

	// Token is valid and doesn't need refresh
	if status.Valid && !status.NeedsRefresh {
		logf("- Token valid for %s (expires: %s)", serverName, status.ExpiresAt.Format(time.RFC3339))
		return nil
	}

	// Token needs refresh - coordinate with other concurrent requests
	logf("- Token needs refresh for %s (expires: %s)", serverName, status.ExpiresAt.Format(time.RFC3339))
	state := c.getOrCreateState(serverName)

	state.mu.Lock()

	// Check if refresh is already in progress
	if state.refreshInProgress {
		// Join existing refresh - create buffered channel
		logf("- Request joining existing refresh for %s (follower)", serverName)
		waitCh := make(chan RefreshResult, 1)
		state.pendingRequests = append(state.pendingRequests, waitCh)
		state.mu.Unlock()

		// Wait for result with timeout
		select {
		case result := <-waitCh:
			if result.Success {
				return nil
			}
			return result.Error
		case <-time.After(5 * time.Second):
			return fmt.Errorf("timeout waiting for token refresh for %s", serverName)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// No refresh in progress - this request becomes the leader
	logf("- Request initiating token refresh for %s (leader)", serverName)
	state.refreshInProgress = true
	leaderWaitCh := make(chan RefreshResult, 1)
	state.pendingRequests = append(state.pendingRequests, leaderWaitCh)
	state.mu.Unlock()

	// Leader triggers the refresh in a goroutine
	go func() {
		authClient := desktop.NewAuthClient()
		_, err := authClient.GetOAuthApp(ctx, serverName)
		if err != nil {
			// Refresh failed - broadcast error to all waiters
			c.BroadcastResult(serverName, RefreshResult{
				Success: false,
				Error:   fmt.Errorf("OAuth token refresh failed for %s: %w", serverName, err),
			})
			return
		}
		// Success will be broadcast by handleOAuthEvent after reload completes
	}()

	// Leader also waits for result
	select {
	case result := <-leaderWaitCh:
		if result.Success {
			return nil
		}
		return result.Error
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout waiting for token refresh for %s", serverName)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// BroadcastResult sends refresh result to all waiting requests and resets state
// This should be called by handleOAuthEvent after successful reload, or by GetOAuthApp goroutine on error
func (c *RefreshCoordinator) BroadcastResult(serverName string, result RefreshResult) {
	state := c.getOrCreateState(serverName)

	state.mu.Lock()
	waiters := state.pendingRequests
	state.pendingRequests = make([]chan RefreshResult, 0)
	state.refreshInProgress = false
	state.mu.Unlock()

	// Broadcast to all waiters
	for _, ch := range waiters {
		select {
		case ch <- result:
			// Sent successfully
		default:
			// Channel full or waiter timed out - skip
		}
		close(ch)
	}

	if result.Success {
		logf("- OAuth token refresh successful for %s (%d requests completed)", serverName, len(waiters))
	} else {
		logf("- OAuth token refresh failed for %s: %v", serverName, result.Error)
	}
}
