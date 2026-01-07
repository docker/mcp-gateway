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

	// OpenTelemetry instruments
	meter                        metric.Meter
	toolCallCounter              metric.Int64Counter
	toolCallDuration             metric.Float64Histogram
	toolErrorCounter             metric.Int64Counter
	gatewayStartCounter          metric.Int64Counter
	initializeCounter            metric.Int64Counter
	listToolsCounter             metric.Int64Counter
	toolsDiscovered              metric.Int64Gauge
	catalogOperationsCounter     metric.Int64Counter
	catalogOperationDuration     metric.Float64Histogram
	catalogServersGauge          metric.Int64Gauge
	promptGetCounter             metric.Int64Counter
	promptDuration               metric.Float64Histogram
	promptErrorCounter           metric.Int64Counter
	promptsDiscovered            metric.Int64Gauge
	listPromptsCounter           metric.Int64Counter
	resourceReadCounter          metric.Int64Counter
	resourceDuration             metric.Float64Histogram
	resourceErrorCounter         metric.Int64Counter
	resourcesDiscovered          metric.Int64Gauge
	listResourcesCounter         metric.Int64Counter
	resourceTemplateReadCounter  metric.Int64Counter
	resourceTemplateDuration     metric.Float64Histogram
	resourceTemplateErrorCounter metric.Int64Counter
	resourceTemplatesDiscovered  metric.Int64Gauge
	listResourceTemplatesCounter metric.Int64Counter
}

// NewServer creates a new telemetry MCP server
func NewServer(port int) *Server {
	s := &Server{
		port: port,
	}

	// Initialize OpenTelemetry meter
	s.meter = otel.GetMeterProvider().Meter("github.com/docker/mcp-gateway/telemetry-server")
	s.initializeMetrics()

	// Create MCP server
	s.mcpServer = mcp.NewServer(&mcp.Implementation{
		Name:    "telemetry-server",
		Version: "1.0.0",
	}, nil)

	// Register telemetry tools
	s.registerTools()

	return s
}

