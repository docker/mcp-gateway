package secret

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrSecretNotFound = errors.New("secret not found")

type Envelope struct {
	ID       string `json:"id"`
	Value    []byte `json:"value"`
	Provider string `json:"provider"`
}

func socketPath() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "docker-secrets-engine", "engine.sock")
	}
	return filepath.Join(os.TempDir(), "docker-secrets-engine", "engine.sock")
}

// Mutex to serialize GetSecrets calls - concurrent Unix socket requests can hang
var getSecretsMu sync.Mutex

// newHTTPClient creates a fresh HTTP client for each request.
// This avoids connection state issues with Unix sockets that can cause hangs.
func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socketPath())
			},
			DisableKeepAlives: true,
		},
	}
}

func GetSecrets(ctx context.Context) ([]Envelope, error) {
	getSecretsMu.Lock()
	defer getSecretsMu.Unlock()

	pattern := `{"pattern": "docker/mcp/**"}`

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost/resolver.v1.ResolverService/GetSecrets", bytes.NewReader([]byte(pattern)))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")

	client := newHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// No secrets found
	if resp.StatusCode == http.StatusNotFound {
		return []Envelope{}, nil
	}

	var secrets map[string][]Envelope
	if err := json.NewDecoder(resp.Body).Decode(&secrets); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secrets response: %w", err)
	}

	return secrets["envelopes"], nil
}

// GetSecret retrieves a single secret by its full key (e.g., "docker/mcp/oauth/github").
// Returns ErrSecretNotFound if the secret does not exist.
func GetSecret(ctx context.Context, key string) (*Envelope, error) {
	getSecretsMu.Lock()
	defer getSecretsMu.Unlock()

	pattern := fmt.Sprintf(`{"pattern": "%s"}`, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"http://localhost/resolver.v1.ResolverService/GetSecrets",
		bytes.NewReader([]byte(pattern)))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")

	client := newHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrSecretNotFound
	}

	var secrets map[string][]Envelope
	if err := json.NewDecoder(resp.Body).Decode(&secrets); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secret response: %w", err)
	}

	envelopes := secrets["envelopes"]
	if len(envelopes) == 0 {
		return nil, ErrSecretNotFound
	}

	return &envelopes[0], nil
}
