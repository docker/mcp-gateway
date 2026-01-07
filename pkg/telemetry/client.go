package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"
)

var (
	// mcpClient is the MCP client for telemetry recording
	mcpClient     *mcp.Client
	mcpSession    *mcp.ClientSession
	mcpClientOnce sync.Once
	mcpClientMu   sync.RWMutex
)

// InitMCPClient initializes the MCP client for telemetry recording.
func InitMCPClient(ctx context.Context, host string, port int) error {
	var initErr error
	mcpClientOnce.Do(func() {
		endpoint := fmt.Sprintf("http://%s:%d/mcp", host, port)

		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY-CLIENT] Connecting to telemetry server at %s\n", endpoint)
		}

		transport := &mcp.StreamableClientTransport{
			Endpoint: endpoint,
		}

		client := mcp.NewClient(&mcp.Implementation{
			Name:    "mcp-gateway-telemetry-client",
			Version: "1.0.0",
		}, nil)

		session, err := client.Connect(ctx, transport, nil)
		if err != nil {
			initErr = fmt.Errorf("failed to connect to telemetry server: %w", err)
			return
		}

		mcpClientMu.Lock()
		mcpClient = client
		mcpSession = session
		mcpClientMu.Unlock()

		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY-CLIENT] Connected to telemetry server\n")
		}
	})

	return initErr
}

// IsMCPClientInitialized returns true if the MCP client is initialized
func IsMCPClientInitialized() bool {
	mcpClientMu.RLock()
	defer mcpClientMu.RUnlock()
	return mcpSession != nil
}

// CloseMCPClient closes the MCP client connection
func CloseMCPClient() error {
	mcpClientMu.Lock()
	defer mcpClientMu.Unlock()

	if mcpSession != nil {
		err := mcpSession.Close()
		mcpSession = nil
		mcpClient = nil
		return err
	}
	return nil
}

// callTool makes an MCP tool call to the telemetry server
func callTool(ctx context.Context, toolName string, args any) error {
	mcpClientMu.RLock()
	session := mcpSession
	mcpClientMu.RUnlock()

	if session == nil {
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY-CLIENT] Session not initialized, skipping %s\n", toolName)
		}
		return nil // Silently skip if not initialized
	}

	argsMap, ok := args.(map[string]any)
	if !ok {
		// Convert struct to map
		data, err := json.Marshal(args)
		if err != nil {
			return fmt.Errorf("failed to marshal args: %w", err)
		}
		if err := json.Unmarshal(data, &argsMap); err != nil {
			return fmt.Errorf("failed to unmarshal args to map: %w", err)
		}
	}

	_, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: argsMap,
	})

	if err != nil && os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[MCP-TELEMETRY-CLIENT] Error calling tool %s: %v\n", toolName, err)
	}

	return err
}

// recordGatewayStart records a gateway start event
func recordGatewayStart(ctx context.Context, transportMode string) {
	_ = callTool(ctx, "record-gateway-start", map[string]any{
		"transport_mode": transportMode,
	})
}

// recordInitialize records an initialize event
func recordInitialize(ctx context.Context, clientName, clientVersion string) {
	_ = callTool(ctx, "record-initialize", map[string]any{
		"client_name":    clientName,
		"client_version": clientVersion,
	})
}

// recordToolCall records a tool call event
func recordToolCall(ctx context.Context, serverName, serverType, toolName, clientName string) {
	_ = callTool(ctx, "record-tool-call", map[string]any{
		"server_name": serverName,
		"server_type": serverType,
		"tool_name":   toolName,
		"client_name": clientName,
	})
}

// recordToolDuration records tool call duration
func recordToolDuration(ctx context.Context, serverName, serverType, toolName, clientName string, durationMs float64) {
	_ = callTool(ctx, "record-tool-duration", map[string]any{
		"server_name": serverName,
		"server_type": serverType,
		"tool_name":   toolName,
		"client_name": clientName,
		"duration_ms": durationMs,
	})
}

// recordToolError records a tool call error
func recordToolError(ctx context.Context, span trace.Span, serverName, serverType, toolName string) {
	// Record error in span if provided (keeping local tracing)
	if span != nil {
		span.RecordError(nil)
	}

	_ = callTool(ctx, "record-tool-error", map[string]any{
		"server_name": serverName,
		"server_type": serverType,
		"tool_name":   toolName,
	})
}

// recordListTools records a list tools call
func recordListTools(ctx context.Context, clientName string) {
	_ = callTool(ctx, "record-list-tools", map[string]any{
		"client_name": clientName,
	})
}

// recordToolsDiscovered records the number of tools discovered
func recordToolsDiscovered(ctx context.Context, serverName string, toolCount int) {
	_ = callTool(ctx, "record-tools-discovered", map[string]any{
		"server_name": serverName,
		"tool_count":  int64(toolCount),
	})
}

// recordPromptGet records a prompt get operation
func recordPromptGet(ctx context.Context, promptName, serverName, clientName string) {
	_ = callTool(ctx, "record-prompt-get", map[string]any{
		"prompt_name": promptName,
		"server_name": serverName,
		"client_name": clientName,
	})
}