func (s *Server) initializeMetrics() {
	var err error

	s.toolCallCounter, err = s.meter.Int64Counter("mcp.tool.calls",
		metric.WithDescription("Number of tool calls executed"),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating tool call counter: %v", err)
	}

	s.toolCallDuration, err = s.meter.Float64Histogram("mcp.tool.duration",
		metric.WithDescription("Duration of tool call execution"),
		metric.WithUnit("ms"))
	if err != nil {
		debugLog("Error creating tool duration histogram: %v", err)
	}

	s.toolErrorCounter, err = s.meter.Int64Counter("mcp.tool.errors",
		metric.WithDescription("Number of tool call errors"),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating tool error counter: %v", err)
	}

	s.gatewayStartCounter, err = s.meter.Int64Counter("mcp.gateway.starts",
		metric.WithDescription("Number of gateway starts"),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating gateway start counter: %v", err)
	}

	s.initializeCounter, err = s.meter.Int64Counter("mcp.initialize",
		metric.WithDescription("Number of initialize calls"),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating initialize counter: %v", err)
	}

	s.listToolsCounter, err = s.meter.Int64Counter("mcp.list.tools",
		metric.WithDescription("Number of list tools calls"),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating list tools counter: %v", err)
	}

	s.toolsDiscovered, err = s.meter.Int64Gauge("mcp.tools.discovered",
		metric.WithDescription("Number of tools discovered from servers"),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating tools discovered gauge: %v", err)
	}

	s.catalogOperationsCounter, err = s.meter.Int64Counter("mcp.catalog.operations",
		metric.WithDescription("Number of catalog operations"),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating catalog operations counter: %v", err)
	}

	s.catalogOperationDuration, err = s.meter.Float64Histogram("mcp.catalog.operation.duration",
		metric.WithDescription("Duration of catalog operations"),
		metric.WithUnit("ms"))
	if err != nil {
		debugLog("Error creating catalog duration histogram: %v", err)
	}

	s.catalogServersGauge, err = s.meter.Int64Gauge("mcp.catalog.servers",
		metric.WithDescription("Number of servers in catalogs"),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating catalog servers gauge: %v", err)
	}

	// Prompt metrics
	s.promptGetCounter, err = s.meter.Int64Counter("mcp.prompt.gets",
		metric.WithDescription("Number of prompt get operations"),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating prompt get counter: %v", err)
	}

	s.promptDuration, err = s.meter.Float64Histogram("mcp.prompt.duration",
		metric.WithDescription("Duration of prompt operations"),
		metric.WithUnit("ms"))
	if err != nil {
		debugLog("Error creating prompt duration histogram: %v", err)
	}

	s.promptErrorCounter, err = s.meter.Int64Counter("mcp.prompt.errors",
		metric.WithDescription("Number of prompt operation errors"),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating prompt error counter: %v", err)
	}

	s.promptsDiscovered, err = s.meter.Int64Gauge("mcp.prompts.discovered",
		metric.WithDescription("Number of prompts discovered from servers"),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating prompts discovered gauge: %v", err)
	}

	s.listPromptsCounter, err = s.meter.Int64Counter("mcp.list.prompts",
		metric.WithDescription("Number of list prompts calls"),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating list prompts counter: %v", err)
	}

	// Resource metrics
	s.resourceReadCounter, err = s.meter.Int64Counter("mcp.resource.reads",
		metric.WithDescription("Number of resource read operations"),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating resource read counter: %v", err)
	}

	s.resourceDuration, err = s.meter.Float64Histogram("mcp.resource.duration",
		metric.WithDescription("Duration of resource operations"),
		metric.WithUnit("ms"))
	if err != nil {
		debugLog("Error creating resource duration histogram: %v", err)
	}

	s.resourceErrorCounter, err = s.meter.Int64Counter("mcp.resource.errors",
		metric.WithDescription("Number of resource operation errors"),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating resource error counter: %v", err)
	}

	s.resourcesDiscovered, err = s.meter.Int64Gauge("mcp.resources.discovered",
		metric.WithDescription("Number of resources discovered from servers"),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating resources discovered gauge: %v", err)
	}

	s.listResourcesCounter, err = s.meter.Int64Counter("mcp.list.resources",
		metric.WithDescription("Number of list resources calls"),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating list resources counter: %v", err)
	}

	// Resource template metrics
	s.resourceTemplateReadCounter, err = s.meter.Int64Counter("mcp.resource_template.reads",
		metric.WithDescription("Number of resource template read operations"),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating resource template read counter: %v", err)
	}

	s.resourceTemplateDuration, err = s.meter.Float64Histogram("mcp.resource_template.duration",
		metric.WithDescription("Duration of resource template operations"),
		metric.WithUnit("ms"))
	if err != nil {
		debugLog("Error creating resource template duration histogram: %v", err)
	}

	s.resourceTemplateErrorCounter, err = s.meter.Int64Counter("mcp.resource_template.errors",
		metric.WithDescription("Number of resource template operation errors"),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating resource template error counter: %v", err)
	}

	s.resourceTemplatesDiscovered, err = s.meter.Int64Gauge("mcp.resource_templates.discovered",
		metric.WithDescription("Number of resource templates discovered from servers"),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating resource templates discovered gauge: %v", err)
	}

	s.listResourceTemplatesCounter, err = s.meter.Int64Counter("mcp.list.resource_templates",
		metric.WithDescription("Number of list resource templates calls"),
		metric.WithUnit("1"))
	if err != nil {
		debugLog("Error creating list resource templates counter: %v", err)
	}
}

