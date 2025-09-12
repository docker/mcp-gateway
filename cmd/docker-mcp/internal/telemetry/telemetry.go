package telemetry

import (
	"context"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	// ServiceName is the service name for MCP Gateway telemetry
	ServiceName = "docker-mcp-gateway"

	// TracerName is the tracer name for MCP Gateway
	TracerName = "github.com/docker/mcp-gateway"

	// MeterName is the meter name for MCP Gateway
	MeterName = "github.com/docker/mcp-gateway"
)

var (
	// tracer is the global tracer for MCP Gateway
	tracer trace.Tracer

	// meter is the global meter for MCP Gateway
	meter metric.Meter

	// ToolCallCounter tracks the number of tool calls with server attribution
	ToolCallCounter metric.Int64Counter

	// ToolCallDuration tracks the duration of tool calls in milliseconds
	ToolCallDuration metric.Float64Histogram

	// ToolErrorCounter tracks tool call errors by type and server
	ToolErrorCounter metric.Int64Counter

	// GatewayStartCounter tracks gateway starts
	GatewayStartCounter metric.Int64Counter

	// InitializeCounter tracks initialize calls
	InitializeCounter metric.Int64Counter

	// ListToolsCounter tracks list tools calls
	ListToolsCounter metric.Int64Counter

	// Catalog operation metrics
	CatalogOperationsCounter metric.Int64Counter
	CatalogOperationDuration metric.Float64Histogram
	CatalogServersGauge      metric.Int64Gauge

	// Tool discovery metrics
	ToolsDiscovered metric.Int64Gauge

	// Prompt operation metrics
	PromptGetCounter   metric.Int64Counter
	PromptDuration     metric.Float64Histogram
	PromptErrorCounter metric.Int64Counter
	PromptsDiscovered  metric.Int64Gauge
	ListPromptsCounter metric.Int64Counter

	// Resource operation metrics
	ResourceReadCounter  metric.Int64Counter
	ResourceDuration     metric.Float64Histogram
	ResourceErrorCounter metric.Int64Counter
	ResourcesDiscovered  metric.Int64Gauge
	ListResourcesCounter metric.Int64Counter

	// Resource template operation metrics
	ResourceTemplateReadCounter  metric.Int64Counter
	ResourceTemplateDuration     metric.Float64Histogram
	ResourceTemplateErrorCounter metric.Int64Counter
	ResourceTemplatesDiscovered  metric.Int64Gauge
	ListResourceTemplatesCounter metric.Int64Counter
)

