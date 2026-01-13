package gateway

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestSSEServerAuthentication tests that the SSE server properly enforces authentication
func TestSSEServerAuthentication(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Create a minimal gateway with SSE transport
	g := &Gateway{
		Options: Options{
			Port:      0, // Let the OS assign a port
			Transport: "sse",
		},
	}
	g.health.SetHealthy() // Mark as healthy for testing

	// Initialize an empty MCP server
	g.mcpServer = mcp.NewServer(&mcp.Implementation{Name: "test-auth-gateway", Version: "1.0.0"}, nil)

	// Generate auth token
	token, wasGenerated, err := getOrGenerateAuthToken()
	if err != nil {
		t.Fatalf("failed to generate auth token: %v", err)
	}
	g.authToken = token
	g.authTokenWasGenerated = wasGenerated

	// Start the SSE server in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a listener on a random available port
	lc := &net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- g.startSseServer(ctx, ln)
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Test 1: Health endpoint should be accessible without auth
	t.Run("HealthEndpointNoAuth", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/health", port), nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("health check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200 for /health, got %d", resp.StatusCode)
		}
	})

	// Test 2: SSE endpoint should reject requests without auth
	t.Run("SSEEndpointNoAuth", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/sse", port), nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401 for /sse without auth, got %d", resp.StatusCode)
		}
	})

	// Test 3: SSE endpoint should accept valid bearer auth
	t.Run("SSEEndpointBearerAuth", func(t *testing.T) {
		client := &http.Client{}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/sse", port), nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("expected successful response for /sse with valid bearer auth, got %d: %s", resp.StatusCode, string(body))
		}
	})

	// Test 4: SSE endpoint should reject invalid bearer auth
	t.Run("SSEEndpointInvalidBearerAuth", func(t *testing.T) {
		client := &http.Client{}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/sse", port), nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer wrong-token")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401 for /sse with invalid bearer auth, got %d", resp.StatusCode)
		}
	})

	// Cancel context to stop server
	cancel()

	// Wait a bit for cleanup
	select {
	case <-serverErr:
		// Server stopped
	case <-time.After(1 * time.Second):
		t.Error("server did not stop in time")
	}
}

// TestStreamingServerAuthentication tests that the streaming server properly enforces authentication
func TestStreamingServerAuthentication(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Create a minimal gateway with streaming transport
	g := &Gateway{
		Options: Options{
			Port:      0,
			Transport: "streaming",
		},
	}
	g.health.SetHealthy() // Mark as healthy for testing

	// Initialize an empty MCP server
	g.mcpServer = mcp.NewServer(&mcp.Implementation{Name: "test-auth-gateway", Version: "1.0.0"}, nil)

	// Generate auth token
	token, wasGenerated, err := getOrGenerateAuthToken()
	if err != nil {
		t.Fatalf("failed to generate auth token: %v", err)
	}
	g.authToken = token
	g.authTokenWasGenerated = wasGenerated

	// Start the streaming server in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a listener on a random available port
	lc := &net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- g.startStreamingServer(ctx, ln)
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Test 1: Health endpoint should be accessible without auth
	t.Run("HealthEndpointNoAuth", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/health", port), nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("health check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200 for /health, got %d", resp.StatusCode)
		}
	})

	// Test 2: MCP endpoint should reject requests without auth
	t.Run("MCPEndpointNoAuth", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/mcp", port), nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401 for /mcp without auth, got %d", resp.StatusCode)
		}
	})

	// Test 3: MCP endpoint should accept valid bearer auth
	t.Run("MCPEndpointBearerAuth", func(t *testing.T) {
		client := &http.Client{}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/mcp", port), nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized {
			t.Errorf("expected non-401 status for /mcp with valid bearer auth, got %d", resp.StatusCode)
		}
	})

	// Cancel context to stop server
	cancel()

	// Wait a bit for cleanup
	select {
	case <-serverErr:
		// Server stopped
	case <-time.After(1 * time.Second):
		t.Error("server did not stop in time")
	}
}

// TestAuthTokenFromEnvironment tests that the auth token is read from the environment
func TestAuthTokenFromEnvironment(t *testing.T) {
	expectedToken := "my-custom-token-from-env"
	os.Setenv("MCP_GATEWAY_AUTH_TOKEN", expectedToken)
	defer os.Unsetenv("MCP_GATEWAY_AUTH_TOKEN")

	token, wasGenerated, err := getOrGenerateAuthToken()
	if err != nil {
		t.Fatalf("failed to get auth token: %v", err)
	}

	if token != expectedToken {
		t.Errorf("expected token %q, got %q", expectedToken, token)
	}

	if wasGenerated {
		t.Error("expected wasGenerated to be false when token is from environment")
	}
}

// TestFormatGatewayURLIntegration tests that the gateway URL is formatted correctly
func TestFormatGatewayURLIntegration(t *testing.T) {
	port := 8811
	endpoint := "/sse"

	url := formatGatewayURL(port, endpoint)

	expected := fmt.Sprintf("http://localhost:%d%s", port, endpoint)
	if url != expected {
		t.Errorf("expected URL %q, got %q", expected, url)
	}
}

// TestFormatBearerTokenEncoding tests that bearer token is properly formatted
func TestFormatBearerTokenEncoding(t *testing.T) {
	token := "test-token-abc123"
	authHeader := formatBearerToken(token)

	// Should start with "Authorization: Bearer "
	if !strings.HasPrefix(authHeader, "Authorization: Bearer ") {
		t.Errorf("auth header should start with 'Authorization: Bearer ', got %q", authHeader)
	}

	// Extract the token part
	parts := strings.SplitN(authHeader, " ", 3)
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts in auth header, got %d", len(parts))
	}

	// The third part should be the token
	if parts[2] != token {
		t.Errorf("expected token %q, got %q", token, parts[2])
	}

	// Verify the complete format
	expected := fmt.Sprintf("Authorization: Bearer %s", token)
	if authHeader != expected {
		t.Errorf("expected auth header %q, got %q", expected, authHeader)
	}
}
