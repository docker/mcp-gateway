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

	"github.com/docker/mcp-gateway/pkg/plugins"
)

const (
	// ServiceName is the service name for MCP Gateway telemetry
	ServiceName = "docker-mcp-gateway"

	// TracerName is the tracer name for MCP Gateway
	TracerName = "github.com/docker/mcp-gateway"

	// MeterName is the meter name for MCP Gateway
	MeterName = "github.com/docker/mcp-gateway"
)

// Metric names - these are sent to the telemetry plugin
const (
	MetricToolCalls              = "mcp.tool.calls"
	MetricToolDuration           = "mcp.tool.duration"
	MetricToolErrors             = "mcp.tool.errors"
	MetricGatewayStarts          = "mcp.gateway.starts"
	MetricInitialize             = "mcp.initialize"
	MetricListTools              = "mcp.list.tools"
	MetricToolsDiscovered        = "mcp.tools.discovered"
	MetricCatalogOperations      = "mcp.catalog.operations"
	MetricCatalogDuration        = "mcp.catalog.operation.duration"
	MetricCatalogServers         = "mcp.catalog.servers"
	MetricPromptGets             = "mcp.prompt.gets"
	MetricPromptDuration         = "mcp.prompt.duration"
	MetricPromptErrors           = "mcp.prompt.errors"
	MetricPromptsDiscovered      = "mcp.prompts.discovered"
	MetricListPrompts            = "mcp.list.prompts"
	MetricResourceReads          = "mcp.resource.reads"
	MetricResourceDuration       = "mcp.resource.duration"
	MetricResourceErrors         = "mcp.resource.errors"
	MetricResourcesDiscovered    = "mcp.resources.discovered"
	MetricListResources          = "mcp.list.resources"
	MetricResourceTemplateReads  = "mcp.resource_template.reads"
	MetricResourceTemplateDur    = "mcp.resource_template.duration"
	MetricResourceTemplateErrors = "mcp.resource_template.errors"
	MetricResourceTemplatesDisc  = "mcp.resource_templates.discovered"
	MetricListResourceTemplates  = "mcp.list.resource_templates"
)

var (
	// tracer is the global tracer for MCP Gateway
	tracer trace.Tracer

	// meter is the global meter for MCP Gateway (used for local metrics)
	meter metric.Meter

	// Local OTel metrics - used when no plugin is registered
	ToolCallCounter              metric.Int64Counter
	ToolCallDuration             metric.Float64Histogram
	ToolErrorCounter             metric.Int64Counter
	GatewayStartCounter          metric.Int64Counter
	InitializeCounter            metric.Int64Counter
	ListToolsCounter             metric.Int64Counter
	CatalogOperationsCounter     metric.Int64Counter
	CatalogOperationDuration     metric.Float64Histogram
	CatalogServersGauge          metric.Int64Gauge
	ToolsDiscovered              metric.Int64Gauge
	PromptGetCounter             metric.Int64Counter
	PromptDuration               metric.Float64Histogram
	PromptErrorCounter           metric.Int64Counter
	PromptsDiscovered            metric.Int64Gauge
	ListPromptsCounter           metric.Int64Counter
	ResourceReadCounter          metric.Int64Counter
	ResourceDuration             metric.Float64Histogram
	ResourceErrorCounter         metric.Int64Counter
	ResourcesDiscovered          metric.Int64Gauge
	ListResourcesCounter         metric.Int64Counter
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

	debugLog("Init called")
	debugLog("TracerName=%s, MeterName=%s", TracerName, MeterName)

	// Create local metrics (used when no plugin is registered)
	initLocalMetrics()

	debugLog("Metrics created successfully")
}

