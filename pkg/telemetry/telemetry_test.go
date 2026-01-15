package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// setupTestTelemetry creates test providers with in-memory exporters
func setupTestTelemetry(t *testing.T) (*tracetest.SpanRecorder, *sdkmetric.ManualReader) {
	t.Helper()
	// Create in-memory span recorder for traces
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(
		trace.WithSpanProcessor(spanRecorder),
	)
	otel.SetTracerProvider(tracerProvider)

	// Create manual reader for metrics
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
	)
	otel.SetMeterProvider(meterProvider)

	// Cleanup
	t.Cleanup(func() {
		// Reset to noop providers after test
		otel.SetTracerProvider(trace.NewTracerProvider())
		otel.SetMeterProvider(sdkmetric.NewMeterProvider())
	})

	return spanRecorder, reader
}

func TestInitialization(t *testing.T) {
	spanRecorder, metricReader := setupTestTelemetry(t)

	// Initialize the telemetry package
	Init()

	t.Run("tracer_initialized", func(t *testing.T) {
		// The tracer should be initialized from global provider
		assert.NotNil(t, tracer, "tracer should be initialized")

		// Create a test span to verify tracer works
		ctx := context.Background()
		_, span := tracer.Start(ctx, "test.span")
		span.End()

		// Verify span was recorded
		spans := spanRecorder.Ended()
		require.Len(t, spans, 1, "should have recorded one span")
		assert.Equal(t, "test.span", spans[0].Name())
	})

	t.Run("meter_initialized", func(t *testing.T) {
		// The meter should be initialized from global provider
		assert.NotNil(t, meter, "meter should be initialized")
	})

	t.Run("tool_call_counter_created", func(t *testing.T) {
		assert.NotNil(t, ToolCallCounter, "ToolCallCounter should be created")

		// Test recording a value
		ctx := context.Background()
		ToolCallCounter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("mcp.tool.name", "test_tool"),
				attribute.String("mcp.server.origin", "test_server"),
			))

		// Collect metrics
		var rm metricdata.ResourceMetrics
		err := metricReader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Find our counter in the metrics
		found := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "mcp.tool.calls" {
					found = true
					sum := m.Data.(metricdata.Sum[int64])
					assert.Equal(t, int64(1), sum.DataPoints[0].Value)

					// Check attributes
					attrs := sum.DataPoints[0].Attributes
					toolName, _ := attrs.Value(attribute.Key("mcp.tool.name"))
					assert.Equal(t, "test_tool", toolName.AsString())

					serverOrigin, _ := attrs.Value(attribute.Key("mcp.server.origin"))
					assert.Equal(t, "test_server", serverOrigin.AsString())
				}
			}
		}
		assert.True(t, found, "mcp.tool.calls metric should be recorded")
	})

	t.Run("tool_duration_histogram_created", func(t *testing.T) {
		assert.NotNil(t, ToolCallDuration, "ToolCallDuration should be created")

		// Test recording a value
		ctx := context.Background()
		ToolCallDuration.Record(ctx, 150.5,
			metric.WithAttributes(
				attribute.String("mcp.tool.name", "test_tool"),
				attribute.String("mcp.server.origin", "test_server"),
			))

		// Collect metrics
		var rm metricdata.ResourceMetrics
		err := metricReader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Find our histogram in the metrics
		found := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "mcp.tool.duration" {
					found = true
					hist := m.Data.(metricdata.Histogram[float64])
					assert.Positive(t, hist.DataPoints[0].Count)
					assert.InEpsilon(t, 150.5, hist.DataPoints[0].Sum, 0.01)
				}
			}
		}
		assert.True(t, found, "mcp.tool.duration metric should be recorded")
	})

	t.Run("tool_error_counter_created", func(t *testing.T) {
		assert.NotNil(t, ToolErrorCounter, "ToolErrorCounter should be created")

		// Test recording an error
		ctx := context.Background()
		ToolErrorCounter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("mcp.tool.name", "test_tool"),
				attribute.String("mcp.server.origin", "test_server"),
				attribute.String("error.type", "timeout"),
			))

		// Collect metrics
		var rm metricdata.ResourceMetrics
		err := metricReader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Find our counter in the metrics
		found := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "mcp.tool.errors" {
					found = true
					sum := m.Data.(metricdata.Sum[int64])
					assert.Equal(t, int64(1), sum.DataPoints[0].Value)

					// Check error type attribute
					attrs := sum.DataPoints[0].Attributes
					errorType, _ := attrs.Value(attribute.Key("error.type"))
					assert.Equal(t, "timeout", errorType.AsString())
				}
			}
		}
		assert.True(t, found, "mcp.tool.errors metric should be recorded")
	})
}

