// Package mcp provides MCP-based plugin implementations.
//
// MCP plugins communicate via JSON-RPC over HTTP to MCP servers.
// They support two deployment modes:
//   - Local MCP: Containerized servers managed by gateway (Desktop) or sidecars (K8s)
//   - Remote MCP: External HTTP endpoints
//
// The MCP provider uses a tools-only subset of the MCP protocol:
//   - tools/list: Returns available plugin operations
//   - tools/call: Invokes plugin operations
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/docker/mcp-gateway/pkg/plugins"
)

// Provider implements plugins.PluginProvider for MCP-based plugins.
type Provider struct {
	containerMgr plugins.ContainerManager
	httpClient   *http.Client
	mu           sync.RWMutex
	servers      map[string]*mcpServer // endpoint -> server
}

// mcpServer represents a connected MCP server.
type mcpServer struct {
	endpoint string
	tools    []string
}

// NewProvider creates a new MCP provider.
// containerMgr is used to start local containers (Desktop mode).
// For Kubernetes, pass a noop container manager since sidecars are pre-started.
func NewProvider(containerMgr plugins.ContainerManager) *Provider {
	return &Provider{
		containerMgr: containerMgr,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		servers: make(map[string]*mcpServer),
	}
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "mcp"
}

// resolveServer resolves server configuration to an endpoint.
func (p *Provider) resolveServer(ctx context.Context, config plugins.PluginConfig) (string, error) {
	if config.Server == nil {
		return "", fmt.Errorf("server configuration is required for MCP provider")
	}

	// Check if it's a string (catalog reference or direct endpoint)
	if serverStr, ok := config.Server.(string); ok {
		return p.resolveServerString(ctx, serverStr)
	}

	// Check if it's a map (inline server config)
	if serverMap, ok := config.Server.(map[string]any); ok {
		return p.resolveServerConfig(ctx, serverMap)
	}

	return "", fmt.Errorf("invalid server configuration type: %T", config.Server)
}

// resolveServerString resolves a server string (catalog:// or direct endpoint).
func (p *Provider) resolveServerString(ctx context.Context, serverStr string) (string, error) {
	// Catalog reference: catalog://docker.io/docker/mcp-plugins:v1/auth-k8s-secret
	if strings.HasPrefix(serverStr, "catalog://") {
		// TODO: Implement catalog resolution
		// For now, return error indicating catalog support is not yet implemented
		return "", fmt.Errorf("catalog references not yet implemented: %s", serverStr)
	}

	// Direct HTTP endpoint
	if strings.HasPrefix(serverStr, "http://") || strings.HasPrefix(serverStr, "https://") {
		return serverStr, nil
	}

	return "", fmt.Errorf("invalid server string: %s (expected catalog:// or http(s)://)", serverStr)
}

// resolveServerConfig resolves an inline server configuration.
func (p *Provider) resolveServerConfig(ctx context.Context, serverMap map[string]any) (string, error) {
	serverType, _ := serverMap["type"].(string)

	switch serverType {
	case "image":
		// Container image - use container manager to start it
		image, _ := serverMap["image"].(string)
		if image == "" {
			return "", fmt.Errorf("image is required for type: image")
		}

		port := 8080
		if portVal, ok := serverMap["port"].(float64); ok {
			port = int(portVal)
		}

		var env map[string]string
		if envMap, ok := serverMap["env"].(map[string]any); ok {
			env = make(map[string]string)
			for k, v := range envMap {
				if vs, ok := v.(string); ok {
					env[k] = vs
				}
			}
		}

		if p.containerMgr == nil {
			return "", fmt.Errorf("container manager not configured for image-based plugins")
		}

		endpoint, err := p.containerMgr.EnsureRunning(ctx, plugins.ServerConfig{
			Type:  "image",
			Image: image,
			Port:  port,
			Env:   env,
		})
		if err != nil {
			return "", fmt.Errorf("failed to start container: %w", err)
		}
		return endpoint, nil

	case "remote":
		// Remote HTTP endpoint
		endpoint, _ := serverMap["endpoint"].(string)
		if endpoint == "" {
			return "", fmt.Errorf("endpoint is required for type: remote")
		}
		return endpoint, nil

	case "registry":
		// MCP Registry reference
		source, _ := serverMap["source"].(string)
		if source == "" {
			return "", fmt.Errorf("source is required for type: registry")
		}
		// TODO: Implement registry resolution
		return "", fmt.Errorf("registry references not yet implemented: %s", source)

	default:
		return "", fmt.Errorf("unknown server type: %s", serverType)
	}
}