func initLocalMetrics() {
	var err error

	ToolCallCounter, err = meter.Int64Counter(MetricToolCalls,
		metric.WithDescription("Number of tool calls executed"),
		metric.WithUnit("1"))
	logMetricError("tool call counter", err)

	ToolCallDuration, err = meter.Float64Histogram(MetricToolDuration,
		metric.WithDescription("Duration of tool call execution"),
		metric.WithUnit("ms"))
	logMetricError("tool duration histogram", err)

	ToolErrorCounter, err = meter.Int64Counter(MetricToolErrors,
		metric.WithDescription("Number of tool call errors"),
		metric.WithUnit("1"))
	logMetricError("tool error counter", err)

	GatewayStartCounter, err = meter.Int64Counter(MetricGatewayStarts,
		metric.WithDescription("Number of gateway starts"),
		metric.WithUnit("1"))
	logMetricError("gateway start counter", err)

	InitializeCounter, err = meter.Int64Counter(MetricInitialize,
		metric.WithDescription("Number of initialize calls"),
		metric.WithUnit("1"))
	logMetricError("initialize counter", err)

	ListToolsCounter, err = meter.Int64Counter(MetricListTools,
		metric.WithDescription("Number of list tools calls"),
		metric.WithUnit("1"))
	logMetricError("list tools counter", err)

	ToolsDiscovered, err = meter.Int64Gauge(MetricToolsDiscovered,
		metric.WithDescription("Number of tools discovered from servers"),
		metric.WithUnit("1"))
	logMetricError("tools discovered gauge", err)

	CatalogOperationsCounter, err = meter.Int64Counter(MetricCatalogOperations,
		metric.WithDescription("Number of catalog operations"),
		metric.WithUnit("1"))
	logMetricError("catalog operations counter", err)

	CatalogOperationDuration, err = meter.Float64Histogram(MetricCatalogDuration,
		metric.WithDescription("Duration of catalog operations"),
		metric.WithUnit("ms"))
	logMetricError("catalog duration histogram", err)

	CatalogServersGauge, err = meter.Int64Gauge(MetricCatalogServers,
		metric.WithDescription("Number of servers in catalogs"),
		metric.WithUnit("1"))
	logMetricError("catalog servers gauge", err)

	PromptGetCounter, err = meter.Int64Counter(MetricPromptGets,
		metric.WithDescription("Number of prompt get operations"),
		metric.WithUnit("1"))
	logMetricError("prompt get counter", err)

	PromptDuration, err = meter.Float64Histogram(MetricPromptDuration,
		metric.WithDescription("Duration of prompt operations"),
		metric.WithUnit("ms"))
	logMetricError("prompt duration histogram", err)

	PromptErrorCounter, err = meter.Int64Counter(MetricPromptErrors,
		metric.WithDescription("Number of prompt operation errors"),
		metric.WithUnit("1"))
	logMetricError("prompt error counter", err)

	PromptsDiscovered, err = meter.Int64Gauge(MetricPromptsDiscovered,
		metric.WithDescription("Number of prompts discovered from servers"),
		metric.WithUnit("1"))
	logMetricError("prompts discovered gauge", err)

	ListPromptsCounter, err = meter.Int64Counter(MetricListPrompts,
		metric.WithDescription("Number of list prompts calls"),
		metric.WithUnit("1"))
	logMetricError("list prompts counter", err)

	ResourceReadCounter, err = meter.Int64Counter(MetricResourceReads,
		metric.WithDescription("Number of resource read operations"),
		metric.WithUnit("1"))
	logMetricError("resource read counter", err)

	ResourceDuration, err = meter.Float64Histogram(MetricResourceDuration,
		metric.WithDescription("Duration of resource operations"),
		metric.WithUnit("ms"))
	logMetricError("resource duration histogram", err)

	ResourceErrorCounter, err = meter.Int64Counter(MetricResourceErrors,
		metric.WithDescription("Number of resource operation errors"),
		metric.WithUnit("1"))
	logMetricError("resource error counter", err)

	ResourcesDiscovered, err = meter.Int64Gauge(MetricResourcesDiscovered,
		metric.WithDescription("Number of resources discovered from servers"),
		metric.WithUnit("1"))
	logMetricError("resources discovered gauge", err)

	ListResourcesCounter, err = meter.Int64Counter(MetricListResources,
		metric.WithDescription("Number of list resources calls"),
		metric.WithUnit("1"))
	logMetricError("list resources counter", err)

	ResourceTemplateReadCounter, err = meter.Int64Counter(MetricResourceTemplateReads,
		metric.WithDescription("Number of resource template read operations"),
		metric.WithUnit("1"))
	logMetricError("resource template read counter", err)

	ResourceTemplateDuration, err = meter.Float64Histogram(MetricResourceTemplateDur,
		metric.WithDescription("Duration of resource template operations"),
		metric.WithUnit("ms"))
	logMetricError("resource template duration histogram", err)

	ResourceTemplateErrorCounter, err = meter.Int64Counter(MetricResourceTemplateErrors,
		metric.WithDescription("Number of resource template operation errors"),
		metric.WithUnit("1"))
	logMetricError("resource template error counter", err)

	ResourceTemplatesDiscovered, err = meter.Int64Gauge(MetricResourceTemplatesDisc,
		metric.WithDescription("Number of resource templates discovered from servers"),
		metric.WithUnit("1"))
	logMetricError("resource templates discovered gauge", err)

	ListResourceTemplatesCounter, err = meter.Int64Counter(MetricListResourceTemplates,
		metric.WithDescription("Number of list resource templates calls"),
		metric.WithUnit("1"))
	logMetricError("list resource templates counter", err)
}

