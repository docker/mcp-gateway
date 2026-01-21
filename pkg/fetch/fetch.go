package fetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

// Fetches a URL and returns the body as a byte slice.
// The body is limited to 5MB to prevent abuse.
// The timeout is 30 seconds.
func Untrusted(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: desktop.ProxyTransport(),
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch %s: %s", url, resp.Status)
	}

	buf, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // 5MB
	if err != nil {
		return nil, err
	}

	return buf, nil
}
