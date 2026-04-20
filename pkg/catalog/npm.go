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
	defaultNodeVersion = "24"
	nodeBaseImage      = "node"
)

var (
	nodePinVersionRe    = regexp.MustCompile(`(?:\^|~)\s*(\d+)`)
	nodeLowerBoundRe    = regexp.MustCompile(`(?:>=|>)\s*\d+`)
	bareNodeVersionRe   = regexp.MustCompile(`^\s*(\d+)\s*$`)
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

// parseNodeVersion extracts the Node.js major version from a semver constraint.
// Pinning operators (^ or ~) resolve to a specific major version.
// >= and > mean "this or newer", so we use the latest (default).
// A bare major version (e.g., "18") is also accepted.
// Examples: "^20.0.0" -> "20", "~18" -> "18", ">=18" -> "" (use latest), ">16" -> "" (use latest),
// "18" -> "18", "" -> "" (use default)
func parseNodeVersion(enginesNode string) string {
	if enginesNode == "" {
		return ""
	}

	// Check for pinning constraints (^ or ~) first - these specify a major version
	if match := nodePinVersionRe.FindStringSubmatch(enginesNode); match != nil {
		return match[1]
	}

	// >= and > mean "this or newer", so use the latest (default)
	if nodeLowerBoundRe.MatchString(enginesNode) {
		return ""
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
