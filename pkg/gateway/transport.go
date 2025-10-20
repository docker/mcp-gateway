package gateway

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"

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
	// Wrap with Origin validation to prevent DNS rebinding
	mux.Handle("/sse", originSecurityHandler(sseHandler))

	// Wrap entire mux with authentication middleware (excludes /health)
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
	// Wrap with Origin validation to prevent DNS rebinding
	mux.Handle("/mcp", originSecurityHandler(streamHandler))

	// Wrap entire mux with authentication middleware (excludes /health)
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

// originSecurityHandler validates Origin header to prevent DNS rebinding attacks.
// This implements the security requirement from the MCP specification:
// https://modelcontextprotocol.io/specification/2024-11-05/basic/transports#security-warning
func originSecurityHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Allow requests with no Origin header
		// This handles:
		// - Non-browser clients (curl, SDKs) - no Origin header sent
		// - Same-origin requests - browsers don't send Origin for same-origin
		if origin != "" {
			// For cross-origin requests (browser-based), only allow localhost origins
			// This prevents DNS rebinding attacks using 0.0.0.0 or malicious domains
			allowed := origin == "http://localhost" ||
				origin == "https://localhost" ||
				origin == "http://127.0.0.1" ||
				origin == "https://127.0.0.1" ||
				strings.HasPrefix(origin, "http://localhost:") ||
				strings.HasPrefix(origin, "https://localhost:") ||
				strings.HasPrefix(origin, "http://127.0.0.1:") ||
				strings.HasPrefix(origin, "https://127.0.0.1:")

			if !allowed {
				http.Error(w, "Forbidden: Invalid Origin header", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
