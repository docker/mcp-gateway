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
	// Workaround: Query multiple patterns since docker/mcp/** double-wildcard isn't working
	// TODO: Remove once Secrets Engine fixes pattern matching bug
	patterns := []string{
		fmt.Sprintf(`{"pattern": "%s*"}`, NamespaceGeneric),  // Generic secrets (docker pass)
		fmt.Sprintf(`{"pattern": "%s*"}`, NamespaceOAuth),    // OAuth tokens
		fmt.Sprintf(`{"pattern": "%s*"}`, NamespaceOAuthDCR), // DCR configs
	}

	allSecrets := make(map[string]Envelope) // Use map to deduplicate by ID

	for _, pattern := range patterns {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost/resolver.v1.ResolverService/GetSecrets", bytes.NewReader([]byte(pattern)))
		if err != nil {
			return nil, err
		}
		req.Header.Add("Content-Type", "application/json")

		resp, err := c.Do(req)
		if err != nil {
			return nil, err
		}

		var secrets map[string][]Envelope
		if err := json.NewDecoder(resp.Body).Decode(&secrets); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		// Merge results, deduplicating by ID
		for _, env := range secrets["envelopes"] {
			allSecrets[env.ID] = env
		}
	}

	// Convert map back to slice
	result := make([]Envelope, 0, len(allSecrets))
	for _, env := range allSecrets {
		result = append(result, env)
	}

	return result, nil
}
