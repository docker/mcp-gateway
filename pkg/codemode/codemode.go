package codemode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const Prompt = `Run a Javascript script to call MCP tools.

Instead of calling individual tools directly, use this to write a Javascript script that calls as many tools as needed.
This allows you to combine multiple tool calls in a single request, perform conditional logic,
and manipulate the results before returning them.

Instructions:
 - The script has access to all the tools as plain javascript functions.
 - "await"/"async" are never needed. All the tool calls are synchronous.
 - Every tool function returns a string result.
 - The script must return a string result.

Available tools/functions:

`

type RunToolsWithJavascriptArgs struct {
	Script string `json:"script"`
}

// ToolSet represents a collection of MCP tools with lifecycle management
type ToolSet interface {
	Tools(ctx context.Context) ([]*ToolWithHandler, error)
}

// ToolWithHandler combines an MCP Tool definition with its handler function
type ToolWithHandler struct {
	Tool    *mcp.Tool
	Handler mcp.ToolHandler
}

type tool struct {
	toolsets []ToolSet
}

func (c *tool) Tools(ctx context.Context) ([]*ToolWithHandler, error) {
	var functionsDoc []string

	for _, toolset := range c.toolsets {
		allTools, err := toolset.Tools(ctx)
		if err != nil {
			return nil, err
		}

		for _, tool := range allTools {
			functionsDoc = append(functionsDoc, toolToJsDoc(tool.Tool))
		}
	}

	description := Prompt + strings.Join(functionsDoc, "\n")

	jstool := &ToolWithHandler{
		Tool: &mcp.Tool{
			Name:        "run_tools_with_javascript",
			Description: description,
			Annotations: &mcp.ToolAnnotations{
				Title: "Run tools with Javascript",
			},
			InputSchema: map[string]any{
				"type":     "object",
				"required": []string{"script"},
				"properties": map[string]any{
					"script": map[string]any{
						"type":        "string",
						"description": "script to execute",
					},
				},
			},
		},
		Handler: func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var args RunToolsWithJavascriptArgs
			if err := json.Unmarshal(request.Params.Arguments, &args); err != nil {
				return nil, fmt.Errorf("parsing arguments: %w", err)
			}

			output, err := c.runJavascript(ctx, args.Script)
			if err != nil {
				return nil, err
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: output},
				},
			}, nil
		},
	}

	return []*ToolWithHandler{jstool}, nil
}

func Wrap(toolsets []ToolSet) ToolSet {
	return &tool{
		toolsets: toolsets,
	}
}
