package desktop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

var ClientBackend = newRawClient(dialBackend)

var (
	desktopProxyTransportOnce sync.Once
	desktopProxyTransportInst http.RoundTripper
)

// ProxyTransport returns an HTTP transport configured to proxy HTTP requests.
// If HTTP_PROXY/HTTPS_PROXY environment variables are set, they take precedence and
// are respected via http.ProxyFromEnvironment. Otherwise, when Docker Desktop is running,
// traffic is routed through Docker Desktop's HTTP proxy socket. If neither applies,
// http.DefaultTransport is returned.
// The transport is initialized once using sync.Once and cached for subsequent calls.
func ProxyTransport() http.RoundTripper {
	desktopProxyTransportOnce.Do(func() {
		// Env proxy vars take precedence over Docker Desktop's proxy socket.
		if hasEnvProxyVars() {
			desktopProxyTransportInst = &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			}
			return
		}

		ctx := context.Background()
		if !IsRunningInDockerDesktop(ctx) {
			desktopProxyTransportInst = http.DefaultTransport
			return
		}

		desktopProxyTransportInst = &http.Transport{
			Proxy: http.ProxyURL(&url.URL{
				Scheme: "http",
			}),
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return dialHTTPProxy(ctx)
			},
		}
	})

	return desktopProxyTransportInst
}

// hasEnvProxyVars returns true if any HTTP proxy environment variables are set.
func hasEnvProxyVars() bool {
	for _, name := range []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy"} {
		if os.Getenv(name) != "" {
			return true
		}
	}
	return false
}

func AvoidResourceSaverMode(ctx context.Context) {
	_ = ClientBackend.Post(ctx, "/idle/make-busy", nil, nil)
}

type RawClient struct {
	client  func() *http.Client
	timeout time.Duration
}

func newRawClient(dialer func(ctx context.Context) (net.Conn, error)) *RawClient {
	return &RawClient{
		client: func() *http.Client {
			return &http.Client{
				Transport: &http.Transport{
					DialContext: func(ctx context.Context, _, _ string) (conn net.Conn, err error) {
						return dialer(ctx)
					},
				},
			}
		},
		timeout: 10 * time.Second,
	}
}

func (c *RawClient) Get(ctx context.Context, endpoint string, v any) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost"+endpoint, http.NoBody)
	if err != nil {
		return err
	}

	response, err := c.client().Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	buf, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	// Check HTTP status code - return error for non-2xx responses
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		// Try to parse error message from response
		var errorMsg struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(buf, &errorMsg) == nil && errorMsg.Message != "" {
			return fmt.Errorf("HTTP %d: %s", response.StatusCode, errorMsg.Message)
		}
		return fmt.Errorf("HTTP %d: %s", response.StatusCode, string(buf))
	}

	if err := json.Unmarshal(buf, &v); err != nil {
		return err
	}
	return nil
}

func (c *RawClient) Post(ctx context.Context, endpoint string, v any, result any) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	var body io.Reader
	if v != nil {
		buf, err := json.Marshal(v)
		if err != nil {
			return err
		}
		body = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost"+endpoint, body)
	if err != nil {
		return err
	}

	if v != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	response, err := c.client().Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	buf, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	// Check HTTP status code - return error for non-2xx responses
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		// Try to parse error message from response
		var errorMsg struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(buf, &errorMsg) == nil && errorMsg.Message != "" {
			return fmt.Errorf("HTTP %d: %s", response.StatusCode, errorMsg.Message)
		}
		return fmt.Errorf("HTTP %d: %s", response.StatusCode, string(buf))
	}

	if result == nil {
		return nil
	}

	if err := json.Unmarshal(buf, &result); err != nil {
		return err
	}
	return nil
}

func (c *RawClient) Delete(ctx context.Context, endpoint string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, "http://localhost"+endpoint, http.NoBody)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	response, err := c.client().Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	buf, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	// Check HTTP status code - return error for non-2xx responses
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		// Try to parse error message from response
		var errorMsg struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(buf, &errorMsg) == nil && errorMsg.Message != "" {
			return fmt.Errorf("HTTP %d: %s", response.StatusCode, errorMsg.Message)
		}
		return fmt.Errorf("HTTP %d: %s", response.StatusCode, string(buf))
	}

	return nil
}
