package tools

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallNoToolName(t *testing.T) {
	err := Call(context.Background(), "2", []string{}, false, []string{})
	require.Error(t, err)
	assert.Equal(t, "no tool name provided", err.Error())
}

func TestToText(t *testing.T) {
	// Test basic functionality - joining multiple text contents
	response := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "First"},
			&mcp.TextContent{Text: "Second"},
		},
	}
	result := toText(response)
	assert.Equal(t, "First\nSecond", result)
}

func TestParseArgs(t *testing.T) {
	// Test key=value parsing
	result := parseArgs([]string{"key1=value1", "key2=value2"})
	expected := map[string]any{"key1": "value1", "key2": "value2"}
	assert.Equal(t, expected, result)

	// Test duplicate keys become arrays
	result = parseArgs([]string{"tag=red", "tag=blue"})
	expected = map[string]any{"tag": []any{"red", "blue"}}
	assert.Equal(t, expected, result)
}
