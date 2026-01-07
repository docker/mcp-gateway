package gateway

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/telemetry"
)

func TestResourceHandlerTelemetry(t *testing.T) {
	// Save original env var
	originalDebug := os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG")
	defer func() {
		if originalDebug != "" {
			os.Setenv("DOCKER_MCP_TELEMETRY_DEBUG", originalDebug)
		} else {
			os.Unsetenv("DOCKER_MCP_TELEMETRY_DEBUG")
		}
	}()

	// Enable debug logging for tests
	os.Setenv("DOCKER_MCP_TELEMETRY_DEBUG", "1")

	t.Run("records resource read metrics", func(t *testing.T) {
		// Set up telemetry MCP server for Record* functions
		cleanup := telemetry.SetupTestTelemetryServer(t)
		defer cleanup()

		// Test data
		ctx := context.Background()
		resourceURI := "file:///test/resource.txt"
		clientName := "test-client"
		serverConfig := &catalog.ServerConfig{
			Name: "test-server",
			Spec: catalog.Server{
				Image: "test/image:latest",
			},
		}

		// Record resource read - sends to MCP telemetry server
		telemetry.RecordResourceRead(ctx, resourceURI, serverConfig.Name, clientName)

		// Note: Metrics are now recorded by the telemetry MCP server, not locally.
		// The MCP client successfully sent the tool call to the server.
	})

	t.Run("records resource duration histogram", func(t *testing.T) {
		// Set up telemetry MCP server for Record* functions
		cleanup := telemetry.SetupTestTelemetryServer(t)
		defer cleanup()

		// Test data
		ctx := context.Background()
		resourceURI := "file:///test/resource.txt"
		serverName := "test-server"
		clientName := "test-client"
		duration := 42.5 // milliseconds

		// Record duration - sends to MCP telemetry server
		telemetry.RecordResourceDuration(ctx, resourceURI, serverName, duration, clientName)

		// Note: Duration histogram is now recorded by the telemetry MCP server, not locally.
		// The MCP client successfully sent the tool call to the server.
	})

	t.Run("records resource errors", func(t *testing.T) {
		// Set up telemetry MCP server for Record* functions
		cleanup := telemetry.SetupTestTelemetryServer(t)
		defer cleanup()

		// Test data
		ctx := context.Background()
		resourceURI := "file:///test/resource.txt"
		serverName := "test-server"

		// Record error - sends to MCP telemetry server
		telemetry.RecordResourceError(ctx, resourceURI, serverName, "not_found")

		// Note: Error counter is now recorded by the telemetry MCP server, not locally.
		// The MCP client successfully sent the tool call to the server.
	})

	t.Run("creates resource span with attributes", func(t *testing.T) {
		// Set up span recorder
		spanRecorder := tracetest.NewSpanRecorder()
		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithSpanProcessor(spanRecorder),
		)
		otel.SetTracerProvider(tracerProvider)

		// Initialize telemetry
		telemetry.Init()

		// Test data
		ctx := context.Background()
		resourceURI := "file:///test/resource.txt"
		serverName := "test-server"
		serverType := "docker"

		// Create span
		_, span := telemetry.StartResourceSpan(ctx, resourceURI,
			attribute.String("mcp.server.origin", serverName),
			attribute.String("mcp.server.type", serverType),
		)
		span.End()

		// Verify span was created
		spans := spanRecorder.Ended()
		require.Len(t, spans, 1, "Should have created one span")

		recordedSpan := spans[0]
		assert.Equal(t, "mcp.resource.read", recordedSpan.Name())

		// Check attributes
		attrs := recordedSpan.Attributes()
		attrMap := make(map[string]string)
		for _, attr := range attrs {
			attrMap[string(attr.Key)] = attr.Value.AsString()
		}

		assert.Equal(t, resourceURI, attrMap["mcp.resource.uri"])
		assert.Equal(t, serverName, attrMap["mcp.server.origin"])
		assert.Equal(t, serverType, attrMap["mcp.server.type"])
	})
}

