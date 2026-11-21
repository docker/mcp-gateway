package gateway

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
)

func TestAddServerHandlerDoesNotAppendServerWhenValidationFails(t *testing.T) {
	g := &Gateway{
		configuration: Configuration{
			serverNames: []string{"existing"},
			servers: map[string]catalog.Server{
				"needs-config": {
					Image: "mcp/needs-config:latest",
					Config: []any{
						map[string]any{"name": "url", "type": "string"},
					},
				},
			},
			config: map[string]map[string]any{},
		},
	}
	handler := addServerHandler(g, nil)
	args, err := json.Marshal(map[string]any{"name": "needs-config"})
	require.NoError(t, err)

	result, err := handler(context.Background(), &mcp.CallToolRequest{
		Session: &mcp.ServerSession{},
		Params: &mcp.CallToolParamsRaw{
			Arguments: args,
		},
	})

	require.NoError(t, err)
	require.Len(t, result.Content, 1)
	text, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, text.Text, "The server was not added")
	assert.Equal(t, []string{"existing"}, g.configuration.serverNames)
}
