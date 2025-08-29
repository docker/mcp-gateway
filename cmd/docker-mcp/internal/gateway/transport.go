package gateway

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/health"
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
	})
	mux.Handle("/sse", sseHandler)
	httpServer := &http.Server{
		Handler: mux,
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
	httpServer := &http.Server{
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	return httpServer.Serve(ln)
}

func (g *Gateway) startCentralStreamingServer(ctx context.Context, ln net.Listener, configuration Configuration) error {
	mux := http.NewServeMux()
	mux.Handle("/health", healthHandler(&g.health))
	mux.Handle("/", redirectHandler("/mcp"))

	var lock sync.Mutex
	handlersPerSelectionOfServers := map[string]*mcp.StreamableHTTPHandler{}
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		serverNames := r.Header.Get("x-mcp-servers")
		if len(serverNames) == 0 {
			log("No server names provided in the request header 'x-mcp-servers'")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, "No server names provided in the request header 'x-mcp-servers'")
			return
		}

		lock.Lock()
		handler := handlersPerSelectionOfServers[serverNames]
		if handler == nil {
			if err := g.reloadConfiguration(ctx, configuration, parseServerNames(serverNames), nil); err != nil {
				lock.Unlock()
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = io.WriteString(w, "Failed to reload configuration")
				return
			}
			handler = mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
				return g.mcpServer
			}, nil)
			handlersPerSelectionOfServers[serverNames] = handler
		}
		lock.Unlock()

		handler.ServeHTTP(w, r)
	})
	httpServer := &http.Server{
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	g.health.SetHealthy()
	return httpServer.Serve(ln)
}

func parseServerNames(serverNames string) []string {
	var names []string
	for name := range strings.SplitSeq(serverNames, ",") {
		name := strings.TrimSpace(name)
		if name == "" {
			continue
		}

		names = append(names, name)
	}
	return names
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
