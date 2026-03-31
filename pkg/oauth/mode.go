package oauth

import (
	"context"
	"os"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

// OAuthMode determines which credential storage backend to use for a server.
type OAuthMode int

const (
	// OAuthModeAuto auto-detects mode at runtime: IsCEMode() -> CE, else Desktop.
	// Used for backward compatibility when callers have not yet been updated
	// to pass an explicit mode.
	OAuthModeAuto OAuthMode = iota
	// OAuthModeDesktop reads/writes via Secrets Engine (Desktop catalog servers).
	OAuthModeDesktop
	// OAuthModeCE reads/writes via the system credential helper (CE standalone).
	OAuthModeCE
	// OAuthModeCommunity reads/writes via docker pass (Desktop community servers).
	OAuthModeCommunity
)

// DetermineOAuthMode returns the credential storage mode for a server.
//
//   - CE mode (no Desktop): OAuthModeCE
//   - Desktop + catalog server: OAuthModeDesktop
//   - Desktop + community server + McpGatewayOAuth flag ON: OAuthModeCommunity
//   - Desktop + community server + flag OFF/error: OAuthModeDesktop (fallback)
func DetermineOAuthMode(ctx context.Context, isCommunity bool) OAuthMode {
	return determineOAuthMode(ctx, IsCEMode(), isCommunity, desktop.CheckFeatureFlagIsEnabled)
}

// determineOAuthMode is the testable core. ceMode is pre-resolved so tests
// don't need to mock env/OS detection or the Desktop backend socket.
func determineOAuthMode(ctx context.Context, ceMode bool, isCommunity bool, checkFlag featureFlagChecker) OAuthMode {
	if ceMode {
		return OAuthModeCE
	}
	if isCommunity {
		enabled, err := checkFlag(ctx, "McpGatewayOAuth")
		if err == nil && enabled {
			return OAuthModeCommunity
		}
	}
	return OAuthModeDesktop
}

// IsCEMode returns true if running in Docker CE mode (standalone OAuth flows).
// When false, uses Docker Desktop for OAuth orchestration.
//
// This uses the same logic as the feature flag system (features.IsRunningInDockerDesktop):
// - Container mode → CE mode (skip Desktop)
// - Windows/macOS → assume Docker Desktop (not CE mode)
// - Linux → check if Docker Desktop is running
//
// Set DOCKER_MCP_USE_CE=true to force CE mode.
func IsCEMode() bool {
	// Allow explicit override via environment variable
	if os.Getenv("DOCKER_MCP_USE_CE") == "true" {
		return true
	}

	// Use the same logic as feature flags
	// IsCEMode is the inverse of IsRunningInDockerDesktop
	return !desktop.IsRunningInDockerDesktop(context.Background())
}

// featureFlagChecker abstracts feature flag queries for testing.
type featureFlagChecker func(ctx context.Context, featureName string) (bool, error)

// ShouldUseGatewayOAuth returns true when the Gateway should own the OAuth
// lifecycle for a server (localhost callback, PKCE, token storage via
// credential helper or docker pass).
//
// Decision logic:
//   - CE mode (no Desktop): always true
//   - Desktop + catalog server (isCommunity=false): false (Desktop owns OAuth)
//   - Desktop + community server + McpGatewayOAuth flag ON: true
//   - Desktop + community server + McpGatewayOAuth flag OFF or error: false
//
// IsCEMode() remains the global decision for the notification monitor
// (pkg/gateway/run.go). This function is the per-server decision that later
// tickets (MCPT-482 through MCPT-486) will wire into call sites.
func ShouldUseGatewayOAuth(ctx context.Context, isCommunity bool) bool {
	return shouldUseGatewayOAuth(ctx, IsCEMode(), isCommunity, desktop.CheckFeatureFlagIsEnabled)
}

// shouldUseGatewayOAuth is the testable core. ceMode is pre-resolved so tests
// don't need to mock env/OS detection or the Desktop backend socket.
func shouldUseGatewayOAuth(ctx context.Context, ceMode bool, isCommunity bool, checkFlag featureFlagChecker) bool {
	if ceMode {
		return true
	}

	// Desktop mode: catalog servers continue to use Desktop OAuth.
	if !isCommunity {
		return false
	}

	// Desktop mode + community server: gate on the Unleash feature flag
	// exposed by the Desktop backend. If the flag is not registered yet
	// (MCPT-480 not deployed) or the backend is unreachable, treat as
	// disabled -- callers fall back to Desktop OAuth.
	enabled, err := checkFlag(ctx, "McpGatewayOAuth")
	if err != nil {
		return false
	}
	return enabled
}
