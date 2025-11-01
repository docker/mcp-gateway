package registryapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	registryapi "github.com/modelcontextprotocol/registry/pkg/api/v0"
)

type Client interface {
	GetServer(ctx context.Context, url *ServerURL) (registryapi.ServerResponse, error)
	GetServerVersions(ctx context.Context, url *ServerURL) (registryapi.ServerListResponse, error)
}

type client struct {
	client *http.Client
}

func NewClient() Client {
	return &client{
		client: &http.Client{Timeout: 20 * time.Second},
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
