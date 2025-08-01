package tools

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

func TestToolDescription(t *testing.T) {
	tests := []struct {
		name     string
		tool     *mcp.Tool
		expected string
	}{
		{
			name: "uses title from annotations when available",
			tool: &mcp.Tool{
				Name:        "test-tool",
				Description: "Longer description",
				Annotations: &mcp.ToolAnnotations{
					Title: "Short Title",
				},
			},
			expected: "Short Title",
		},
		{
			name: "falls back to description when no title",
			tool: &mcp.Tool{
				Name:        "test-tool",
				Description: "Simple description.",
			},
			expected: "Simple description.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toolDescription(tt.tool)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDescriptionSummary(t *testing.T) {
	tests := []struct {
		name        string
		description string
		expected    string
	}{
		{
			name:        "single sentence",
			description: "This is a simple description.",
			expected:    "This is a simple description.",
		},
		{
			name:        "multiple sentences - takes first",
			description: "First sentence. Second sentence.",
			expected:    "First sentence.",
		},
		{
			name:        "empty description",
			description: "",
			expected:    "",
		},
		{
			name:        "stops at Error Responses",
			description: "Tool description.\nError Responses:\n- 404 if not found",
			expected:    "Tool description.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := descriptionSummary(tt.description)
			assert.Equal(t, tt.expected, result)
		})
	}
}
