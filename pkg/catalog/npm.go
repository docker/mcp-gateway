package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

const (
	defaultNodeVersion = "22"
	nodeBaseImage      = "node"
)

var (
	nodeVersionRe     = regexp.MustCompile(`(?:>=|>|\^|~)\s*(\d+)`)
	bareNodeVersionRe = regexp.MustCompile(`^\s*(\d+)\s*$`)
)

// NPMVersionResolver resolves the minimum Node.js version for an npm package.
// It returns the minimum Node.js major version string (e.g., "18") or empty string if unknown,
// and a boolean indicating whether the package was found.
type NPMVersionResolver func(ctx context.Context, identifier, version, registryBaseURL string) (string, bool)

type npmPackageInfo struct {
	Engines struct {
		Node string `json:"node"`
	} `json:"engines"`
}

// NewNPMVersionResolver creates a resolver that queries the npm registry API.
func NewNPMVersionResolver(httpClient *http.Client) NPMVersionResolver {
	return func(ctx context.Context, identifier, version, registryBaseURL string) (string, bool) {
		// Only query npm for standard npm registry
		if registryBaseURL != "" && registryBaseURL != "https://registry.npmjs.org" {
			return "", true // assume found for non-standard registries
		}

		var url string
		if version != "" {
			url = fmt.Sprintf("https://registry.npmjs.org/%s/%s", identifier, version)
		} else {
			url = fmt.Sprintf("https://registry.npmjs.org/%s/latest", identifier)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", false
		}

		resp, err := httpClient.Do(req)
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
}

// DefaultNPMVersionResolver creates a resolver using the default HTTP client with proxy transport.
func DefaultNPMVersionResolver() NPMVersionResolver {
	client := &http.Client{
		Transport: desktop.ProxyTransport(),
		Timeout:   10 * time.Second,
	}
	return NewNPMVersionResolver(client)
}

// parseNodeVersion extracts the minimum major Node.js version from a semver constraint.
// It looks for the first >=, >, ^, or ~ operator and returns the major version number.
// A bare major version (e.g., "18") is also accepted.
// Examples: ">=18" -> "18", "^20.0.0" -> "20", ">=16.17.0" -> "16", "18" -> "18", "" -> "" (use default)
func parseNodeVersion(enginesNode string) string {
	if enginesNode == "" {
		return ""
	}

	match := nodeVersionRe.FindStringSubmatch(enginesNode)
	if match != nil {
		return match[1]
	}

	// Fall back to bare major version (e.g., "18")
	if bare := bareNodeVersionRe.FindStringSubmatch(enginesNode); bare != nil {
		return bare[1]
	}

	return ""
}

// nodeVersionToImageTag maps a Node.js major version to the appropriate Docker image tag.
func nodeVersionToImageTag(nodeVersion string) string {
	if nodeVersion == "" {
		nodeVersion = defaultNodeVersion
	}

	return fmt.Sprintf("%s:%s-bookworm-slim", nodeBaseImage, nodeVersion)
}