// Init initializes the telemetry package with global providers
func Init() {
	// Get tracer from global provider (set by Docker CLI)
	tracer = otel.GetTracerProvider().Tracer(TracerName)

	// Get meter from global provider (set by Docker CLI)
	meter = otel.GetMeterProvider().Meter(MeterName)

	// Debug logging to stderr - remove in production
	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Init called\n")
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] TracerName=%s, MeterName=%s\n", TracerName, MeterName)
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Tracer provider type: %T\n", otel.GetTracerProvider())
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Meter provider type: %T\n", otel.GetMeterProvider())
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] OTEL endpoint env: %s\n", os.Getenv("DOCKER_CLI_OTEL_EXPORTER_OTLP_ENDPOINT"))
	}

	// Create metrics
	var err error

	ToolCallCounter, err = meter.Int64Counter("mcp.tool.calls",
		metric.WithDescription("Number of tool calls executed"),
		metric.WithUnit("1"))
	if err != nil {
		// Log error but don't fail - telemetry should not break the application
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating tool call counter: %v\n", err)
		}
	}

	ToolCallDuration, err = meter.Float64Histogram("mcp.tool.duration",
		metric.WithDescription("Duration of tool call execution"),
		metric.WithUnit("ms"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating tool duration histogram: %v\n", err)
		}
	}

	ToolErrorCounter, err = meter.Int64Counter("mcp.tool.errors",
		metric.WithDescription("Number of tool call errors"),
		metric.WithUnit("1"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating tool error counter: %v\n", err)
		}
	}

	GatewayStartCounter, err = meter.Int64Counter("mcp.gateway.starts",
		metric.WithDescription("Number of gateway starts"),
		metric.WithUnit("1"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating gateway start counter: %v\n", err)
		}
	}

	InitializeCounter, err = meter.Int64Counter("mcp.initialize",
		metric.WithDescription("Number of initialize calls"),
		metric.WithUnit("1"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating initialize counter: %v\n", err)
		}
	}

	ListToolsCounter, err = meter.Int64Counter("mcp.list.tools",
		metric.WithDescription("Number of list tools calls"),
		metric.WithUnit("1"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating list tools counter: %v\n", err)
		}
	}

	ToolsDiscovered, err = meter.Int64Gauge("mcp.tools.discovered",
		metric.WithDescription("Number of tools discovered from servers"),
		metric.WithUnit("1"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating tools discovered gauge: %v\n", err)
		}
	}

	CatalogOperationsCounter, err = meter.Int64Counter("mcp.catalog.operations",
		metric.WithDescription("Number of catalog operations"),
		metric.WithUnit("1"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating catalog operations counter: %v\n", err)
		}
	}

	CatalogOperationDuration, err = meter.Float64Histogram("mcp.catalog.operation.duration",
		metric.WithDescription("Duration of catalog operations"),
		metric.WithUnit("ms"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating catalog duration histogram: %v\n", err)
		}
	}

	CatalogServersGauge, err = meter.Int64Gauge("mcp.catalog.servers",
		metric.WithDescription("Number of servers in catalogs"),
		metric.WithUnit("1"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating catalog servers gauge: %v\n", err)
		}
	}

	// Initialize prompt metrics
	PromptGetCounter, err = meter.Int64Counter("mcp.prompt.gets",
		metric.WithDescription("Number of prompt get operations"),
		metric.WithUnit("1"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating prompt get counter: %v\n", err)
		}
	}

	PromptDuration, err = meter.Float64Histogram("mcp.prompt.duration",
		metric.WithDescription("Duration of prompt operations"),
		metric.WithUnit("ms"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating prompt duration histogram: %v\n", err)
		}
	}

	PromptErrorCounter, err = meter.Int64Counter("mcp.prompt.errors",
		metric.WithDescription("Number of prompt operation errors"),
		metric.WithUnit("1"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating prompt error counter: %v\n", err)
		}
	}

	PromptsDiscovered, err = meter.Int64Gauge("mcp.prompts.discovered",
		metric.WithDescription("Number of prompts discovered from servers"),
		metric.WithUnit("1"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating prompts discovered gauge: %v\n", err)
		}
	}

	ListPromptsCounter, err = meter.Int64Counter("mcp.list.prompts",
		metric.WithDescription("Number of list prompts calls"),
		metric.WithUnit("1"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating list prompts counter: %v\n", err)
		}
	}

	// Initialize resource metrics
	ResourceReadCounter, err = meter.Int64Counter("mcp.resource.reads",
		metric.WithDescription("Number of resource read operations"),
		metric.WithUnit("1"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating resource read counter: %v\n", err)
		}
	}

	ResourceDuration, err = meter.Float64Histogram("mcp.resource.duration",
		metric.WithDescription("Duration of resource operations"),
		metric.WithUnit("ms"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating resource duration histogram: %v\n", err)
		}
	}

	ResourceErrorCounter, err = meter.Int64Counter("mcp.resource.errors",
		metric.WithDescription("Number of resource operation errors"),
		metric.WithUnit("1"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating resource error counter: %v\n", err)
		}
	}

	ResourcesDiscovered, err = meter.Int64Gauge("mcp.resources.discovered",
		metric.WithDescription("Number of resources discovered from servers"),
		metric.WithUnit("1"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating resources discovered gauge: %v\n", err)
		}
	}

	ListResourcesCounter, err = meter.Int64Counter("mcp.list.resources",
		metric.WithDescription("Number of list resources calls"),
		metric.WithUnit("1"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating list resources counter: %v\n", err)
		}
	}

	// Initialize resource template metrics
	ResourceTemplateReadCounter, err = meter.Int64Counter("mcp.resource_template.reads",
		metric.WithDescription("Number of resource template read operations"),
		metric.WithUnit("1"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating resource template read counter: %v\n", err)
		}
	}

	ResourceTemplateDuration, err = meter.Float64Histogram("mcp.resource_template.duration",
		metric.WithDescription("Duration of resource template operations"),
		metric.WithUnit("ms"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating resource template duration histogram: %v\n", err)
		}
	}

	ResourceTemplateErrorCounter, err = meter.Int64Counter("mcp.resource_template.errors",
		metric.WithDescription("Number of resource template operation errors"),
		metric.WithUnit("1"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating resource template error counter: %v\n", err)
		}
	}

	ResourceTemplatesDiscovered, err = meter.Int64Gauge("mcp.resource_templates.discovered",
		metric.WithDescription("Number of resource templates discovered from servers"),
		metric.WithUnit("1"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating resource templates discovered gauge: %v\n", err)
		}
	}

	ListResourceTemplatesCounter, err = meter.Int64Counter("mcp.list.resource_templates",
		metric.WithDescription("Number of list resource templates calls"),
		metric.WithUnit("1"))
	if err != nil {
		// Log error but don't fail
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Error creating list resource templates counter: %v\n", err)
		}
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Metrics created successfully\n")
	}
}

