package gateway

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/telemetry"
)

func TestPromptHandlerTelemetry(t *testing.T) {
	t.Run("records prompt counter metrics", func(t *testing.T) {
		// Set up test telemetry
		spanRecorder := tracetest.NewSpanRecorder()
		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithSpanProcessor(spanRecorder),
		)
		otel.SetTracerProvider(tracerProvider)

		reader := sdkmetric.NewManualReader()
		meterProvider := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(reader),
		)
		otel.SetMeterProvider(meterProvider)

		// Initialize telemetry
		telemetry.Init()

		// Create test server config
		serverConfig := &catalog.ServerConfig{
			Name: "test-prompt-server",
			Spec: catalog.Server{
				Command: []string{"test"},
			},
		}

		// Test prompt name
		promptName := "test-prompt"

		// Record prompt call
		ctx := context.Background()
		telemetry.RecordPromptGet(ctx, promptName, serverConfig.Name, "test-client")

		// Collect metrics
		var rm metricdata.ResourceMetrics
		err := reader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Find prompt counter metric
		var foundCounter bool
		for _, sm := range rm.ScopeMetrics {
			for _, metric := range sm.Metrics {
				if metric.Name == "mcp.prompt.gets" {
					foundCounter = true

					// Check it's a counter (Sum)
					sum, ok := metric.Data.(metricdata.Sum[int64])
					assert.True(t, ok, "Expected Sum[int64] for counter")

					// Verify data points
					assert.Len(t, sum.DataPoints, 1)
					dp := sum.DataPoints[0]

					// Check counter value
					assert.Equal(t, int64(1), dp.Value)

					// Verify attributes
					attrs := dp.Attributes.ToSlice()
					assert.Contains(t, attrs,
						attribute.String("mcp.prompt.name", promptName))
					assert.Contains(t, attrs,
						attribute.String("mcp.server.origin", serverConfig.Name))
					assert.Contains(t, attrs,
						attribute.String("mcp.client.name", "test-client"))
				}
			}
		}
		assert.True(t, foundCounter, "mcp.prompt.gets metric not found")
	})

	t.Run("records prompt duration histogram", func(t *testing.T) {
		// Set up test telemetry
		reader := sdkmetric.NewManualReader()
		meterProvider := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(reader),
		)
		otel.SetMeterProvider(meterProvider)

		// Initialize telemetry
		telemetry.Init()

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

		// Record prompt duration
		ctx := context.Background()
		telemetry.RecordPromptDuration(ctx, promptName, serverConfig.Name, duration, "test-client")

		// Collect metrics
		var rm metricdata.ResourceMetrics
		err := reader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Find prompt duration metric
		var foundHistogram bool
		for _, sm := range rm.ScopeMetrics {
			for _, metric := range sm.Metrics {
				if metric.Name == "mcp.prompt.duration" {
					foundHistogram = true

					// Check it's a histogram
					hist, ok := metric.Data.(metricdata.Histogram[float64])
					assert.True(t, ok, "Expected Histogram[float64] for duration")

					// Verify data points
					assert.Len(t, hist.DataPoints, 1)
					dp := hist.DataPoints[0]

					// Check histogram has recorded value
					assert.Equal(t, uint64(1), dp.Count)
					assert.InEpsilon(t, duration, dp.Sum, 0.01)

					// Verify attributes
					attrs := dp.Attributes.ToSlice()
					assert.Contains(t, attrs,
						attribute.String("mcp.prompt.name", promptName))
					assert.Contains(t, attrs,
						attribute.String("mcp.server.origin", serverConfig.Name))
					assert.Contains(t, attrs,
						attribute.String("mcp.client.name", "test-client"))
				}
			}
		}
		assert.True(t, foundHistogram, "mcp.prompt.duration metric not found")
	})

	t.Run("records prompt errors", func(t *testing.T) {
		// Set up test telemetry
		reader := sdkmetric.NewManualReader()
		meterProvider := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(reader),
		)
		otel.SetMeterProvider(meterProvider)

		// Initialize telemetry
		telemetry.Init()

		// Test error recording
		ctx := context.Background()
		promptName := "failing-prompt"
		serverName := "error-server"
		errorType := "prompt_not_found"

		telemetry.RecordPromptError(ctx, promptName, serverName, errorType)

		// Collect metrics
		var rm metricdata.ResourceMetrics
		err := reader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Find error counter metric
		var foundErrorCounter bool
		for _, sm := range rm.ScopeMetrics {
			for _, metric := range sm.Metrics {
				if metric.Name == "mcp.prompt.errors" {
					foundErrorCounter = true

					// Check it's a counter
					sum, ok := metric.Data.(metricdata.Sum[int64])
					assert.True(t, ok, "Expected Sum[int64] for error counter")

					// Verify data points
					assert.Len(t, sum.DataPoints, 1)
					dp := sum.DataPoints[0]

					// Check counter value
					assert.Equal(t, int64(1), dp.Value)

					// Verify attributes
					attrs := dp.Attributes.ToSlice()
					assert.Contains(t, attrs,
						attribute.String("mcp.prompt.name", promptName))
					assert.Contains(t, attrs,
						attribute.String("mcp.server.origin", serverName))
					assert.Contains(t, attrs,
						attribute.String("mcp.error.type", errorType))
				}
			}
		}
		assert.True(t, foundErrorCounter, "mcp.prompt.errors metric not found")
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
		serverType := inferServerTransportType(serverConfig)

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
				serverType := inferServerTransportType(tc.config)
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
		// Set up test telemetry
		reader := sdkmetric.NewManualReader()
		meterProvider := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(reader),
		)
		otel.SetMeterProvider(meterProvider)

		// Initialize telemetry
		telemetry.Init()

		// Record prompt list
		ctx := context.Background()
		serverName := "prompt-list-server"
		promptCount := 5

		telemetry.RecordPromptList(ctx, serverName, promptCount)

		// Collect metrics
		var rm metricdata.ResourceMetrics
		err := reader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Find prompt list metric
		var foundGauge bool
		for _, sm := range rm.ScopeMetrics {
			for _, metric := range sm.Metrics {
				if metric.Name == "mcp.prompts.discovered" {
					foundGauge = true

					// Check it's a gauge
					gauge, ok := metric.Data.(metricdata.Gauge[int64])
					assert.True(t, ok, "Expected Gauge[int64] for prompts discovered")

					// Verify data points
					assert.Len(t, gauge.DataPoints, 1)
					dp := gauge.DataPoints[0]

					// Check gauge value
					assert.Equal(t, int64(promptCount), dp.Value)

					// Verify attributes
					attrs := dp.Attributes.ToSlice()
					assert.Contains(t, attrs,
						attribute.String("mcp.server.origin", serverName))
				}
			}
		}
		assert.True(t, foundGauge, "mcp.prompts.discovered metric not found")
	})
}
