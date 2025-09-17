package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Client interface wraps the official MCP SDK client with our legacy interface
type Client interface {
	Initialize(ctx context.Context, params *mcp.InitializeParams, debug bool, serverSession *mcp.ServerSession, server *mcp.Server, refresher CapabilityRefresher) error
	Session() *mcp.ClientSession
	GetClient() *mcp.Client
	AddRoots(roots []*mcp.Root)
}

// CapabilityRefresher interface allows the notification handlers to refresh server capabilities
type CapabilityRefresher interface {
	RefreshCapabilities(ctx context.Context, server *mcp.Server, serverSession *mcp.ServerSession) error
}

func notifications(serverSession *mcp.ServerSession, server *mcp.Server, refresher CapabilityRefresher) *mcp.ClientOptions {
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
		ToolListChangedHandler: func(ctx context.Context, _ *mcp.ToolListChangedRequest) {
			if refresher != nil && server != nil && serverSession != nil {
				_ = refresher.RefreshCapabilities(ctx, server, serverSession)
			}
		},
		ResourceListChangedHandler: func(ctx context.Context, _ *mcp.ResourceListChangedRequest) {
			if refresher != nil && server != nil && serverSession != nil {
				_ = refresher.RefreshCapabilities(ctx, server, serverSession)
			}
		},
		PromptListChangedHandler: func(ctx context.Context, _ *mcp.PromptListChangedRequest) {
			if refresher != nil && server != nil && serverSession != nil {
				_ = refresher.RefreshCapabilities(ctx, server, serverSession)
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