// StartToolCallSpan starts a new span for a tool call with server attribution
func StartToolCallSpan(ctx context.Context, toolName string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	// Add the tool name as a mandatory attribute
	allAttrs := append([]attribute.KeyValue{
		attribute.String("mcp.tool.name", toolName),
	}, attrs...)

	return tracer.Start(ctx, "mcp.tool.call",
		trace.WithAttributes(allAttrs...),
		trace.WithSpanKind(trace.SpanKindClient))
}

// StartCommandSpan starts a new span for a command execution
func StartCommandSpan(ctx context.Context, commandPath string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	// Add the command path as an attribute
	allAttrs := append([]attribute.KeyValue{
		attribute.String("mcp.command.path", commandPath),
	}, attrs...)

	spanName := "mcp.command." + commandPath

	return tracer.Start(ctx, spanName,
		trace.WithAttributes(allAttrs...),
		trace.WithSpanKind(trace.SpanKindServer))
}

// RecordToolError records a tool error with appropriate attributes
func RecordToolError(ctx context.Context, span trace.Span, serverName, serverType, toolName string) {
	if ToolErrorCounter == nil {
		return // Telemetry not initialized
	}

	// Record error in span if provided
	if span != nil {
		span.RecordError(nil, trace.WithAttributes(
			attribute.String("mcp.server.name", serverName),
			attribute.String("mcp.server.type", serverType),
		))
	}

	ToolErrorCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("mcp.tool.name", toolName),
			attribute.String("mcp.server.name", serverName),
			attribute.String("mcp.server.type", serverType),
		))
}

// StartPromptSpan starts a new span for a prompt operation
func StartPromptSpan(ctx context.Context, promptName string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	allAttrs := append([]attribute.KeyValue{
		attribute.String("mcp.prompt.name", promptName),
	}, attrs...)

	return tracer.Start(ctx, "mcp.prompt.get",
		trace.WithAttributes(allAttrs...),
		trace.WithSpanKind(trace.SpanKindClient))
}

// StartListSpan starts a new span for a list operation (tools, prompts, resources)
func StartInitializeSpan(ctx context.Context, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return tracer.Start(ctx, "mcp.initialize",
		trace.WithAttributes(attrs...),
		trace.WithSpanKind(trace.SpanKindServer))
}

