package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/hints"
)

func List(ctx context.Context, version string, gatewayArgs []string, debug bool, show, tool, format string) error {
	// Initialize telemetry for CLI operations
	meter := otel.GetMeterProvider().Meter("github.com/docker/mcp-gateway")
	toolsDiscoveredGauge, _ := meter.Int64Gauge("mcp.cli.tools.discovered",
		metric.WithDescription("Number of tools discovered by CLI"),
		metric.WithUnit("1"))

	c, err := start(ctx, version, gatewayArgs, debug)
	if err != nil {
		return fmt.Errorf("starting client: %w", err)
	}
	defer c.Close()

	response, err := c.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return fmt.Errorf("listing tools: %w", err)
	}

	// Record how many tools were discovered
	toolsDiscoveredGauge.Record(ctx, int64(len(response.Tools)),
		metric.WithAttributes(
			attribute.String("mcp.cli.command", "tools."+show),
		))

	switch show {
	case "list":
		if format == "json" {
			buf, err := json.MarshalIndent(response.Tools, "", "  ")
			if err != nil {
				return fmt.Errorf("marshalling tools: %w", err)
			}

			fmt.Println(string(buf))
		} else {
			fmt.Println(len(response.Tools), "tools:")
			for _, tool := range response.Tools {
				fmt.Println(" -", tool.Name, "-", toolDescription(tool))
			}
			if hints.Enabled() {
				hints.TipCyan.Print("Tip: For tool details, use ")
				hints.TipCyanBoldItalic.Print("docker mcp tools inspect <tool-name>")
				hints.TipCyan.Print(". To test the tool, use ")
				hints.TipCyanBoldItalic.Println("docker mcp tools call <tool-name>")
				fmt.Println()
			}
		}
	case "count":
		if format == "json" {
			fmt.Printf("{\"count\": %d}\n", len(response.Tools))
		} else {
			fmt.Println(len(response.Tools), "tools")
			if hints.Enabled() {
				hints.TipCyan.Print("Tip: To see all available tools, use ")
				hints.TipCyanBoldItalic.Println("docker mcp tools ls")
				fmt.Println()
			}
		}
	case "inspect":
		var found *mcp.Tool
		for _, t := range response.Tools {
			if t.Name == tool {
				found = t
				break
			}
		}
		if found == nil {
			return fmt.Errorf("tool %s not found", tool)
		}

		if format == "json" {
			buf, err := json.MarshalIndent(found, "", "  ")
			if err != nil {
				return fmt.Errorf("marshalling tools: %w", err)
			}

			fmt.Println(string(buf))
		} else {
			fmt.Println("Name:", found.Name)
			fmt.Println("Description:", found.Description)

			// TODO: Need to properly handle the new jsonschema.Schema format
			if found.InputSchema != nil {
				fmt.Println("Input schema: Complex schema (detailed inspection not yet implemented)")
			}
		}
	}

	return nil
}

func toolDescription(tool *mcp.Tool) string {
	if tool.Annotations != nil && tool.Annotations.Title != "" {
		return tool.Annotations.Title
	}
	return descriptionSummary(tool.Description)
}

func descriptionSummary(description string) string {
	var result []string

	for line := range strings.SplitSeq(description, "\n") {
		line := strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "Error Responses:" || line == "Returns:" {
			break
		}

		if strings.Contains(line, ". ") {
			parts := strings.SplitN(line, ". ", 2)
			result = append(result, parts[0]+".")
			break
		}

		result = append(result, line)
		if strings.HasSuffix(line, ".") {
			break
		}
	}

	return strings.TrimSpace(strings.Join(result, " "))
}
