package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// server is the test server implementation
func server() {
	ctx, done := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer done()

	log.SetOutput(os.Stderr)

	server := mcp.NewServer(
		&mcp.Implementation{Name: "repro", Version: "0.1.0"},
		nil)

	server.AddTool(
		&mcp.Tool{
			Name: "trigger_elicit",
			InputSchema: &jsonschema.Schema{
				Type:       "object",
				Properties: map[string]*jsonschema.Schema{},
			},
		},
		func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := req.Session.Elicit(ctx, &mcp.ElicitParams{Message: "elicitation"})
			if err != nil {
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("error %s", err)}}}, nil
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("elicit result: action=%s, content=%+v", result.Action, result.Content)}}}, nil
		},
	)

	server.AddTool(
		&mcp.Tool{
			Name: "trigger_tool_change",
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"action": {
						Type: "string",
						Enum: []interface{}{"add", "remove"}, //nolint:gofmt
					},
					"toolName": {
						Type: "string",
					},
				},
				Required: []string{"action", "toolName"},
			},
		},
		func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			log.Printf("Received arguments: %T = %+v", req.Params.Arguments, req.Params.Arguments)

			var args map[string]any
			switch v := req.Params.Arguments.(type) {
			case map[string]any:
				args = v
			case json.RawMessage:
				if err := json.Unmarshal(v, &args); err != nil {
					return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("failed to unmarshal arguments: %v", err)}}}, nil
				}
			default:
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("arguments must be an object, got %T: %+v", req.Params.Arguments, req.Params.Arguments)}}}, nil
			}

			action, ok := args["action"].(string)
			if !ok {
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "action parameter must be a string"}}}, nil
			}

			toolName, ok := args["toolName"].(string)
			if !ok {
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "toolName parameter must be a string"}}}, nil
			}

			switch action {
			case "add":
				// Add a dynamic tool
				server.AddTool(
					&mcp.Tool{
						Name: toolName,
						InputSchema: &jsonschema.Schema{
							Type:       "object",
							Properties: map[string]*jsonschema.Schema{},
						},
					},
					func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
						return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Dynamic tool '%s' executed", toolName)}}}, nil
					},
				)
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Added tool '%s'", toolName)}}}, nil

			case "remove":
				// Remove the dynamic tool
				server.RemoveTools(toolName)
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Removed tool '%s'", toolName)}}}, nil

			default:
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Unknown action '%s'. Use 'add' or 'remove'", action)}}}, nil
			}
		},
	)

	t := &mcp.LoggingTransport{
		Transport: &mcp.StdioTransport{},
		Writer:    os.Stderr,
	}

	errCh := make(chan error, 1)
	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)

		log.Print("[INFO] stdio server is starting")
		defer log.Print("[INFO] stdio server stopped")

		if err := server.Run(ctx, t); err != nil && !errors.Is(err, mcp.ErrConnectionClosed) {
			log.Print("[INFO] server is terminated")
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	select {
	case err := <-errCh:
		log.Printf("[ERROR] failed to run stdio server: %s", err)
		done()
		os.Exit(1)
	case <-ctx.Done():
		log.Print("[INFO] provided context was closed, triggering shutdown")
	}

	// Wait for goroutine to exit
	log.Print("[INFO] waiting for server to stop")
	<-doneCh
}

func main() {
	server()
}
