package gateway

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/telemetry"
)

// setupTestTelemetry sets up in-memory OTEL exporters for testing
func setupTestTelemetry(_ *testing.T) (*tracetest.SpanRecorder, *sdkmetric.ManualReader) {
	// Set up tracing
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(
		trace.WithSpanProcessor(spanRecorder),
	)
	otel.SetTracerProvider(tracerProvider)

	// Set up metrics
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
	)
	otel.SetMeterProvider(meterProvider)

	// Initialize telemetry package
	telemetry.Init()

	return spanRecorder, reader
}

// TestInferServerType tests the inferServerType helper function
func TestInferServerType(t *testing.T) {
	tests := []struct {
		name         string
		serverConfig *catalog.ServerConfig
		expected     string
	}{
		{
			name: "SSE endpoint",
			serverConfig: &catalog.ServerConfig{
				Spec: catalog.Server{
					Remote: catalog.Remote{
						URL:       "http://example.com/sse",
						Transport: "sse",
					},
				},
			},
			expected: "sse",
		},
		{
			name: "Remote URL with SSE transport",
			serverConfig: &catalog.ServerConfig{
				Spec: catalog.Server{
					Remote: catalog.Remote{
						URL:       "http://example.com/remote",
						Transport: "sse",
					},
				},
			},
			expected: "sse",
		},
		{
			name: "HTTP streaming transport",
			serverConfig: &catalog.ServerConfig{
				Spec: catalog.Server{
					Remote: catalog.Remote{
						URL:       "http://example.com/streaming",
						Transport: "http",
					},
				},
			},
			expected: "streaming",
		},
		{
			name: "Docker image",
			serverConfig: &catalog.ServerConfig{
				Spec: catalog.Server{
					Image: "docker.io/example/image",
				},
			},
			expected: "docker",
		},
		{
			name: "Stdio command",
			serverConfig: &catalog.ServerConfig{
				Spec: catalog.Server{
					Command: []string{"node", "server.js"},
				},
			},
			expected: "unknown", // Command field doesn't determine type in new implementation
		},
		{
			name: "Unknown type",
			serverConfig: &catalog.ServerConfig{
				Spec: catalog.Server{},
			},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inferServerType(tt.serverConfig)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestTelemetrySpanCreation tests that the telemetry package can create spans
func TestTelemetrySpanCreation(t *testing.T) {
	spanRecorder, _ := setupTestTelemetry(t)

	// Simulate what the handler would do
	ctx := t.Context()
	serverConfig := &catalog.ServerConfig{
		Name: "test-server",
		Spec: catalog.Server{
			Image: "test-image",
		},
	}

	// Create a span as the handler would
	_, span := telemetry.StartToolCallSpan(ctx, "test-tool",
		attribute.String("mcp.server.name", serverConfig.Name),
		attribute.String("mcp.server.type", inferServerType(serverConfig)),
	)

	// Add attributes as the handler would
	if serverConfig.Spec.Image != "" {
		span.SetAttributes(attribute.String("mcp.server.image", serverConfig.Spec.Image))
	}

	// End the span before checking
	span.End()

	// Check the span was created correctly
	spans := spanRecorder.Ended()
	require.Len(t, spans, 1)

	recordedSpan := spans[0]
	assert.Equal(t, "mcp.tool.call", recordedSpan.Name())

	// Check attributes
	attrs := recordedSpan.Attributes()
	assertAttribute(t, attrs, "mcp.tool.name", "test-tool")
	assertAttribute(t, attrs, "mcp.server.name", "test-server")
	assertAttribute(t, attrs, "mcp.server.type", "docker")
	assertAttribute(t, attrs, "mcp.server.image", "test-image")
}

// TestTelemetryMetricRecording tests that metrics are recorded correctly
func TestTelemetryMetricRecording(t *testing.T) {
	_, metricReader := setupTestTelemetry(t)

	ctx := t.Context()
	serverName := "test-server"
	serverType := "docker"
	toolName := "test-tool"
	clientName := "test-client"

	// Record metrics as the handler would
	telemetry.ToolCallCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("mcp.server.name", serverName),
			attribute.String("mcp.server.type", serverType),
			attribute.String("mcp.tool.name", toolName),
			attribute.String("mcp.client.name", clientName),
		),
	)

	duration := 150.5 // milliseconds
	telemetry.ToolCallDuration.Record(ctx, duration,
		metric.WithAttributes(
			attribute.String("mcp.server.name", serverName),
			attribute.String("mcp.server.type", serverType),
			attribute.String("mcp.tool.name", toolName),
			attribute.String("mcp.client.name", clientName),
		),
	)

	// Check metrics
	rm := &metricdata.ResourceMetrics{}
	err := metricReader.Collect(ctx, rm)
	require.NoError(t, err)

	var foundCounter, foundHistogram bool
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch m.Name {
			case "mcp.tool.calls":
				foundCounter = true
				data, ok := m.Data.(metricdata.Sum[int64])
				require.True(t, ok)
				require.Len(t, data.DataPoints, 1)
				assert.Equal(t, int64(1), data.DataPoints[0].Value)

				// Check attributes
				attrs := data.DataPoints[0].Attributes
				assertMetricAttribute(t, attrs, "mcp.server.name", serverName)
				assertMetricAttribute(t, attrs, "mcp.server.type", serverType)
				assertMetricAttribute(t, attrs, "mcp.tool.name", toolName)
				assertMetricAttribute(t, attrs, "mcp.client.name", clientName)

			case "mcp.tool.duration":
				foundHistogram = true
				data, ok := m.Data.(metricdata.Histogram[float64])
				require.True(t, ok)
				require.Len(t, data.DataPoints, 1)
				assert.InEpsilon(t, duration, data.DataPoints[0].Sum, 0.01)
			}
		}
	}

	assert.True(t, foundCounter, "Tool call counter not found")
	assert.True(t, foundHistogram, "Tool duration histogram not found")
}

