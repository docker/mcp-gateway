package gateway

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
)

func (g *Gateway) mcpToolHandler(tool catalog.Tool) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return g.clientPool.runToolContainer(ctx, tool, request)
	}
}

func (g *Gateway) mcpServerToolHandler(serverConfig ServerConfig, annotations mcp.ToolAnnotation) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := g.clientPool.AcquireClient(ctx, serverConfig, annotations.ReadOnlyHint)
		if err != nil {
			return nil, err
		}
		defer g.clientPool.ReleaseClient(client)

		return client.CallTool(ctx, request)
	}
}

func (g *Gateway) mcpServerPromptHandler(serverConfig ServerConfig) server.PromptHandlerFunc {
	return func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		client, err := g.clientPool.AcquireClient(ctx, serverConfig, &readOnly)
		if err != nil {
			return nil, err
		}
		defer g.clientPool.ReleaseClient(client)

		return client.GetPrompt(ctx, request)
	}
}

func (g *Gateway) mcpServerResourceHandler(serverConfig ServerConfig) server.ResourceHandlerFunc {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		client, err := g.clientPool.AcquireClient(ctx, serverConfig, &readOnly)
		if err != nil {
			return nil, err
		}
		defer g.clientPool.ReleaseClient(client)

		result, err := client.ReadResource(ctx, request)
		if err != nil {
			return nil, err
		}

		return result.Contents, nil
	}
}

func (g *Gateway) mcpServerResourceTemplateHandler(serverConfig ServerConfig) server.ResourceTemplateHandlerFunc {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		client, err := g.clientPool.AcquireClient(ctx, serverConfig, &readOnly)
		if err != nil {
			return nil, err
		}
		defer g.clientPool.ReleaseClient(client)

		result, err := client.ReadResource(ctx, request)
		if err != nil {
			return nil, err
		}

		return result.Contents, nil
	}
}
