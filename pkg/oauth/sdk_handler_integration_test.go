package oauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSDKHandler_Integration_TokenInjection verifies that an SDKHandler
// backed by a fake credential store correctly injects a Bearer token into
// requests made by the SDK's StreamableClientTransport.
//
// Flow:
//  1. Start an in-process MCP server that records the Authorization header.
//  2. Populate a fake credential store with a DCR client and token.
//  3. Create a StreamableClientTransport with OAuthHandler = our SDKHandler.
//  4. Connect and call a tool.
//  5. Assert the server received "Bearer <token>".
func TestSDKHandler_Integration_TokenInjection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	const serverName = "integ-test-server"
	const accessToken = "integ-test-access-token-42"

	// -- fake credential store --
	fake := newFakeCredentialHelper()
	dcrJSON, _ := json.Marshal(map[string]string{
		"serverName":            serverName,
		"providerName":          serverName,
		"clientId":              "c-123",
		"authorizationEndpoint": "https://auth.example.com/authorize",
		"tokenEndpoint":         "https://auth.example.com/token",
	})
	_ = fake.Add(&credentials.Credentials{
		ServerURL: fmt.Sprintf("https://%s.mcp-dcr", serverName),
		Username:  "dcr_client",
		Secret:    base64.StdEncoding.EncodeToString(dcrJSON),
	})
	tokJSON, _ := json.Marshal(map[string]string{
		"access_token": accessToken,
		"token_type":   "Bearer",
	})
	_ = fake.Add(&credentials.Credentials{
		ServerURL: fmt.Sprintf("https://auth.example.com/authorize/%s", serverName),
		Username:  fmt.Sprintf("oauth2_%s", serverName),
		Secret:    base64.StdEncoding.EncodeToString(tokJSON),
	})

	handler := &SDKHandler{
		serverName: serverName,
		mode:       ModeCE,
		credHelper: &CredentialHelper{
			credentialHelper: fake,
			mode:             ModeCE,
		},
	}

	// -- in-process MCP server --
	var receivedAuth atomic.Value
	receivedAuth.Store("")

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "integ-test-mcp",
		Version: "1.0.0",
	}, nil)

	mcp.AddTool(mcpServer, &mcp.Tool{
		Name:        "echo_auth",
		Description: "Returns the Authorization header the server received",
		InputSchema: &jsonschema.Schema{
			Type:       "object",
			Properties: map[string]*jsonschema.Schema{},
		},
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: receivedAuth.Load().(string)},
			},
		}, nil, nil
	})

	httpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		receivedAuth.Store(r.Header.Get("Authorization"))
		return mcpServer
	}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	lc := &net.ListenConfig{}
	listener, err := lc.Listen(ctx, "tcp", ":0")
	require.NoError(t, err)
	defer listener.Close()

	srv := &http.Server{Handler: httpHandler}
	go func() { _ = srv.Serve(listener) }()
	defer func() { _ = srv.Shutdown(context.Background()) }()
	time.Sleep(50 * time.Millisecond)

	serverURL := fmt.Sprintf("http://localhost:%d", listener.Addr().(*net.TCPAddr).Port)

	// -- SDK client with OAuthHandler --
	transport := &mcp.StreamableClientTransport{
		Endpoint:     serverURL,
		OAuthHandler: handler,
	}

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "integ-test-client",
		Version: "1.0.0",
	}, nil)

	session, err := client.Connect(ctx, transport, nil)
	require.NoError(t, err)
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "echo_auth",
	})
	require.NoError(t, err)
	require.Len(t, result.Content, 1)

	text, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected TextContent")
	assert.Equal(t, "Bearer "+accessToken, text.Text,
		"OAuthHandler should inject the correct Bearer token")
}

// TestSDKHandler_Integration_401Retry verifies that the SDK's
// StreamableClientTransport calls Authorize on 401 and that our SDKHandler
// returns an informative error (the gateway cannot interactively auth).
//
// Flow:
//  1. Start an HTTP server that returns 401 on the first POST (the
//     initialize request) and proxies subsequent requests to a real MCP server.
//  2. Create a StreamableClientTransport with OAuthHandler = our SDKHandler.
//  3. Attempt to connect. The SDK should call Authorize after the 401.
//  4. Since our Authorize returns an error, the connection should fail
//     with the expected error message.
func TestSDKHandler_Integration_401Retry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	const serverName = "auth-required-server"

	// SDKHandler with empty credential store (no tokens available).
	handler := &SDKHandler{
		serverName: serverName,
		mode:       ModeCE,
		credHelper: &CredentialHelper{
			credentialHelper: newFakeCredentialHelper(),
			mode:             ModeCE,
		},
	}

	// Real MCP server behind the 401 gate.
	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "gated-mcp",
		Version: "1.0.0",
	}, nil)

	httpHandler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return mcpServer
	}, nil)

	// Wrapper that returns 401 on the first request.
	var firstRequest atomic.Bool
	firstRequest.Store(true)
	var mu sync.Mutex

	gatedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		isFirst := firstRequest.Load()
		if isFirst {
			firstRequest.Store(false)
		}
		mu.Unlock()

		if isFirst {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		httpHandler.ServeHTTP(w, r)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	lc := &net.ListenConfig{}
	listener, err := lc.Listen(ctx, "tcp", ":0")
	require.NoError(t, err)
	defer listener.Close()

	srv := &http.Server{Handler: gatedHandler}
	go func() { _ = srv.Serve(listener) }()
	defer func() { _ = srv.Shutdown(context.Background()) }()
	time.Sleep(50 * time.Millisecond)

	serverURL := fmt.Sprintf("http://localhost:%d", listener.Addr().(*net.TCPAddr).Port)

	transport := &mcp.StreamableClientTransport{
		Endpoint:     serverURL,
		OAuthHandler: handler,
	}

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "auth-test-client",
		Version: "1.0.0",
	}, nil)

	_, err = client.Connect(ctx, transport, nil)
	require.Error(t, err, "Connect should fail because Authorize returns an error")
	assert.Contains(t, err.Error(), "OAuth authorization required for "+serverName,
		"Error should contain the server name and instruction to authenticate")
	assert.Contains(t, err.Error(), "docker mcp oauth authorize",
		"Error should instruct the user to run the authorize command")
}
