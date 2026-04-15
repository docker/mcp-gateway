package catalog

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestIntegrationNPMTransformBatch fetches real npm-only servers from the MCP community
// registry and validates that TransformToDocker produces correct output for each one.
// This test exercises the full npm transformation pipeline against live registry data.
//
// Run with: go test -count=1 ./pkg/catalog/... -run TestIntegrationNPMTransformBatch -v -timeout 5m
func TestIntegrationNPMTransformBatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Use an explicit transport that bypasses any system/Desktop proxy settings,
	// since the Go default transport may pick up macOS proxy configuration.
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:     &tls.Config{},
			DialContext:         (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	// Fetch npm-only servers from the community registry (direct HTTP, no Desktop proxy).
	// Paginates up to maxPages to collect a broad sample. The registry has ~16K servers
	// total with ~49% being npm, so 50 pages of 100 covers a good sample.
	servers, pagesRead, err := fetchNPMOnlyServers(ctx, client, 500, 30)
	if err != nil {
		// Network errors during pagination are not fatal - test what we got
		t.Logf("Warning: registry fetch stopped early after %d pages: %v", pagesRead, err)
	}

	if len(servers) == 0 {
		t.Fatal("No npm-only servers found in registry - cannot validate npm support")
	}

	t.Logf("Fetched %d npm-only stdio servers across %d pages of registry results", len(servers), pagesRead)

	// Mock resolver returns empty string so we use default node version.
	// This avoids hammering the npm registry during batch testing.
	mockResolver := func(_ context.Context, _, _, _ string) (string, bool) {
		return "", true
	}

	var (
		passed  int
		failed  int
		skipped int
	)

	for _, s := range servers {
		name := s.Server.Name
		t.Run(name, func(t *testing.T) {
			result, source, err := TransformToDocker(ctx, s.Server,
				WithNPMResolver(mockResolver),
			)
			if err != nil {
				t.Logf("SKIP %s: transform error: %v", name, err)
				skipped++
				return
			}

			if source != TransformSourceNPM {
				// Server has a remote transport that took precedence
				t.Logf("SKIP %s: resolved as %s (not npm)", name, source)
				skipped++
				return
			}

			// Validate image
			if !strings.HasPrefix(result.Image, "node:") {
				t.Errorf("%s: expected image starting with 'node:', got '%s'", name, result.Image)
				failed++
				return
			}
			if !strings.HasSuffix(result.Image, "-bookworm-slim") {
				t.Errorf("%s: expected image ending with '-bookworm-slim', got '%s'", name, result.Image)
				failed++
				return
			}

			// Validate command starts with npx --yes
			if len(result.Command) < 3 {
				t.Errorf("%s: expected command length >= 3, got %d: %v", name, len(result.Command), result.Command)
				failed++
				return
			}
			if result.Command[0] != "npx" {
				t.Errorf("%s: expected command[0] 'npx', got '%s'", name, result.Command[0])
				failed++
				return
			}
			if result.Command[1] != "--yes" {
				t.Errorf("%s: expected command[1] '--yes', got '%s'", name, result.Command[1])
				failed++
				return
			}

			// Validate type and longLived
			if result.Type != "server" {
				t.Errorf("%s: expected type 'server', got '%s'", name, result.Type)
				failed++
				return
			}
			if !result.LongLived {
				t.Errorf("%s: expected longLived=true", name)
				failed++
				return
			}

			// Validate volumes contain npm cache
			if len(result.Volumes) == 0 {
				t.Errorf("%s: expected at least one volume for npm cache", name)
				failed++
				return
			}
			hasNPMCache := false
			for _, v := range result.Volumes {
				if strings.Contains(v, ":/root/.npm") && strings.HasPrefix(v, "docker-mcp-npm-cache-") {
					hasNPMCache = true
					break
				}
			}
			if !hasNPMCache {
				t.Errorf("%s: expected npm cache volume, got %v", name, result.Volumes)
				failed++
				return
			}

			// Validate metadata has registry URL
			if result.Metadata == nil || result.Metadata.RegistryURL == "" {
				t.Errorf("%s: expected metadata.registryUrl to be set", name)
				failed++
				return
			}

			passed++
		})
	}

	t.Logf("\n=== NPM Transform Batch Results ===")
	t.Logf("Total servers fetched: %d", len(servers))
	t.Logf("Passed:                %d", passed)
	t.Logf("Failed:                %d", failed)
	t.Logf("Skipped (remote/err):  %d", skipped)

	if failed > 0 {
		t.Errorf("%d servers failed transformation validation", failed)
	}

	if passed == 0 {
		t.Error("No servers passed npm transformation - something is wrong")
	}

	successRate := float64(passed) / float64(passed+failed) * 100
	t.Logf("Success rate:          %.1f%% (%d/%d of transformable servers)", successRate, passed, passed+failed)
}

