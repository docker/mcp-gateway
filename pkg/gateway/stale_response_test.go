package gateway

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

func TestIsStaleEmptySuccess(t *testing.T) {
	t.Run("nil result", func(t *testing.T) {
		assert.False(t, isStaleEmptySuccess(nil))
	})

	t.Run("error result", func(t *testing.T) {
		assert.False(t, isStaleEmptySuccess(&mcp.CallToolResult{
			IsError: true,
		}))
	})

	t.Run("empty content success", func(t *testing.T) {
		assert.True(t, isStaleEmptySuccess(&mcp.CallToolResult{
			Content: []mcp.Content{},
		}))
	})

	t.Run("nil content success", func(t *testing.T) {
		assert.True(t, isStaleEmptySuccess(&mcp.CallToolResult{}))
	})

	t.Run("non-empty content", func(t *testing.T) {
		assert.False(t, isStaleEmptySuccess(&mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: `{"issues":[]}`}},
		}))
	})

	t.Run("structured content only", func(t *testing.T) {
		assert.False(t, isStaleEmptySuccess(&mcp.CallToolResult{
			StructuredContent: map[string]any{"issues": []any{}},
		}))
	})
}

func TestIsSafeToRetryTool(t *testing.T) {
	t.Run("nil annotations", func(t *testing.T) {
		assert.False(t, isSafeToRetryTool(nil))
	})

	t.Run("read only", func(t *testing.T) {
		assert.True(t, isSafeToRetryTool(&mcp.ToolAnnotations{ReadOnlyHint: true}))
	})

	t.Run("idempotent write", func(t *testing.T) {
		assert.True(t, isSafeToRetryTool(&mcp.ToolAnnotations{IdempotentHint: true}))
	})

	t.Run("no hints", func(t *testing.T) {
		assert.False(t, isSafeToRetryTool(&mcp.ToolAnnotations{}))
	})
}
