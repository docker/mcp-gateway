package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/codemode"
	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/telemetry"
)

// mcpFindTool implements a tool for finding MCP servers in the catalog
func (g *Gateway) createMcpFindTool(_ Configuration, handler mcp.ToolHandler) *ToolRegistration {
	tool := &mcp.Tool{
		Name:        "mcp-find",
		Description: "Find MCP servers in the current catalog by name, title, or description. If the user is looking for new capabilities, use this tool to search the MCP catalog for servers that should potentially be enabled.  This will not enable the server but will return information about servers that could be enabled. If we find an mcp server, it can be added with the mcp-add tool, and configured with mcp-config-set.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"prompt": {
					Type:        "string",
					Description: "describe the use case that might benefit from adding a new MCP server.",
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of results to return (default: 10)",
				},
			},
			Required: []string{"query"},
		},
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: withToolTelemetry("mcp-find", handler),
	}
}

func (g *Gateway) createCodeModeTool(_ *clientConfig) *ToolRegistration {
	tool := &mcp.Tool{
		Name: "code-mode",
		Description: `Create a JavaScript-enabled tool that combines multiple MCP server tools. 
This allows you to write scripts that call multiple tools and combine their results.
Use the mcp-find tool to find servers and make sure they are are ready with the mcp-add tool. When running
mcp-add, we don't have to activate the tools.
`,
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"servers": {
					Type:        "array",
					Description: "List of MCP server names whose tools should be available in the JavaScript environment",
					Items: &jsonschema.Schema{
						Type: "string",
					},
				},
				"name": {
					Type:        "string",
					Description: "Name for the new code-mode tool (will be prefixed with 'code-mode-')",
				},
			},
			Required: []string{"servers", "name"},
		},
	}
	handler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse parameters
		var params struct {
			Servers []string `json:"servers"`
			Name    string   `json:"name"`
		}

		if req.Params.Arguments == nil {
			return nil, fmt.Errorf("missing arguments")
		}

		paramsBytes, err := json.Marshal(req.Params.Arguments)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal arguments: %w", err)
		}

		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return nil, fmt.Errorf("failed to parse arguments: %w", err)
		}

		if len(params.Servers) == 0 {
			return nil, fmt.Errorf("servers parameter is required and must not be empty")
		}

		if params.Name == "" {
			return nil, fmt.Errorf("name parameter is required")
		}

		// Validate that all requested servers exist
		for _, serverName := range params.Servers {
			if _, _, found := g.configuration.Find(serverName); !found {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{
						Text: fmt.Sprintf("Error: Server '%s' not found in configuration. Use mcp-find to search for available servers.", serverName),
					}},
				}, nil
			}
		}

		// Create a tool set adapter for each server
		var toolSets []codemode.ToolSet
		for _, serverName := range params.Servers {
			serverConfig, _, _ := g.configuration.Find(serverName)
			toolSets = append(toolSets, &serverToolSetAdapter{
				gateway:      g,
				serverName:   serverName,
				serverConfig: serverConfig,
				session:      req.Session,
			})
		}

		// Wrap the tool sets with codemode
		wrappedToolSet := codemode.Wrap(toolSets)

		// Get the generated tool from the wrapped toolset
		tools, err := wrappedToolSet.Tools(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create code-mode tools: %w", err)
		}

		// Use the first tool (the JavaScript execution tool with all servers' tools available)
		if len(tools) == 0 {
			return nil, fmt.Errorf("no tools generated from wrapped toolset")
		}

		customTool := tools[0]
		toolName := fmt.Sprintf("code-mode-%s", params.Name)

		// Customize the tool name and description
		customTool.Tool.Name = toolName

		// Add the tool to the gateway's MCP server
		g.mcpServer.AddTool(customTool.Tool, customTool.Handler)

		// Track the tool registration for capabilities and mcp-exec
		g.capabilitiesMu.Lock()
		g.toolRegistrations[toolName] = ToolRegistration{
			ServerName: "code-mode",
			Tool:       customTool.Tool,
			Handler:    customTool.Handler,
		}
		g.capabilitiesMu.Unlock()

		// Build detailed response with tool information
		var responseText strings.Builder
		responseText.WriteString(fmt.Sprintf("Successfully created code-mode tool '%s'\n\n", toolName))

		// Tool description
		responseText.WriteString("## Tool Details\n")
		responseText.WriteString(fmt.Sprintf("**Name:** %s\n", toolName))
		responseText.WriteString(fmt.Sprintf("**Description:** %s\n\n", customTool.Tool.Description))

		// Input schema information
		responseText.WriteString("## Input Schema\n")
		if customTool.Tool.InputSchema != nil {
			schemaJSON, err := json.MarshalIndent(customTool.Tool.InputSchema, "", "  ")
			if err == nil {
				responseText.WriteString("```json\n")
				responseText.WriteString(string(schemaJSON))
				responseText.WriteString("\n```\n\n")
			}
		}

		// Available servers
		responseText.WriteString("## Available Servers\n")
		responseText.WriteString(fmt.Sprintf("This tool has access to tools from: %s\n\n", strings.Join(params.Servers, ", ")))

		// Usage instructions
		responseText.WriteString("## How to Use\n")
		responseText.WriteString("You can call this tool using the **mcp-exec** tool:\n")
		responseText.WriteString("```json\n")
		responseText.WriteString("{\n")
		responseText.WriteString(fmt.Sprintf("  \"name\": \"%s\",\n", toolName))
		responseText.WriteString("  \"arguments\": {\n")
		responseText.WriteString("    \"script\": \"<your JavaScript code here>\"\n")
		responseText.WriteString("  }\n")
		responseText.WriteString("}\n")
		responseText.WriteString("```\n\n")
		responseText.WriteString("The tool is now available in your session and can be executed via mcp-exec.")

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Text: responseText.String(),
			}},
		}, nil
	}
	return &ToolRegistration{
		Tool:    tool,
		Handler: withToolTelemetry("code-mode", handler),
	}
}

