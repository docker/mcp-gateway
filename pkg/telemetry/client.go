package telemetry

import (
	"context"

	"github.com/docker/mcp-gateway/pkg/plugins"
	mcpplugin "github.com/docker/mcp-gateway/pkg/plugins/mcp"
)

// InitMCPClient initializes the MCP telemetry plugin and registers it with the plugin registry.
// This creates an MCP adapter that connects to the telemetry server at the specified host and port.
func InitMCPClient(ctx context.Context, host string, port int) error {
	adapter, err := mcpplugin.NewTelemetryAdapter(ctx, host, port)
	if err != nil {
		return err
	}

	return plugins.Global().RegisterTelemetryPlugin(adapter)
}

// CloseMCPClient closes the MCP telemetry plugin.
func CloseMCPClient() error {
	return plugins.Global().UnregisterTelemetryPlugin()
}

// IsMCPClientInitialized returns true if the MCP telemetry plugin is registered.
// Deprecated: Use plugins.Global().HasTelemetryPlugin() instead.
func IsMCPClientInitialized() bool {
	return plugins.Global().HasTelemetryPlugin()
}

// ResetForTesting resets the plugin registry state for testing purposes.
// This should only be called from tests.
func ResetForTesting() {
	plugins.Global().ResetForTesting()
}
