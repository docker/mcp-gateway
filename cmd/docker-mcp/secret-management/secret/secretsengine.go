package secret

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
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

// This is a temporary Client and should be removed once we have the secrets engine
// client SDK imported.
var c = http.Client{
	Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			d := &net.Dialer{}
			return d.DialContext(ctx, "unix", socketPath())
		},
	},
}

func GetSecrets(ctx context.Context) ([]Envelope, error) {
	pattern := `{"pattern": "docker/mcp/**"}`

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost/resolver.v1.ResolverService/GetSecrets", bytes.NewReader([]byte(pattern)))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.Do(req)
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
