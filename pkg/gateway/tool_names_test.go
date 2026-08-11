package gateway

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/policy"
)

func TestSanitizeMcpToolNameReplacesInvalidCharacters(t *testing.T) {
	assert.Equal(t, "Husqvarna_Automowers_Status", sanitizeMcpToolName("Husqvarna Automowers Status"))
	assert.Equal(t, "foo_bar_baz", sanitizeMcpToolName("foo:bar.baz"))
	assert.Equal(t, "search", sanitizeMcpToolName("search"))
}

func TestSanitizeMcpToolNameEnforcesMaxLength(t *testing.T) {
	longName := strings.Repeat("a", 80)
	sanitized := sanitizeMcpToolName(longName)
	require.Len(t, sanitized, maxMcpToolNameLength)
	assert.Equal(t, strings.Repeat("a", maxMcpToolNameLength), sanitized)
}

func TestExposeToolNameAppliesPrefixAndSanitization(t *testing.T) {
	assert.Equal(t, "husqvarna-automower__Husqvarna_Automowers_Status", exposeToolName("husqvarna-automower", "Husqvarna Automowers Status"))
	assert.Equal(t, "Husqvarna_Automowers_Status", exposeToolName("", "Husqvarna Automowers Status"))
}

func TestUniqueExposeToolNameDedupesSanitizedCollisions(t *testing.T) {
	seen := make(map[string]struct{})

	first := uniqueExposeToolName("", "Foo Bar", seen)
	second := uniqueExposeToolName("", "Foo_Bar", seen)

	assert.Equal(t, "Foo_Bar", first)
	assert.Equal(t, "Foo_Bar_2", second)
}

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

func TestValidateExternalCapabilityNameCollisionsRejectsDuplicatePrompts(t *testing.T) {
	err := validateExternalCapabilityNameCollisions(&Capabilities{
		Prompts: []PromptRegistration{
			{
				ServerName: "alpha",
				Prompt:     &mcp.Prompt{Name: "summarize"},
			},
			{
				ServerName: "beta",
				Prompt:     &mcp.Prompt{Name: "summarize"},
			},
		},
	}, capabilityNameIndexes{}, true)

	require.ErrorIs(t, err, errCapabilityNameCollision)
	assert.Contains(t, err.Error(), "prompt name collision")
	assert.Contains(t, err.Error(), `server "alpha"`)
	assert.Contains(t, err.Error(), `server "beta"`)
	assert.Contains(t, err.Error(), `"summarize"`)
}

func TestValidateExternalCapabilityNameCollisionsRejectsReservedDynamicPromptName(t *testing.T) {
	caps := &Capabilities{
		Prompts: []PromptRegistration{
			{
				ServerName: "candidate",
				Prompt:     &mcp.Prompt{Name: "mcp-discover"},
			},
		},
	}

	err := validateExternalCapabilityNameCollisions(caps, capabilityNameIndexes{}, true)
	require.ErrorIs(t, err, errCapabilityNameCollision)
	assert.Contains(t, err.Error(), "reserved gateway prompt name")
	assert.Contains(t, err.Error(), "mcp-discover")

	require.NoError(t, validateExternalCapabilityNameCollisions(caps, capabilityNameIndexes{}, false))
}

func TestValidateExternalCapabilityNameCollisionsUsesRawIdentifiersForComparison(t *testing.T) {
	err := validateExternalCapabilityNameCollisions(&Capabilities{
		Prompts: []PromptRegistration{
			{
				ServerName: "alpha",
				Prompt:     &mcp.Prompt{Name: "summarize"},
			},
			{
				ServerName: "beta",
				Prompt:     &mcp.Prompt{Name: " summarize "},
			},
			{
				ServerName: "gamma",
				Prompt:     &mcp.Prompt{Name: " mcp-discover "},
			},
		},
	}, capabilityNameIndexes{}, true)

	require.NoError(t, err)
}

func TestValidateExternalCapabilityNameCollisionsRejectsPromptShadow(t *testing.T) {
	err := validateExternalCapabilityNameCollisions(&Capabilities{
		Prompts: []PromptRegistration{
			{
				ServerName: "candidate",
				Prompt:     &mcp.Prompt{Name: "summarize"},
			},
		},
	}, capabilityNameIndexes{
		Prompts: map[string]string{"summarize": "trusted"},
	}, true)

	require.ErrorIs(t, err, errCapabilityNameCollision)
	assert.Contains(t, err.Error(), `server "candidate" would shadow server "trusted"`)
	assert.Contains(t, err.Error(), `"summarize"`)
}

