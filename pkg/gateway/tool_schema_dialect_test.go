package gateway

import (
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/toolschema"
)

const draft7Dialect = "http://json-schema.org/draft-07/schema#"

// draft7Tool is shaped like what @modelcontextprotocol/sdk emits for every tool
// of a zod-based server: both schemas declare draft-07, which is what
// 2020-12-only clients reject before the tool is ever called.
func draft7Tool() *mcp.Tool {
	return &mcp.Tool{
		Name: "list_bases",
		InputSchema: map[string]any{
			"$schema":    draft7Dialect,
			"type":       "object",
			"properties": map[string]any{},
		},
		OutputSchema: map[string]any{
			"$schema": draft7Dialect,
			"type":    "object",
			"properties": map[string]any{
				"bases": map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
			},
			"required": []any{"bases"},
		},
	}
}

func dialectOf(t *testing.T, schema any) any {
	t.Helper()
	asMap, ok := schema.(map[string]any)
	require.True(t, ok, "schema is %T, want map[string]any", schema)
	return asMap["$schema"]
}

func testServerConfig() *catalog.ServerConfig {
	return &catalog.ServerConfig{Name: "airtable-mcp-server"}
}

func TestToolRegistrationTranslatesSchemaDialects(t *testing.T) {
	g := &Gateway{}
	upstream := draft7Tool()
	before, err := json.Marshal(upstream)
	require.NoError(t, err)

	registration := g.toolRegistration(t.Context(), testServerConfig(), upstream, "")

	require.Equal(t, toolschema.Dialect202012, dialectOf(t, registration.Tool.InputSchema))
	require.Equal(t, toolschema.Dialect202012, dialectOf(t, registration.Tool.OutputSchema))

	// The upstream tool belongs to the client session's cached list. Rewriting
	// it in place would corrupt that cache and race the concurrent discovery of
	// other servers.
	after, err := json.Marshal(upstream)
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after), "the upstream tool was mutated")
	require.Equal(t, draft7Dialect, dialectOf(t, upstream.OutputSchema))
}

func TestToolRegistrationPreservesDialectsWhenAsked(t *testing.T) {
	g := &Gateway{}
	g.PreserveToolSchemaDialect = true

	registration := g.toolRegistration(t.Context(), testServerConfig(), draft7Tool(), "")

	require.Equal(t, draft7Dialect, dialectOf(t, registration.Tool.InputSchema))
	require.Equal(t, draft7Dialect, dialectOf(t, registration.Tool.OutputSchema))
}

// TestToolRegistrationStillPrefixesNames pins the behaviour that shared this
// code path before normalization was added, so a future edit to one cannot
// quietly drop the other.
func TestToolRegistrationStillPrefixesNames(t *testing.T) {
	g := &Gateway{}
	upstream := draft7Tool()

	registration := g.toolRegistration(t.Context(), testServerConfig(), upstream, "airtable")

	require.Equal(t, "airtable__list_bases", registration.Tool.Name)
	require.Equal(t, "list_bases", upstream.Name, "the upstream tool was renamed in place")
	require.NotNil(t, registration.Handler)
}

// TestToolRegistrationRelaysSchemasItCannotTranslate covers the seam's side of
// the package's bail-out: a schema with no faithful 2020-12 equivalent must
// still reach the client, unchanged, rather than being dropped or half-rewritten.
func TestToolRegistrationRelaysSchemasItCannotTranslate(t *testing.T) {
	g := &Gateway{}
	// draft-07 ignores the assertion beside a $ref; 2020-12 enforces it.
	untranslatable := map[string]any{
		"$schema": draft7Dialect,
		"type":    "object",
		"properties": map[string]any{
			"a": map[string]any{"$ref": "#/$defs/x", "maxLength": float64(3)},
		},
	}
	upstream := draft7Tool()
	upstream.OutputSchema = untranslatable

	registration := g.toolRegistration(t.Context(), testServerConfig(), upstream, "")

	require.Equal(t, untranslatable, registration.Tool.OutputSchema)
	// The input schema is independent and still translated.
	require.Equal(t, toolschema.Dialect202012, dialectOf(t, registration.Tool.InputSchema))
}

// TestToolRegistrationLeavesTypedSchemasAlone guards the catalog-defined (POCI)
// tools, whose schemas the gateway builds itself as *jsonschema.Schema rather
// than decoding from an upstream server.
func TestToolRegistrationLeavesTypedSchemasAlone(t *testing.T) {
	g := &Gateway{}
	upstream := &mcp.Tool{Name: "curl", InputSchema: map[string]any{"type": "object"}}

	registration := g.toolRegistration(t.Context(), testServerConfig(), upstream, "")

	require.Equal(t, map[string]any{"type": "object"}, registration.Tool.InputSchema)
	require.Nil(t, registration.Tool.OutputSchema)
}
