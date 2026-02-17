package registryapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
	ListServers(ctx context.Context, baseURL string, cursor string) ([]registryapi.ServerResponse, error)
}

type client struct {
	client *http.Client
}

func NewClient() Client {
	return &client{
		client: &http.Client{
			Transport: desktop.ProxyTransport(),
			Timeout:   20 * time.Second,
		},
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

func (c *client) ListServers(ctx context.Context, baseURL string, cursor string) ([]registryapi.ServerResponse, error) {
	var all []registryapi.ServerResponse
	for {
		u := fmt.Sprintf("%s/v0/servers?version=latest&limit=100", baseURL)
		if cursor != "" {
			u += "&cursor=" + url.QueryEscape(cursor)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
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

		var listResp registryapi.ServerListResponse
		if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}
		resp.Body.Close()

		all = append(all, listResp.Servers...)

		if listResp.Metadata.NextCursor == "" {
			break
		}
		cursor = listResp.Metadata.NextCursor
	}
	return all, nil
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