// serverToolSetAdapter adapts a gateway server to the codemode.ToolSet interface
type serverToolSetAdapter struct {
	gateway      *Gateway
	serverName   string
	serverConfig *catalog.ServerConfig
	session      *mcp.ServerSession
}

func (a *serverToolSetAdapter) Tools(ctx context.Context) ([]*codemode.ToolWithHandler, error) {
	// Get a client for this server
	clientConfig := &clientConfig{
		serverSession: a.session,
		server:        a.gateway.mcpServer,
	}

	client, err := a.gateway.clientPool.AcquireClient(ctx, a.serverConfig, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire client for server %s: %w", a.serverName, err)
	}

	// List tools from the server
	listResult, err := client.Session().ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tools from server %s: %w", a.serverName, err)
	}

	// Convert MCP tools to ToolWithHandler
	var result []*codemode.ToolWithHandler
	for _, tool := range listResult.Tools {
		// Create a handler that calls the tool on the remote server
		handler := func(tool *mcp.Tool) mcp.ToolHandler {
			return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				// Forward the tool call to the actual server
				return client.Session().CallTool(ctx, &mcp.CallToolParams{
					Name:      tool.Name,
					Arguments: req.Params.Arguments,
				})
			}
		}(tool)

		result = append(result, &codemode.ToolWithHandler{
			Tool:    tool,
			Handler: handler,
		})
	}

	return result, nil
}

