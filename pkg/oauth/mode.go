package oauth

import (
	"context"
	"os"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

// Mode determines which credential storage backend to use for a server.
type Mode int

const (
	// ModeAuto auto-detects mode at runtime: IsCEMode() -> CE, else Desktop.
	// Used for backward compatibility when callers have not yet been updated
	// to pass an explicit mode.
	ModeAuto Mode = iota
	// ModeDesktop reads/writes via Secrets Engine (Desktop catalog servers).
	ModeDesktop
	// ModeCE reads/writes via the system credential helper (CE standalone).
	ModeCE
	// ModeCommunity reads/writes via docker pass (Desktop community servers).
	ModeCommunity
)

// String returns a human-readable label for the mode.
func (m Mode) String() string {
	switch m {
	case ModeDesktop:
		return "Desktop"
	case ModeCE:
		return "CE"
	case ModeCommunity:
		return "Community"
	default:
		return "Auto"
	}
}

// DetermineMode returns the credential storage mode for a server.
//
//   - CE mode (no Desktop): ModeCE
//   - Desktop + catalog server: ModeDesktop
//   - Desktop + community server + McpGatewayOAuth flag ON: ModeCommunity
//   - Desktop + community server + flag OFF/error: ModeDesktop (fallback)
func DetermineMode(ctx context.Context, isCommunity bool) Mode {
	return determineMode(ctx, IsCEMode(), isCommunity, desktop.CheckFeatureFlagIsEnabled)
}

// determineMode is the testable core. ceMode is pre-resolved so tests
// don't need to mock env/OS detection or the Desktop backend socket.
func determineMode(ctx context.Context, ceMode bool, isCommunity bool, checkFlag featureFlagChecker) Mode {
	if ceMode {
		return ModeCE
	}
	if isCommunity {
		enabled, err := checkFlag(ctx, "McpGatewayOAuth")
		if err == nil && enabled {
			return ModeCommunity
		}
	}
	return ModeDesktop
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
// credential helper or docker pass). This is a convenience wrapper around
// DetermineMode -- Gateway owns OAuth for every mode except ModeDesktop.
//
// IsCEMode() remains the global decision for the notification monitor
// (pkg/gateway/run.go). This function is the per-server decision used by
// MCPT-483 through MCPT-486 call sites.
func ShouldUseGatewayOAuth(ctx context.Context, isCommunity bool) bool {
	return DetermineMode(ctx, isCommunity) != ModeDesktop
}
