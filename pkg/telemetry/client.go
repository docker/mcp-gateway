package telemetry

import (
	"context"

	"github.com/docker/mcp-gateway/pkg/plugin"
	mcpplugin "github.com/docker/mcp-gateway/pkg/plugin/mcp"
)

// InitMCPClient initializes the MCP telemetry plugin and registers it with the plugin registry.
// This creates an MCP adapter that connects to the telemetry server at the specified host and port.
func InitMCPClient(ctx context.Context, host string, port int) error {
	adapter, err := mcpplugin.NewTelemetryAdapter(ctx, host, port)
	if err != nil {
		return err
	}

	return plugin.Global().RegisterTelemetryPlugin(adapter)
}

// CloseMCPClient closes the MCP telemetry plugin.
func CloseMCPClient() error {
	return plugin.Global().UnregisterTelemetryPlugin()
}

// IsMCPClientInitialized returns true if the MCP telemetry plugin is registered.
// Deprecated: Use plugin.Global().HasTelemetryPlugin() instead.
func IsMCPClientInitialized() bool {
	return plugin.Global().HasTelemetryPlugin()
}

// ResetForTesting resets the plugin registry state for testing purposes.
// This should only be called from tests.
func ResetForTesting() {
	plugin.Global().ResetForTesting()
}
