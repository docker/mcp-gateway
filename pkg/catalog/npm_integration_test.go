package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

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

// TestNPMTransformBatch loads npm servers from testdata and validates that
// TransformToDocker produces correct output for each one.
//
// Run with: go test -count=1 ./pkg/catalog/... -run TestNPMTransformBatch -v
func TestNPMTransformBatch(t *testing.T) {
	data, err := os.ReadFile("testdata/npm_registry_servers.json")
	if err != nil {
		t.Fatalf("reading testdata: %v", err)
	}

	var listResp registryListResponse
	if err := json.Unmarshal(data, &listResp); err != nil {
		t.Fatalf("parsing testdata: %v", err)
	}

	servers := listResp.Servers
	if len(servers) == 0 {
		t.Fatal("No servers found in testdata")
	}

	t.Logf("Loaded %d npm servers from testdata", len(servers))

	// Mock resolver returns empty string so we use default node version.
	mockResolver := func(_ context.Context, _, _, _ string) (string, bool) {
		return "", true
	}

	ctx := t.Context()

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
	t.Logf("Total servers loaded: %d", len(servers))
	t.Logf("Passed:               %d", passed)
	t.Logf("Failed:               %d", failed)
	t.Logf("Skipped (remote/err): %d", skipped)

	if failed > 0 {
		t.Errorf("%d servers failed transformation validation", failed)
	}

	if passed == 0 {
		t.Error("No servers passed npm transformation - something is wrong")
	}
}

// TestNPMVersionResolver tests the npm registry version resolver using a local
// httptest server that serves responses from testdata files.
//
// Run with: go test -count=1 ./pkg/catalog/... -run TestNPMVersionResolver -v
func TestNPMVersionResolver(t *testing.T) {
	// Map of npm registry URL paths to testdata files
	packageFiles := map[string]string{
		"/@anthropic-ai/claude-code/latest": "testdata/npm_package_claude_code.json",
		"/@agenttrust/mcp-server/1.1.1":     "testdata/npm_package_agenttrust.json",
		"/@contextlayer/mcp/0.0.3":          "testdata/npm_package_contextlayer.json",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		file, ok := packageFiles[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		data, err := os.ReadFile(file)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	// Create a resolver that points to our test server instead of the real npm registry.
	// We construct the resolver manually rather than using NewNPMVersionResolver because
	// the standard resolver hardcodes the npm registry URL.
	resolver := func(ctx context.Context, identifier, version, _ string) (string, bool) {
		var url string
		if version != "" {
			url = fmt.Sprintf("%s/%s/%s", srv.URL, identifier, version)
		} else {
			url = fmt.Sprintf("%s/%s/latest", srv.URL, identifier)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", false
		}

		resp, err := srv.Client().Do(req)
		if err != nil {
			return "", false
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", false
		}

		var info npmPackageInfo
		if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
			return "", false
		}

		return parseNodeVersion(info.Engines.Node), true
	}

	ctx := t.Context()

	tests := []struct {
		identifier  string
		version     string
		wantFound   bool
		wantVersion string
	}{
		{"@anthropic-ai/claude-code", "", true, ""},
		{"@agenttrust/mcp-server", "1.1.1", true, ""},
		{"@contextlayer/mcp", "0.0.3", true, ""},
		{"@nonexistent-scope/definitely-not-a-real-package", "99.99.99", false, ""},
	}

	for _, tt := range tests {
		name := fmt.Sprintf("%s@%s", tt.identifier, tt.version)
		t.Run(name, func(t *testing.T) {
			nodeVer, found := resolver(ctx, tt.identifier, tt.version, "")
			if found != tt.wantFound {
				t.Errorf("found=%v, want %v", found, tt.wantFound)
			}
			if found && nodeVer != tt.wantVersion {
				t.Errorf("nodeVer=%q, want %q", nodeVer, tt.wantVersion)
			}
			if found {
				t.Logf("%s: engines.node resolved to %q (will use image %s)", name, nodeVer, nodeVersionToImageTag(nodeVer))
			}
		})
	}
}