func logMetricError(name string, err error) {
	if err != nil {
		debugLog("Error creating %s: %v", name, err)
	}
}

// getPlugin returns the telemetry plugin if registered, or nil
func getPlugin() plugins.TelemetryPlugin {
	return plugins.Global().TelemetryPlugin()
}

// recordCounter records a counter metric via plugin or locally
//
//nolint:unparam // value is kept for interface consistency with plugins.TelemetryPlugin
func recordCounter(ctx context.Context, name string, value int64, attrs map[string]string) {
	if p := getPlugin(); p != nil {
		p.RecordCounter(ctx, name, value, attrs)
	}
	// Note: local metrics can also be recorded here if needed
}

// recordHistogram records a histogram metric via plugin or locally
func recordHistogram(ctx context.Context, name string, value float64, attrs map[string]string) {
	if p := getPlugin(); p != nil {
		p.RecordHistogram(ctx, name, value, attrs)
	}
}

// recordGauge records a gauge metric via plugin or locally
func recordGauge(ctx context.Context, name string, value int64, attrs map[string]string) {
	if p := getPlugin(); p != nil {
		p.RecordGauge(ctx, name, value, attrs)
	}
}

// hasPlugin returns true if a telemetry plugin is registered
func hasPlugin() bool {
	return plugins.Global().HasTelemetryPlugin()
}

// StartToolCallSpan starts a new span for a tool call with server attribution
func StartToolCallSpan(ctx context.Context, toolName string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	allAttrs := append([]attribute.KeyValue{
		attribute.String("mcp.tool.name", toolName),
	}, attrs...)

	return tracer.Start(ctx, "mcp.tool.call",
		trace.WithAttributes(allAttrs...),
		trace.WithSpanKind(trace.SpanKindClient))
}

// StartCommandSpan starts a new span for a command execution
func StartCommandSpan(ctx context.Context, commandPath string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
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
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordToolError: no telemetry plugin registered")
		return
	}

	debugLog("Tool error: %s on %s", toolName, serverName)

	// Record error in span if provided (keeping local tracing)
	if span != nil {
		span.RecordError(nil)
	}

	recordCounter(ctx, MetricToolErrors, 1, map[string]string{
		"mcp.server.name": serverName,
		"mcp.server.type": serverType,
		"mcp.tool.name":   toolName,
	})
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

// StartInitializeSpan starts a new span for an initialize operation
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
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordGatewayStart: no telemetry plugin registered")
		return
	}

	debugLog("Gateway started with transport: %s", transportMode)

	recordCounter(ctx, MetricGatewayStarts, 1, map[string]string{
		"mcp.gateway.transport": transportMode,
	})
}

// RecordInitialize records an initialize event
func RecordInitialize(ctx context.Context, params *mcp.InitializeParams) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordInitialize: no telemetry plugin registered")
		return
	}

	debugLog("Initialize called")

	recordCounter(ctx, MetricInitialize, 1, map[string]string{
		"mcp.client.name":    params.ClientInfo.Name,
		"mcp.client.version": params.ClientInfo.Version,
	})
}

// RecordListTools records a list tools call
func RecordListTools(ctx context.Context, clientName string) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordListTools: no telemetry plugin registered")
		return
	}

	debugLog("List tools called")

	recordCounter(ctx, MetricListTools, 1, map[string]string{
		"mcp.client.name": clientName,
	})
}

// RecordToolCall records a tool call event
func RecordToolCall(ctx context.Context, serverName, serverType, toolName, clientName string) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordToolCall: no telemetry plugin registered")
		return
	}

	debugLog("Tool call: %s on %s", toolName, serverName)

	recordCounter(ctx, MetricToolCalls, 1, map[string]string{
		"mcp.server.name": serverName,
		"mcp.server.type": serverType,
		"mcp.tool.name":   toolName,
		"mcp.client.name": clientName,
	})
}

// RecordToolDuration records tool call duration
func RecordToolDuration(ctx context.Context, serverName, serverType, toolName, clientName string, durationMs float64) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordToolDuration: no telemetry plugin registered")
		return
	}

	debugLog("Tool duration: %s took %.2fms", toolName, durationMs)

	recordHistogram(ctx, MetricToolDuration, durationMs, map[string]string{
		"mcp.server.name": serverName,
		"mcp.server.type": serverType,
		"mcp.tool.name":   toolName,
		"mcp.client.name": clientName,
	})
}