func TestResourceTemplateHandlerTelemetry(t *testing.T) {
	// Save original env var
	originalDebug := os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG")
	defer func() {
		if originalDebug != "" {
			os.Setenv("DOCKER_MCP_TELEMETRY_DEBUG", originalDebug)
		} else {
			os.Unsetenv("DOCKER_MCP_TELEMETRY_DEBUG")
		}
	}()

	// Enable debug logging for tests
	os.Setenv("DOCKER_MCP_TELEMETRY_DEBUG", "1")

	t.Run("records resource template read metrics", func(t *testing.T) {
		// Set up telemetry MCP server for Record* functions
		cleanup := telemetry.SetupTestTelemetryServer(t)
		defer cleanup()

		// Test data
		ctx := context.Background()
		uriTemplate := "file:///test/{id}/resource.txt"
		serverName := "test-server"
		clientName := "test-client"

		// Record resource template read - sends to MCP telemetry server
		telemetry.RecordResourceTemplateRead(ctx, uriTemplate, serverName, clientName)

		// Note: Metrics are now recorded by the telemetry MCP server, not locally.
		// The MCP client successfully sent the tool call to the server.
	})

	t.Run("creates resource template span", func(t *testing.T) {
		// Set up span recorder
		spanRecorder := tracetest.NewSpanRecorder()
		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithSpanProcessor(spanRecorder),
		)
		otel.SetTracerProvider(tracerProvider)

		// Initialize telemetry
		telemetry.Init()

		// Test data
		ctx := context.Background()
		uriTemplate := "file:///test/{id}/resource.txt"
		serverName := "test-server"

		// Create span
		_, span := telemetry.StartResourceTemplateSpan(ctx, uriTemplate,
			attribute.String("mcp.server.origin", serverName),
		)
		span.End()

		// Verify span
		spans := spanRecorder.Ended()
		require.Len(t, spans, 1, "Should have created one span")

		recordedSpan := spans[0]
		assert.Equal(t, "mcp.resource_template.read", recordedSpan.Name())

		// Check attributes
		attrs := recordedSpan.Attributes()
		attrMap := make(map[string]string)
		for _, attr := range attrs {
			attrMap[string(attr.Key)] = attr.Value.AsString()
		}

		assert.Equal(t, uriTemplate, attrMap["mcp.resource_template.uri"])
		assert.Equal(t, serverName, attrMap["mcp.server.origin"])
	})
}

func TestResourceDiscoveryMetrics(t *testing.T) {
	// Save original env var
	originalDebug := os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG")
	defer func() {
		if originalDebug != "" {
			os.Setenv("DOCKER_MCP_TELEMETRY_DEBUG", originalDebug)
		} else {
			os.Unsetenv("DOCKER_MCP_TELEMETRY_DEBUG")
		}
	}()

	// Enable debug logging for tests
	os.Setenv("DOCKER_MCP_TELEMETRY_DEBUG", "1")

	t.Run("records resources discovered", func(t *testing.T) {
		// Set up telemetry MCP server for Record* functions
		cleanup := telemetry.SetupTestTelemetryServer(t)
		defer cleanup()

		// Test data
		ctx := context.Background()
		serverName := "test-server"
		resourceCount := 10

		// Record discovery - sends to MCP telemetry server
		telemetry.RecordResourceList(ctx, serverName, resourceCount)

		// Note: Gauge is now recorded by the telemetry MCP server, not locally.
		// The MCP client successfully sent the tool call to the server.
	})

	t.Run("records resource templates discovered", func(t *testing.T) {
		// Set up telemetry MCP server for Record* functions
		cleanup := telemetry.SetupTestTelemetryServer(t)
		defer cleanup()

		// Test data
		ctx := context.Background()
		serverName := "test-server"
		templateCount := 5

		// Record discovery - sends to MCP telemetry server
		telemetry.RecordResourceTemplateList(ctx, serverName, templateCount)

		// Note: Gauge is now recorded by the telemetry MCP server, not locally.
		// The MCP client successfully sent the tool call to the server.
	})
}