// TestTelemetryErrorRecording tests error recording in telemetry
func TestTelemetryErrorRecording(t *testing.T) {
	spanRecorder, metricReader := setupTestTelemetry(t)

	ctx := t.Context()
	serverName := "error-server"
	serverType := "sse"
	toolName := "error-tool"

	// Start span
	ctx, span := telemetry.StartToolCallSpan(ctx, toolName,
		attribute.String("mcp.server.name", serverName),
		attribute.String("mcp.server.type", serverType),
	)

	// Record an error
	telemetry.RecordToolError(ctx, span, serverName, serverType, toolName)
	span.SetStatus(otelcodes.Error, "tool execution failed")
	span.End()

	// Check span recorded error
	spans := spanRecorder.Ended()
	require.Len(t, spans, 1)

	recordedSpan := spans[0]
	assert.Equal(t, "mcp.tool.call", recordedSpan.Name())
	assert.Equal(t, otelcodes.Error, recordedSpan.Status().Code)

	// Check error counter
	rm := &metricdata.ResourceMetrics{}
	err := metricReader.Collect(ctx, rm)
	require.NoError(t, err)

	var foundErrorCounter bool
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "mcp.tool.errors" {
				foundErrorCounter = true
				data, ok := m.Data.(metricdata.Sum[int64])
				require.True(t, ok)
				require.Len(t, data.DataPoints, 1)
				assert.Equal(t, int64(1), data.DataPoints[0].Value)
			}
		}
	}
	assert.True(t, foundErrorCounter, "Error counter not found")
}

// TestHandlerInstrumentationIntegration is a placeholder for full integration test
func TestHandlerInstrumentationIntegration(t *testing.T) {
	t.Skip("Full integration test will be enabled after handler instrumentation is complete")

	// This test will verify the complete flow once the handler is instrumented:
	// 1. Handler receives a tool call request
	// 2. Telemetry span is created with proper attributes
	// 3. Tool is executed via client pool
	// 4. Metrics are recorded (counter, duration)
	// 5. Span is closed with proper status
	// 6. Server lineage is preserved throughout
}

// TestToolCallDurationMeasurement tests that duration is measured correctly
func TestToolCallDurationMeasurement(t *testing.T) {
	_, metricReader := setupTestTelemetry(t)

	ctx := t.Context()
	startTime := time.Now()

	// Simulate some work
	time.Sleep(10 * time.Millisecond)

	// Record duration as the handler would
	duration := time.Since(startTime).Milliseconds()
	telemetry.ToolCallDuration.Record(ctx, float64(duration),
		metric.WithAttributes(
			attribute.String("mcp.server.name", "test-server"),
			attribute.String("mcp.server.type", "docker"),
			attribute.String("mcp.tool.name", "test-tool"),
			attribute.String("mcp.client.name", "test-client"),
		),
	)

	// Check the duration was recorded
	rm := &metricdata.ResourceMetrics{}
	err := metricReader.Collect(ctx, rm)
	require.NoError(t, err)

	var foundHistogram bool
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "mcp.tool.duration" {
				foundHistogram = true
				data, ok := m.Data.(metricdata.Histogram[float64])
				require.True(t, ok)
				require.Len(t, data.DataPoints, 1)
				// Duration should be at least 10ms
				assert.GreaterOrEqual(t, data.DataPoints[0].Sum, float64(10))
			}
		}
	}
	assert.True(t, foundHistogram, "Duration histogram not found")
}

// Helper function to assert span attributes
func assertAttribute(t *testing.T, attrs []attribute.KeyValue, key string, expectedValue string) {
	t.Helper()
	for _, attr := range attrs {
		if string(attr.Key) == key {
			assert.Equal(t, expectedValue, attr.Value.AsString())
			return
		}
	}
	t.Errorf("Attribute %s not found", key)
}

// Helper function to assert metric attributes
func assertMetricAttribute(t *testing.T, set attribute.Set, key string, expectedValue string) {
	t.Helper()
	value, found := set.Value(attribute.Key(key))
	assert.True(t, found, "Attribute %s not found", key)
	assert.Equal(t, expectedValue, value.AsString())
}
