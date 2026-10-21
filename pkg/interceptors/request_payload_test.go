package interceptors

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBeforeInterceptorStreamableHTTP(t *testing.T) {
	intercepted := make(chan []byte, 1)
	endpoint := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		intercepted <- body
	}))
	defer endpoint.Close()

	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	i := Interceptor{When: "before", Type: "http", Argument: endpoint.URL}
	server.AddReceivingMiddleware(i.ToMiddleware())
	mcp.AddTool(server, &mcp.Tool{Name: "echo"}, func(_ context.Context, req *mcp.CallToolRequest, args struct {
		Text string `json:"text"`
	},
	) (*mcp.CallToolResult, any, error) {
		assert.NotNil(t, req.Extra)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: args.Text}}}, nil, nil
	})
	transport := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, nil))
	defer transport.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: transport.URL}, nil)
	require.NoError(t, err)
	defer func() { assert.NoError(t, session.Close()) }()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "echo", Arguments: map[string]any{"text": "hello"}})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Len(t, result.Content, 1)
	assert.Equal(t, "hello", result.Content[0].(*mcp.TextContent).Text)
	select {
	case body := <-intercepted:
		assert.JSONEq(t, `{"method":"tools/call","params":{"name":"echo","arguments":{"text":"hello"}}}`, string(body))
	default:
		t.Fatal("tool call did not reach the interceptor")
	}
}

func TestBeforeInterceptorRequestPayload(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra *mcp.RequestExtra
	}{
		{name: "stdio"},
		{name: "empty extra", extra: &mcp.RequestExtra{}},
		{name: "streaming", extra: &mcp.RequestExtra{
			Header: http.Header{"Authorization": {"Bearer transport-secret"}},
			CloseSSEStream: func(mcp.CloseSSEStreamArgs) {
				t.Error("serialization must not call the transport callback")
			},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, override := range []bool{false, true} {
				t.Run(map[bool]string{false: "passthrough", true: "override"}[override], func(t *testing.T) {
					bodies := make(chan []byte, 1)
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						body, err := io.ReadAll(r.Body)
						assert.NoError(t, err)
						bodies <- body
						if override {
							_, err = io.WriteString(w, `{"content":[{"type":"text","text":"intercepted"}]}`)
							assert.NoError(t, err)
						}
					}))
					defer server.Close()

					req := &mcp.CallToolRequest{
						Params: &mcp.CallToolParamsRaw{
							Name:      "search",
							Arguments: json.RawMessage(`{"query":"hello","options":{"limit":3},"enabled":false}`),
							Meta:      mcp.Meta{"progressToken": "progress-1"},
						},
						Extra: tc.extra,
					}
					called := false
					want := &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "original"}}}
					next := func(_ context.Context, method string, got mcp.Request) (mcp.Result, error) {
						called = true
						assert.Equal(t, "tools/call", method)
						assert.Same(t, req, got)
						return want, nil
					}
					interceptor := Interceptor{When: "before", Type: "http", Argument: server.URL}
					result, err := interceptor.ToMiddleware()(next)(context.Background(), "tools/call", req)
					require.NoError(t, err)
					select {
					case body := <-bodies:
						assert.JSONEq(t, `{"method":"tools/call","params":{"name":"search","arguments":{"query":"hello","options":{"limit":3},"enabled":false},"_meta":{"progressToken":"progress-1"}}}`, string(body))
					default:
						t.Fatal("interceptor was not invoked")
					}
					assert.Equal(t, !override, called)
					if override {
						assert.Equal(t, "intercepted", result.(*mcp.CallToolResult).Content[0].(*mcp.TextContent).Text)
					} else {
						assert.Same(t, want, result)
					}
				})
			}
		})
	}
}
