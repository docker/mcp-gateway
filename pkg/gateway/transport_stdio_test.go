package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestStartStdioServer_DoesNotBlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	impl := &mcp.Implementation{}
	opts := &mcp.ServerOptions{}

	g := &Gateway{
		mcpServer: mcp.NewServer(impl, opts),
	}

	done := make(chan struct{})

	go func() {
		err := g.startStdioServer(ctx, nil, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(100 * time.Millisecond):
		t.Fatal("startStdioServer blocked, expected non-blocking behavior")
	}
}
