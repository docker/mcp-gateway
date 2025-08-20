package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// server is the test server implementation
//
//nolint:unused
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
		func(ctx context.Context, ss *mcp.ServerSession, _ *mcp.CallToolParamsFor[map[string]any]) (*mcp.CallToolResult, error) {
			result, err := ss.Elicit(ctx, &mcp.ElicitParams{Message: "elicitation"})
			if err != nil {
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("error %s", err)}}}, nil
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("elicit result: action=%s, content=%+v", result.Action, result.Content)}}}, nil
		},
	)

	t := mcp.NewLoggingTransport(mcp.NewStdioTransport(), os.Stderr)

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
