package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func List(ctx context.Context, version string, debug bool, show, tool, format string) error {
	c, err := start(ctx, version, debug)
	if err != nil {
		return fmt.Errorf("starting client: %w", err)
	}
	defer c.Close()

	response, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("listing tools: %w", err)
	}

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
		}
	case "count":
		if format == "json" {
			fmt.Printf("{\"count\": %d}\n", len(response.Tools))
		} else {
			fmt.Println(len(response.Tools), "tools")
		}
	case "inspect":
		var found *mcp.Tool
		for _, t := range response.Tools {
			if t.Name == tool {
				found = &t
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
			for name, v := range found.InputSchema.Properties {
				propertyType := v.(map[string]any)["type"]
				if propertyType == "" || propertyType == nil {
					propertyType = "string"
				}

				desc := v.(map[string]any)["description"]
				if desc == nil {
					// Why duckduckgo is using that?
					desc = v.(map[string]any)["title"]
				}

				fmt.Printf(" - %s (%v): %v\n", name, propertyType, desc)
			}
		}
	}

	return nil
}

func toolDescription(tool mcp.Tool) string {
	if tool.Annotations.Title != "" {
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
