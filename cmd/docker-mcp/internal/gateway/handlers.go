package gateway

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/docker/mcp-cli/cmd/docker-mcp/internal/catalog"
)

func (g *Gateway) mcpToolHandler(tool catalog.Tool) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return g.runToolContainer(ctx, tool, request)
	}
}

func (g *Gateway) mcpServerToolHandlerSSE(serverName string, endpoint string, annotations mcp.ToolAnnotation) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := g.startSSEMCPClient(ctx, serverName, endpoint, annotations.ReadOnlyHint)
		if err != nil {
			return nil, err
		}
		defer client.Close()

		return client.CallTool(ctx, request)
	}
}

func (g *Gateway) mcpServerPromptHandlerSSE(serverName string, endpoint string) server.PromptHandlerFunc {
	return func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		client, err := g.startSSEMCPClient(ctx, serverName, endpoint, &readOnly)
		if err != nil {
			return nil, err
		}
		defer client.Close()

		return client.GetPrompt(ctx, request)
	}
}

func (g *Gateway) mcpServerResourceHandlerSSE(serverName string, endpoint string) server.ResourceHandlerFunc {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		client, err := g.startSSEMCPClient(ctx, serverName, endpoint, &readOnly)
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

func (g *Gateway) mcpServerResourceTemplateHandlerSSE(serverName string, endpoint string) server.ResourceTemplateHandlerFunc {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		client, err := g.startSSEMCPClient(ctx, serverName, endpoint, &readOnly)
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

func (g *Gateway) mcpServerToolHandlerStdio(serverConfig ServerConfig, annotations mcp.ToolAnnotation) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := g.startStdioMCPClient(ctx, serverConfig, annotations.ReadOnlyHint)
		if err != nil {
			return nil, err
		}
		defer client.Close()

		return client.CallTool(ctx, request)
	}
}

func (g *Gateway) mcpServerPromptHandlerStdio(serverConfig ServerConfig) server.PromptHandlerFunc {
	return func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		client, err := g.startStdioMCPClient(ctx, serverConfig, &readOnly)
		if err != nil {
			return nil, err
		}
		defer client.Close()

		return client.GetPrompt(ctx, request)
	}
}

func (g *Gateway) mcpServerResourceHandlerStdio(serverConfig ServerConfig) server.ResourceHandlerFunc {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		client, err := g.startStdioMCPClient(ctx, serverConfig, &readOnly)
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

func (g *Gateway) mcpServerResourceTemplateHandlerStdio(serverConfig ServerConfig) server.ResourceTemplateHandlerFunc {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		client, err := g.startStdioMCPClient(ctx, serverConfig, &readOnly)
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
