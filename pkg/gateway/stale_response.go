package gateway

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/catalog"
	mcpclient "github.com/docker/mcp-gateway/pkg/mcp"
)

// isStaleEmptySuccess reports whether a tool result is a structurally empty
// success response that typically indicates a stale remote session.
func isStaleEmptySuccess(result *mcp.CallToolResult) bool {
	if result == nil || result.IsError {
		return false
	}
	if len(result.Content) > 0 {
		return false
	}
	return result.StructuredContent == nil
}

// isSafeToRetryTool reports whether an empty-success recovery retry is safe
// based on MCP tool annotation hints.
func isSafeToRetryTool(annotations *mcp.ToolAnnotations) bool {
	if annotations == nil {
		return false
	}
	if annotations.ReadOnlyHint {
		return true
	}
	return annotations.IdempotentHint
}

func (g *Gateway) callRemoteTool(
	ctx context.Context,
	serverConfig *catalog.ServerConfig,
	server *mcp.Server,
	annotations *mcp.ToolAnnotations,
	originalToolName string,
	params *mcp.CallToolParams,
	session *mcp.ServerSession,
	client mcpclient.Client,
) (*mcp.CallToolResult, error) {
	result, err := client.Session().CallTool(ctx, params)
	if err != nil {
		return nil, err
	}

	if !serverConfig.IsRemote() || !isStaleEmptySuccess(result) {
		return result, nil
	}

	clientConfig := getClientConfig(session, server)
	g.clientPool.InvalidateKeptClient(serverConfig, clientConfig)

	if !isSafeToRetryTool(annotations) {
		return nil, fmt.Errorf(
			"remote tool %q on server %q returned an empty success response (stale session)",
			originalToolName,
			serverConfig.Name,
		)
	}

	retryClient, retryErr := g.clientPool.AcquireClient(ctx, serverConfig, clientConfig)
	if retryErr != nil {
		return nil, fmt.Errorf(
			"remote tool %q returned empty result and failed to refresh session: %w",
			originalToolName,
			retryErr,
		)
	}
	defer g.clientPool.ReleaseClient(retryClient)

	result, err = retryClient.Session().CallTool(ctx, params)
	if err != nil {
		return nil, err
	}
	if isStaleEmptySuccess(result) {
		return nil, fmt.Errorf(
			"remote tool %q on server %q returned an empty success response after session refresh",
			originalToolName,
			serverConfig.Name,
		)
	}
	return result, nil
}
