package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegrationRemoteWithCustomHeaders tests that custom headers (including Authorization
// with Bearer tokens from secrets) are properly transmitted to remote MCP servers.
func TestIntegrationRemoteWithCustomHeaders(t *testing.T) {
	thisIsAnIntegrationTest(t)
	// Start a test MCP server that can validate the Authorization header
	server := newTestMCPServer(t)
	defer server.close()
	// Create temporary directory for test files
	tmp := t.TempDir()

	// Create a secrets file with our test token
	testToken := "test-bearer-token-12345"
	writeFile(t, tmp, ".env", fmt.Sprintf("AUTH_TOKEN=%s", testToken))

	// Create a custom catalog with a remote server that uses Authorization header
	catalogContent := fmt.Sprintf(`
name: test-catalog
registry:
  test-server:
    remote:
      url: %s
      transport_type: streamable
      headers:
        Authorization: "Bearer $AUTH_TOKEN"
    secrets:
      - name: AUTH_TOKEN
        env: AUTH_TOKEN
`, server.url)

	writeFile(t, tmp, "catalog.yaml", catalogContent)

	// Optional: uncomment for debugging
	// fmt.Printf("DEBUG: Catalog content:\n%s\n", catalogContent)
	// fmt.Printf("DEBUG: Server URL: %s\n", server.url)

	// Run the gateway with our custom catalog and call a tool
	gatewayArgs := []string{
		"--servers=test-server",
		"--secrets=" + filepath.Join(tmp, ".env"),
		"--catalog=" + filepath.Join(tmp, "catalog.yaml"),
	}

	out := runDockerMCP(t, "tools", "call", "--gateway-arg="+strings.Join(gatewayArgs, ","), "get_auth_header")

	// Verify that the server received our Authorization header
	assert.Contains(t, out, fmt.Sprintf("Bearer %s", testToken), "Server should receive the Authorization header with Bearer token")
}

// testMCPServer is a minimal MCP server implementation using go-sdk that can validate received headers
type testMCPServer struct {
	server       *http.Server
	listener     net.Listener
	url          string
	receivedAuth string
	mu           sync.RWMutex
	mcpServer    *mcp.Server
}

func newTestMCPServer(t *testing.T) *testMCPServer {
	t.Helper()

	s := &testMCPServer{}

	// Create a listener to get a random port
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err, "Failed to create listener")

	s.listener = listener
	s.url = fmt.Sprintf("http://localhost:%d", listener.Addr().(*net.TCPAddr).Port)

	// Create MCP server using go-sdk
	s.mcpServer = mcp.NewServer(&mcp.Implementation{
		Name:    "test-mcp-server",
		Version: "1.0.0",
	}, nil)

	// Add our test tool that returns the Authorization header
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_auth_header",
		Description: "Returns the Authorization header received by the server",
		InputSchema: &jsonschema.Schema{
			Type:       "object",
			Properties: map[string]*jsonschema.Schema{},
		},
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		// Get the Authorization header from the server's stored value
		s.mu.RLock()
		authHeader := s.receivedAuth
		s.mu.RUnlock()

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: authHeader},
			},
		}, nil, nil
	})

	// Create HTTP handler using StreamableHTTPHandler from go-sdk
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		// Store the Authorization header for our test
		s.mu.Lock()
		s.receivedAuth = r.Header.Get("Authorization")
		s.mu.Unlock()

		return s.mcpServer
	}, nil)

	s.server = &http.Server{
		Handler: handler,
	}

	// Start server in background
	go func() {
		if err := s.server.Serve(listener); err != http.ErrServerClosed {
			t.Errorf("Server failed: %v", err)
		}
	}()

	// Wait a moment for server to start
	time.Sleep(100 * time.Millisecond)

	return s
}

func (s *testMCPServer) close() {
	if s.server != nil {
		_ = s.server.Shutdown(context.Background())
	}
	if s.listener != nil {
		s.listener.Close()
	}
}
