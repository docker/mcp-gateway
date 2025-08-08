package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Client interface wraps the official MCP SDK client with our legacy interface
type Client interface {
	Initialize(ctx context.Context, params *mcp.InitializeParams, debug bool, serverSession *mcp.ServerSession, server *mcp.Server) error
	Session() *mcp.ClientSession
}

func notifications(serverSession *mcp.ServerSession, server *mcp.Server) *mcp.ClientOptions {
	return &mcp.ClientOptions{
		ResourceUpdatedHandler: func(ctx context.Context, _ *mcp.ClientSession, params *mcp.ResourceUpdatedNotificationParams) {
			if server != nil {
				_ = server.ResourceUpdated(ctx, params)
			}
		},
		CreateMessageHandler: func(_ context.Context, _ *mcp.ClientSession, _ *mcp.CreateMessageParams) (*mcp.CreateMessageResult, error) {
			// Handle create messages if needed
			return nil, fmt.Errorf("create messages not supported")
		},
		ToolListChangedHandler: func(ctx context.Context, _ *mcp.ClientSession, params *mcp.ToolListChangedParams) {
			if serverSession != nil {
				_ = mcp.HandleNotify(ctx, serverSession, "notifications/tools/list_changed", params)
			}
		},
		ResourceListChangedHandler: func(ctx context.Context, _ *mcp.ClientSession, params *mcp.ResourceListChangedParams) {
			if serverSession != nil {
				_ = mcp.HandleNotify(ctx, serverSession, "notifications/resources/list_changed", params)
			}
		},
		PromptListChangedHandler: func(ctx context.Context, _ *mcp.ClientSession, params *mcp.PromptListChangedParams) {
			if serverSession != nil {
				_ = mcp.HandleNotify(ctx, serverSession, "notifications/prompts/list_changed", params)
			}
		},
		ProgressNotificationHandler: func(ctx context.Context, _ *mcp.ClientSession, params *mcp.ProgressNotificationParams) {
			if serverSession != nil {
				_ = serverSession.NotifyProgress(ctx, params)
			}
		},
		LoggingMessageHandler: func(ctx context.Context, _ *mcp.ClientSession, params *mcp.LoggingMessageParams) {
			if serverSession != nil {
				_ = serverSession.Log(ctx, params)
			}
		},
		ElicitationHandler: func(ctx context.Context, _ *mcp.ClientSession, params *mcp.ElicitParams) (*mcp.ElicitResult, error) {
			if serverSession != nil {
				return serverSession.Elicit(ctx, params)
			}
			return nil, fmt.Errorf("elicitation handled without server session")
		},
	}
}