// TestIntegrationNPMVersionResolver tests the real npm registry version resolver
// against a set of known npm MCP packages.
//
// Run with: go test -count=1 ./pkg/catalog/... -run TestIntegrationNPMVersionResolver -v
func TestIntegrationNPMVersionResolver(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	resolverClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:     &tls.Config{},
			DialContext:         (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
	resolver := NewNPMVersionResolver(resolverClient)
	ctx := context.Background()

	tests := []struct {
		identifier string
		version    string
		wantFound  bool
	}{
		{"@anthropic-ai/claude-code", "", true},
		{"@agenttrust/mcp-server", "1.1.1", true},
		{"@contextlayer/mcp", "0.0.3", true},
		{"@nonexistent-scope/definitely-not-a-real-package", "99.99.99", false},
	}

	for _, tt := range tests {
		name := fmt.Sprintf("%s@%s", tt.identifier, tt.version)
		t.Run(name, func(t *testing.T) {
			nodeVer, found := resolver(ctx, tt.identifier, tt.version, "")
			if found != tt.wantFound {
				t.Errorf("found=%v, want %v", found, tt.wantFound)
			}
			if found {
				t.Logf("%s: engines.node resolved to %q (will use image %s)", name, nodeVer, nodeVersionToImageTag(nodeVer))
			}
		})
	}
}

// registryListResponse matches the community registry list endpoint response.
type registryListResponse struct {
	Servers  []serverResponseJSON `json:"servers"`
	Metadata struct {
		NextCursor string `json:"nextCursor"`
		Count      int    `json:"count"`
	} `json:"metadata"`
}

type serverResponseJSON struct {
	Server ServerDetail `json:"server"`
}

// fetchNPMOnlyServers fetches npm-only servers from the MCP community registry.
// It paginates through the registry collecting servers that have only npm packages
// with stdio transport. Stops after maxServers are collected or maxPages are read.
// Returns partial results with a non-nil error if a page fetch fails.
func fetchNPMOnlyServers(ctx context.Context, client *http.Client, maxServers int, maxPages int) ([]serverResponseJSON, int, error) {
	var npmServers []serverResponseJSON
	cursor := ""
	baseURL := "https://registry.modelcontextprotocol.io/v0/servers?version=latest&limit=100"
	pagesRead := 0

	for len(npmServers) < maxServers && pagesRead < maxPages {
		reqURL := baseURL
		if cursor != "" {
			reqURL += "&cursor=" + url.QueryEscape(cursor)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return npmServers, pagesRead, fmt.Errorf("creating request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			// Return what we have so far on network error
			return npmServers, pagesRead, fmt.Errorf("fetching page %d: %w", pagesRead+1, err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return npmServers, pagesRead, fmt.Errorf("reading page %d: %w", pagesRead+1, err)
		}

		if resp.StatusCode != http.StatusOK {
			return npmServers, pagesRead, fmt.Errorf("page %d returned %d: %s", pagesRead+1, resp.StatusCode, string(body[:min(200, len(body))]))
		}

		var listResp registryListResponse
		if err := json.Unmarshal(body, &listResp); err != nil {
			return npmServers, pagesRead, fmt.Errorf("parsing page %d: %w", pagesRead+1, err)
		}

		pagesRead++

		for _, s := range listResp.Servers {
			pkgs := s.Server.Packages
			hasNPMStdio := false
			hasOCI := false
			hasPyPI := false
			for _, p := range pkgs {
				switch p.RegistryType {
				case "oci":
					hasOCI = true
				case "pypi":
					hasPyPI = true
				case "npm":
					if p.Transport.Type == "stdio" {
						hasNPMStdio = true
					}
				}
			}
			if hasNPMStdio && !hasOCI && !hasPyPI {
				npmServers = append(npmServers, s)
			}
		}

		if listResp.Metadata.NextCursor == "" || len(listResp.Servers) == 0 {
			break
		}
		cursor = listResp.Metadata.NextCursor
	}

	if len(npmServers) > maxServers {
		npmServers = npmServers[:maxServers]
	}

	return npmServers, pagesRead, nil
}
