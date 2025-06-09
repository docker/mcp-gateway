package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

type Client interface {
	Initialize(ctx context.Context, request mcp.InitializeRequest, debug bool) (*mcp.InitializeResult, error)
	ListTools(ctx context.Context, request mcp.ListToolsRequest) (*mcp.ListToolsResult, error)
	ListPrompts(ctx context.Context, request mcp.ListPromptsRequest) (*mcp.ListPromptsResult, error)
	ListResources(ctx context.Context, request mcp.ListResourcesRequest) (*mcp.ListResourcesResult, error)
	ListResourceTemplates(ctx context.Context, request mcp.ListResourceTemplatesRequest) (*mcp.ListResourceTemplatesResult, error)
	CallTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
	GetPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error)
	ReadResource(ctx context.Context, request mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error)
	Close() error
}
