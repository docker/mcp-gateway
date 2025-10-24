package gateway

import (
	"context"
	"io"
	"net"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/health"
)

func (g *Gateway) startStdioServer(ctx context.Context, _ io.Reader, _ io.Writer) error {
	transport := &mcp.StdioTransport{}
	return g.mcpServer.Run(ctx, transport)
}

func (g *Gateway) startSseServer(ctx context.Context, ln net.Listener) error {
	mux := http.NewServeMux()
	mux.Handle("/health", healthHandler(&g.health))
	mux.Handle("/", redirectHandler("/sse"))
	sseHandler := mcp.NewSSEHandler(func(_ *http.Request) *mcp.Server {
		return g.mcpServer
	}, nil)
	mux.Handle("/sse", sseHandler)

	// Wrap with authentication middleware
	var handler http.Handler = mux
	if g.authToken != "" {
		handler = authenticationMiddleware(g.authToken, mux)
	}

	httpServer := &http.Server{
		Handler: handler,
	}
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	return httpServer.Serve(ln)
}

func (g *Gateway) startStreamingServer(ctx context.Context, ln net.Listener) error {
	mux := http.NewServeMux()
	mux.Handle("/health", healthHandler(&g.health))
	mux.Handle("/", redirectHandler("/mcp"))
	streamHandler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return g.mcpServer
	}, nil)
	mux.Handle("/mcp", streamHandler)

	// Wrap with authentication middleware
	var handler http.Handler = mux
	if g.authToken != "" {
		handler = authenticationMiddleware(g.authToken, mux)
	}

	httpServer := &http.Server{
		Handler: handler,
	}

	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	return httpServer.Serve(ln)
}

func redirectHandler(target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	}
}

func healthHandler(state *health.State) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if state.IsHealthy() {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}
}
