package oauth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/docker/mcp-gateway/pkg/log"
)

// DefaultOAuthPort is the default port for the OAuth callback server
// Can be overridden with MCP_GATEWAY_OAUTH_PORT environment variable
const DefaultOAuthPort = 5000

// CallbackData represents the data received from an OAuth callback
type CallbackData struct {
	Code  string
	State string
}

// CallbackServer is a temporary HTTP server that receives OAuth callbacks on localhost
type CallbackServer struct {
	port     int
	server   *http.Server
	listener net.Listener
	codeCh   chan CallbackData
	errCh    chan error
}

// getOAuthPort returns the OAuth callback port from environment variable or default
// Port validation ensures the value is in the valid range (1024-65535)
func getOAuthPort() int {
	if envPort := os.Getenv("MCP_GATEWAY_OAUTH_PORT"); envPort != "" {
		if port, err := strconv.Atoi(envPort); err == nil {
			// Validate port range
			if port > 1024 && port <= 65535 {
				return port
			}
			log.Logf("! Invalid MCP_GATEWAY_OAUTH_PORT %s (must be 1024-65535), using default %d", envPort, DefaultOAuthPort)
		} else {
			log.Logf("! Invalid MCP_GATEWAY_OAUTH_PORT %s (not a number), using default %d", envPort, DefaultOAuthPort)
		}
	}
	return DefaultOAuthPort
}

// NewCallbackServer creates a new callback server on a fixed port (default 5000)
// The port can be customized via MCP_GATEWAY_OAUTH_PORT environment variable
func NewCallbackServer() (*CallbackServer, error) {
	port := getOAuthPort()

	// Bind to the fixed port
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, fmt.Errorf(
			"OAuth callback port %d is already in use.\n\n"+
				"Solutions:\n"+
				"  1. Stop the service using port %d\n"+
				"  2. Set a custom port: export MCP_GATEWAY_OAUTH_PORT=5001\n"+
				"  3. Check what's using the port: lsof -i :%d\n\n"+
				"Error: %w",
			port, port, port, err,
		)
	}

	log.Logf("OAuth callback server bound to localhost:%d", port)

	return &CallbackServer{
		port:     port,
		listener: listener,
		codeCh:   make(chan CallbackData, 1),
		errCh:    make(chan error, 1),
	}, nil
}

// Port returns the port the server is listening on
func (s *CallbackServer) Port() int {
	return s.port
}

// URL returns the full callback URL
func (s *CallbackServer) URL() string {
	return fmt.Sprintf("http://localhost:%d/callback", s.port)
}

// Start starts the HTTP server
// Should be called in a goroutine
func (s *CallbackServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", s.handleCallback)

	s.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Logf("- Callback server listening on http://localhost:%d/callback", s.port)

	if err := s.server.Serve(s.listener); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("callback server error: %w", err)
	}

	return nil
}

// handleCallback processes the OAuth callback request
func (s *CallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	code := query.Get("code")
	state := query.Get("state")

	if code == "" {
		errMsg := "Missing authorization code in callback"
		if errParam := query.Get("error"); errParam != "" {
			errDesc := query.Get("error_description")
			if errDesc != "" {
				errMsg = fmt.Sprintf("OAuth error: %s - %s", errParam, errDesc)
			} else {
				errMsg = fmt.Sprintf("OAuth error: %s", errParam)
			}
		}

		log.Logf("! Callback error: %s", errMsg)
		s.errCh <- fmt.Errorf("%s", errMsg)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	if state == "" {
		errMsg := "Missing state parameter in callback"
		log.Logf("! %s", errMsg)
		s.errCh <- fmt.Errorf("%s", errMsg)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}

	log.Logf("- Received OAuth callback with code and state")

	// Send callback data to waiting channel
	s.codeCh <- CallbackData{
		Code:  code,
		State: state,
	}

	// Return success page to user
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head>
    <title>Authorization Successful</title>
    <style>
        body { font-family: system-ui, -apple-system, sans-serif; text-align: center; padding: 50px; }
        .success { color: #28a745; font-size: 24px; margin-bottom: 20px; }
        .message { color: #6c757d; }
    </style>
</head>
<body>
    <div class="success">✓ Authorization Successful!</div>
    <div class="message">You can close this window and return to the terminal.</div>
</body>
</html>`)
}

// Wait blocks until a callback is received, an error occurs, or the context is cancelled
// Returns the authorization code and state parameter
func (s *CallbackServer) Wait(ctx context.Context) (code string, state string, err error) {
	select {
	case data := <-s.codeCh:
		return data.Code, data.State, nil
	case err := <-s.errCh:
		return "", "", err
	case <-ctx.Done():
		return "", "", fmt.Errorf("callback timeout: %w", ctx.Err())
	}
}

// Shutdown gracefully shuts down the callback server
func (s *CallbackServer) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}
