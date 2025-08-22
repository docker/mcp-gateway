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
	GetClient() *mcp.Client
	AddRoots(roots []*mcp.Root)
}

func notifications(serverSession *mcp.ServerSession, server *mcp.Server) *mcp.ClientOptions {
	return &mcp.ClientOptions{
		ResourceUpdatedHandler: func(ctx context.Context, req *mcp.ResourceUpdatedNotificationRequest) {
			if server != nil {
				_ = server.ResourceUpdated(ctx, req.Params)
			}
		},
		CreateMessageHandler: func(_ context.Context, _ *mcp.CreateMessageRequest) (*mcp.CreateMessageResult, error) {
			// Handle create messages if needed
			return nil, fmt.Errorf("create messages not supported")
		},
		ToolListChangedHandler: func(ctx context.Context, req *mcp.ToolListChangedRequest) {
			if serverSession != nil {
				_ = mcp.HandleNotify(ctx, "notifications/tools/list_changed", req)
			}
		},
		ResourceListChangedHandler: func(ctx context.Context, req *mcp.ResourceListChangedRequest) {
			if serverSession != nil {
				_ = mcp.HandleNotify(ctx, "notifications/resources/list_changed", req)
			}
		},
		PromptListChangedHandler: func(ctx context.Context, req *mcp.PromptListChangedRequest) {
			if serverSession != nil {
				_ = mcp.HandleNotify(ctx, "notifications/prompts/list_changed", req)
			}
		},
		ProgressNotificationHandler: func(ctx context.Context, req *mcp.ProgressNotificationClientRequest) {
			if serverSession != nil {
				_ = serverSession.NotifyProgress(ctx, req.Params)
			}
		},
		LoggingMessageHandler: func(ctx context.Context, req *mcp.LoggingMessageRequest) {
			if serverSession != nil {
				_ = serverSession.Log(ctx, req.Params)
			}
		},
		ElicitationHandler: func(ctx context.Context, req *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			if serverSession != nil {
				return serverSession.Elicit(ctx, req.Params)
			}
			return nil, fmt.Errorf("elicitation handled without server session")
		},
	}
}