func TestStartInitializeSpan(t *testing.T) {
	spanRecorder, _ := setupTestTelemetry(t)
	Init()

	ctx := context.Background()
	clientName := "claude-ai"
	clientVersion := "1.0.0"

	// Start a tool call span
	newCtx, span := StartInitializeSpan(ctx,
		attribute.String("mcp.client.name", clientName),
		attribute.String("mcp.client.version", clientVersion),
	)

	// Verify context was updated
	assert.NotEqual(t, ctx, newCtx, "should return new context with span")

	// End the span
	span.End()

	// Verify span attributes
	spans := spanRecorder.Ended()
	require.Len(t, spans, 1)

	recordedSpan := spans[0]
	assert.Equal(t, "mcp.initialize", recordedSpan.Name())

	// Check attributes
	attrs := recordedSpan.Attributes()
	attrMap := make(map[string]string)
	for _, attr := range attrs {
		attrMap[string(attr.Key)] = attr.Value.AsString()
	}

	assert.Equal(t, clientName, attrMap["mcp.client.name"])
	assert.Equal(t, clientVersion, attrMap["mcp.client.version"])
}

func TestStartToolCallSpan(t *testing.T) {
	spanRecorder, _ := setupTestTelemetry(t)
	Init()

	ctx := context.Background()
	toolName := "docker_ps"
	serverName := "docker_server"
	serverType := "stdio"

	// Start a tool call span
	newCtx, span := StartToolCallSpan(ctx, toolName,
		attribute.String("mcp.server.origin", serverName),
		attribute.String("mcp.server.type", serverType),
	)

	// Verify context was updated
	assert.NotEqual(t, ctx, newCtx, "should return new context with span")

	// End the span
	span.End()

	// Verify span attributes
	spans := spanRecorder.Ended()
	require.Len(t, spans, 1)

	recordedSpan := spans[0]
	assert.Equal(t, "mcp.tool.call", recordedSpan.Name())

	// Check attributes
	attrs := recordedSpan.Attributes()
	attrMap := make(map[string]string)
	for _, attr := range attrs {
		attrMap[string(attr.Key)] = attr.Value.AsString()
	}

	assert.Equal(t, toolName, attrMap["mcp.tool.name"])
	assert.Equal(t, serverName, attrMap["mcp.server.origin"])
	assert.Equal(t, serverType, attrMap["mcp.server.type"])
}

func TestStartCommandSpan(t *testing.T) {
	spanRecorder, _ := setupTestTelemetry(t)
	Init()

	ctx := context.Background()
	commandPath := "docker mcp gateway run"

	// Start a command span
	newCtx, span := StartCommandSpan(ctx, commandPath)

	// Verify context was updated
	assert.NotEqual(t, ctx, newCtx, "should return new context with span")

	// End the span
	span.End()

	// Verify span attributes
	spans := spanRecorder.Ended()
	require.Len(t, spans, 1)

	recordedSpan := spans[0]
	assert.Equal(t, "mcp.command.docker mcp gateway run", recordedSpan.Name())

	// Check attributes
	attrs := recordedSpan.Attributes()
	attrMap := make(map[string]string)
	for _, attr := range attrs {
		attrMap[string(attr.Key)] = attr.Value.AsString()
	}

	assert.Equal(t, commandPath, attrMap["mcp.command.path"])
}

func TestRecordToolError(t *testing.T) {
	_, metricReader := setupTestTelemetry(t)
	Init()

	ctx := context.Background()
	toolName := "test_tool"
	serverName := "test_server"
	serverType := "docker"

	// Record a tool error (nil span is ok for testing)
	RecordToolError(ctx, nil, serverName, serverType, toolName)

	// Collect metrics
	var rm metricdata.ResourceMetrics
	err := metricReader.Collect(ctx, &rm)
	require.NoError(t, err)

	// Find the error counter
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "mcp.tool.errors" {
				found = true
				sum := m.Data.(metricdata.Sum[int64])
				assert.Equal(t, int64(1), sum.DataPoints[0].Value)

				// Check attributes
				attrs := sum.DataPoints[0].Attributes

				toolNameAttr, _ := attrs.Value(attribute.Key("mcp.tool.name"))
				assert.Equal(t, toolName, toolNameAttr.AsString())

				serverNameAttr, _ := attrs.Value(attribute.Key("mcp.server.name"))
				assert.Equal(t, serverName, serverNameAttr.AsString())

				serverTypeAttr, _ := attrs.Value(attribute.Key("mcp.server.type"))
				assert.Equal(t, serverType, serverTypeAttr.AsString())
			}
		}
	}
	assert.True(t, found, "tool error should be recorded")
}

func TestConcurrentMetricRecording(t *testing.T) {
	_, metricReader := setupTestTelemetry(t)
	Init()

	ctx := context.Background()

	// Simulate concurrent tool calls
	done := make(chan bool, 10)
	for i := range 10 {
		go func(_ int) {
			ToolCallCounter.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("mcp.tool.name", "concurrent_tool"),
					attribute.String("mcp.server.origin", "server"),
				))
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for range 10 {
		<-done
	}

	// Collect and verify
	var rm metricdata.ResourceMetrics
	err := metricReader.Collect(ctx, &rm)
	require.NoError(t, err)

	// Find our counter
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "mcp.tool.calls" {
				found = true
				sum := m.Data.(metricdata.Sum[int64])
				// Should have recorded all 10 calls
				assert.Equal(t, int64(10), sum.DataPoints[0].Value)
			}
		}
	}
	assert.True(t, found, "concurrent metrics should be recorded")
}

