package gateway

import (
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
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
	return &catalog.ServerConfig{Name: "example-mcp-server"}
}

func TestToolRegistrationTranslatesSchemaDialects(t *testing.T) {
	g := &Gateway{}
	upstream := draft7Tool()
	before, err := json.Marshal(upstream)
	require.NoError(t, err)

	registration := g.toolRegistration(t.Context(), testServerConfig(), upstream, "", newRelayedDialects())

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

	registration := g.toolRegistration(t.Context(), testServerConfig(), draft7Tool(), "", newRelayedDialects())

	require.Equal(t, draft7Dialect, dialectOf(t, registration.Tool.InputSchema))
	require.Equal(t, draft7Dialect, dialectOf(t, registration.Tool.OutputSchema))
}

// TestToolRegistrationStillPrefixesNames pins the behaviour that shared this
// code path before normalization was added, so a future edit to one cannot
// quietly drop the other.
func TestToolRegistrationStillPrefixesNames(t *testing.T) {
	g := &Gateway{}
	upstream := draft7Tool()

	registration := g.toolRegistration(t.Context(), testServerConfig(), upstream, "example", newRelayedDialects())

	require.Equal(t, "example__list_bases", registration.Tool.Name)
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
			"a": map[string]any{"$ref": "#/definitions/x", "maxLength": float64(3)},
		},
	}
	upstream := draft7Tool()
	upstream.OutputSchema = untranslatable

	registration := g.toolRegistration(t.Context(), testServerConfig(), upstream, "", newRelayedDialects())

	require.Equal(t, untranslatable, registration.Tool.OutputSchema)
	// The input schema is independent and still translated.
	require.Equal(t, toolschema.Dialect202012, dialectOf(t, registration.Tool.InputSchema))
}

// TestToolRegistrationLeavesTypedSchemasAlone guards the catalog-defined (POCI)
// tools, whose schemas the gateway builds itself as *jsonschema.Schema rather
// than decoding from an upstream server. Same, not Equal: the claim is that the
// gateway's own schema object reaches registration untouched.
func TestToolRegistrationLeavesTypedSchemasAlone(t *testing.T) {
	g := &Gateway{}
	built := &jsonschema.Schema{Type: "object"}
	upstream := &mcp.Tool{Name: "curl", InputSchema: built}

	registration := g.toolRegistration(t.Context(), testServerConfig(), upstream, "", newRelayedDialects())

	require.Same(t, built, registration.Tool.InputSchema)
	require.Nil(t, registration.Tool.OutputSchema)
}

// TestRelayedDialectsReportsOncePerReason pins the log volume. Normalization
// runs per tool per schema field, so reporting there would write two lines per
// affected tool on every capability refresh, where every neighbouring
// diagnostic in listCapabilities is one line per server.
func TestRelayedDialectsReportsOncePerReason(t *testing.T) {
	relayed := newRelayedDialects()
	for range 50 {
		relayed.record("outputSchema", "some reason")
	}
	relayed.record("inputSchema", "some reason")
	relayed.record("outputSchema", "another reason")

	require.Len(t, relayed.counts, 3, "reasons must collapse per field and reason")
	require.Equal(t, 50, relayed.counts["outputSchema: some reason"])

	// A nil collector is the zero-work path for callers that do not want one.
	var absent *relayedDialects
	require.NotPanics(t, func() {
		absent.record("outputSchema", "x")
		absent.report("server")
	})
}

// TestToolRegistrationRecordsADialectItCannotTranslate covers the seam for a
// dialect outside the translatable set. The schema is relayed like any other
// untranslatable one, but it is reported under its own outcome, because the
// remedy is different: nothing about the schema can be fixed here.
func TestToolRegistrationRecordsADialectItCannotTranslate(t *testing.T) {
	g := &Gateway{}
	upstream := draft7Tool()
	upstream.OutputSchema = map[string]any{
		"$schema": "http://json-schema.org/draft-03/schema#",
		"type":    "object",
	}
	relayed := newRelayedDialects()

	registration := g.toolRegistration(t.Context(), testServerConfig(), upstream, "", relayed)

	require.Equal(t, "http://json-schema.org/draft-03/schema#", dialectOf(t, registration.Tool.OutputSchema))
	require.Equal(t, 1, relayed.counts["outputSchema: the declared dialect is not one the gateway translates"])
	// The input schema is draft-07 and independent, so it is still translated
	// and contributes nothing to the relay report.
	require.Equal(t, toolschema.Dialect202012, dialectOf(t, registration.Tool.InputSchema))
	require.Len(t, relayed.counts, 1)
}

// TestToolRegistrationTranslatesTheOutputSchema exists because an earlier
// version of these tests read only InputSchema, so deleting the OutputSchema
// line left the whole suite green. outputSchema is half the advertised surface
// and the field the original bug report named.
func TestToolRegistrationTranslatesTheOutputSchema(t *testing.T) {
	g := &Gateway{}
	upstream := draft7Tool()
	// Make the two fields distinguishable, so translating only one cannot pass.
	upstream.InputSchema = map[string]any{"type": "object"}

	registration := g.toolRegistration(t.Context(), testServerConfig(), upstream, "", newRelayedDialects())

	require.Equal(t, toolschema.Dialect202012, dialectOf(t, registration.Tool.OutputSchema),
		"the output schema was not translated")
	// And the untouched input schema declares nothing, so it must gain nothing.
	require.Nil(t, dialectOf(t, registration.Tool.InputSchema))
}
