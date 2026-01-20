package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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

func TestResourceHandlerTelemetry(t *testing.T) {
	// Enable debug logging for tests
	t.Setenv("DOCKER_MCP_TELEMETRY_DEBUG", "1")

	t.Run("records resource read metrics", func(t *testing.T) {
		// Set up span recorder and metric reader
		spanRecorder := tracetest.NewSpanRecorder()
		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithSpanProcessor(spanRecorder),
		)
		otel.SetTracerProvider(tracerProvider)

		metricReader := sdkmetric.NewManualReader()
		meterProvider := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(metricReader),
		)
		otel.SetMeterProvider(meterProvider)

		// Initialize telemetry
		telemetry.Init()

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

		// Record resource read
		telemetry.RecordResourceRead(ctx, resourceURI, serverConfig.Name, clientName)

		// Verify metrics were collected
		var rm metricdata.ResourceMetrics
		err := metricReader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Check that we have metrics
		assert.NotEmpty(t, rm.ScopeMetrics, "Should have scope metrics")

		// Find our counter metric
		foundCounter := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "mcp.resource.reads" {
					foundCounter = true
					// Verify it's a sum (counter)
					sum, ok := m.Data.(metricdata.Sum[int64])
					assert.True(t, ok, "Should be a Sum metric")
					assert.NotEmpty(t, sum.DataPoints, "Should have data points")

					// Check the value
					if len(sum.DataPoints) > 0 {
						assert.Equal(t, int64(1), sum.DataPoints[0].Value, "Counter should be 1")

						// Check attributes
						attrs := sum.DataPoints[0].Attributes
						hasURI := false
						hasServer := false
						hasClient := false
						for _, attr := range attrs.ToSlice() {
							if attr.Key == "mcp.resource.uri" {
								assert.Equal(t, resourceURI, attr.Value.AsString())
								hasURI = true
							}
							if attr.Key == "mcp.server.origin" {
								assert.Equal(t, serverConfig.Name, attr.Value.AsString())
								hasServer = true
							}
							if attr.Key == "mcp.client.name" {
								assert.Equal(t, clientName, attr.Value.AsString())
								hasClient = true
							}
						}
						assert.True(t, hasURI, "Should have resource URI attribute")
						assert.True(t, hasServer, "Should have server origin attribute")
						assert.True(t, hasClient, "Should have client name attribute")
					}
				}
			}
		}
		assert.True(t, foundCounter, "Should have found mcp.resource.reads counter")
	})

	t.Run("records resource duration histogram", func(t *testing.T) {
		// Set up metric reader
		metricReader := sdkmetric.NewManualReader()
		meterProvider := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(metricReader),
		)
		otel.SetMeterProvider(meterProvider)

		// Initialize telemetry
		telemetry.Init()

		// Test data
		ctx := context.Background()
		resourceURI := "file:///test/resource.txt"
		serverName := "test-server"
		clientName := "test-client"
		duration := 42.5 // milliseconds

		// Record duration
		telemetry.RecordResourceDuration(ctx, resourceURI, serverName, duration, clientName)

		// Verify metrics
		var rm metricdata.ResourceMetrics
		err := metricReader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Find histogram
		foundHistogram := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "mcp.resource.duration" {
					foundHistogram = true
					hist, ok := m.Data.(metricdata.Histogram[float64])
					assert.True(t, ok, "Should be a Histogram metric")
					assert.NotEmpty(t, hist.DataPoints, "Should have data points")

					if len(hist.DataPoints) > 0 {
						dp := hist.DataPoints[0]
						assert.Equal(t, uint64(1), dp.Count, "Should have 1 observation")
						assert.InEpsilon(t, duration, dp.Sum, 0.01, "Sum should equal duration")
					}
				}
			}
		}
		assert.True(t, foundHistogram, "Should have found mcp.resource.duration histogram")
	})

	t.Run("records resource errors", func(t *testing.T) {
		// Set up metric reader
		metricReader := sdkmetric.NewManualReader()
		meterProvider := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(metricReader),
		)
		otel.SetMeterProvider(meterProvider)

		// Initialize telemetry
		telemetry.Init()

		// Test data
		ctx := context.Background()
		resourceURI := "file:///test/resource.txt"
		serverName := "test-server"

		// Record error
		telemetry.RecordResourceError(ctx, resourceURI, serverName, "not_found")

		// Verify metrics
		var rm metricdata.ResourceMetrics
		err := metricReader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Find error counter
		foundErrorCounter := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "mcp.resource.errors" {
					foundErrorCounter = true
					sum, ok := m.Data.(metricdata.Sum[int64])
					assert.True(t, ok, "Should be a Sum metric")
					assert.NotEmpty(t, sum.DataPoints, "Should have data points")

					if len(sum.DataPoints) > 0 {
						assert.Equal(t, int64(1), sum.DataPoints[0].Value, "Error counter should be 1")

						// Check for error type attribute
						attrs := sum.DataPoints[0].Attributes
						hasErrorType := false
						for _, attr := range attrs.ToSlice() {
							if attr.Key == "mcp.error.type" {
								assert.Equal(t, "not_found", attr.Value.AsString())
								hasErrorType = true
							}
						}
						assert.True(t, hasErrorType, "Should have error type attribute")
					}
				}
			}
		}
		assert.True(t, foundErrorCounter, "Should have found mcp.resource.errors counter")
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
	// Enable debug logging for tests
	t.Setenv("DOCKER_MCP_TELEMETRY_DEBUG", "1")

	t.Run("records resource template read metrics", func(t *testing.T) {
		// Set up metric reader
		metricReader := sdkmetric.NewManualReader()
		meterProvider := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(metricReader),
		)
		otel.SetMeterProvider(meterProvider)

		// Initialize telemetry
		telemetry.Init()

		// Test data
		ctx := context.Background()
		uriTemplate := "file:///test/{id}/resource.txt"
		serverName := "test-server"
		clientName := "test-client"

		// Record resource template read
		telemetry.RecordResourceTemplateRead(ctx, uriTemplate, serverName, clientName)

		// Verify metrics
		var rm metricdata.ResourceMetrics
		err := metricReader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Find counter
		foundCounter := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "mcp.resource_template.reads" {
					foundCounter = true
					sum, ok := m.Data.(metricdata.Sum[int64])
					assert.True(t, ok, "Should be a Sum metric")
					assert.NotEmpty(t, sum.DataPoints, "Should have data points")

					if len(sum.DataPoints) > 0 {
						assert.Equal(t, int64(1), sum.DataPoints[0].Value, "Counter should be 1")
					}
				}
			}
		}
		assert.True(t, foundCounter, "Should have found mcp.resource_template.reads counter")
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
	// Enable debug logging for tests
	t.Setenv("DOCKER_MCP_TELEMETRY_DEBUG", "1")

	t.Run("records resources discovered", func(t *testing.T) {
		// Set up metric reader
		metricReader := sdkmetric.NewManualReader()
		meterProvider := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(metricReader),
		)
		otel.SetMeterProvider(meterProvider)

		// Initialize telemetry
		telemetry.Init()

		// Test data
		ctx := context.Background()
		serverName := "test-server"
		resourceCount := 10

		// Record discovery
		telemetry.RecordResourceList(ctx, serverName, resourceCount)

		// Verify metrics
		var rm metricdata.ResourceMetrics
		err := metricReader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Find gauge
		foundGauge := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "mcp.resources.discovered" {
					foundGauge = true
					gauge, ok := m.Data.(metricdata.Gauge[int64])
					assert.True(t, ok, "Should be a Gauge metric")
					assert.NotEmpty(t, gauge.DataPoints, "Should have data points")

					if len(gauge.DataPoints) > 0 {
						assert.Equal(t, int64(resourceCount), gauge.DataPoints[0].Value)
					}
				}
			}
		}
		assert.True(t, foundGauge, "Should have found mcp.resources.discovered gauge")
	})

	t.Run("records resource templates discovered", func(t *testing.T) {
		// Set up metric reader
		metricReader := sdkmetric.NewManualReader()
		meterProvider := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(metricReader),
		)
		otel.SetMeterProvider(meterProvider)

		// Initialize telemetry
		telemetry.Init()

		// Test data
		ctx := context.Background()
		serverName := "test-server"
		templateCount := 5

		// Record discovery
		telemetry.RecordResourceTemplateList(ctx, serverName, templateCount)

		// Verify metrics
		var rm metricdata.ResourceMetrics
		err := metricReader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Find gauge
		foundGauge := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "mcp.resource_templates.discovered" {
					foundGauge = true
					gauge, ok := m.Data.(metricdata.Gauge[int64])
					assert.True(t, ok, "Should be a Gauge metric")
					assert.NotEmpty(t, gauge.DataPoints, "Should have data points")

					if len(gauge.DataPoints) > 0 {
						assert.Equal(t, int64(templateCount), gauge.DataPoints[0].Value)
					}
				}
			}
		}
		assert.True(t, foundGauge, "Should have found mcp.resource_templates.discovered gauge")
	})
}

