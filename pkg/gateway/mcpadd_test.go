package gateway

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/oauth"
)

func TestAddServerHandlerNilSessionMissingConfig(t *testing.T) {
	t.Parallel()

	g := &Gateway{
		configuration: Configuration{
			serverNames: []string{},
			servers: map[string]catalog.Server{
				"webhook-mcp": {
					Name:  "webhook-mcp",
					Image: "example/webhook-mcp:latest",
					Config: []any{
						map[string]any{
							"name": "endpoint",
							"type": "string",
						},
					},
				},
			},
		},
	}

	handler := addServerHandler(g, nil)
	args, err := json.Marshal(map[string]any{
		"name": "webhook-mcp",
	})
	if err != nil {
		t.Fatalf("marshal arguments: %v", err)
	}

	req := &mcp.CallToolRequest{
		Session: nil,
		Params: &mcp.CallToolParamsRaw{
			Arguments: args,
		},
	}

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected tool result content")
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", result.Content[0])
	}
	if !strings.Contains(text.Text, "Missing required") {
		t.Fatalf("expected missing requirements error, got: %s", text.Text)
	}
}

func TestGetRemoteOAuthServerStatusNilSession(t *testing.T) {
	t.Parallel()

	g := &Gateway{
		Options: Options{McpOAuthDcrEnabled: true},
		configuration: Configuration{
			servers: map[string]catalog.Server{
				"remote-oauth": {
					Name: "remote-oauth",
					Type: "remote",
					Remote: catalog.Remote{
						URL: "https://example.com/mcp",
					},
				},
			},
		},
		oauthProviders: map[string]*oauth.Provider{
			"remote-oauth": {},
		},
	}

	req := &mcp.CallToolRequest{Session: nil}
	authorized, message := g.getRemoteOAuthServerStatus(
		context.Background(),
		"remote-oauth",
		req,
		false,
	)

	if authorized {
		t.Fatalf("expected unauthorized flow without session, got authorized with message: %q", message)
	}
	if !strings.Contains(message, "authorize") {
		t.Fatalf("expected authorization instructions, got: %s", message)
	}
}
