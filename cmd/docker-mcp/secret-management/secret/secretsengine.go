package secret

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

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
	log.Printf("[GetSecrets] Waiting for mutex...")
	getSecretsMu.Lock()
	log.Printf("[GetSecrets] Acquired mutex")
	defer func() {
		getSecretsMu.Unlock()
		log.Printf("[GetSecrets] Released mutex")
	}()

	pattern := `{"pattern": "docker/mcp/**"}`

	log.Printf("[GetSecrets] Creating request...")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost/resolver.v1.ResolverService/GetSecrets", bytes.NewReader([]byte(pattern)))
	if err != nil {
		log.Printf("[GetSecrets] Failed to create request: %v", err)
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")

	// Create fresh HTTP client for each request
	client := newHTTPClient()

	log.Printf("[GetSecrets] Sending request to Unix socket...")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[GetSecrets] Request failed: %v", err)
		return nil, err
	}
	log.Printf("[GetSecrets] Got response: status=%d", resp.StatusCode)
	defer resp.Body.Close()

	// No secrets found
	if resp.StatusCode == http.StatusNotFound {
		log.Printf("[GetSecrets] No secrets found (404)")
		return []Envelope{}, nil
	}

	var secrets map[string][]Envelope
	if err := json.NewDecoder(resp.Body).Decode(&secrets); err != nil {
		log.Printf("[GetSecrets] Failed to decode response: %v", err)
		return nil, fmt.Errorf("failed to unmarshal secrets response: %w", err)
	}

	log.Printf("[GetSecrets] Successfully retrieved %d secrets", len(secrets["envelopes"]))
	return secrets["envelopes"], nil
}
