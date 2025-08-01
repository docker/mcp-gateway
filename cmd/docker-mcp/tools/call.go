package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func Call(ctx context.Context, version string, gatewayArgs []string, debug bool, args []string) error {
	if len(args) == 0 {
		return errors.New("no tool name provided")
	}
	toolName := args[0]

	c, err := start(ctx, version, gatewayArgs, debug)
	if err != nil {
		return fmt.Errorf("starting client: %w", err)
	}
	defer c.Close()

	params := &mcp.CallToolParams{
		Name:      toolName,
		Arguments: parseArgs(args[1:]),
	}

	start := time.Now()
	response, err := c.CallTool(ctx, params)
	if err != nil {
		return fmt.Errorf("listing tools: %w", err)
	}
	fmt.Println("Tool call took:", time.Since(start))

	if response.IsError {
		return fmt.Errorf("error calling tool %s: %s", toolName, toText(response))
	}

	fmt.Println(toText(response))

	return nil
}

func toText(response *mcp.CallToolResult) string {
	var contents []string

	for _, content := range response.Content {
		if textContent, ok := content.(*mcp.TextContent); ok {
			contents = append(contents, textContent.Text)
		} else {
			contents = append(contents, fmt.Sprintf("%v", content))
		}
	}

	return strings.Join(contents, "\n")
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

	return parsed
}
