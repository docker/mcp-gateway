package mcp

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
)

type remoteMCPClient struct {
	config      catalog.ServerConfig
	client      *mcp.Client
	session     *mcp.ClientSession
	initialized atomic.Bool
}

func NewRemoteMCPClient(config catalog.ServerConfig) Client {
	return &remoteMCPClient{
		config: config,
	}
}

func (c *remoteMCPClient) Initialize(ctx context.Context, _ *mcp.InitializeParams, _ bool, _ *mcp.ServerSession) error {
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

	var mcpTransport mcp.Transport
	var err error

	switch strings.ToLower(transport) {
	case "sse":
		// TODO: Need to implement custom HTTP client with headers for SSE
		mcpTransport = mcp.NewSSEClientTransport(url, &mcp.SSEClientTransportOptions{})
	case "http", "streamable", "streaming", "streamable-http":
		// TODO: Need to implement custom HTTP client with headers for streaming
		mcpTransport = mcp.NewStreamableClientTransport(url, &mcp.StreamableClientTransportOptions{})
	default:
		return fmt.Errorf("unsupported remote transport: %s", transport)
	}

	c.client = mcp.NewClient(&mcp.Implementation{
		Name:    "docker-mcp-gateway",
		Version: "1.0.0",
	}, nil)

	session, err := c.client.Connect(ctx, mcpTransport)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.session = session
	c.initialized.Store(true)

	return nil
}

func (c *remoteMCPClient) Session() *mcp.ClientSession { return c.session }

func expandEnv(value string, secrets map[string]string) string {
	return os.Expand(value, func(name string) string {
		return secrets[name]
	})
}