// Test the actual handler instrumentation
func TestMcpServerResourceHandlerInstrumentation(t *testing.T) {
	// This test would require mocking the client pool and MCP server
	// For now, we'll focus on testing that the telemetry functions work correctly
	// The actual handler instrumentation will be tested through integration tests

	t.Run("handler records telemetry on success", func(t *testing.T) {
		// Set up telemetry
		spanRecorder := tracetest.NewSpanRecorder()
		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithSpanProcessor(spanRecorder),
		)
		otel.SetTracerProvider(tracerProvider)

		metricReader := sdkmetric.NewManualReader()
		meterProvider := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(metricReader),
		)
		otel.SetMeterProvider(meterProvider)

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

		// Record counter (as handler would)
		telemetry.RecordResourceRead(ctx, params.URI, serverConfig.Name, clientName)

		// Simulate some work
		time.Sleep(10 * time.Millisecond)

		// Record duration (as handler would)
		duration := time.Since(startTime).Milliseconds()
		telemetry.RecordResourceDuration(ctx, params.URI, serverConfig.Name, float64(duration), clientName)

		// End span
		span.End()

		// Verify span was created
		spans := spanRecorder.Ended()
		require.Len(t, spans, 1, "Should have created one span")

		// Verify metrics
		var rm metricdata.ResourceMetrics
		err := metricReader.Collect(ctx, &rm)
		require.NoError(t, err)

		// Check for counter and histogram
		foundCounter := false
		foundHistogram := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				switch m.Name {
				case "mcp.resource.reads":
					foundCounter = true
				case "mcp.resource.duration":
					foundHistogram = true
				}
			}
		}
		assert.True(t, foundCounter, "Should have recorded counter")
		assert.True(t, foundHistogram, "Should have recorded duration")
	})

	t.Run("handler records error telemetry on failure", func(t *testing.T) {
		// Set up telemetry
		spanRecorder := tracetest.NewSpanRecorder()
		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithSpanProcessor(spanRecorder),
		)
		otel.SetTracerProvider(tracerProvider)

		metricReader := sdkmetric.NewManualReader()
		meterProvider := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(metricReader),
		)
		otel.SetMeterProvider(meterProvider)

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

		// Record counter
		telemetry.RecordResourceRead(ctx, params.URI, serverConfig.Name, clientName)

		// Simulate error
		err := errors.New("resource not found")
		span.RecordError(err)
		telemetry.RecordResourceError(ctx, params.URI, serverConfig.Name, "not_found")

		span.End()

		// Verify error was recorded
		spans := spanRecorder.Ended()
		require.Len(t, spans, 1)
		assert.Len(t, spans[0].Events(), 1, "Should have error event")

		// Verify error counter
		var rm metricdata.ResourceMetrics
		require.NoError(t, metricReader.Collect(ctx, &rm))

		foundErrorCounter := false
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "mcp.resource.errors" {
					foundErrorCounter = true
				}
			}
		}
		assert.True(t, foundErrorCounter, "Should have recorded error counter")
	})
}
