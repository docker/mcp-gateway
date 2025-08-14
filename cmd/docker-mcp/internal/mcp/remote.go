package mcp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
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

	// Debug logging
	log.Printf("Remote MCP Client Debug:")
	log.Printf("  URL: %s", url)
	log.Printf("  Transport: %s", transport)
	log.Printf("  Headers:")
	for k, v := range headers {
		log.Printf("    %s: %s", k, v)
	}

	// Create HTTP client with custom headers
	httpClient := &http.Client{
		Transport: &headerRoundTripper{
			base:    http.DefaultTransport,
			headers: headers,
		},
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

// headerRoundTripper adds custom headers to all HTTP requests
type headerRoundTripper struct {
	base    http.RoundTripper
	headers map[string]string
}

func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original
	newReq := req.Clone(req.Context())

	// Add custom headers
	for k, v := range h.headers {
		if strings.ToLower(k) == "host" {
			// Host header requires special handling
			newReq.Host = v
		} else {
			newReq.Header.Set(k, v)
		}
	}

	// Debug logging for request
	log.Printf("HTTP Request Debug:")
	log.Printf("  Method: %s", newReq.Method)
	log.Printf("  URL: %s", newReq.URL.String())
	log.Printf("  Request Headers:")
	for k, v := range newReq.Header {
		log.Printf("    %s: %v", k, v)
	}

	// Make the request
	resp, err := h.base.RoundTrip(newReq)
	if err != nil {
		log.Printf("HTTP Request Error: %v", err)
		return resp, err
	}

	// Debug logging for response
	log.Printf("HTTP Response Debug:")
	log.Printf("  Status: %s", resp.Status)
	log.Printf("  Response Headers:")
	for k, v := range resp.Header {
		log.Printf("    %s: %v", k, v)
	}

	return resp, err
}

func expandEnv(value string, secrets map[string]string) string {
	return os.Expand(value, func(name string) string {
		return secrets[name]
	})
}
