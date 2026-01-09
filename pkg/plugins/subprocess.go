package plugins

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SubprocessTransport implements PluginTransport for subprocess-based plugins.
// It spawns a plugin as a child process and communicates via HTTP.
type SubprocessTransport struct {
	config SubprocessConfig
	hooks  *PluginLifecycleHooks
}

// NewSubprocessTransport creates a new subprocess transport with the given configuration.
func NewSubprocessTransport(config SubprocessConfig, hooks *PluginLifecycleHooks) *SubprocessTransport {
	return &SubprocessTransport{
		config: config,
		hooks:  hooks,
	}
}

// Connect starts the subprocess and establishes a connection.
func (t *SubprocessTransport) Connect(ctx context.Context) (PluginClient, error) {
	client := &SubprocessClient{
		config: t.config,
		hooks:  t.hooks,
	}

	if err := client.start(ctx); err != nil {
		return nil, err
	}

	return client, nil
}

// SubprocessClient implements PluginClient for subprocess-based plugins.
type SubprocessClient struct {
	config     SubprocessConfig
	hooks      *PluginLifecycleHooks
	cmd        *exec.Cmd
	httpClient *http.Client
	endpoint   string
	mu         sync.RWMutex
	closed     bool
}

// start spawns the subprocess and waits for it to be ready.
func (c *SubprocessClient) start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Build command
	c.cmd = exec.CommandContext(ctx, c.config.Exec, c.config.Args...)

	// Set working directory if specified
	if c.config.WorkDir != "" {
		c.cmd.Dir = c.config.WorkDir
	}

	// Set environment variables
	c.cmd.Env = os.Environ()
	for k, v := range c.config.Env {
		c.cmd.Env = append(c.cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Capture stdout to read the port
	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Capture stderr for logging
	stderr, err := c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start plugin process: %w", err)
	}

	// Read the port from stdout (first line should be: PORT=<port>)
	port, err := c.readPort(ctx, stdout)
	if err != nil {
		_ = c.cmd.Process.Kill()
		return fmt.Errorf("failed to read plugin port: %w", err)
	}

	// Start goroutine to drain stderr
	go c.drainStderr(stderr)

	// Configure HTTP client
	c.endpoint = fmt.Sprintf("http://127.0.0.1:%d", port)
	c.httpClient = &http.Client{
		Timeout: 30 * time.Second,
	}

	// Wait for the plugin to be ready
	if err := c.waitForReady(ctx); err != nil {
		_ = c.cmd.Process.Kill()
		return fmt.Errorf("plugin failed to become ready: %w", err)
	}

	return nil
}

// readPort reads the port number from the plugin's stdout.
// The plugin should output "PORT=<port>" as its first line.
func (c *SubprocessClient) readPort(ctx context.Context, stdout io.Reader) (int, error) {
	scanner := bufio.NewScanner(stdout)

	// Create a channel to receive the port
	portCh := make(chan int, 1)
	errCh := make(chan error, 1)

	go func() {
		if scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "PORT=") {
				portStr := strings.TrimPrefix(line, "PORT=")
				port, err := strconv.Atoi(portStr)
				if err != nil {
					errCh <- fmt.Errorf("invalid port number: %s", portStr)
					return
				}
				portCh <- port
				return
			}
			errCh <- fmt.Errorf("expected PORT=<port>, got: %s", line)
			return
		}
		if err := scanner.Err(); err != nil {
			errCh <- fmt.Errorf("error reading stdout: %w", err)
			return
		}
		errCh <- fmt.Errorf("plugin closed stdout without reporting port")
	}()

	select {
	case port := <-portCh:
		return port, nil
	case err := <-errCh:
		return 0, err
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-time.After(10 * time.Second):
		return 0, fmt.Errorf("timeout waiting for plugin port")
	}
}

// drainStderr reads and logs stderr output from the plugin.
func (c *SubprocessClient) drainStderr(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		// Log stderr output (could be enhanced with proper logging)
		fmt.Fprintf(os.Stderr, "[plugin] %s\n", scanner.Text())
	}
}

// waitForReady polls the plugin's health endpoint until it's ready.
func (c *SubprocessClient) waitForReady(ctx context.Context) error {
	deadline := time.Now().Add(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for plugin to be ready")
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+"/health", nil)
			if err != nil {
				continue
			}

			resp, err := c.httpClient.Do(req)
			if err != nil {
				continue
			}
			_ = resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
	}
}

// Call invokes a method on the plugin via HTTP POST.
func (c *SubprocessClient) Call(ctx context.Context, method string, params any) ([]byte, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("plugin client is closed")
	}
	httpClient := c.httpClient
	endpoint := c.endpoint
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/call", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := httpClient.Do(req)
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

// Close terminates the subprocess.
func (c *SubprocessClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	if c.cmd != nil && c.cmd.Process != nil {
		// Send SIGTERM first
		if err := c.cmd.Process.Signal(os.Interrupt); err != nil {
			// If SIGTERM fails, force kill
			_ = c.cmd.Process.Kill()
		}

		// Wait for process to exit with timeout
		done := make(chan error, 1)
		go func() {
			done <- c.cmd.Wait()
		}()

		select {
		case <-done:
			// Process exited
		case <-time.After(5 * time.Second):
			// Force kill if still running
			_ = c.cmd.Process.Kill()
		}
	}

	return nil
}

// Verify SubprocessTransport implements PluginTransport
var _ PluginTransport = (*SubprocessTransport)(nil)

// Verify SubprocessClient implements PluginClient
var _ PluginClient = (*SubprocessClient)(nil)
