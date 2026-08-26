package interceptors

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	gcf "github.com/blackwell-systems/gcf-go"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// uniformRecords is a representative tool result: an array of uniform records, the shape
// most list/query MCP tools return. GCF factors the repeated field names into one header.
const uniformRecords = `[` +
	`{"id":101,"title":"Fix auth bug","state":"open","assignee":"ada","points":5},` +
	`{"id":102,"title":"Ship onboarding","state":"open","assignee":"grace","points":3},` +
	`{"id":103,"title":"Write RFC","state":"closed","assignee":"alan","points":8},` +
	`{"id":104,"title":"Migrate DB","state":"open","assignee":"kat","points":13}` +
	`]`

func TestGcfEncodeIfBeneficial(t *testing.T) {
	t.Run("re-encodes a uniform record array (smaller and lossless)", func(t *testing.T) {
		got, changed := gcfEncodeIfBeneficial(uniformRecords)
		require.True(t, changed, "a uniform record array should be re-encoded")
		assert.Less(t, len(got), len(uniformRecords), "GCF output must be smaller than the JSON")
		assertLosslessAgainstJSON(t, uniformRecords, got)
	})

	t.Run("passes through non-JSON prose unchanged", func(t *testing.T) {
		text := "The migration completed successfully. 4 tables were updated."
		got, changed := gcfEncodeIfBeneficial(text)
		assert.False(t, changed)
		assert.Equal(t, text, got)
	})

	t.Run("passes through malformed JSON unchanged", func(t *testing.T) {
		text := `{"id":1,"title":`
		got, changed := gcfEncodeIfBeneficial(text)
		assert.False(t, changed)
		assert.Equal(t, text, got)
	})

	t.Run("passes through a tiny object where GCF would not be smaller", func(t *testing.T) {
		text := `{"ok":true}`
		got, changed := gcfEncodeIfBeneficial(text)
		assert.False(t, changed)
		assert.Equal(t, text, got)
	})

	t.Run("preserves integers beyond 2^53 exactly or falls back", func(t *testing.T) {
		// 9007199254740993 = 2^53 + 1, not representable as float64. It must never be
		// silently rounded: either the round-trip is exact, or we keep the JSON.
		text := `[` +
			`{"id":9007199254740993,"n":1},` +
			`{"id":9007199254740994,"n":2},` +
			`{"id":9007199254740995,"n":3}` +
			`]`
		got, changed := gcfEncodeIfBeneficial(text)
		if changed {
			assertLosslessAgainstJSON(t, text, got)
			assert.Contains(t, got, "9007199254740993", "the exact large integer must survive")
		} else {
			assert.Equal(t, text, got)
		}
	})

	t.Run("passes through values outside the int64 domain", func(t *testing.T) {
		// 2^63 is above math.MaxInt64; EncodeGenericChecked returns an error, so we keep JSON.
		text := `[{"id":9223372036854775808,"n":1},{"id":9223372036854775809,"n":2}]`
		got, changed := gcfEncodeIfBeneficial(text)
		assert.False(t, changed)
		assert.Equal(t, text, got)
	})
}

func TestGCFOutputMiddleware(t *testing.T) {
	t.Run("ignores non-tools-call methods", func(t *testing.T) {
		handler := func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
			return &mcp.ListResourcesResult{}, nil
		}
		result, err := GCFOutputMiddleware()(handler)(context.Background(), "resources/list", &mcp.ListResourcesRequest{})
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("passes through handler errors unchanged", func(t *testing.T) {
		wantErr := errors.New("downstream failure")
		handler := func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
			return nil, wantErr
		}
		result, err := GCFOutputMiddleware()(handler)(context.Background(), "tools/call", &mcp.CallToolRequest{})
		require.ErrorIs(t, err, wantErr)
		assert.Nil(t, result)
	})

	t.Run("re-encodes a record-array text result and preserves StructuredContent", func(t *testing.T) {
		structured := map[string]any{"kept": true}
		handler := func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
			return &mcp.CallToolResult{
				Content:           []mcp.Content{&mcp.TextContent{Text: uniformRecords}},
				StructuredContent: structured,
			}, nil
		}
		result, err := GCFOutputMiddleware()(handler)(context.Background(), "tools/call", &mcp.CallToolRequest{})
		require.NoError(t, err)

		callResult, ok := result.(*mcp.CallToolResult)
		require.True(t, ok)
		text, ok := callResult.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		assert.Less(t, len(text.Text), len(uniformRecords), "text should be re-encoded smaller")
		assertLosslessAgainstJSON(t, uniformRecords, text.Text)
		assert.Equal(t, structured, callResult.StructuredContent, "StructuredContent must be untouched")
	})

	t.Run("leaves error results unchanged", func(t *testing.T) {
		handler := func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: uniformRecords}},
			}, nil
		}
		result, err := GCFOutputMiddleware()(handler)(context.Background(), "tools/call", &mcp.CallToolRequest{})
		require.NoError(t, err)
		text := result.(*mcp.CallToolResult).Content[0].(*mcp.TextContent)
		// Still the original JSON, not re-encoded to GCF (GCF output is not valid JSON).
		assert.JSONEq(t, uniformRecords, text.Text, "error result text must be left alone")
	})

	t.Run("leaves prose text results unchanged", func(t *testing.T) {
		prose := "Done. Nothing structured here."
		handler := func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: prose}}}, nil
		}
		result, err := GCFOutputMiddleware()(handler)(context.Background(), "tools/call", &mcp.CallToolRequest{})
		require.NoError(t, err)
		text := result.(*mcp.CallToolResult).Content[0].(*mcp.TextContent)
		assert.Equal(t, prose, text.Text)
	})
}

// assertLosslessAgainstJSON decodes the GCF wire and asserts it equals the original JSON
// value, order-insensitively and with exact numbers.
func assertLosslessAgainstJSON(t *testing.T, originalJSON, gcfWire string) {
	t.Helper()
	original, err := gcf.ParseJSONOrdered([]byte(originalJSON))
	require.NoError(t, err)
	roundTrip, err := gcf.DecodeGeneric(gcfWire)
	require.NoError(t, err)

	origBytes, err := json.Marshal(normalizeForCompare(original))
	require.NoError(t, err)
	rtBytes, err := json.Marshal(normalizeForCompare(roundTrip))
	require.NoError(t, err)
	assert.JSONEq(t, string(origBytes), string(rtBytes), "GCF must round-trip to the same value")
}
