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

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected map[string]any
	}{
		{
			name:     "empty args",
			args:     []string{},
			expected: map[string]any{},
		},
		{
			name: "simple key-value pairs",
			args: []string{"key1=value1", "key2=value2"},
			expected: map[string]any{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "key without value",
			args: []string{"flag1", "key2=value2"},
			expected: map[string]any{
				"flag1": nil,
				"key2":  "value2",
			},
		},
		{
			name: "duplicate keys create array",
			args: []string{"tag=red", "tag=blue", "tag=green"},
			expected: map[string]any{
				"tag": []any{"red", "blue", "green"},
			},
		},
		{
			name: "values with equals signs",
			args: []string{"url=https://example.com/path?param=value"},
			expected: map[string]any{
				"url": "https://example.com/path?param=value",
			},
		},
		{
			name: "empty values",
			args: []string{"empty=", "key=value"},
			expected: map[string]any{
				"empty": "",
				"key":   "value",
			},
		},
		{
			name: "mixed duplicate and single keys",
			args: []string{"name=john", "tag=red", "tag=blue", "age=30"},
			expected: map[string]any{
				"name": "john",
				"tag":  []any{"red", "blue"},
				"age":  "30",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseArgs(tt.args)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToText(t *testing.T) {
	tests := []struct {
		name     string
		response *mcp.CallToolResult
		expected string
	}{
		{
			name: "single text content",
			response: &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "Hello world"},
				},
			},
			expected: "Hello world",
		},
		{
			name: "multiple text contents",
			response: &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "First line"},
					&mcp.TextContent{Text: "Second line"},
				},
			},
			expected: "First line\nSecond line",
		},
		{
			name: "empty content",
			response: &mcp.CallToolResult{
				Content: []mcp.Content{},
			},
			expected: "",
		},
		{
			name: "nil content",
			response: &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: "Before nil"},
					nil,
					&mcp.TextContent{Text: "After nil"},
				},
			},
			expected: "Before nil\n<nil>\nAfter nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toText(tt.response)
			assert.Equal(t, tt.expected, result)
		})
	}
}
