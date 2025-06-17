package catalog

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/user"
)

func Get(ctx context.Context) (Catalog, error) {
	return ReadFrom(ctx, "docker-mcp.yaml")
}

func ReadFrom(ctx context.Context, fileOrURL string) (Catalog, error) {
	servers, err := readMCPServers(ctx, fileOrURL)
	if err != nil {
		return Catalog{}, err
	}
	if servers == nil {
		servers = map[string]Server{}
	}

	return Catalog{
		Servers: servers,
	}, nil
}

func readMCPServers(ctx context.Context, fileOrURL string) (map[string]Server, error) {
	buf, err := readFileOrURL(ctx, fileOrURL)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Server{}, nil
		}
		return nil, err
	}

	var topLevel topLevel
	if err := yaml.Unmarshal(buf, &topLevel); err != nil {
		return nil, err
	}

	return topLevel.Registry, nil
}

func readFileOrURL(ctx context.Context, fileOrURL string) ([]byte, error) {
	switch {
	case strings.HasPrefix(fileOrURL, "http://") || strings.HasPrefix(fileOrURL, "https://"):
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileOrURL, nil)
		if err != nil {
			return nil, err
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to fetch URL: %s, status: %s", fileOrURL, resp.Status)
		}

		return io.ReadAll(resp.Body)

	case filepath.IsAbs(fileOrURL):
		buf, err := os.ReadFile(fileOrURL)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		return buf, nil

	default:
		homeDir, err := user.HomeDir()
		if err != nil {
			return nil, err
		}

		path := filepath.Join(homeDir, ".docker", "mcp", "catalogs", fileOrURL)

		buf, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		return buf, nil
	}
}
