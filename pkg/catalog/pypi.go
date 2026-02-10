package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

const (
	defaultPythonVersion = "3.14"
	uvBaseImage          = "ghcr.io/astral-sh/uv"
)

// PyPIVersionResolver resolves the minimum Python version for a PyPI package.
// It returns the minimum Python version string (e.g., "3.10") or empty string if unknown.
type PyPIVersionResolver func(ctx context.Context, identifier, version, registryBaseURL string) string

type pypiPackageInfo struct {
	Info struct {
		RequiresPython string `json:"requires_python"`
	} `json:"info"`
}

// NewPyPIVersionResolver creates a resolver that queries the PyPI JSON API.
func NewPyPIVersionResolver(httpClient *http.Client) PyPIVersionResolver {
	return func(ctx context.Context, identifier, version, registryBaseURL string) string {
		// Only query PyPI for standard PyPI registry
		if registryBaseURL != "" && registryBaseURL != "https://pypi.org" {
			return ""
		}

		var url string
		if version != "" {
			url = fmt.Sprintf("https://pypi.org/pypi/%s/%s/json", identifier, version)
		} else {
			url = fmt.Sprintf("https://pypi.org/pypi/%s/json", identifier)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return ""
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			return ""
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return ""
		}

		var info pypiPackageInfo
		if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
			return ""
		}

		return parsePythonVersion(info.Info.RequiresPython)
	}
}

// DefaultPyPIVersionResolver creates a resolver using the default HTTP client with proxy transport.
func DefaultPyPIVersionResolver() PyPIVersionResolver {
	client := &http.Client{
		Transport: desktop.ProxyTransport(),
		Timeout:   10 * time.Second,
	}
	return NewPyPIVersionResolver(client)
}

// parsePythonVersion extracts a pinned major.minor Python version from a PEP 440 specifier.
// Pinning operators (~=, ==, <=, <) resolve to a specific version.
// >= and > mean "this or newer", so we use the latest.
// Examples: "~=3.10" -> "3.10", "==3.12" -> "3.12", ">=3.10" -> "" (use latest),
// "<=3.10" -> "3.10", "<3.10" -> "3.9"
func parsePythonVersion(requiresPython string) string {
	if requiresPython == "" {
		return ""
	}

	// Match all operator-version pairs in the specifier
	re := regexp.MustCompile(`(~=|==|!=|<=|>=|<|>)\s*(\d+)(?:\.(\d+))?`)
	allMatches := re.FindAllStringSubmatch(requiresPython, -1)

	// If any >= or > constraint exists, the package supports newer versions — use latest
	for _, m := range allMatches {
		if m[1] == ">=" || m[1] == ">" {
			return ""
		}
	}

	// Look for pinning constraints
	for _, m := range allMatches {
		if m[3] == "" {
			continue // no minor version component, skip
		}
		switch m[1] {
		case "~=", "==", "<=":
			return fmt.Sprintf("%s.%s", m[2], m[3])
		case "<":
			minor, err := strconv.Atoi(m[3])
			if err != nil || minor <= 0 {
				continue
			}
			return fmt.Sprintf("%s.%d", m[2], minor-1)
		}
	}

	return ""
}

// pythonVersionToImageTag maps a Python version to the appropriate uv Docker image tag.
func pythonVersionToImageTag(pythonVersion string) string {
	if pythonVersion == "" {
		pythonVersion = defaultPythonVersion
	}

	distro := "bookworm-slim"

	return fmt.Sprintf("%s:python%s-%s", uvBaseImage, pythonVersion, distro)
}
