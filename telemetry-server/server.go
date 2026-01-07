package telemetryserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Server is the telemetry MCP server
type Server struct {
	mcpServer *mcp.Server
	port      int
	listener  net.Listener

	// OpenTelemetry meter for creating instruments on demand
	meter metric.Meter

	// Cache of created instruments
	counters   map[string]metric.Int64Counter
	histograms map[string]metric.Float64Histogram
	gauges     map[string]metric.Int64Gauge
}

// NewServer creates a new telemetry MCP server
func NewServer(port int) *Server {
	s := &Server{
		port:       port,
		counters:   make(map[string]metric.Int64Counter),
		histograms: make(map[string]metric.Float64Histogram),
		gauges:     make(map[string]metric.Int64Gauge),
	}

	// Initialize OpenTelemetry meter
	s.meter = otel.GetMeterProvider().Meter("github.com/docker/mcp-gateway/telemetry-server")

	// Create MCP server
	s.mcpServer = mcp.NewServer(&mcp.Implementation{
		Name:    "telemetry-server",
		Version: "1.0.0",
	}, nil)

	// Register telemetry tools
	s.registerTools()

	return s
}

// getOrCreateCounter returns an existing counter or creates a new one
func (s *Server) getOrCreateCounter(name string) metric.Int64Counter {
	if counter, ok := s.counters[name]; ok {
		return counter
	}

	counter, err := s.meter.Int64Counter(name,
		metric.WithDescription(fmt.Sprintf("Counter for %s", name)),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating counter %s: %v", name, err)
		return nil
	}

	s.counters[name] = counter
	return counter
}

// getOrCreateHistogram returns an existing histogram or creates a new one
func (s *Server) getOrCreateHistogram(name string) metric.Float64Histogram {
	if histogram, ok := s.histograms[name]; ok {
		return histogram
	}

	histogram, err := s.meter.Float64Histogram(name,
		metric.WithDescription(fmt.Sprintf("Histogram for %s", name)),
		metric.WithUnit("ms"))
	if err != nil {
		debugLog("Error creating histogram %s: %v", name, err)
		return nil
	}

	s.histograms[name] = histogram
	return histogram
}

// getOrCreateGauge returns an existing gauge or creates a new one
func (s *Server) getOrCreateGauge(name string) metric.Int64Gauge {
	if gauge, ok := s.gauges[name]; ok {
		return gauge
	}

	gauge, err := s.meter.Int64Gauge(name,
		metric.WithDescription(fmt.Sprintf("Gauge for %s", name)),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating gauge %s: %v", name, err)
		return nil
	}

	s.gauges[name] = gauge
	return gauge
}

// convertAttributes converts a map of string attributes to OTel attributes
func convertAttributes(attrs map[string]string) []attribute.KeyValue {
	if attrs == nil {
		return nil
	}

	result := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		result = append(result, attribute.String(k, v))
	}
	return result
}

func (s *Server) registerTools() {
	// Record counter - generic counter recording
	type RecordCounterArgs struct {
		Name       string            `json:"name"`
		Value      int64             `json:"value"`
		Attributes map[string]string `json:"attributes"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-counter",
		Description: "Record a counter metric increment",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordCounterArgs) (*mcp.CallToolResult, any, error) {
		counter := s.getOrCreateCounter(args.Name)
		if counter != nil {
			attrs := convertAttributes(args.Attributes)
			counter.Add(ctx, args.Value, metric.WithAttributes(attrs...))
		}
		debugLog("Recorded counter %s: %d", args.Name, args.Value)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record histogram - generic histogram recording
	type RecordHistogramArgs struct {
		Name       string            `json:"name"`
		Value      float64           `json:"value"`
		Attributes map[string]string `json:"attributes"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-histogram",
		Description: "Record a histogram metric value",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordHistogramArgs) (*mcp.CallToolResult, any, error) {
		histogram := s.getOrCreateHistogram(args.Name)
		if histogram != nil {
			attrs := convertAttributes(args.Attributes)
			histogram.Record(ctx, args.Value, metric.WithAttributes(attrs...))
		}
		debugLog("Recorded histogram %s: %.2f", args.Name, args.Value)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record gauge - generic gauge recording
	type RecordGaugeArgs struct {
		Name       string            `json:"name"`
		Value      int64             `json:"value"`
		Attributes map[string]string `json:"attributes"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-gauge",
		Description: "Record a gauge metric value",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordGaugeArgs) (*mcp.CallToolResult, any, error) {
		gauge := s.getOrCreateGauge(args.Name)
		if gauge != nil {
			attrs := convertAttributes(args.Attributes)
			gauge.Record(ctx, args.Value, metric.WithAttributes(attrs...))
		}
		debugLog("Recorded gauge %s: %d", args.Name, args.Value)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})
}

// Start starts the telemetry server
func (s *Server) Start(ctx context.Context) error {
	var err error
	s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", s.port, err)
	}

	// Get actual port (useful if port was 0)
	s.port = s.listener.Addr().(*net.TCPAddr).Port

	handler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return s.mcpServer
	}, nil)

	httpServer := &http.Server{
		Handler: handler,
	}

	go func() {
		<-ctx.Done()
		httpServer.Close()
	}()

	debugLog("Telemetry server starting on port %d", s.port)

	go func() {
		if err := httpServer.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			debugLog("Telemetry server error: %v", err)
		}
	}()

	return nil
}

// Port returns the port the server is listening on
func (s *Server) Port() int {
	return s.port
}

// Stop stops the telemetry server
func (s *Server) Stop() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

func debugLog(format string, args ...any) {
	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY-SERVER] "+format+"\n", args...)
	}
}