// StartListSpan starts a new span for a list operation (tools, prompts, resources)
func StartListSpan(ctx context.Context, listType string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	allAttrs := append([]attribute.KeyValue{
		attribute.String("mcp.list.type", listType),
	}, attrs...)

	spanName := "mcp.list." + listType

	return tracer.Start(ctx, spanName,
		trace.WithAttributes(allAttrs...),
		trace.WithSpanKind(trace.SpanKindServer))
}

// StartResourceSpan starts a new span for a resource operation
func StartResourceSpan(ctx context.Context, resourceURI string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	allAttrs := append([]attribute.KeyValue{
		attribute.String("mcp.resource.uri", resourceURI),
	}, attrs...)

	return tracer.Start(ctx, "mcp.resource.read",
		trace.WithAttributes(allAttrs...),
		trace.WithSpanKind(trace.SpanKindClient))
}

// StartResourceTemplateSpan starts a new span for a resource template operation
func StartResourceTemplateSpan(ctx context.Context, uriTemplate string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	allAttrs := append([]attribute.KeyValue{
		attribute.String("mcp.resource_template.uri", uriTemplate),
	}, attrs...)

	return tracer.Start(ctx, "mcp.resource_template.read",
		trace.WithAttributes(allAttrs...),
		trace.WithSpanKind(trace.SpanKindClient))
}

// StartInterceptorSpan starts a new span for interceptor execution
func StartInterceptorSpan(ctx context.Context, when, interceptorType string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	allAttrs := append([]attribute.KeyValue{
		attribute.String("mcp.interceptor.when", when),
		attribute.String("mcp.interceptor.type", interceptorType),
	}, attrs...)

	spanName := "mcp.interceptor." + interceptorType

	return tracer.Start(ctx, spanName,
		trace.WithAttributes(allAttrs...),
		trace.WithSpanKind(trace.SpanKindInternal))
}

// RecordGatewayStart records a gateway start event
func RecordGatewayStart(ctx context.Context, transportMode string) {
	if GatewayStartCounter == nil {
		return // Telemetry not initialized
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Gateway started with transport: %s\n", transportMode)
	}

	GatewayStartCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("mcp.gateway.transport", transportMode),
		))
}

func RecordInitialize(ctx context.Context, params *mcp.InitializeParams) {
	if InitializeCounter == nil {
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] WARNING: InitializeCounter is nil - metrics not initialized\n")
		}
		return // Telemetry not initialized
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Initialize called - adding to counter\n")
	}

	InitializeCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("mcp.client.name", params.ClientInfo.Name),
			attribute.String("mcp.client.version", params.ClientInfo.Version),
		))
}

// RecordListTools records a list tools call
func RecordListTools(ctx context.Context, clientName string) {
	if ListToolsCounter == nil {
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] WARNING: ListToolsCounter is nil - metrics not initialized\n")
		}
		return // Telemetry not initialized
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] List tools called - adding to counter\n")
	}

	ListToolsCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("mcp.client.name", clientName),
		))

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] List tools counter incremented\n")
	}
}

// RecordToolList records the number of tools discovered from a server
func RecordToolList(ctx context.Context, serverName string, toolCount int) {
	if ToolsDiscovered == nil {
		return // Telemetry not initialized
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Tools discovered: %d from server %s\n",
			toolCount, serverName)
	}

	ToolsDiscovered.Record(ctx, int64(toolCount),
		metric.WithAttributes(
			attribute.String("mcp.server.origin", serverName),
		))
}

// RecordCatalogOperation records a catalog operation with duration
func RecordCatalogOperation(ctx context.Context, operation string, catalogName string, durationMs float64, success bool) {
	if CatalogOperationsCounter == nil || CatalogOperationDuration == nil {
		return // Telemetry not initialized
	}

	attrs := []attribute.KeyValue{
		attribute.String("mcp.catalog.operation", operation),
		attribute.String("mcp.catalog.name", catalogName),
		attribute.Bool("mcp.catalog.success", success),
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Catalog operation: %s on %s, duration: %.2fms, success: %v\n",
			operation, catalogName, durationMs, success)
	}

	CatalogOperationsCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	CatalogOperationDuration.Record(ctx, durationMs, metric.WithAttributes(attrs...))
}