// RecordToolList records the number of tools discovered from a server
func RecordToolList(ctx context.Context, serverName string, toolCount int) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordToolList: no telemetry plugin registered")
		return
	}

	debugLog("Tools discovered: %d from server %s", toolCount, serverName)

	recordGauge(ctx, MetricToolsDiscovered, int64(toolCount), map[string]string{
		"mcp.server.origin": serverName,
	})
}

// RecordCatalogOperation records a catalog operation with duration
func RecordCatalogOperation(ctx context.Context, operation string, catalogName string, durationMs float64, success bool) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordCatalogOperation: no telemetry plugin registered")
		return
	}

	debugLog("Catalog operation: %s on %s, duration: %.2fms, success: %v", operation, catalogName, durationMs, success)

	successStr := "false"
	if success {
		successStr = "true"
	}

	attrs := map[string]string{
		"mcp.catalog.operation": operation,
		"mcp.catalog.name":      catalogName,
		"mcp.catalog.success":   successStr,
	}

	recordCounter(ctx, MetricCatalogOperations, 1, attrs)
	recordHistogram(ctx, MetricCatalogDuration, durationMs, attrs)
}

// RecordCatalogServers records the number of servers in catalogs
func RecordCatalogServers(ctx context.Context, catalogName string, serverCount int64) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordCatalogServers: no telemetry plugin registered")
		return
	}

	debugLog("Catalog %s has %d servers", catalogName, serverCount)

	recordGauge(ctx, MetricCatalogServers, serverCount, map[string]string{
		"mcp.catalog.name": catalogName,
	})
}

// RecordPromptGet records a prompt get operation
func RecordPromptGet(ctx context.Context, promptName string, serverName string, clientName string) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordPromptGet: no telemetry plugin registered")
		return
	}

	debugLog("Prompt get: %s from server %s", promptName, serverName)

	recordCounter(ctx, MetricPromptGets, 1, map[string]string{
		"mcp.prompt.name":   promptName,
		"mcp.server.origin": serverName,
		"mcp.client.name":   clientName,
	})
}

// RecordPromptDuration records the duration of a prompt operation
func RecordPromptDuration(ctx context.Context, promptName string, serverName string, durationMs float64, clientName string) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordPromptDuration: no telemetry plugin registered")
		return
	}

	debugLog("Prompt duration: %s from %s took %.2fms", promptName, serverName, durationMs)

	recordHistogram(ctx, MetricPromptDuration, durationMs, map[string]string{
		"mcp.prompt.name":   promptName,
		"mcp.server.origin": serverName,
		"mcp.client.name":   clientName,
	})
}

// RecordPromptError records a prompt operation error
func RecordPromptError(ctx context.Context, promptName string, serverName string, errorType string) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordPromptError: no telemetry plugin registered")
		return
	}

	debugLog("Prompt error: %s from %s, error: %s", promptName, serverName, errorType)

	recordCounter(ctx, MetricPromptErrors, 1, map[string]string{
		"mcp.prompt.name":   promptName,
		"mcp.server.origin": serverName,
		"mcp.error.type":    errorType,
	})
}

// RecordPromptList records the number of prompts discovered from a server
func RecordPromptList(ctx context.Context, serverName string, promptCount int) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordPromptList: no telemetry plugin registered")
		return
	}

	debugLog("Prompts discovered: %d from server %s", promptCount, serverName)

	recordGauge(ctx, MetricPromptsDiscovered, int64(promptCount), map[string]string{
		"mcp.server.origin": serverName,
	})
}

// RecordListPrompts records a list prompts call
func RecordListPrompts(ctx context.Context, clientName string) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordListPrompts: no telemetry plugin registered")
		return
	}

	debugLog("List prompts called")

	recordCounter(ctx, MetricListPrompts, 1, map[string]string{
		"mcp.client.name": clientName,
	})
}

// RecordListResources records a list resources call
func RecordListResources(ctx context.Context, clientName string) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordListResources: no telemetry plugin registered")
		return
	}

	debugLog("List resources called")

	recordCounter(ctx, MetricListResources, 1, map[string]string{
		"mcp.client.name": clientName,
	})
}

