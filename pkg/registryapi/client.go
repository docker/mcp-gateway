package registryapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	registryapi "github.com/modelcontextprotocol/registry/pkg/api/v0"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

// CommunityRegistryBaseURL is the base URL for the community MCP registry
const CommunityRegistryBaseURL = "https://registry.modelcontextprotocol.io"

// BuildServerURL constructs the full registry URL for a server
// Format: https://registry.modelcontextprotocol.io/v0/servers/{encoded_name}/versions/{version}
func BuildServerURL(serverName, version string) string {
	encodedName := url.PathEscape(serverName)
	return fmt.Sprintf("%s/v0/servers/%s/versions/%s", CommunityRegistryBaseURL, encodedName, version)
}

type Client interface {
	GetServer(ctx context.Context, url *ServerURL) (registryapi.ServerResponse, error)
	GetServerVersions(ctx context.Context, url *ServerURL) (registryapi.ServerListResponse, error)
	// ListServers lists all servers from the community registry with optional search query
	ListServers(ctx context.Context, query string) ([]registryapi.ServerResponse, error)
}

type client struct {
	client  *http.Client
	baseURL string
}

func NewClient() Client {
	return &client{
		client: &http.Client{
			Transport: desktop.ProxyTransport(),
			Timeout:   20 * time.Second,
		},
		baseURL: CommunityRegistryBaseURL,
	}
}

func (c *client) GetServer(ctx context.Context, url *ServerURL) (registryapi.ServerResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
	if err != nil {
		return registryapi.ServerResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return registryapi.ServerResponse{}, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return registryapi.ServerResponse{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var serverResp registryapi.ServerResponse
	if err := json.NewDecoder(resp.Body).Decode(&serverResp); err != nil {
		return registryapi.ServerResponse{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return serverResp, nil
}

func (c *client) GetServerVersions(ctx context.Context, url *ServerURL) (registryapi.ServerListResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.VersionsListURL(), nil)
	if err != nil {
		return registryapi.ServerListResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return registryapi.ServerListResponse{}, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return registryapi.ServerListResponse{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var serverListResp registryapi.ServerListResponse
	if err := json.NewDecoder(resp.Body).Decode(&serverListResp); err != nil {
		return registryapi.ServerListResponse{}, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return serverListResp, nil
}

// ListServers lists servers from the community registry with optional search query
// The registry API supports query parameters for filtering
// This function automatically handles pagination to return all results
func (c *client) ListServers(ctx context.Context, query string) ([]registryapi.ServerResponse, error) {
	return c.fetchAllServers(ctx, query)
}

// fetchAllServers fetches all servers from the registry with pagination
func (c *client) fetchAllServers(ctx context.Context, query string) ([]registryapi.ServerResponse, error) {
	var allServers []registryapi.ServerResponse
	cursor := ""

	for {
		url := fmt.Sprintf("%s/v0/servers", c.baseURL)

		// Build query parameters
		// Always use version=latest to only get latest versions (~500 vs ~4000 results)
		params := []string{"version=latest"}
		if query != "" {
			params = append(params, fmt.Sprintf("q=%s", query))
		}
		if cursor != "" {
			params = append(params, fmt.Sprintf("cursor=%s", cursor))
		}
		url = fmt.Sprintf("%s?%s", url, strings.Join(params, "&"))

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := c.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to execute request: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		var serverListResp registryapi.ServerListResponse
		if err := json.NewDecoder(resp.Body).Decode(&serverListResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}
		resp.Body.Close()

		allServers = append(allServers, serverListResp.Servers...)

		// Check if there are more pages
		if serverListResp.Metadata.NextCursor == "" {
			break
		}
		cursor = serverListResp.Metadata.NextCursor
	}

	return allServers, nil
}
