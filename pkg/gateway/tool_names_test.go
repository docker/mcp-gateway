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

func TestValidateExternalToolNameCollisionsRejectsDuplicateBaseServers(t *testing.T) {
	err := validateExternalToolNameCollisions([]ToolRegistration{
		{
			ServerName: "alpha",
			Tool:       &mcp.Tool{Name: "search"},
		},
		{
			ServerName: "beta",
			Tool:       &mcp.Tool{Name: "search"},
		},
	}, nil)

	require.ErrorIs(t, err, errToolNameCollision)
	assert.Contains(t, err.Error(), `server "alpha"`)
	assert.Contains(t, err.Error(), `server "beta"`)
	assert.Contains(t, err.Error(), `"search"`)
}

func TestValidateExternalToolNameCollisionsAllowsPrefixedDuplicateRawNames(t *testing.T) {
	err := validateExternalToolNameCollisions([]ToolRegistration{
		{
			ServerName: "alpha",
			Tool:       &mcp.Tool{Name: "alpha__search"},
		},
		{
			ServerName: "beta",
			Tool:       &mcp.Tool{Name: "beta__search"},
		},
	}, nil)

	require.NoError(t, err)
}

func TestValidateExternalToolNameCollisionsRejectsReservedGatewayToolName(t *testing.T) {
	err := validateExternalToolNameCollisions([]ToolRegistration{
		{
			ServerName: "untrusted",
			Tool:       &mcp.Tool{Name: "mcp-exec"},
		},
	}, nil)

	require.ErrorIs(t, err, errToolNameCollision)
	assert.Contains(t, err.Error(), "reserved gateway tool name")
	assert.Contains(t, err.Error(), "mcp-exec")
}

func TestValidateExternalToolNameCollisionsRejectsDynamicMcpExecShadow(t *testing.T) {
	existing := map[string]ToolRegistration{
		"deploy": {
			ServerName: "trusted",
			Tool:       &mcp.Tool{Name: "deploy"},
		},
	}

	err := validateExternalToolNameCollisions([]ToolRegistration{
		{
			ServerName: "untrusted",
			Tool:       &mcp.Tool{Name: "deploy"},
		},
	}, existing)

	require.ErrorIs(t, err, errToolNameCollision)
	assert.Contains(t, err.Error(), `server "untrusted" would shadow server "trusted"`)
	assert.Equal(t, "trusted", existing["deploy"].ServerName)
}

func TestPolicyFilteredDynamicToolsDoNotTriggerCollisionValidation(t *testing.T) {
	caps := &Capabilities{
		Tools: []ToolRegistration{
			{
				ServerName: "candidate",
				Tool:       &mcp.Tool{Name: "mcp-exec"},
			},
			{
				ServerName: "candidate",
				Tool:       &mcp.Tool{Name: "safe-tool"},
			},
		},
	}

	filtered := filterCapabilitiesByAllowedTools(caps, []bool{false, true})

	require.Len(t, filtered.Tools, 1)
	assert.Equal(t, "safe-tool", filtered.Tools[0].Tool.Name)
	require.NoError(t, validateExternalToolNameCollisions(filtered.Tools, nil))
}

func TestCatalogToolNameWarningsReportPotentialMcpFindCollisions(t *testing.T) {
	g := &Gateway{
		toolRegistrations: map[string]ToolRegistration{
			"deploy": {
				ServerName: "trusted",
				Tool:       &mcp.Tool{Name: "deploy"},
			},
		},
	}

	warnings := g.catalogToolNameWarnings("candidate", catalog.Server{
		Tools: []catalog.Tool{
			{Name: "mcp-add"},
			{Name: "deploy"},
			{Name: "duplicate"},
			{Name: "duplicate"},
		},
	})

	require.Len(t, warnings, 3)
	assert.Contains(t, warnings[0]+warnings[1]+warnings[2], "reserved for a gateway internal tool")
	assert.Contains(t, warnings[0]+warnings[1]+warnings[2], `conflicts with server "trusted"`)
	assert.Contains(t, warnings[0]+warnings[1]+warnings[2], "duplicates tool")
}

func TestBM25StrategyIncludesToolNameWarnings(t *testing.T) {
	g := &Gateway{
		configuration: Configuration{
			serverNames: []string{"candidate"},
			servers: map[string]catalog.Server{
				"candidate": {
					Name:        "candidate",
					Title:       "Candidate",
					Description: "candidate server",
					Tools: []catalog.Tool{
						{Name: "mcp-add", Description: "confusing internal tool"},
					},
				},
			},
		},
	}

	args, err := json.Marshal(map[string]any{
		"query": "candidate",
		"limit": 1,
	})
	require.NoError(t, err)

	result, err := bm25Strategy(g)(context.Background(), &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Arguments: args,
		},
	})
	require.NoError(t, err)
	require.Len(t, result.Content, 1)

	text, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)

	var response struct {
		Servers []struct {
			Name             string   `json:"name"`
			ToolNameWarnings []string `json:"tool_name_warnings"`
		} `json:"servers"`
	}
	require.NoError(t, json.Unmarshal([]byte(text.Text), &response))
	require.Len(t, response.Servers, 1)
	assert.Equal(t, "candidate", response.Servers[0].Name)
	require.Len(t, response.Servers[0].ToolNameWarnings, 1)
	assert.Contains(t, response.Servers[0].ToolNameWarnings[0], "reserved for a gateway internal tool")
}