// RecordCatalogServers records the number of servers in catalogs
func RecordCatalogServers(ctx context.Context, catalogName string, serverCount int64) {
	if CatalogServersGauge == nil {
		return // Telemetry not initialized
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Catalog %s has %d servers\n", catalogName, serverCount)
	}

	CatalogServersGauge.Record(ctx, serverCount,
		metric.WithAttributes(
			attribute.String("mcp.catalog.name", catalogName),
		))
}

// RecordPromptGet records a prompt get operation
func RecordPromptGet(ctx context.Context, promptName string, serverName string, clientName string) {
	if PromptGetCounter == nil {
		return // Telemetry not initialized
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Prompt get: %s from server %s\n", promptName, serverName)
	}

	PromptGetCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("mcp.prompt.name", promptName),
			attribute.String("mcp.server.origin", serverName),
			attribute.String("mcp.client.name", clientName),
		))
}

// RecordPromptDuration records the duration of a prompt operation
func RecordPromptDuration(ctx context.Context, promptName string, serverName string, durationMs float64, clientName string) {
	if PromptDuration == nil {
		return // Telemetry not initialized
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Prompt duration: %s from %s took %.2fms\n",
			promptName, serverName, durationMs)
	}

	PromptDuration.Record(ctx, durationMs,
		metric.WithAttributes(
			attribute.String("mcp.prompt.name", promptName),
			attribute.String("mcp.server.origin", serverName),
			attribute.String("mcp.client.name", clientName),
		))
}

// RecordPromptError records a prompt operation error
func RecordPromptError(ctx context.Context, promptName string, serverName string, errorType string) {
	if PromptErrorCounter == nil {
		return // Telemetry not initialized
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Prompt error: %s from %s, error: %s\n",
			promptName, serverName, errorType)
	}

	PromptErrorCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("mcp.prompt.name", promptName),
			attribute.String("mcp.server.origin", serverName),
			attribute.String("mcp.error.type", errorType),
		))
}

// RecordPromptList records the number of prompts discovered from a server
func RecordPromptList(ctx context.Context, serverName string, promptCount int) {
	if PromptsDiscovered == nil {
		return // Telemetry not initialized
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Prompts discovered: %d from server %s\n",
			promptCount, serverName)
	}

	PromptsDiscovered.Record(ctx, int64(promptCount),
		metric.WithAttributes(
			attribute.String("mcp.server.origin", serverName),
		))
}

// RecordListPrompts records a list prompts call (similar to RecordListTools)
func RecordListPrompts(ctx context.Context, clientName string) {
	if ListPromptsCounter == nil {
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] WARNING: ListPromptsCounter is nil - metrics not initialized\n")
		}
		return // Telemetry not initialized
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] List prompts called - adding to counter\n")
	}

	ListPromptsCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("mcp.client.name", clientName),
		))

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] List prompts counter incremented\n")
	}
}

// RecordListResources records a list resources call
func RecordListResources(ctx context.Context, clientName string) {
	if ListResourcesCounter == nil {
		return // Telemetry not initialized
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] List resources called\n")
	}

	ListResourcesCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("mcp.client.name", clientName),
		))
}

// RecordResourceRead records a resource read operation
func RecordResourceRead(ctx context.Context, resourceURI string, serverName string, clientName string) {
	if ResourceReadCounter == nil {
		return // Telemetry not initialized
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Resource read: %s from server %s\n", resourceURI, serverName)
	}

	ResourceReadCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("mcp.resource.uri", resourceURI),
			attribute.String("mcp.server.origin", serverName),
			attribute.String("mcp.client.name", clientName),
		))
}

