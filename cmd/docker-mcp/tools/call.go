package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func Call(ctx context.Context, version string, gatewayArgs []string, debug bool, args []string) error {
	if len(args) == 0 {
		return errors.New("no tool name provided")
	}
	toolName := args[0]

	// Initialize telemetry for CLI tool calls
	meter := otel.GetMeterProvider().Meter("github.com/docker/mcp-gateway")
	toolCallCounter, _ := meter.Int64Counter("mcp.cli.tool.calls",
		metric.WithDescription("Tool calls from CLI"),
		metric.WithUnit("1"))
	toolCallDuration, _ := meter.Float64Histogram("mcp.cli.tool.duration",
		metric.WithDescription("Tool call duration from CLI"),
		metric.WithUnit("ms"))

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
	duration := time.Since(start)

	// Record metrics
	attrs := []attribute.KeyValue{
		attribute.String("mcp.tool.name", toolName),
		attribute.String("mcp.cli.command", "tools.call"),
	}

	if err != nil {
		attrs = append(attrs, attribute.Bool("mcp.tool.error", true))
		toolCallCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
		return fmt.Errorf("calling tool: %w", err)
	}

	attrs = append(attrs, attribute.Bool("mcp.tool.error", response.IsError))
	toolCallCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	toolCallDuration.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs...))

	fmt.Println("Tool call took:", duration)

	if response.IsError {
		return fmt.Errorf("error calling tool %s: %s", toolName, toText(response))
	}

	fmt.Println(toText(response))

	return nil
}

// toText flattens a CallToolResult into a human-readable string for CLI output.
//
// TextContent entries are rendered using their raw text, while any non-text
// content types fall back to a generic fmt.Sprintf representation. Multiple
// content blocks are joined with newlines to preserve ordering.
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

// parseArgs converts CLI arguments in the form key=value into a map suitable
// for MCP tool invocation.
//
// It supports simple values as well as complex JSON payloads. When a value
// looks like valid JSON (objects, arrays, numbers, booleans, or null), it is
// automatically unmarshaled into its corresponding Go type. Otherwise, the
// value is treated as a plain string.
//
// Repeated keys are aggregated into a slice to preserve all provided values.
func parseArgs(args []string) map[string]any {
	parsed := map[string]any{}

	for _, arg := range args {
		var (
			key   string
			value any
		)

		// Split argument into key=value (only once)
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			key = parts[0]
			rawValue := parts[1]

			// Attempt to parse the value as JSON.
			// If successful, use the parsed object (map/slice/etc).
			// Otherwise, fall back to treating it as a plain string.
			var parsedValue any
			if err := json.Unmarshal([]byte(rawValue), &parsedValue); err == nil {
				value = parsedValue
			} else {
				value = rawValue
			}
		} else {
			// Flag-style argument without an explicit value
			key = arg
			value = nil
		}

		// Handle repeated keys by aggregating values into a slice
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