// mcpRemoveTool implements a tool for removing servers from the registry
func (g *Gateway) createMcpRemoveTool() *ToolRegistration {
	tool := &mcp.Tool{
		Name:        "mcp-remove",
		Description: "Remove an MCP server from the registry and reload the configuration. This will disable the server.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Name of the MCP server to remove from the registry",
				},
			},
			Required: []string{"name"},
		},
	}

	handler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse parameters
		var params struct {
			Name string `json:"name"`
		}

		if req.Params.Arguments == nil {
			return nil, fmt.Errorf("missing arguments")
		}

		paramsBytes, err := json.Marshal(req.Params.Arguments)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal arguments: %w", err)
		}

		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return nil, fmt.Errorf("failed to parse arguments: %w", err)
		}

		if params.Name == "" {
			return nil, fmt.Errorf("name parameter is required")
		}

		serverName := strings.TrimSpace(params.Name)

		// Remove the server from the current serverNames
		updatedServerNames := slices.DeleteFunc(slices.Clone(g.configuration.serverNames), func(name string) bool {
			return name == serverName
		})

		// Update the current configuration state
		g.configuration.serverNames = updatedServerNames

		// Stop OAuth provider if this is an OAuth server
		if g.McpOAuthDcrEnabled {
			g.stopProvider(serverName)
		}

		if err := g.removeServerConfiguration(ctx, serverName); err != nil {
			return nil, fmt.Errorf("failed to remove server configuration: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Text: fmt.Sprintf("Successfully removed server '%s'.", serverName),
			}},
		}, nil
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: withToolTelemetry("mcp-remove", handler),
	}
}

//nolint:unused
func (g *Gateway) createMcpRegistryImportTool(configuration Configuration, _ *clientConfig) *ToolRegistration {
	tool := &mcp.Tool{
		Name:        "mcp-registry-import",
		Description: "Import MCP servers from an MCP registry URL. Fetches server definitions via HTTP GET and adds them to the local catalog.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"url": {
					Type:        "string",
					Description: "URL to fetch the server details JSON (must be a valid HTTP/HTTPS URL)",
				},
			},
			Required: []string{"url"},
		},
	}

	handler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse parameters
		var params struct {
			URL string `json:"url"`
		}

		if req.Params.Arguments == nil {
			return nil, fmt.Errorf("missing arguments")
		}

		paramsBytes, err := json.Marshal(req.Params.Arguments)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal arguments: %w", err)
		}

		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return nil, fmt.Errorf("failed to parse arguments: %w", err)
		}

		if params.URL == "" {
			return nil, fmt.Errorf("url parameter is required")
		}

		registryURL := strings.TrimSpace(params.URL)

		// Validate URL scheme
		if !strings.HasPrefix(registryURL, "http://") && !strings.HasPrefix(registryURL, "https://") {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf("Error: URL must start with http:// or https://, got: %s", registryURL),
				}},
			}, nil
		}

		// Fetch servers from the URL
		servers, err := g.readServersFromURL(ctx, registryURL)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf("Error fetching servers from URL %s: %v", registryURL, err),
				}},
			}, nil
		}

		if len(servers) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf("No servers found at URL: %s", registryURL),
				}},
			}, nil
		}

		// Add the imported servers to the current configuration and build detailed summary
		var importedServerNames []string
		var serverSummaries []string

		for serverName, server := range servers {
			if _, exists := configuration.servers[serverName]; exists {
				log.Log(fmt.Sprintf("Warning: server '%s' from URL %s overwrites existing server", serverName, registryURL))
			}
			configuration.servers[serverName] = server
			importedServerNames = append(importedServerNames, serverName)

			// Build detailed summary for this server
			summary := fmt.Sprintf("• %s", serverName)

			if server.Description != "" {
				summary += fmt.Sprintf("\n  Description: %s", server.Description)
			}

			if server.Image != "" {
				summary += fmt.Sprintf("\n  Image: %s", server.Image)
			}

			// List required secrets
			if len(server.Secrets) > 0 {
				var secretNames []string
				for _, secret := range server.Secrets {
					secretNames = append(secretNames, secret.Name)
				}
				summary += fmt.Sprintf("\n  Required Secrets: %s", strings.Join(secretNames, ", "))
				summary += "\n  ⚠️  Configure these secrets before using this server"
			}

			// List configuration schemas available
			if len(server.Config) > 0 {
				summary += fmt.Sprintf("\n  Configuration Schemas: %d available", len(server.Config))
				summary += "\n  ℹ️  Use mcp-config-set to configure optional settings"
			}

			if server.LongLived {
				summary += "\n  🔄 Long-lived server (stays running)"
			}

			serverSummaries = append(serverSummaries, summary)
		}

		// Create comprehensive result message
		resultText := fmt.Sprintf("Successfully imported %d servers from %s\n\n", len(importedServerNames), registryURL)
		resultText += strings.Join(serverSummaries, "\n\n")

		if len(importedServerNames) > 0 {
			resultText += fmt.Sprintf("\n\n✅ Servers ready to use: %s", strings.Join(importedServerNames, ", "))
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Text: resultText,
			}},
		}, nil
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: withToolTelemetry("mcp-registry-import", handler),
	}
}