// RecordResourceRead records a resource read operation
func RecordResourceRead(ctx context.Context, resourceURI string, serverName string, clientName string) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordResourceRead: no telemetry plugin registered")
		return
	}

	debugLog("Resource read: %s from server %s", resourceURI, serverName)

	recordCounter(ctx, MetricResourceReads, 1, map[string]string{
		"mcp.resource.uri":  resourceURI,
		"mcp.server.origin": serverName,
		"mcp.client.name":   clientName,
	})
}

// RecordResourceDuration records the duration of a resource operation
func RecordResourceDuration(ctx context.Context, resourceURI string, serverName string, durationMs float64, clientName string) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordResourceDuration: no telemetry plugin registered")
		return
	}

	debugLog("Resource duration: %s from %s took %.2fms", resourceURI, serverName, durationMs)

	recordHistogram(ctx, MetricResourceDuration, durationMs, map[string]string{
		"mcp.resource.uri":  resourceURI,
		"mcp.server.origin": serverName,
		"mcp.client.name":   clientName,
	})
}

// RecordResourceError records a resource operation error
func RecordResourceError(ctx context.Context, resourceURI string, serverName string, errorType string) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordResourceError: no telemetry plugin registered")
		return
	}

	debugLog("Resource error: %s from %s, error: %s", resourceURI, serverName, errorType)

	recordCounter(ctx, MetricResourceErrors, 1, map[string]string{
		"mcp.resource.uri":  resourceURI,
		"mcp.server.origin": serverName,
		"mcp.error.type":    errorType,
	})
}

// RecordResourceList records the number of resources discovered from a server
func RecordResourceList(ctx context.Context, serverName string, resourceCount int) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordResourceList: no telemetry plugin registered")
		return
	}

	debugLog("Resources discovered: %d from server %s", resourceCount, serverName)

	recordGauge(ctx, MetricResourcesDiscovered, int64(resourceCount), map[string]string{
		"mcp.server.origin": serverName,
	})
}

// RecordListResourceTemplates records a list resource templates call
func RecordListResourceTemplates(ctx context.Context, clientName string) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordListResourceTemplates: no telemetry plugin registered")
		return
	}

	debugLog("List resource templates called")

	recordCounter(ctx, MetricListResourceTemplates, 1, map[string]string{
		"mcp.client.name": clientName,
	})
}

// RecordResourceTemplateRead records a resource template read operation
func RecordResourceTemplateRead(ctx context.Context, uriTemplate string, serverName string, clientName string) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordResourceTemplateRead: no telemetry plugin registered")
		return
	}

	debugLog("Resource template read: %s from server %s", uriTemplate, serverName)

	recordCounter(ctx, MetricResourceTemplateReads, 1, map[string]string{
		"mcp.resource_template.uri": uriTemplate,
		"mcp.server.origin":         serverName,
		"mcp.client.name":           clientName,
	})
}

// RecordResourceTemplateDuration records the duration of a resource template operation
func RecordResourceTemplateDuration(ctx context.Context, uriTemplate string, serverName string, durationMs float64, clientName string) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordResourceTemplateDuration: no telemetry plugin registered")
		return
	}

	debugLog("Resource template duration: %s from %s took %.2fms", uriTemplate, serverName, durationMs)

	recordHistogram(ctx, MetricResourceTemplateDur, durationMs, map[string]string{
		"mcp.resource_template.uri": uriTemplate,
		"mcp.server.origin":         serverName,
		"mcp.client.name":           clientName,
	})
}

// RecordResourceTemplateError records a resource template operation error
func RecordResourceTemplateError(ctx context.Context, uriTemplate string, serverName string, errorType string) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordResourceTemplateError: no telemetry plugin registered")
		return
	}

	debugLog("Resource template error: %s from %s, error: %s", uriTemplate, serverName, errorType)

	recordCounter(ctx, MetricResourceTemplateErrors, 1, map[string]string{
		"mcp.resource_template.uri": uriTemplate,
		"mcp.server.origin":         serverName,
		"mcp.error.type":            errorType,
	})
}

// RecordResourceTemplateList records the number of resource templates discovered from a server
func RecordResourceTemplateList(ctx context.Context, serverName string, templateCount int) {
	if !hasPlugin() {
		debugLog("WARNING: Skipping RecordResourceTemplateList: no telemetry plugin registered")
		return
	}

	debugLog("Resource templates discovered: %d from server %s", templateCount, serverName)

	recordGauge(ctx, MetricResourceTemplatesDisc, int64(templateCount), map[string]string{
		"mcp.server.origin": serverName,
	})
}

func debugLog(format string, args ...any) {
	if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY] "+format+"\n", args...)
	}
}
