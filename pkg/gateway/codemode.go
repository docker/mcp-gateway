package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/codemode"
)

// serverToolSetAdapter adapts a gateway server to the codemode.ToolSet interface
type serverToolSetAdapter struct {
	gateway    *Gateway
	serverName string
}

func (a *serverToolSetAdapter) Tools(_ context.Context) ([]*codemode.ToolWithHandler, error) {
	a.gateway.capabilitiesMu.RLock()
	registrations := make([]ToolRegistration, 0)
	for _, registration := range a.gateway.toolRegistrations {
		if registration.ServerName == a.serverName {
			registrations = append(registrations, registration)
		}
	}
	a.gateway.capabilitiesMu.RUnlock()

	// Registration order comes from a map. Keep generated documentation and
	// function installation deterministic without changing tool-name identity.
	slices.SortFunc(registrations, func(left, right ToolRegistration) int {
		return strings.Compare(left.Tool.Name, right.Tool.Name)
	})

	result := make([]*codemode.ToolWithHandler, 0, len(registrations))
	for _, registration := range registrations {
		result = append(result, &codemode.ToolWithHandler{
			Tool:    registration.Tool,
			Handler: registration.Handler,
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

		// Validate against the enabled-server set, not the full catalog. In
		// profile mode configuration.servers includes catalog-only entries.
		for _, serverName := range params.Servers {
			if !slices.Contains(g.configuration.ServerNames(), serverName) {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{
						Text: fmt.Sprintf("Error: Server '%s' is not enabled for this gateway session.", serverName),
					}},
				}, nil
			}
		}

		// Create a tool set adapter for each server
		var toolSets []codemode.ToolSet
		for _, serverName := range params.Servers {
			toolSets = append(toolSets, &serverToolSetAdapter{
				gateway:    g,
				serverName: serverName,
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

		// Track the tool registration for capabilities and mcp-exec
		registration := ToolRegistration{
			ServerName: "code-mode",
			Tool:       customTool.Tool,
			Handler:    customTool.Handler,
		}
		g.capabilitiesMu.Lock()
		if err := validateToolNameCollisions([]ToolRegistration{registration}, g.toolRegistrations, false); err != nil {
			g.capabilitiesMu.Unlock()
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf("Error: Cannot create code-mode tool '%s'. %s", toolName, err),
				}},
			}, nil
		}
		g.mcpServer.AddTool(customTool.Tool, customTool.Handler)
		g.toolRegistrations[toolName] = registration
		g.capabilitiesMu.Unlock()

		// Build detailed response with tool information
		var responseText strings.Builder
		fmt.Fprintf(&responseText, "Successfully created code-mode tool '%s'\n\n", toolName)

		// Tool description
		responseText.WriteString("## Tool Details\n")
		fmt.Fprintf(&responseText, "**Name:** %s\n", toolName)
		fmt.Fprintf(&responseText, "**Description:** %s\n\n", customTool.Tool.Description)

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
		fmt.Fprintf(&responseText, "This tool has access to tools from: %s\n\n", strings.Join(params.Servers, ", "))

		// Usage instructions
		responseText.WriteString("## How to Use\n")
		responseText.WriteString("You can call this tool using the **mcp-exec** tool:\n")
		responseText.WriteString("```json\n")
		responseText.WriteString("{\n")
		fmt.Fprintf(&responseText, "  \"name\": \"%s\",\n", toolName)
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