func TestRecordWorkingSetOperation(t *testing.T) {
	_, metricReader := setupTestTelemetry(t)
	Init()
	ctx := context.Background()

	// Test successful profile operation
	RecordWorkingSetOperation(ctx, "create", "test-profile", 123.45, true)

	// Collect and verify metrics
	var rm metricdata.ResourceMetrics
	err := metricReader.Collect(ctx, &rm)
	require.NoError(t, err)

	// Find and verify the counter metric
	foundCounter := false
	foundHistogram := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "mcp.profile.operations" {
				foundCounter = true
				sum := m.Data.(metricdata.Sum[int64])
				require.Len(t, sum.DataPoints, 1)
				assert.Equal(t, int64(1), sum.DataPoints[0].Value)

				// Verify attributes
				attrs := sum.DataPoints[0].Attributes
				assert.Contains(t, attrs.ToSlice(), attribute.String("mcp.profile.operation", "create"))
				assert.Contains(t, attrs.ToSlice(), attribute.String("mcp.profile.id", "test-profile"))
				assert.Contains(t, attrs.ToSlice(), attribute.Bool("mcp.profile.success", true))
			}
			if m.Name == "mcp.profile.operation.duration" {
				foundHistogram = true
				histogram := m.Data.(metricdata.Histogram[float64])
				require.Len(t, histogram.DataPoints, 1)
				assert.InDelta(t, float64(123.45), histogram.DataPoints[0].Sum, 0.001)
			}
		}
	}

	assert.True(t, foundCounter, "Profile operations counter metric not found")
	assert.True(t, foundHistogram, "Profile operation duration histogram not found")
}

func TestRecordWorkingSetServers(t *testing.T) {
	_, metricReader := setupTestTelemetry(t)
	Init()
	ctx := context.Background()

	// Record server count for a profile
	RecordWorkingSetServers(ctx, "test-profile", 5)

	// Collect and verify metrics
	var rm metricdata.ResourceMetrics
	err := metricReader.Collect(ctx, &rm)
	require.NoError(t, err)

	// Find and verify the gauge metric
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "mcp.profile.servers" {
				found = true
				gauge := m.Data.(metricdata.Gauge[int64])
				require.Len(t, gauge.DataPoints, 1)
				assert.Equal(t, int64(5), gauge.DataPoints[0].Value)

				// Verify attributes
				attrs := gauge.DataPoints[0].Attributes
				assert.Contains(t, attrs.ToSlice(), attribute.String("mcp.profile.id", "test-profile"))
			}
		}
	}

	assert.True(t, found, "Profile servers gauge metric not found")
}

func TestRecordGatewayStart(t *testing.T) {
	tests := []struct {
		name             string
		transport        string
		workingSetID     string
		wantTransport    string
		wantWorkingSetID string
	}{
		{
			name:             "stdio without profile",
			transport:        "stdio",
			workingSetID:     "",
			wantTransport:    "stdio",
			wantWorkingSetID: "",
		},
		{
			name:             "stdio with profile",
			transport:        "stdio",
			workingSetID:     "my-profile",
			wantTransport:    "stdio",
			wantWorkingSetID: "my-profile",
		},
		{
			name:             "sse with profile",
			transport:        "sse",
			workingSetID:     "test-profile",
			wantTransport:    "sse",
			wantWorkingSetID: "test-profile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup fresh telemetry for each test case to avoid metric accumulation
			_, metricReader := setupTestTelemetry(t)
			Init()
			ctx := context.Background()

			// Record gateway start
			RecordGatewayStart(ctx, tt.transport, tt.workingSetID)

			// Collect and verify metrics
			var rm metricdata.ResourceMetrics
			err := metricReader.Collect(ctx, &rm)
			require.NoError(t, err)

			// Find and verify the counter metric
			found := false
			for _, sm := range rm.ScopeMetrics {
				for _, m := range sm.Metrics {
					if m.Name == "mcp.gateway.starts" {
						found = true
						sum := m.Data.(metricdata.Sum[int64])
						require.Len(t, sum.DataPoints, 1, "should have exactly one data point")

						dp := sum.DataPoints[0]
						attrs := dp.Attributes

						// Verify transport attribute
						transportAttr, hasTransport := attrs.Value(attribute.Key("mcp.gateway.transport"))
						assert.True(t, hasTransport, "transport attribute should be present")
						assert.Equal(t, tt.wantTransport, transportAttr.AsString())

						// Verify profile attribute if expected
						profileAttr, hasProfile := attrs.Value(attribute.Key("mcp.gateway.profile"))
						if tt.wantWorkingSetID != "" {
							assert.True(t, hasProfile, "profile attribute should be present")
							assert.Equal(t, tt.wantWorkingSetID, profileAttr.AsString())
						} else {
							assert.False(t, hasProfile, "profile attribute should not be present when empty")
						}

						assert.Equal(t, int64(1), dp.Value)
					}
				}
			}

			assert.True(t, found, "Gateway starts counter metric not found")
		})
	}
}
