package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// SidecarTransport implements PluginTransport for sidecar-based plugins.
// It connects to a plugin running as a sidecar container via HTTP.
// This is the preferred transport for Kubernetes deployments where plugins
// run as sidecar containers in the same pod.
type SidecarTransport struct {
	config SidecarConfig
	hooks  *PluginLifecycleHooks
}

// NewSidecarTransport creates a new sidecar transport with the given configuration.
func NewSidecarTransport(config SidecarConfig, hooks *PluginLifecycleHooks) *SidecarTransport {
	return &SidecarTransport{
		config: config,
		hooks:  hooks,
	}
}

// Connect establishes a connection to the sidecar plugin.
func (t *SidecarTransport) Connect(ctx context.Context) (PluginClient, error) {
	client := &SidecarClient{
		config: t.config,
		hooks:  t.hooks,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Wait for the sidecar to be ready
	if err := client.waitForReady(ctx); err != nil {
		return nil, fmt.Errorf("sidecar not ready: %w", err)
	}

	return client, nil
}

// SidecarClient implements PluginClient for sidecar-based plugins.
type SidecarClient struct {
	config     SidecarConfig
	hooks      *PluginLifecycleHooks
	httpClient *http.Client
	mu         sync.RWMutex
	closed     bool
}

// waitForReady polls the sidecar's health endpoint until it's ready.
func (c *SidecarClient) waitForReady(ctx context.Context) error {
	deadline := time.Now().Add(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				if lastErr != nil {
					return fmt.Errorf("timeout waiting for sidecar to be ready: %w", lastErr)
				}
				return fmt.Errorf("timeout waiting for sidecar to be ready")
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.URL+"/health", nil)
			if err != nil {
				lastErr = err
				continue
			}

			// Add configured headers
			for k, v := range c.config.Headers {
				req.Header.Set(k, v)
			}

			resp, err := c.httpClient.Do(req)
			if err != nil {
				lastErr = err
				continue
			}
			_ = resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("health check returned status %d", resp.StatusCode)
		}
	}
}

// Call invokes a method on the sidecar plugin via HTTP POST.
func (c *SidecarClient) Call(ctx context.Context, method string, params any) ([]byte, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("plugin client is closed")
	}
	c.mu.RUnlock()

	// Serialize params to JSON
	body, err := json.Marshal(map[string]any{
		"method": method,
		"params": params,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.URL+"/call", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Add configured headers
	for k, v := range c.config.Headers {
		req.Header.Set(k, v)
	}

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call plugin: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plugin returned error: %s (status %d)", string(respBody), resp.StatusCode)
	}

	return respBody, nil
}

// Close marks the client as closed.
// For sidecar plugins, we don't actually stop the sidecar since it's managed by Kubernetes.
func (c *SidecarClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

// Reconnect attempts to reconnect to the sidecar after a connection failure.
// This is useful for handling transient network issues or sidecar restarts.
func (c *SidecarClient) Reconnect(ctx context.Context) error {
	c.mu.Lock()
	c.closed = false
	c.mu.Unlock()

	return c.waitForReady(ctx)
}

// Verify SidecarTransport implements PluginTransport
var _ PluginTransport = (*SidecarTransport)(nil)

// Verify SidecarClient implements PluginClient
var _ PluginClient = (*SidecarClient)(nil)
