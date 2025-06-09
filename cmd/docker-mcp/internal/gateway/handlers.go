package gateway

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/docker/mcp-cli/cmd/docker-mcp/internal/catalog"
	mcpclient "github.com/docker/mcp-cli/cmd/docker-mcp/internal/mcp"
)

func (g *Gateway) mcpToolHandler(tool catalog.Tool) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return g.runToolContainer(ctx, tool, request)
	}
}

func (g *Gateway) mcpServerToolHandler(serverConfig ServerConfig, annotations mcp.ToolAnnotation) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var client mcpclient.MCPClient
		var err error
		if serverConfig.Spec.SSEEndpoint != "" {
			client, err = g.startSSEMCPClient(ctx, serverConfig.Name, serverConfig.Spec.SSEEndpoint, annotations.ReadOnlyHint)
		} else {
			client, err = g.startStdioMCPClient(ctx, serverConfig, annotations.ReadOnlyHint)
		}
		if err != nil {
			return nil, err
		}
		defer client.Close()

		return client.CallTool(ctx, request)
	}
}

func (g *Gateway) mcpServerPromptHandler(serverConfig ServerConfig) server.PromptHandlerFunc {
	return func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		var client mcpclient.MCPClient
		var err error
		if serverConfig.Spec.SSEEndpoint != "" {
			client, err = g.startSSEMCPClient(ctx, serverConfig.Name, serverConfig.Spec.SSEEndpoint, &readOnly)
		} else {
			client, err = g.startStdioMCPClient(ctx, serverConfig, &readOnly)
		}
		if err != nil {
			return nil, err
		}
		defer client.Close()

		return client.GetPrompt(ctx, request)
	}
}

func (g *Gateway) mcpServerResourceHandler(serverConfig ServerConfig) server.ResourceHandlerFunc {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		var client mcpclient.MCPClient
		var err error
		if serverConfig.Spec.SSEEndpoint != "" {
			client, err = g.startSSEMCPClient(ctx, serverConfig.Name, serverConfig.Spec.SSEEndpoint, &readOnly)
		} else {
			client, err = g.startStdioMCPClient(ctx, serverConfig, &readOnly)
		}
		if err != nil {
			return nil, err
		}
		defer client.Close()

		result, err := client.ReadResource(ctx, request)
		if err != nil {
			return nil, err
		}

		return result.Contents, nil
	}
}

func (g *Gateway) mcpServerResourceTemplateHandler(serverConfig ServerConfig) server.ResourceTemplateHandlerFunc {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		var client mcpclient.MCPClient
		var err error
		if serverConfig.Spec.SSEEndpoint != "" {
			client, err = g.startSSEMCPClient(ctx, serverConfig.Name, serverConfig.Spec.SSEEndpoint, &readOnly)
		} else {
			client, err = g.startStdioMCPClient(ctx, serverConfig, &readOnly)
		}
		if err != nil {
			return nil, err
		}
		defer client.Close()

		result, err := client.ReadResource(ctx, request)
		if err != nil {
			return nil, err
		}

		return result.Contents, nil
	}
}
