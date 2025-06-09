package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

func Call(ctx context.Context, version string, debug bool, args []string) error {
	if len(args) == 0 {
		return errors.New("no tool name provided")
	}
	toolName := args[0]

	c, err := start(ctx, version, debug)
	if err != nil {
		return fmt.Errorf("starting client: %w", err)
	}
	defer c.Close()

	request := mcp.CallToolRequest{}
	request.Params.Name = toolName
	request.Params.Arguments = parseArgs(args[1:])

	start := time.Now()
	response, err := c.CallTool(ctx, request)
	if err != nil {
		return fmt.Errorf("listing tools: %w", err)
	}
	fmt.Println("Tool call took:", time.Since(start))

	if response.IsError {
		return fmt.Errorf("error calling tool: %s", toolName)
	}

	for _, content := range response.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			fmt.Println(textContent.Text)
		} else {
			fmt.Println(content)
		}
	}

	return nil
}

func parseArgs(args []string) map[string]any {
	parsed := map[string]any{}

	for _, arg := range args {
		var (
			key   string
			value any
		)

		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			key = parts[0]
			value = parts[1]
		} else {
			key = arg
			value = nil
		}

		if previous, found := parsed[key]; found {
			switch previous := previous.(type) {
			case []any:
				parsed[key] = append(previous, value)
			default:
				parsed[key] = []any{previous, value}
			}
		} else {
			parsed[key] = value
		}
	}

	// MCP servers return an error if the args are empty, so we make sure
	// there is at least one argument
	if len(parsed) == 0 {
		parsed["args"] = "..."
	}

	return parsed
}
