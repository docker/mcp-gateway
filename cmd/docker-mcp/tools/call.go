package tools

import (
	"context"
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
