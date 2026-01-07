package telemetry

import (
	"context"
	"testing"

	telemetryserver "github.com/docker/mcp-gateway/telemetry-server"
)

// TestServer holds the test telemetry server and provides cleanup.
type TestServer struct {
	server *telemetryserver.Server
	t      *testing.T
}

// SetupTestTelemetryServer starts a telemetry MCP server for testing
// and initializes the MCP client to connect to it.
// Call cleanup() when done to shut down the server.
func SetupTestTelemetryServer(t *testing.T) (cleanup func()) {
	t.Helper()

	// Reset any previous state
	ResetForTesting()

	// Initialize local telemetry (OpenTelemetry meters, etc.)
	Init()

	// Start the telemetry server on a random port
	server := telemetryserver.NewServer(0)
	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start telemetry server: %v", err)
	}

	port := server.Port()

	// Initialize the MCP client to connect to the server
	if err := InitMCPClient(ctx, "127.0.0.1", port); err != nil {
		server.Stop()
		t.Fatalf("Failed to initialize MCP client: %v", err)
	}

	// Return cleanup function
	return func() {
		CloseMCPClient()
		server.Stop()
		ResetForTesting()
	}
}