func (s *Server) registerTools() {
	// Record gateway start
	type RecordGatewayStartArgs struct {
		TransportMode string `json:"transport_mode"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-gateway-start",
		Description: "Record a gateway start event",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordGatewayStartArgs) (*mcp.CallToolResult, any, error) {
		if s.gatewayStartCounter != nil {
			s.gatewayStartCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("mcp.gateway.transport", args.TransportMode),
			))
		}
		debugLog("Recorded gateway start: transport=%s", args.TransportMode)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record initialize
	type RecordInitializeArgs struct {
		ClientName    string `json:"client_name"`
		ClientVersion string `json:"client_version"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-initialize",
		Description: "Record an initialize event",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordInitializeArgs) (*mcp.CallToolResult, any, error) {
		if s.initializeCounter != nil {
			s.initializeCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("mcp.client.name", args.ClientName),
				attribute.String("mcp.client.version", args.ClientVersion),
			))
		}
		debugLog("Recorded initialize: client=%s@%s", args.ClientName, args.ClientVersion)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record tool call
	type RecordToolCallArgs struct {
		ServerName string `json:"server_name"`
		ServerType string `json:"server_type"`
		ToolName   string `json:"tool_name"`
		ClientName string `json:"client_name"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-tool-call",
		Description: "Record a tool call event",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordToolCallArgs) (*mcp.CallToolResult, any, error) {
		if s.toolCallCounter != nil {
			s.toolCallCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("mcp.server.name", args.ServerName),
				attribute.String("mcp.server.type", args.ServerType),
				attribute.String("mcp.tool.name", args.ToolName),
				attribute.String("mcp.client.name", args.ClientName),
			))
		}
		debugLog("Recorded tool call: %s on %s", args.ToolName, args.ServerName)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record tool duration
	type RecordToolDurationArgs struct {
		ServerName string  `json:"server_name"`
		ServerType string  `json:"server_type"`
		ToolName   string  `json:"tool_name"`
		ClientName string  `json:"client_name"`
		DurationMs float64 `json:"duration_ms"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-tool-duration",
		Description: "Record tool call duration",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordToolDurationArgs) (*mcp.CallToolResult, any, error) {
		if s.toolCallDuration != nil {
			s.toolCallDuration.Record(ctx, args.DurationMs, metric.WithAttributes(
				attribute.String("mcp.server.name", args.ServerName),
				attribute.String("mcp.server.type", args.ServerType),
				attribute.String("mcp.tool.name", args.ToolName),
				attribute.String("mcp.client.name", args.ClientName),
			))
		}
		debugLog("Recorded tool duration: %s took %.2fms", args.ToolName, args.DurationMs)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record tool error
	type RecordToolErrorArgs struct {
		ServerName string `json:"server_name"`
		ServerType string `json:"server_type"`
		ToolName   string `json:"tool_name"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-tool-error",
		Description: "Record a tool call error",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordToolErrorArgs) (*mcp.CallToolResult, any, error) {
		if s.toolErrorCounter != nil {
			s.toolErrorCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("mcp.server.name", args.ServerName),
				attribute.String("mcp.server.type", args.ServerType),
				attribute.String("mcp.tool.name", args.ToolName),
			))
		}
		debugLog("Recorded tool error: %s on %s", args.ToolName, args.ServerName)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record list tools
	type RecordListToolsArgs struct {
		ClientName string `json:"client_name"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-list-tools",
		Description: "Record a list tools call",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordListToolsArgs) (*mcp.CallToolResult, any, error) {
		if s.listToolsCounter != nil {
			s.listToolsCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("mcp.client.name", args.ClientName),
			))
		}
		debugLog("Recorded list tools from %s", args.ClientName)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record tools discovered
	type RecordToolsDiscoveredArgs struct {
		ServerName string `json:"server_name"`
		ToolCount  int64  `json:"tool_count"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-tools-discovered",
		Description: "Record number of tools discovered from a server",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordToolsDiscoveredArgs) (*mcp.CallToolResult, any, error) {
		if s.toolsDiscovered != nil {
			s.toolsDiscovered.Record(ctx, args.ToolCount, metric.WithAttributes(
				attribute.String("mcp.server.origin", args.ServerName),
			))
		}
		debugLog("Recorded %d tools discovered from %s", args.ToolCount, args.ServerName)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record prompt get
	type RecordPromptGetArgs struct {
		PromptName string `json:"prompt_name"`
		ServerName string `json:"server_name"`
		ClientName string `json:"client_name"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-prompt-get",
		Description: "Record a prompt get operation",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordPromptGetArgs) (*mcp.CallToolResult, any, error) {
		if s.promptGetCounter != nil {
			s.promptGetCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("mcp.prompt.name", args.PromptName),
				attribute.String("mcp.server.origin", args.ServerName),
				attribute.String("mcp.client.name", args.ClientName),
			))
		}
		debugLog("Recorded prompt get: %s from %s", args.PromptName, args.ServerName)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record prompt duration
	type RecordPromptDurationArgs struct {
		PromptName string  `json:"prompt_name"`
		ServerName string  `json:"server_name"`
		ClientName string  `json:"client_name"`
		DurationMs float64 `json:"duration_ms"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-prompt-duration",
		Description: "Record prompt operation duration",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordPromptDurationArgs) (*mcp.CallToolResult, any, error) {
		if s.promptDuration != nil {
			s.promptDuration.Record(ctx, args.DurationMs, metric.WithAttributes(
				attribute.String("mcp.prompt.name", args.PromptName),
				attribute.String("mcp.server.origin", args.ServerName),
				attribute.String("mcp.client.name", args.ClientName),
			))
		}
		debugLog("Recorded prompt duration: %s took %.2fms", args.PromptName, args.DurationMs)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record prompt error
	type RecordPromptErrorArgs struct {
		PromptName string `json:"prompt_name"`
		ServerName string `json:"server_name"`
		ErrorType  string `json:"error_type"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-prompt-error",
		Description: "Record a prompt operation error",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordPromptErrorArgs) (*mcp.CallToolResult, any, error) {
		if s.promptErrorCounter != nil {
			s.promptErrorCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("mcp.prompt.name", args.PromptName),
				attribute.String("mcp.server.origin", args.ServerName),
				attribute.String("mcp.error.type", args.ErrorType),
			))
		}
		debugLog("Recorded prompt error: %s from %s", args.PromptName, args.ServerName)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record list prompts
	type RecordListPromptsArgs struct {
		ClientName string `json:"client_name"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-list-prompts",
		Description: "Record a list prompts call",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordListPromptsArgs) (*mcp.CallToolResult, any, error) {
		if s.listPromptsCounter != nil {
			s.listPromptsCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("mcp.client.name", args.ClientName),
			))
		}
		debugLog("Recorded list prompts from %s", args.ClientName)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record prompts discovered
	type RecordPromptsDiscoveredArgs struct {
		ServerName  string `json:"server_name"`
		PromptCount int64  `json:"prompt_count"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-prompts-discovered",
		Description: "Record number of prompts discovered from a server",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordPromptsDiscoveredArgs) (*mcp.CallToolResult, any, error) {
		if s.promptsDiscovered != nil {
			s.promptsDiscovered.Record(ctx, args.PromptCount, metric.WithAttributes(
				attribute.String("mcp.server.origin", args.ServerName),
			))
		}
		debugLog("Recorded %d prompts discovered from %s", args.PromptCount, args.ServerName)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record resource read
	type RecordResourceReadArgs struct {
		ResourceURI string `json:"resource_uri"`
		ServerName  string `json:"server_name"`
		ClientName  string `json:"client_name"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-resource-read",
		Description: "Record a resource read operation",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordResourceReadArgs) (*mcp.CallToolResult, any, error) {
		if s.resourceReadCounter != nil {
			s.resourceReadCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("mcp.resource.uri", args.ResourceURI),
				attribute.String("mcp.server.origin", args.ServerName),
				attribute.String("mcp.client.name", args.ClientName),
			))
		}
		debugLog("Recorded resource read: %s from %s", args.ResourceURI, args.ServerName)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record resource duration
	type RecordResourceDurationArgs struct {
		ResourceURI string  `json:"resource_uri"`
		ServerName  string  `json:"server_name"`
		ClientName  string  `json:"client_name"`
		DurationMs  float64 `json:"duration_ms"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-resource-duration",
		Description: "Record resource operation duration",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordResourceDurationArgs) (*mcp.CallToolResult, any, error) {
		if s.resourceDuration != nil {
			s.resourceDuration.Record(ctx, args.DurationMs, metric.WithAttributes(
				attribute.String("mcp.resource.uri", args.ResourceURI),
				attribute.String("mcp.server.origin", args.ServerName),
				attribute.String("mcp.client.name", args.ClientName),
			))
		}
		debugLog("Recorded resource duration: %s took %.2fms", args.ResourceURI, args.DurationMs)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record resource error
	type RecordResourceErrorArgs struct {
		ResourceURI string `json:"resource_uri"`
		ServerName  string `json:"server_name"`
		ErrorType   string `json:"error_type"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-resource-error",
		Description: "Record a resource operation error",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordResourceErrorArgs) (*mcp.CallToolResult, any, error) {
		if s.resourceErrorCounter != nil {
			s.resourceErrorCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("mcp.resource.uri", args.ResourceURI),
				attribute.String("mcp.server.origin", args.ServerName),
				attribute.String("mcp.error.type", args.ErrorType),
			))
		}
		debugLog("Recorded resource error: %s from %s", args.ResourceURI, args.ServerName)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record list resources
	type RecordListResourcesArgs struct {
		ClientName string `json:"client_name"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-list-resources",
		Description: "Record a list resources call",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordListResourcesArgs) (*mcp.CallToolResult, any, error) {
		if s.listResourcesCounter != nil {
			s.listResourcesCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("mcp.client.name", args.ClientName),
			))
		}
		debugLog("Recorded list resources from %s", args.ClientName)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record resources discovered
	type RecordResourcesDiscoveredArgs struct {
		ServerName    string `json:"server_name"`
		ResourceCount int64  `json:"resource_count"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-resources-discovered",
		Description: "Record number of resources discovered from a server",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordResourcesDiscoveredArgs) (*mcp.CallToolResult, any, error) {
		if s.resourcesDiscovered != nil {
			s.resourcesDiscovered.Record(ctx, args.ResourceCount, metric.WithAttributes(
				attribute.String("mcp.server.origin", args.ServerName),
			))
		}
		debugLog("Recorded %d resources discovered from %s", args.ResourceCount, args.ServerName)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record list resource templates
	type RecordListResourceTemplatesArgs struct {
		ClientName string `json:"client_name"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-list-resource-templates",
		Description: "Record a list resource templates call",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordListResourceTemplatesArgs) (*mcp.CallToolResult, any, error) {
		if s.listResourceTemplatesCounter != nil {
			s.listResourceTemplatesCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("mcp.client.name", args.ClientName),
			))
		}
		debugLog("Recorded list resource templates from %s", args.ClientName)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record resource template read
	type RecordResourceTemplateReadArgs struct {
		URITemplate string `json:"uri_template"`
		ServerName  string `json:"server_name"`
		ClientName  string `json:"client_name"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-resource-template-read",
		Description: "Record a resource template read operation",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordResourceTemplateReadArgs) (*mcp.CallToolResult, any, error) {
		if s.resourceTemplateReadCounter != nil {
			s.resourceTemplateReadCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("mcp.resource_template.uri", args.URITemplate),
				attribute.String("mcp.server.origin", args.ServerName),
				attribute.String("mcp.client.name", args.ClientName),
			))
		}
		debugLog("Recorded resource template read: %s from %s", args.URITemplate, args.ServerName)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record resource template duration
	type RecordResourceTemplateDurationArgs struct {
		URITemplate string  `json:"uri_template"`
		ServerName  string  `json:"server_name"`
		ClientName  string  `json:"client_name"`
		DurationMs  float64 `json:"duration_ms"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-resource-template-duration",
		Description: "Record resource template operation duration",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordResourceTemplateDurationArgs) (*mcp.CallToolResult, any, error) {
		if s.resourceTemplateDuration != nil {
			s.resourceTemplateDuration.Record(ctx, args.DurationMs, metric.WithAttributes(
				attribute.String("mcp.resource_template.uri", args.URITemplate),
				attribute.String("mcp.server.origin", args.ServerName),
				attribute.String("mcp.client.name", args.ClientName),
			))
		}
		debugLog("Recorded resource template duration: %s took %.2fms", args.URITemplate, args.DurationMs)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record resource template error
	type RecordResourceTemplateErrorArgs struct {
		URITemplate string `json:"uri_template"`
		ServerName  string `json:"server_name"`
		ErrorType   string `json:"error_type"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-resource-template-error",
		Description: "Record a resource template operation error",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordResourceTemplateErrorArgs) (*mcp.CallToolResult, any, error) {
		if s.resourceTemplateErrorCounter != nil {
			s.resourceTemplateErrorCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("mcp.resource_template.uri", args.URITemplate),
				attribute.String("mcp.server.origin", args.ServerName),
				attribute.String("mcp.error.type", args.ErrorType),
			))
		}
		debugLog("Recorded resource template error: %s from %s", args.URITemplate, args.ServerName)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record resource templates discovered
	type RecordResourceTemplatesDiscoveredArgs struct {
		ServerName    string `json:"server_name"`
		TemplateCount int64  `json:"template_count"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-resource-templates-discovered",
		Description: "Record number of resource templates discovered from a server",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordResourceTemplatesDiscoveredArgs) (*mcp.CallToolResult, any, error) {
		if s.resourceTemplatesDiscovered != nil {
			s.resourceTemplatesDiscovered.Record(ctx, args.TemplateCount, metric.WithAttributes(
				attribute.String("mcp.server.origin", args.ServerName),
			))
		}
		debugLog("Recorded %d resource templates discovered from %s", args.TemplateCount, args.ServerName)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record catalog operation
	type RecordCatalogOperationArgs struct {
		Operation   string  `json:"operation"`
		CatalogName string  `json:"catalog_name"`
		DurationMs  float64 `json:"duration_ms"`
		Success     bool    `json:"success"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-catalog-operation",
		Description: "Record a catalog operation",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordCatalogOperationArgs) (*mcp.CallToolResult, any, error) {
		attrs := []attribute.KeyValue{
			attribute.String("mcp.catalog.operation", args.Operation),
			attribute.String("mcp.catalog.name", args.CatalogName),
			attribute.Bool("mcp.catalog.success", args.Success),
		}
		if s.catalogOperationsCounter != nil {
			s.catalogOperationsCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
		}
		if s.catalogOperationDuration != nil {
			s.catalogOperationDuration.Record(ctx, args.DurationMs, metric.WithAttributes(attrs...))
		}
		debugLog("Recorded catalog operation: %s on %s", args.Operation, args.CatalogName)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "recorded"}}}, nil, nil
	})

	// Record catalog servers
	type RecordCatalogServersArgs struct {
		CatalogName string `json:"catalog_name"`
		ServerCount int64  `json:"server_count"`
	}
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "record-catalog-servers",
		Description: "Record number of servers in a catalog",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args RecordCatalogServersArgs) (*mcp.CallToolResult, any, error) {
		if s.catalogServersGauge != nil {
			s.catalogServersGauge.Record(ctx, args.ServerCount, metric.WithAttributes(
				attribute.String("mcp.catalog.name", args.CatalogName),
			))
		}
		debugLog("Recorded %d servers in catalog %s", args.ServerCount, args.CatalogName)
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
		fmt.Fprintf(os.Stderr, "[TELEMETRY-SERVER] "+format+"\n", args...)
	}
}
