package main

import (
	"context"
	_ "embed"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed ui.html
var uiHTML string

func main() {
	// Create a server with a single tool that says "Hi".
	server := mcp.NewServer(&mcp.Implementation{Name: "greeter"}, nil)

	// Register the UI HTML as a resource for ChatGPT Apps
	server.AddResource(&mcp.Resource{
		URI:         "ui://greeter/widget.html",
		Name:        "Greeter Widget",
		Description: "Interactive UI for the greeter tool",
		MIMEType:    "text/html+skybridge",
	}, func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      "ui://greeter/widget.html",
					MIMEType: "text/html+skybridge",
					Text:     uiHTML,
				},
			},
		}, nil
	})

	// Using the generic AddTool automatically populates the input and output
	// schema of the tool.
	//
	// The schema considers 'json' and 'jsonschema' struct tags to get argument
	// names and descriptions.
	type args struct {
		Name         string `json:"name" jsonschema:"the person to greet"`
		GreetingType string `json:"greetingType,omitempty" jsonschema:"type of greeting (Hi, Hey, or Hello)"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "greet",
		Description: "greet someone with a customizable greeting",
		Meta: mcp.Meta{
			"openai/outputTemplate":          "ui://greeter/widget.html",
			"openai/toolInvocation/invoking": "Greeting...",
			"openai/widgetAccessible":        true,
		},
	}, func(_ context.Context, _ *mcp.CallToolRequest, args args) (*mcp.CallToolResult, any, error) {
		// Default to "Hi" if not specified
		greetingType := args.GreetingType
		if greetingType == "" {
			greetingType = "Hi"
		}
		greeting := greetingType + " " + args.Name + "!"

		// Return structured data that the UI can consume
		structuredData := map[string]any{
			"greeting":     greeting,
			"name":         args.Name,
			"greetingType": greetingType,
		}

		result := &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: greeting},
			},
		}

		// Add metadata linking to the UI resource
		result.SetMeta(map[string]any{
			"outputTemplate":    "ui://greeter/widget.html",
			"structuredContent": structuredData,
		})

		return result, structuredData, nil
	})

	// server.Run runs the server on the given transport.
	//
	// In this case, the server communicates over stdin/stdout.
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("Server failed: %v", err)
	}
}
