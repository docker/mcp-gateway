package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/codemode"
)

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

func addCodemodeHandler(g *Gateway) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
}
