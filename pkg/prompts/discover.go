package prompts

import (
	"context"
	_ "embed"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed discover.md
var discoverPrompt string

// AddDiscoverPrompt adds a prompt that explains how to discover and add MCP servers
func AddDiscoverPrompt(server *mcp.Server) {
	server.AddPrompt(&mcp.Prompt{
		Name:        "mcp-discover",
		Description: "Learn how to discover and add MCP servers dynamically",
	},
		func(_ context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{
				Description: "Instructions for discovering and managing MCP servers",
				Messages: []*mcp.PromptMessage{
					{
						Role: "user",
						Content: &mcp.TextContent{
							Text: discoverPrompt,
						},
					},
				},
			}, nil
		})
}