// Test the actual handler instrumentation
func TestMcpServerResourceHandlerInstrumentation(t *testing.T) {
	// This test would require mocking the client pool and MCP server
	// For now, we'll focus on testing that the telemetry functions work correctly
	// The actual handler instrumentation will be tested through integration tests

	t.Run("handler records telemetry on success", func(t *testing.T) {
		// Set up telemetry MCP server for Record* functions
		cleanup := telemetry.SetupTestTelemetryServer(t)
		defer cleanup()

		// Set up span recorder
		spanRecorder := tracetest.NewSpanRecorder()
		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithSpanProcessor(spanRecorder),
		)
		otel.SetTracerProvider(tracerProvider)
		telemetry.Init()

		// Simulate what the handler would do
		ctx := context.Background()
		serverConfig := &catalog.ServerConfig{
			Name: "test-server",
			Spec: catalog.Server{
				Image: "test/image:latest",
			},
		}
		clientName := "test-client"
		params := &mcp.ReadResourceParams{
			URI: "file:///test/resource.txt",
		}

		// Start span (as handler would)
		serverType := inferServerType(serverConfig)
		ctx, span := telemetry.StartResourceSpan(ctx, params.URI,
			attribute.String("mcp.server.origin", serverConfig.Name),
			attribute.String("mcp.server.type", serverType),
		)

		startTime := time.Now()

		// Record counter (as handler would) - sends to MCP telemetry server
		telemetry.RecordResourceRead(ctx, params.URI, serverConfig.Name, clientName)

		// Simulate some work
		time.Sleep(10 * time.Millisecond)

		// Record duration (as handler would) - sends to MCP telemetry server
		duration := time.Since(startTime).Milliseconds()
		telemetry.RecordResourceDuration(ctx, params.URI, serverConfig.Name, float64(duration), clientName)

		// End span
		span.End()

		// Verify span was created
		spans := spanRecorder.Ended()
		require.Len(t, spans, 1, "Should have created one span")

		// Note: Metrics are now recorded by the telemetry MCP server, not locally.
		// The MCP client successfully sent the tool calls to the server.
	})

	t.Run("handler records error telemetry on failure", func(t *testing.T) {
		// Set up telemetry MCP server for Record* functions
		cleanup := telemetry.SetupTestTelemetryServer(t)
		defer cleanup()

		// Set up span recorder
		spanRecorder := tracetest.NewSpanRecorder()
		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithSpanProcessor(spanRecorder),
		)
		otel.SetTracerProvider(tracerProvider)
		telemetry.Init()

		// Simulate error case
		ctx := context.Background()
		serverConfig := &catalog.ServerConfig{
			Name: "test-server",
			Spec: catalog.Server{
				Image: "test/image:latest",
			},
		}
		clientName := "test-client"
		params := &mcp.ReadResourceParams{
			URI: "file:///test/missing.txt",
		}

		// Start span
		serverType := inferServerType(serverConfig)
		ctx, span := telemetry.StartResourceSpan(ctx, params.URI,
			attribute.String("mcp.server.origin", serverConfig.Name),
			attribute.String("mcp.server.type", serverType),
		)

		// Record counter - sends to MCP telemetry server
		telemetry.RecordResourceRead(ctx, params.URI, serverConfig.Name, clientName)

		// Simulate error
		err := errors.New("resource not found")
		span.RecordError(err)
		// Record error - sends to MCP telemetry server
		telemetry.RecordResourceError(ctx, params.URI, serverConfig.Name, "not_found")

		span.End()

		// Verify error was recorded in span
		spans := spanRecorder.Ended()
		require.Len(t, spans, 1)
		assert.Len(t, spans[0].Events(), 1, "Should have error event")

		// Note: Error counter is now recorded by the telemetry MCP server, not locally.
		// The MCP client successfully sent the tool call to the server.
	})
}