// readServersFromURL fetches and parses server definitions from a URL
//
//nolint:unused
func (g *Gateway) readServersFromURL(ctx context.Context, url string) (map[string]catalog.Server, error) {
	servers := make(map[string]catalog.Server)

	log.Log(fmt.Sprintf("  - Reading servers from URL: %s", url))

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set a reasonable user agent
	req.Header.Set("User-Agent", "docker-mcp-gateway/1.0.0")

	// Make the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Try to parse as oci.ServerDetail (the new structure)
	var serverDetail oci.ServerDetail
	if err := json.Unmarshal(body, &serverDetail); err == nil && serverDetail.Name != "" {
		// Successfully parsed as ServerDetail - convert to catalog.Server
		server := serverDetail.ToCatalogServer()

		serverName := serverDetail.Name
		servers[serverName] = server
		log.Log(fmt.Sprintf("  - Added server '%s' from URL %s", serverName, url))
		return servers, nil
	}

	return nil, fmt.Errorf("unable to parse response as OCI catalog or direct catalog format")
}

// mcpConfigSetTool implements a tool for setting configuration values for MCP servers
func (g *Gateway) createMcpConfigSetTool(_ *clientConfig) *ToolRegistration {
	tool := &mcp.Tool{
		Name:        "mcp-config-set",
		Description: "Set configuration for an MCP server. The config object will be validated against the server's config schema. If validation fails, the error message will include the correct schema.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"server": {
					Type:        "string",
					Description: "Name of the MCP server to configure",
				},
				"config": {
					Type:        "object",
					Description: "Configuration object for the server. This will be validated against the server's config schema.",
				},
			},
			Required: []string{"server", "config"},
		},
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: withToolTelemetry("mcp-config-set", configSetHandler(g)),
	}
}

