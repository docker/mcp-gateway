package mcp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
)

type remoteMCPClient struct {
	config      *catalog.ServerConfig
	client      *mcp.Client
	session     *mcp.ClientSession
	roots       []*mcp.Root
	initialized atomic.Bool
}

func NewRemoteMCPClient(config *catalog.ServerConfig) Client {
	return &remoteMCPClient{
		config: config,
	}
}

func (c *remoteMCPClient) Initialize(ctx context.Context, _ *mcp.InitializeParams, _ bool, _ *mcp.ServerSession, _ *mcp.Server, _ CapabilityRefresher) error {
	if c.initialized.Load() {
		return fmt.Errorf("client already initialized")
	}

	// Read configuration.
	var (
		url       string
		transport string
	)
	if c.config.Spec.SSEEndpoint != "" {
		// Deprecated
		url = c.config.Spec.SSEEndpoint
		transport = "sse"
	} else {
		url = c.config.Spec.Remote.URL
		transport = c.config.Spec.Remote.Transport
	}

	// Secrets to env
	env := map[string]string{}
	for _, secret := range c.config.Spec.Secrets {
		env[secret.Env] = c.config.Secrets[secret.Name]
	}

	// Headers
	headers := map[string]string{}
	for k, v := range c.config.Spec.Remote.Headers {
		headers[k] = expandEnv(v, env)
	}

	// Add OAuth token if remote server has OAuth configuration
	if c.config.Spec.OAuth != nil && len(c.config.Spec.OAuth.Providers) > 0 {
		token, err := c.getOAuthToken(ctx)
		if err != nil {
			return fmt.Errorf("failed to get OAuth token: %w", err)
		}
		if token != "" {
			headers["Authorization"] = "Bearer " + token
		}
	}

	var mcpTransport mcp.Transport
	var err error

	// Create HTTP client with custom headers
	httpClient := &http.Client{
		Transport: &headerRoundTripper{
			base:    http.DefaultTransport,
			headers: headers,
		},
	}

	switch strings.ToLower(transport) {
	case "sse":
		mcpTransport = &mcp.SSEClientTransport{
			Endpoint:   url,
			HTTPClient: httpClient,
		}
	case "http", "streamable", "streaming", "streamable-http":
		mcpTransport = &mcp.StreamableClientTransport{
			Endpoint:   url,
			HTTPClient: httpClient,
		}
	default:
		return fmt.Errorf("unsupported remote transport: %s", transport)
	}

	c.client = mcp.NewClient(&mcp.Implementation{
		Name:    "docker-mcp-gateway",
		Version: "1.0.0",
	}, nil)

	c.client.AddRoots(c.roots...)

	session, err := c.client.Connect(ctx, mcpTransport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.session = session
	c.initialized.Store(true)

	return nil
}

func (c *remoteMCPClient) Session() *mcp.ClientSession { return c.session }
func (c *remoteMCPClient) GetClient() *mcp.Client      { return c.client }

func (c *remoteMCPClient) AddRoots(roots []*mcp.Root) {
	if c.initialized.Load() {
		c.client.AddRoots(roots...)
	}
	c.roots = roots
}

func expandEnv(value string, secrets map[string]string) string {
	return os.Expand(value, func(name string) string {
		return secrets[name]
	})
}

func (c *remoteMCPClient) getOAuthToken(ctx context.Context) (string, error) {
	if c.config.Spec.OAuth == nil || len(c.config.Spec.OAuth.Providers) == 0 {
		return "", nil
	}

	// Get the OAuth token from pinata using server name, not provider name
	// OAuth tokens are stored by server name (e.g., "notion-remote"), not provider name (e.g., "notion")
	client := desktop.NewAuthClient()
	app, err := client.GetOAuthApp(ctx, c.config.Name)
	if err != nil || !app.Authorized {
		// Token might not exist if user hasn't authorized yet
		return "", nil
	}

	return app.AccessToken, nil
}

// headerRoundTripper is an http.RoundTripper that adds custom headers to all requests
type headerRoundTripper struct {
	base    http.RoundTripper
	headers map[string]string
}

func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original
	newReq := req.Clone(req.Context())
	
	// Add custom headers
	for key, value := range h.headers {
		// Don't override Accept header if already set by streamable transport
		if key == "Accept" && newReq.Header.Get("Accept") != "" {
			continue
		}
		newReq.Header.Set(key, value)
	}
	
	// TODO: This Accept header logic is a Notion-specific workaround, not part of MCP spec.
	// Consider moving to server-specific configuration or making it configurable per catalog entry.
	// Ensure all requests have proper Accept headers
	// Notion MCP server requires Accept headers to avoid HTTP 405/406 errors
	if newReq.Header.Get("Accept") == "" {
		if newReq.Method == "GET" {
			newReq.Header.Set("Accept", "text/event-stream")
		} else if newReq.Method == "POST" && newReq.Header.Get("Content-Type") != "" {
			// Only add Accept header to POST requests that have a Content-Type
			// These are likely MCP JSON-RPC calls that need JSON responses
			newReq.Header.Set("Accept", "application/json")
		}
		// Skip adding Accept headers to POST requests without Content-Type
		// as these might be session management requests
	}
	
	return h.base.RoundTrip(newReq)
}
