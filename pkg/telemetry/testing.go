package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/docker/mcp-gateway/pkg/plugins"
	telemetryserver "github.com/docker/mcp-gateway/telemetry-server"
)

// TestTelemetryServer holds the test telemetry server and metric reader for verification.
type TestTelemetryServer struct {
	server       *telemetryserver.Server
	MetricReader *sdkmetric.ManualReader
	cleanup      func()
}

// Cleanup shuts down the test telemetry server and resets state.
func (ts *TestTelemetryServer) Cleanup() {
	if ts.cleanup != nil {
		ts.cleanup()
	}
}

// SetupTestTelemetryServer starts a telemetry MCP server for testing
// and initializes the MCP client to connect to it.
// Returns a TestTelemetryServer with a MetricReader for verifying metrics.
// Call Cleanup() when done to shut down the server.
func SetupTestTelemetryServer(t *testing.T) *TestTelemetryServer {
	t.Helper()

	// Reset any previous state
	ResetForTesting()

	// Set up a meter provider with a manual reader for the telemetry server
	// This must be done BEFORE creating the telemetry server so it uses our meter
	metricReader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(metricReader),
	)
	otel.SetMeterProvider(meterProvider)

	// Initialize local telemetry (OpenTelemetry meters, etc.)
	Init()

	// Start the telemetry server on a random port
	// The server will use the meter provider we just set up
	server := telemetryserver.NewServer(0)
	ctx := context.Background()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start telemetry server: %v", err)
	}

	port := server.Port()

	// Initialize the MCP client to connect to the server
	if err := InitMCPClient(ctx, "127.0.0.1", port); err != nil {
		_ = server.Stop()
		t.Fatalf("Failed to initialize MCP client: %v", err)
	}

	// Return test server with cleanup function
	return &TestTelemetryServer{
		server:       server,
		MetricReader: metricReader,
		cleanup: func() {
			_ = CloseMCPClient()
			_ = server.Stop()
			plugins.Global().ResetForTesting()
		},
	}
}
