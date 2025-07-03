package gateway

import (
	"context"
	"io"
	"net"
	"net/http"

	"github.com/mark3labs/mcp-go/server"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/sockets"
)

func (g *Gateway) startStdioServer(ctx context.Context, newMCPServer func() *server.MCPServer, stdin io.Reader, stdout io.Writer) error {
	return server.NewStdioServer(newMCPServer()).Listen(ctx, stdin, stdout)
}

func (g *Gateway) startSseServer(ctx context.Context, newMCPServer func() *server.MCPServer, ln net.Listener) error {
	mux := http.NewServeMux()
	sseServer := server.NewSSEServer(newMCPServer())
	mux.Handle("/sse", sseServer.SSEHandler())
	mux.Handle("/message", sseServer.MessageHandler())
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/sse", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		if g.health.IsHealthy() {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})
	httpServer := &http.Server{
		Handler: mux,
	}
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	return httpServer.Serve(ln)
}

func (g *Gateway) startStreamingServer(ctx context.Context, newMCPServer func() *server.MCPServer, ln net.Listener) error {
	mux := http.NewServeMux()
	mux.Handle("/mcp", server.NewStreamableHTTPServer(newMCPServer()))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/mcp", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		if g.health.IsHealthy() {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})
	httpServer := &http.Server{
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	return httpServer.Serve(ln)
}

func (g *Gateway) startStdioOverTCPServer(ctx context.Context, newMCPServer func() *server.MCPServer, ln net.Listener) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			conn, err := sockets.AcceptWithContext(ctx, ln)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				logf("Error accepting the connection: %v", err)
				continue
			}

			newServer := server.NewStdioServer(newMCPServer())
			go func() {
				defer conn.Close()

				if err := newServer.Listen(ctx, conn, conn); err != nil {
					logf("Error listening: %v", err)
				}
			}()
		}
	}
}