// callTool calls an MCP tool on the server.
func (p *Provider) callTool(ctx context.Context, endpoint, tool string, params any) (json.RawMessage, error) {
	// Build JSON-RPC request
	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      tool,
			"arguments": params,
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call MCP server: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MCP server returned error: %s (status %d)", string(respBody), resp.StatusCode)
	}

	// Parse JSON-RPC response
	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("MCP tool error: %s (code %d)", rpcResp.Error.Message, rpcResp.Error.Code)
	}

	return rpcResp.Result, nil
}

// CreateAuthProvider creates an AuthProvider that calls an MCP server.
func (p *Provider) CreateAuthProvider(ctx context.Context, config plugins.PluginConfig) (plugins.AuthProvider, error) {
	endpoint, err := p.resolveServer(ctx, config)
	if err != nil {
		return nil, err
	}
	return &mcpAuthProvider{provider: p, endpoint: endpoint}, nil
}

// CreateCredentialStorage creates a CredentialStorage that calls an MCP server.
func (p *Provider) CreateCredentialStorage(ctx context.Context, config plugins.PluginConfig) (plugins.CredentialStorage, error) {
	endpoint, err := p.resolveServer(ctx, config)
	if err != nil {
		return nil, err
	}
	return &mcpCredentialStorage{provider: p, endpoint: endpoint}, nil
}

// CreateAuthProxy creates an AuthProxy that calls an MCP server.
func (p *Provider) CreateAuthProxy(ctx context.Context, config plugins.PluginConfig) (plugins.AuthProxy, error) {
	endpoint, err := p.resolveServer(ctx, config)
	if err != nil {
		return nil, err
	}
	return &mcpAuthProxy{provider: p, endpoint: endpoint}, nil
}

// CreateAuditSink creates an AuditSink that calls an MCP server.
func (p *Provider) CreateAuditSink(ctx context.Context, config plugins.PluginConfig) (plugins.AuditSink, error) {
	endpoint, err := p.resolveServer(ctx, config)
	if err != nil {
		return nil, err
	}
	return &mcpAuditSink{provider: p, endpoint: endpoint}, nil
}

// CreatePolicyEvaluator creates a PolicyEvaluator that calls an MCP server.
func (p *Provider) CreatePolicyEvaluator(ctx context.Context, config plugins.PluginConfig) (plugins.PolicyEvaluator, error) {
	endpoint, err := p.resolveServer(ctx, config)
	if err != nil {
		return nil, err
	}
	return &mcpPolicyEvaluator{provider: p, endpoint: endpoint}, nil
}

// CreateMCPProvisioner creates an MCPProvisioner that calls an MCP server.
func (p *Provider) CreateMCPProvisioner(ctx context.Context, config plugins.PluginConfig) (plugins.MCPProvisioner, error) {
	endpoint, err := p.resolveServer(ctx, config)
	if err != nil {
		return nil, err
	}
	return &mcpProvisioner{provider: p, endpoint: endpoint}, nil
}

// CreateTelemetryPlugin creates a TelemetryPlugin that calls an MCP server.
func (p *Provider) CreateTelemetryPlugin(ctx context.Context, config plugins.PluginConfig) (plugins.TelemetryPlugin, error) {
	endpoint, err := p.resolveServer(ctx, config)
	if err != nil {
		return nil, err
	}
	return &mcpTelemetry{provider: p, endpoint: endpoint}, nil
}

// Verify Provider implements plugins.PluginProvider
var _ plugins.PluginProvider = (*Provider)(nil)
