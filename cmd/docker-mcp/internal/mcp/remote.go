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

func (c *remoteMCPClient) Initialize(ctx context.Context, _ *mcp.InitializeParams, _ bool, _ *mcp.ServerSession, _ *mcp.Server) error {
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

	// Add OAuth token if configured
	if c.config.Spec.OAuth != nil && c.config.Spec.OAuth.Enabled {
		token, err := c.getOAuthToken(ctx)
		if err != nil {
			return fmt.Errorf("failed to get OAuth token: %w", err)
		}
		if token != "" {
			headers["Authorization"] = "Bearer " + token
		}
	}

	// Create HTTP client with headers if needed
	var httpClient *http.Client
	if len(headers) > 0 {
		httpClient = &http.Client{
			Transport: &headerTransport{
				headers: headers,
				base:    http.DefaultTransport,
			},
		}
	}

	var mcpTransport mcp.Transport
	var err error

	switch strings.ToLower(transport) {
	case "sse":
		mcpTransport = mcp.NewSSEClientTransport(url, &mcp.SSEClientTransportOptions{
			HTTPClient: httpClient,
		})
	case "http", "streamable", "streaming", "streamable-http":
		mcpTransport = mcp.NewStreamableClientTransport(url, &mcp.StreamableClientTransportOptions{
			HTTPClient: httpClient,
		})
	default:
		return fmt.Errorf("unsupported remote transport: %s", transport)
	}

	c.client = mcp.NewClient(&mcp.Implementation{
		Name:    "docker-mcp-gateway",
		Version: "1.0.0",
	}, nil)

	c.client.AddRoots(c.roots...)

	session, err := c.client.Connect(ctx, mcpTransport)
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
	if c.config.Spec.OAuth == nil || c.config.Spec.OAuth.Provider == "" {
		return "", nil
	}

	// Get the OAuth token from pinata
	client := desktop.NewAuthClient()
	app, err := client.GetOAuthApp(ctx, c.config.Spec.OAuth.Provider)
	if err != nil || !app.Authorized {
		// Token might not exist if user hasn't authorized yet
		return "", nil
	}

	return app.AccessToken, nil
}

// headerTransport is a http.RoundTripper that adds headers to all requests
type headerTransport struct {
	headers map[string]string
	base    http.RoundTripper
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original
	req = req.Clone(req.Context())
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	return t.base.RoundTrip(req)
}
