package gateway

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/health"
)

// startStdioServer starts the MCP server using stdio transport.
//
// The stdio transport blocks while processing input/output, so it must be
// started asynchronously to keep the gateway lifecycle consistent with
// other transports (HTTP/SSE).
func (g *Gateway) startStdioServer(ctx context.Context, _ io.Reader, _ io.Writer) error {
	transport := &mcp.StdioTransport{}

	go func() {
		_ = g.mcpServer.Run(ctx, transport)
	}()

	return nil
}

func (g *Gateway) startSseServer(ctx context.Context, ln net.Listener) error {
	mux := http.NewServeMux()
	mux.Handle("/health", healthHandler(&g.health))
	mux.Handle("/", redirectHandler("/sse"))
	sseHandler := mcp.NewSSEHandler(func(_ *http.Request) *mcp.Server {
		return g.mcpServer
	}, nil)
	mux.Handle("/sse", originSecurityHandler(sseHandler))

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
	mux.Handle("/mcp", originSecurityHandler(streamHandler))

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

// isAllowedOrigin validates that the origin is from localhost.
// Returns true if the origin's hostname is "localhost", "127.0.0.1", or "::1" (IPv6 localhost).
func isAllowedOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false // Invalid URL format
	}

	// Only allow http or https schemes
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	// Extract hostname (without port)
	host := u.Hostname()

	// Only allow localhost, IPv4 loopback, or IPv6 loopback
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// originSecurityHandler validates Origin header to prevent DNS rebinding attacks.
func originSecurityHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Allow requests with no Origin header
		// This handles:
		// - Non-browser clients (curl, SDKs) - no Origin header sent
		// - Same-origin requests - browsers don't send Origin for same-origin
		if origin != "" && !isAllowedOrigin(origin) {
			msg := fmt.Sprintf("Forbidden: Origin, if set, must be localhost, 127.0.0.1, or ::1, got: %s", origin)
			http.Error(w, msg, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