func TestValidateExternalCapabilityNameCollisionsRejectsDuplicateResourceURIs(t *testing.T) {
	err := validateExternalCapabilityNameCollisions(&Capabilities{
		Resources: []ResourceRegistration{
			{
				ServerName: "alpha",
				Resource:   &mcp.Resource{URI: "file://shared/readme"},
			},
			{
				ServerName: "beta",
				Resource:   &mcp.Resource{URI: "file://shared/readme"},
			},
		},
	}, capabilityNameIndexes{}, false)

	require.ErrorIs(t, err, errCapabilityNameCollision)
	assert.Contains(t, err.Error(), "resource URI collision")
	assert.Contains(t, err.Error(), `server "alpha"`)
	assert.Contains(t, err.Error(), `server "beta"`)
	assert.Contains(t, err.Error(), "file://shared/readme")
}

func TestValidateExternalCapabilityNameCollisionsRejectsResourceURIShadow(t *testing.T) {
	err := validateExternalCapabilityNameCollisions(&Capabilities{
		Resources: []ResourceRegistration{
			{
				ServerName: "candidate",
				Resource:   &mcp.Resource{URI: "file://shared/readme"},
			},
		},
	}, capabilityNameIndexes{
		Resources: map[string]string{"file://shared/readme": "trusted"},
	}, false)

	require.ErrorIs(t, err, errCapabilityNameCollision)
	assert.Contains(t, err.Error(), `server "candidate" would shadow server "trusted"`)
	assert.Contains(t, err.Error(), "file://shared/readme")
}

func TestValidateExternalCapabilityNameCollisionsRejectsDuplicateResourceTemplates(t *testing.T) {
	err := validateExternalCapabilityNameCollisions(&Capabilities{
		ResourceTemplates: []ResourceTemplateRegistration{
			{
				ServerName:       "alpha",
				ResourceTemplate: mcp.ResourceTemplate{URITemplate: "file://shared/{name}"},
			},
			{
				ServerName:       "beta",
				ResourceTemplate: mcp.ResourceTemplate{URITemplate: "file://shared/{name}"},
			},
		},
	}, capabilityNameIndexes{}, false)

	require.ErrorIs(t, err, errCapabilityNameCollision)
	assert.Contains(t, err.Error(), "resource template URI template collision")
	assert.Contains(t, err.Error(), `server "alpha"`)
	assert.Contains(t, err.Error(), `server "beta"`)
	assert.Contains(t, err.Error(), "file://shared/{name}")
}

func TestValidateExternalCapabilityNameCollisionsRejectsResourceTemplateShadow(t *testing.T) {
	err := validateExternalCapabilityNameCollisions(&Capabilities{
		ResourceTemplates: []ResourceTemplateRegistration{
			{
				ServerName:       "candidate",
				ResourceTemplate: mcp.ResourceTemplate{URITemplate: "file://shared/{name}"},
			},
		},
	}, capabilityNameIndexes{
		ResourceTemplates: map[string]string{"file://shared/{name}": "trusted"},
	}, false)

	require.ErrorIs(t, err, errCapabilityNameCollision)
	assert.Contains(t, err.Error(), `server "candidate" would shadow server "trusted"`)
	assert.Contains(t, err.Error(), "file://shared/{name}")
}

func TestUpdateServerCapabilitiesRevalidatesCapabilityCollisionsUnderLock(t *testing.T) {
	g := &Gateway{
		serverCapabilities: map[string]*ServerCapabilities{
			"trusted": {
				PromptNames: []string{"summarize"},
			},
		},
		serverAvailableCapabilities: map[string]*Capabilities{
			"candidate": {
				Prompts: []PromptRegistration{
					{
						ServerName: "candidate",
						Prompt:     &mcp.Prompt{Name: "summarize"},
					},
				},
			},
		},
	}

	g.capabilitiesMu.Lock()
	err := g.updateServerCapabilities(
		"candidate",
		&ServerCapabilities{},
		&ServerCapabilities{PromptNames: []string{"summarize"}},
		nil,
	)
	g.capabilitiesMu.Unlock()

	require.ErrorIs(t, err, errCapabilityNameCollision)
	assert.Contains(t, err.Error(), `server "candidate" would shadow server "trusted"`)
	assert.Nil(t, g.serverCapabilities["candidate"])
}

func TestPolicyFilteredDynamicToolsDoNotTriggerCollisionValidation(t *testing.T) {
	mock := newMockPolicyClient()
	mock.deny("candidate", "mcp-exec", policy.ActionLoad, "blocked")
	g := &Gateway{policyClient: mock}

	filtered := g.filterToolCapabilitiesByPolicy(context.Background(), Configuration{
		servers: map[string]catalog.Server{
			"candidate": {Name: "candidate"},
		},
	}, &Capabilities{
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
	}, "tool")

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