// createMcpExecTool implements a tool for executing tools that exist in the current session
// but may not be returned from listTools calls
func (g *Gateway) createMcpExecTool() *ToolRegistration {
	tool := &mcp.Tool{
		Name:        "mcp-exec",
		Description: "Execute a tool that exists in the current session. This allows calling tools that may not be visible in listTools results.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "Name of the tool to execute",
				},
				"arguments": {
					Types:       []string{"string", "number", "boolean", "object", "array", "null"},
					Description: "Arguments to pass to the tool (can be any valid JSON value)",
				},
			},
			Required: []string{"name"},
		},
	}

	handler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse parameters
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}

		if req.Params.Arguments == nil {
			return nil, fmt.Errorf("missing arguments")
		}

		paramsBytes, err := json.Marshal(req.Params.Arguments)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal arguments: %w", err)
		}

		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return nil, fmt.Errorf("failed to parse arguments: %w", err)
		}

		if params.Name == "" {
			return nil, fmt.Errorf("name parameter is required")
		}

		toolName := strings.TrimSpace(params.Name)

		// Look up the tool in current tool registrations
		g.capabilitiesMu.RLock()
		toolReg, found := g.toolRegistrations[toolName]
		g.capabilitiesMu.RUnlock()

		if !found {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf("Error: Tool '%s' not found in current session. Make sure the server providing this tool is added to the session.", toolName),
				}},
			}, nil
		}

		// Handle the case where arguments might be a JSON-encoded string
		// This happens when the schema previously specified Type: "string"
		var toolArguments json.RawMessage
		if len(params.Arguments) > 0 {
			// Try to unmarshal as a string first (for backward compatibility)
			var argString string
			if err := json.Unmarshal(params.Arguments, &argString); err == nil {
				// It was a JSON string, use the unescaped content
				toolArguments = json.RawMessage(argString)
			} else {
				// It's already a proper JSON object/value
				toolArguments = params.Arguments
			}
		}

		// Create a new CallToolRequest with the provided arguments
		log.Logf("calling tool %s with %s", toolName, toolArguments)
		toolCallRequest := &mcp.CallToolRequest{
			Session: req.Session,
			Params: &mcp.CallToolParamsRaw{
				Meta:      req.Params.Meta,
				Name:      toolName,
				Arguments: toolArguments,
			},
			Extra: req.Extra,
		}

		// Execute the tool using its registered handler
		result, err := toolReg.Handler(ctx, toolCallRequest)
		if err != nil {
			return nil, fmt.Errorf("tool execution failed: %w", err)
		}

		return result, nil
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: withToolTelemetry("mcp-exec", handler),
	}
}

//nolint:unused // mcpCatalogTool implements a tool for viewing information about the currently attached catalog
func (g *Gateway) _createMcpCatalogTool() *ToolRegistration {
	tool := &mcp.Tool{
		Name:        "mcp-catalog",
		Description: "Summarize information about the currently attached catalog, including available servers and their configurations.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
		},
	}

	handler := func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return _dockerHubLink(), nil
	}

	return &ToolRegistration{
		Tool:    tool,
		Handler: withToolTelemetry("mcp-catalog", handler),
	}
}

// maxInt returns the maximum of two integers
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// withToolTelemetry wraps a tool handler with telemetry instrumentation
func withToolTelemetry(toolName string, handler mcp.ToolHandler) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		serverName := "dynamic-mcps"

		// Debug logging to stderr
		if os.Getenv("DOCKER_MCP_TELEMETRY_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[MCP-HANDLER] Tool call received: %s from server: %s\n", toolName, serverName)
		}

		// Start telemetry span for tool call
		startTime := time.Now()
		serverType := "dynamic"

		// Build span attributes
		spanAttrs := []attribute.KeyValue{
			attribute.String("mcp.server.name", serverName),
			attribute.String("mcp.server.type", serverType),
		}

		ctx, span := telemetry.StartToolCallSpan(ctx, toolName, spanAttrs...)
		defer span.End()

		// Record tool call counter
		telemetry.ToolCallCounter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("mcp.server.name", serverName),
				attribute.String("mcp.server.type", serverType),
				attribute.String("mcp.tool.name", toolName),
				attribute.String("mcp.client.name", req.Session.InitializeParams().ClientInfo.Name),
			),
		)

		// Execute the wrapped handler
		result, err := handler(ctx, req)

		// Record duration
		duration := time.Since(startTime).Milliseconds()
		telemetry.ToolCallDuration.Record(ctx, float64(duration),
			metric.WithAttributes(
				attribute.String("mcp.server.name", serverName),
				attribute.String("mcp.server.type", serverType),
				attribute.String("mcp.tool.name", toolName),
				attribute.String("mcp.client.name", req.Session.InitializeParams().ClientInfo.Name),
			),
		)

		if err != nil {
			// Record error in telemetry
			telemetry.RecordToolError(ctx, span, serverName, serverType, toolName)
			span.SetStatus(codes.Error, "Tool execution failed")
			return nil, err
		}

		span.SetStatus(codes.Ok, "")
		return result, nil
	}
}