// RecordResourceDuration records the duration of a resource operation
func RecordResourceDuration(ctx context.Context, resourceURI string, serverName string, durationMs float64, clientName string) {
	if ResourceDuration == nil {
		return // Telemetry not initialized
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Resource duration: %s from %s took %.2fms\n",
			resourceURI, serverName, durationMs)
	}

	ResourceDuration.Record(ctx, durationMs,
		metric.WithAttributes(
			attribute.String("mcp.resource.uri", resourceURI),
			attribute.String("mcp.server.origin", serverName),
			attribute.String("mcp.client.name", clientName),
		))
}

// RecordResourceError records a resource operation error
func RecordResourceError(ctx context.Context, resourceURI string, serverName string, errorType string) {
	if ResourceErrorCounter == nil {
		return // Telemetry not initialized
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Resource error: %s from %s, error: %s\n",
			resourceURI, serverName, errorType)
	}

	ResourceErrorCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("mcp.resource.uri", resourceURI),
			attribute.String("mcp.server.origin", serverName),
			attribute.String("mcp.error.type", errorType),
		))
}

// RecordResourceList records the number of resources discovered from a server
func RecordResourceList(ctx context.Context, serverName string, resourceCount int) {
	if ResourcesDiscovered == nil {
		return // Telemetry not initialized
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Resources discovered: %d from server %s\n",
			resourceCount, serverName)
	}

	ResourcesDiscovered.Record(ctx, int64(resourceCount),
		metric.WithAttributes(
			attribute.String("mcp.server.origin", serverName),
		))
}

// RecordListResourceTemplates records a list resource templates call
func RecordListResourceTemplates(ctx context.Context, clientName string) {
	if ListResourceTemplatesCounter == nil {
		return // Telemetry not initialized
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] List resource templates called\n")
	}

	ListResourceTemplatesCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("mcp.client.name", clientName),
		))
}

// RecordResourceTemplateRead records a resource template read operation
func RecordResourceTemplateRead(ctx context.Context, uriTemplate string, serverName string, clientName string) {
	if ResourceTemplateReadCounter == nil {
		return // Telemetry not initialized
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Resource template read: %s from server %s\n", uriTemplate, serverName)
	}

	ResourceTemplateReadCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("mcp.resource_template.uri", uriTemplate),
			attribute.String("mcp.server.origin", serverName),
			attribute.String("mcp.client.name", clientName),
		))
}

// RecordResourceTemplateDuration records the duration of a resource template operation
func RecordResourceTemplateDuration(ctx context.Context, uriTemplate string, serverName string, durationMs float64, clientName string) {
	if ResourceTemplateDuration == nil {
		return // Telemetry not initialized
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Resource template duration: %s from %s took %.2fms\n",
			uriTemplate, serverName, durationMs)
	}

	ResourceTemplateDuration.Record(ctx, durationMs,
		metric.WithAttributes(
			attribute.String("mcp.resource_template.uri", uriTemplate),
			attribute.String("mcp.server.origin", serverName),
			attribute.String("mcp.client.name", clientName),
		))
}

// RecordResourceTemplateError records a resource template operation error
func RecordResourceTemplateError(ctx context.Context, uriTemplate string, serverName string, errorType string) {
	if ResourceTemplateErrorCounter == nil {
		return // Telemetry not initialized
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Resource template error: %s from %s, error: %s\n",
			uriTemplate, serverName, errorType)
	}

	ResourceTemplateErrorCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("mcp.resource_template.uri", uriTemplate),
			attribute.String("mcp.server.origin", serverName),
			attribute.String("mcp.error.type", errorType),
		))
}

// RecordResourceTemplateList records the number of resource templates discovered from a server
func RecordResourceTemplateList(ctx context.Context, serverName string, templateCount int) {
	if ResourceTemplatesDiscovered == nil {
		return // Telemetry not initialized
	}

	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] Resource templates discovered: %d from server %s\n",
			templateCount, serverName)
	}

	ResourceTemplatesDiscovered.Record(ctx, int64(templateCount),
		metric.WithAttributes(
			attribute.String("mcp.server.origin", serverName),
		))
}
