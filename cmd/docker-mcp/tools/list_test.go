package tools

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

func TestToolDescription(t *testing.T) {
	// Test that title annotation takes precedence over description
	tool := &mcp.Tool{
		Description: "Longer description",
		Annotations: &mcp.ToolAnnotations{Title: "Short Title"},
	}
	result := toolDescription(tool)
	assert.Equal(t, "Short Title", result)
}

func TestDescriptionSummary(t *testing.T) {
	// Test key behavior: stops at first sentence
	result := descriptionSummary("First sentence. Second sentence.")
	assert.Equal(t, "First sentence.", result)

	// Test key behavior: stops at "Error Responses:"
	result = descriptionSummary("Tool description.\nError Responses:\n- 404 if not found")
	assert.Equal(t, "Tool description.", result)
}
