package gateway

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
)

func getClientConfig(readOnlyHint *bool, ss *mcp.ServerSession, server *mcp.Server) *clientConfig {
	return &clientConfig{readOnly: readOnlyHint, serverSession: ss, server: server}
}

func (g *Gateway) mcpToolHandler(tool catalog.Tool) mcp.ToolHandler {
	return func(ctx context.Context, _ *mcp.ServerSession, params *mcp.CallToolParamsFor[map[string]any]) (*mcp.CallToolResultFor[any], error) {
		// Convert to the generic version for our internal methods
		genericParams := &mcp.CallToolParams{
			Meta:      params.Meta,
			Name:      params.Name,
			Arguments: params.Arguments,
		}
		return g.clientPool.runToolContainer(ctx, tool, genericParams)
	}
}

func (g *Gateway) mcpServerToolHandler(serverConfig *catalog.ServerConfig, server *mcp.Server, annotations *mcp.ToolAnnotations) mcp.ToolHandler {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[map[string]any]) (*mcp.CallToolResultFor[any], error) {
		var readOnlyHint *bool
		if annotations != nil && annotations.ReadOnlyHint {
			readOnlyHint = &annotations.ReadOnlyHint
		}

		// Convert to the generic version for our internal methods
		genericParams := &mcp.CallToolParams{
			Meta:      params.Meta,
			Name:      params.Name,
			Arguments: params.Arguments,
		}

		client, err := g.clientPool.AcquireClient(ctx, serverConfig, getClientConfig(readOnlyHint, ss, server))
		if err != nil {
			return nil, err
		}
		defer g.clientPool.ReleaseClient(client)

		return client.Session().CallTool(ctx, genericParams)
	}
}

func (g *Gateway) mcpServerPromptHandler(serverConfig *catalog.ServerConfig, server *mcp.Server) mcp.PromptHandler {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.GetPromptParams) (*mcp.GetPromptResult, error) {
		client, err := g.clientPool.AcquireClient(ctx, serverConfig, getClientConfig(nil, ss, server))
		if err != nil {
			return nil, err
		}
		defer g.clientPool.ReleaseClient(client)

		return client.Session().GetPrompt(ctx, params)
	}
}

func (g *Gateway) mcpServerResourceHandler(serverConfig *catalog.ServerConfig, server *mcp.Server) mcp.ResourceHandler {
	return func(ctx context.Context, ss *mcp.ServerSession, params *mcp.ReadResourceParams) (*mcp.ReadResourceResult, error) {
		client, err := g.clientPool.AcquireClient(ctx, serverConfig, getClientConfig(nil, ss, server))
		if err != nil {
			return nil, err
		}
		defer g.clientPool.ReleaseClient(client)

		return client.Session().ReadResource(ctx, params)
	}
}
