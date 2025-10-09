package codemode

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/dop251/goja"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (c *tool) runJavascript(ctx context.Context, script string) (string, error) {
	vm := goja.New()

	// Inject console object to the help the LLM debug its own code.
	_ = vm.Set("console", console())

	// Inject every tool as a javascript function.
	for _, toolset := range c.toolsets {
		allTools, err := toolset.Tools(ctx)
		if err != nil {
			return "", err
		}

		for _, toolWithHandler := range allTools {
			_ = vm.Set(toolWithHandler.Tool.Name, callTool(ctx, toolWithHandler))
		}
	}

	// Wrap the user script in an IIFE to allow top-level returns.
	script = "(() => {\n" + script + "\n})()"

	// Run the script.
	v, err := vm.RunString(script)
	if err != nil {
		return fmt.Sprintf("Error running script: %s", err), nil
	}

	// Some script are fire and forget and don't return anything.
	// In that case we return "done." to please the LLM which can't deal with empty responses.
	result := v.Export()
	if result == nil {
		return "<no output>", nil
	}

	return fmt.Sprintf("%v", result), nil
}

func callTool(ctx context.Context, toolWithHandler *ToolWithHandler) func(args map[string]any) (string, error) {
	return func(args map[string]any) (string, error) {
		// Extract required fields from InputSchema
		var required []string
		if toolWithHandler.Tool.InputSchema != nil {
			if schemaMap, ok := toolWithHandler.Tool.InputSchema.(map[string]any); ok {
				if req, ok := schemaMap["required"].([]any); ok {
					for _, r := range req {
						if reqStr, ok := r.(string); ok {
							required = append(required, reqStr)
						}
					}
				} else if req, ok := schemaMap["required"].([]string); ok {
					required = req
				}
			}
		}

		nonNilArgs := make(map[string]any)
		for k, v := range args {
			if slices.Contains(required, k) || v != nil {
				nonNilArgs[k] = v
			}
		}

		arguments, err := json.Marshal(nonNilArgs)
		if err != nil {
			return "", err
		}

		result, err := toolWithHandler.Handler(ctx, &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Name:      toolWithHandler.Tool.Name,
				Arguments: arguments,
			},
		})
		if err != nil {
			return "", err
		}

		// Extract text from Content
		if len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
				return textContent.Text, nil
			}
		}

		return "", nil
	}
}