// recordPromptDuration records prompt operation duration
func recordPromptDuration(ctx context.Context, promptName, serverName string, durationMs float64, clientName string) {
	_ = callTool(ctx, "record-prompt-duration", map[string]any{
		"prompt_name": promptName,
		"server_name": serverName,
		"client_name": clientName,
		"duration_ms": durationMs,
	})
}

// recordPromptError records a prompt operation error
func recordPromptError(ctx context.Context, promptName, serverName, errorType string) {
	_ = callTool(ctx, "record-prompt-error", map[string]any{
		"prompt_name": promptName,
		"server_name": serverName,
		"error_type":  errorType,
	})
}

// recordListPrompts records a list prompts call
func recordListPrompts(ctx context.Context, clientName string) {
	_ = callTool(ctx, "record-list-prompts", map[string]any{
		"client_name": clientName,
	})
}

// recordPromptsDiscovered records the number of prompts discovered
func recordPromptsDiscovered(ctx context.Context, serverName string, promptCount int) {
	_ = callTool(ctx, "record-prompts-discovered", map[string]any{
		"server_name":  serverName,
		"prompt_count": int64(promptCount),
	})
}

// recordResourceRead records a resource read operation
func recordResourceRead(ctx context.Context, resourceURI, serverName, clientName string) {
	_ = callTool(ctx, "record-resource-read", map[string]any{
		"resource_uri": resourceURI,
		"server_name":  serverName,
		"client_name":  clientName,
	})
}

// recordResourceDuration records resource operation duration
func recordResourceDuration(ctx context.Context, resourceURI, serverName string, durationMs float64, clientName string) {
	_ = callTool(ctx, "record-resource-duration", map[string]any{
		"resource_uri": resourceURI,
		"server_name":  serverName,
		"client_name":  clientName,
		"duration_ms":  durationMs,
	})
}

// recordResourceError records a resource operation error
func recordResourceError(ctx context.Context, resourceURI, serverName, errorType string) {
	_ = callTool(ctx, "record-resource-error", map[string]any{
		"resource_uri": resourceURI,
		"server_name":  serverName,
		"error_type":   errorType,
	})
}

// recordListResources records a list resources call
func recordListResources(ctx context.Context, clientName string) {
	_ = callTool(ctx, "record-list-resources", map[string]any{
		"client_name": clientName,
	})
}

// recordResourcesDiscovered records the number of resources discovered
func recordResourcesDiscovered(ctx context.Context, serverName string, resourceCount int) {
	_ = callTool(ctx, "record-resources-discovered", map[string]any{
		"server_name":    serverName,
		"resource_count": int64(resourceCount),
	})
}

// recordListResourceTemplates records a list resource templates call
func recordListResourceTemplates(ctx context.Context, clientName string) {
	_ = callTool(ctx, "record-list-resource-templates", map[string]any{
		"client_name": clientName,
	})
}

// recordResourceTemplateRead records a resource template read operation
func recordResourceTemplateRead(ctx context.Context, uriTemplate, serverName, clientName string) {
	_ = callTool(ctx, "record-resource-template-read", map[string]any{
		"uri_template": uriTemplate,
		"server_name":  serverName,
		"client_name":  clientName,
	})
}

// recordResourceTemplateDuration records resource template operation duration
func recordResourceTemplateDuration(ctx context.Context, uriTemplate, serverName string, durationMs float64, clientName string) {
	_ = callTool(ctx, "record-resource-template-duration", map[string]any{
		"uri_template": uriTemplate,
		"server_name":  serverName,
		"client_name":  clientName,
		"duration_ms":  durationMs,
	})
}

// recordResourceTemplateError records a resource template operation error
func recordResourceTemplateError(ctx context.Context, uriTemplate, serverName, errorType string) {
	_ = callTool(ctx, "record-resource-template-error", map[string]any{
		"uri_template": uriTemplate,
		"server_name":  serverName,
		"error_type":   errorType,
	})
}

// recordResourceTemplatesDiscovered records the number of resource templates discovered
func recordResourceTemplatesDiscovered(ctx context.Context, serverName string, templateCount int) {
	_ = callTool(ctx, "record-resource-templates-discovered", map[string]any{
		"server_name":    serverName,
		"template_count": int64(templateCount),
	})
}

// recordCatalogOperation records a catalog operation
func recordCatalogOperation(ctx context.Context, operation, catalogName string, durationMs float64, success bool) {
	_ = callTool(ctx, "record-catalog-operation", map[string]any{
		"operation":    operation,
		"catalog_name": catalogName,
		"duration_ms":  durationMs,
		"success":      success,
	})
}

// recordCatalogServers records the number of servers in a catalog
func recordCatalogServers(ctx context.Context, catalogName string, serverCount int64) {
	_ = callTool(ctx, "record-catalog-servers", map[string]any{
		"catalog_name": catalogName,
		"server_count": serverCount,
	})
}
