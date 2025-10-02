package oauth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

// RefreshCoordinator manages OAuth token refresh for multiple providers
type RefreshCoordinator struct {
	mu         sync.RWMutex
	refreshing map[string]bool // serverName → is refresh currently in progress?
	credHelper *CredentialHelper
}

// NewRefreshCoordinator creates a new RefreshCoordinator
func NewRefreshCoordinator() *RefreshCoordinator {
	return &RefreshCoordinator{
		refreshing: make(map[string]bool),
		credHelper: NewOAuthCredentialHelper(),
	}
}

// EnsureValidToken checks token validity and triggers refresh if needed
// This is called by background refresh loops (one per OAuth server)
// Returns nil immediately - refresh happens asynchronously
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

	// Check if refresh is already in progress for this server
	if c.isRefreshing(serverName) {
		logf("- Refresh already in progress for %s, skipping this check", serverName)
		return nil // Will retry on next background loop tick
	}

	// Token needs refresh - mark as in progress and trigger
	logf("- Token needs refresh for %s (expires: %s)", serverName, status.ExpiresAt.Format(time.RFC3339))
	c.setRefreshing(serverName, true)

	// Trigger refresh asynchronously
	go func() {
		authClient := desktop.NewAuthClient()
		_, err := authClient.GetOAuthApp(context.Background(), serverName)
		if err != nil {
			// GetOAuthApp failed - no SSE event will come, clear flag now
			logf("- GetOAuthApp failed for %s: %v", serverName, err)
			c.setRefreshing(serverName, false)
			return
		}
		// Success: GetOAuthApp triggers DD refresh → SSE event → handleOAuthEvent → reload → MarkRefreshComplete
		// Don't clear flag here - wait for handleOAuthEvent to call MarkRefreshComplete after reload
	}()

	return nil
}

// MarkRefreshComplete marks a refresh as complete for a server
// This should be called by handleOAuthEvent after reload completes (success or failure)
func (c *RefreshCoordinator) MarkRefreshComplete(serverName string) {
	c.setRefreshing(serverName, false)
	logf("- Refresh marked complete for %s", serverName)
}

// isRefreshing checks if a refresh is currently in progress for a server
func (c *RefreshCoordinator) isRefreshing(serverName string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.refreshing[serverName]
}

// setRefreshing sets the refresh status for a server
func (c *RefreshCoordinator) setRefreshing(serverName string, refreshing bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refreshing[serverName] = refreshing
}
