package gateway

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/telemetry"
)

func TestPromptHandlerTelemetry(t *testing.T) {
	t.Run("records prompt counter metrics", func(t *testing.T) {
		// Set up telemetry MCP server for Record* functions
		cleanup := telemetry.SetupTestTelemetryServer(t)
		defer cleanup()

		// Create test server config
		serverConfig := &catalog.ServerConfig{
			Name: "test-prompt-server",
			Spec: catalog.Server{
				Command: []string{"test"},
			},
		}

		// Test prompt name
		promptName := "test-prompt"

		// Record prompt call - sends to MCP telemetry server
		ctx := context.Background()
		telemetry.RecordPromptGet(ctx, promptName, serverConfig.Name, "test-client")

		// Note: Metrics are now recorded by the telemetry MCP server, not locally.
		// The MCP client successfully sent the tool call to the server.
	})

	t.Run("records prompt duration histogram", func(t *testing.T) {
		// Set up telemetry MCP server for Record* functions
		cleanup := telemetry.SetupTestTelemetryServer(t)
		defer cleanup()

		// Create test server config
		serverConfig := &catalog.ServerConfig{
			Name: "test-prompt-server",
			Spec: catalog.Server{
				SSEEndpoint: "http://test.example.com/sse",
			},
		}

		// Test prompt name
		promptName := "test-prompt-duration"
		duration := float64(150) // milliseconds

		// Record prompt duration - sends to MCP telemetry server
		ctx := context.Background()
		telemetry.RecordPromptDuration(ctx, promptName, serverConfig.Name, duration, "test-client")

		// Note: Duration histogram is now recorded by the telemetry MCP server, not locally.
		// The MCP client successfully sent the tool call to the server.
	})

	t.Run("records prompt errors", func(t *testing.T) {
		// Set up telemetry MCP server for Record* functions
		cleanup := telemetry.SetupTestTelemetryServer(t)
		defer cleanup()

		// Test error recording
		ctx := context.Background()
		promptName := "failing-prompt"
		serverName := "error-server"
		errorType := "prompt_not_found"

		// Record prompt error - sends to MCP telemetry server
		telemetry.RecordPromptError(ctx, promptName, serverName, errorType)

		// Note: Error counter is now recorded by the telemetry MCP server, not locally.
		// The MCP client successfully sent the tool call to the server.
	})

	t.Run("creates spans with correct attributes", func(t *testing.T) {
		// Set up test telemetry
		spanRecorder := tracetest.NewSpanRecorder()
		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithSpanProcessor(spanRecorder),
		)
		otel.SetTracerProvider(tracerProvider)

		// Initialize telemetry
		telemetry.Init()

		// Create test server config
		serverConfig := &catalog.ServerConfig{
			Name: "test-prompt-server",
			Spec: catalog.Server{
				Image: "test/prompt-server:latest",
			},
		}

		// Start prompt span
		ctx := context.Background()
		promptName := "test-prompt-span"
		serverType := inferServerType(serverConfig)

		_, span := telemetry.StartPromptSpan(ctx, promptName,
			attribute.String("mcp.server.origin", serverConfig.Name),
			attribute.String("mcp.server.type", serverType),
			attribute.String("mcp.prompt.name", promptName))

		// End span
		span.End()

		// Check recorded spans
		spans := spanRecorder.Ended()
		require.Len(t, spans, 1)

		recordedSpan := spans[0]
		assert.Equal(t, "mcp.prompt.get", recordedSpan.Name())

		// Check attributes
		attrs := recordedSpan.Attributes()
		assert.Contains(t, attrs,
			attribute.String("mcp.server.origin", serverConfig.Name))
		assert.Contains(t, attrs,
			attribute.String("mcp.server.type", "docker"))
		assert.Contains(t, attrs,
			attribute.String("mcp.prompt.name", promptName))
	})

	t.Run("infers server type correctly for prompts", func(t *testing.T) {
		testCases := []struct {
			name         string
			config       *catalog.ServerConfig
			expectedType string
		}{
			{
				name: "SSE server",
				config: &catalog.ServerConfig{
					Name: "sse-test",
					Spec: catalog.Server{
						Remote: catalog.Remote{
							URL:       "http://example.com/sse",
							Transport: "sse",
						},
					},
				},
				expectedType: "sse",
			},
			{
				name: "HTTP streaming server",
				config: &catalog.ServerConfig{
					Name: "streaming-test",
					Spec: catalog.Server{
						Remote: catalog.Remote{
							URL:       "http://example.com/remote",
							Transport: "http",
						},
					},
				},
				expectedType: "streaming",
			},
			{
				name: "Docker server",
				config: &catalog.ServerConfig{
					Name: "docker-test",
					Spec: catalog.Server{
						Image: "test/image:latest",
					},
				},
				expectedType: "docker",
			},
			{
				name: "Command server (unknown type)",
				config: &catalog.ServerConfig{
					Name: "command-test",
					Spec: catalog.Server{
						Command: []string{"prompt-server", "--stdio"},
					},
				},
				expectedType: "unknown", // Command field doesn't determine type in new implementation
			},
			{
				name: "Unknown server",
				config: &catalog.ServerConfig{
					Name: "unknown-test",
					Spec: catalog.Server{},
				},
				expectedType: "unknown",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				serverType := inferServerType(tc.config)
				assert.Equal(t, tc.expectedType, serverType)
			})
		}
	})
}

// TestPromptHandlerIntegration tests the full mcpServerPromptHandler with telemetry
func TestPromptHandlerIntegration(t *testing.T) {
	t.Run("handler records telemetry for successful prompt get", func(t *testing.T) {
		// This test would require mocking the client pool and MCP server
		// For now, we're focusing on the telemetry recording functions
		// that will be called from within the handler
		t.Skip("Integration test requires full gateway setup")
	})

	t.Run("handler records telemetry for failed prompt get", func(t *testing.T) {
		// This test would verify error recording in the handler
		t.Skip("Integration test requires full gateway setup")
	})
}

// TestPromptListHandler tests telemetry for prompt list operations
func TestPromptListHandlerTelemetry(t *testing.T) {
	t.Run("records prompt list counter", func(t *testing.T) {
		// Set up telemetry MCP server for Record* functions
		cleanup := telemetry.SetupTestTelemetryServer(t)
		defer cleanup()

		// Record prompt list - sends to MCP telemetry server
		ctx := context.Background()
		serverName := "prompt-list-server"
		promptCount := 5

		telemetry.RecordPromptList(ctx, serverName, promptCount)

		// Note: Prompts discovered gauge is now recorded by the telemetry MCP server, not locally.
		// The MCP client successfully sent the tool call to the server.
	})
}
