package mocks

import (
	"context"

	"github.com/docker/mcp-gateway/pkg/registryapi"
	v0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
)

type mockRegistryAPIClient struct {
	options mockRegistryAPIClientOptions
}

type mockRegistryAPIClientOptions struct {
	serverResponses     map[string]v0.ServerResponse
	serverListResponses map[string]v0.ServerListResponse
}

type mockRegistryAPIClientOption func(*mockRegistryAPIClientOptions)

func WithServerResponses(serverResponses map[string]v0.ServerResponse) mockRegistryAPIClientOption {
	return func(o *mockRegistryAPIClientOptions) {
		o.serverResponses = serverResponses
	}
}

func WithServerListResponses(serverListResponses map[string]v0.ServerListResponse) mockRegistryAPIClientOption {
	return func(o *mockRegistryAPIClientOptions) {
		o.serverListResponses = serverListResponses
	}
}

func NewMockRegistryAPIClient(opts ...mockRegistryAPIClientOption) registryapi.Client {
	options := &mockRegistryAPIClientOptions{
		serverResponses:     make(map[string]v0.ServerResponse),
		serverListResponses: make(map[string]v0.ServerListResponse),
	}
	for _, opt := range opts {
		opt(options)
	}
	return &mockRegistryAPIClient{
		options: *options,
	}
}

func (c *mockRegistryAPIClient) GetServer(ctx context.Context, url *registryapi.ServerURL) (v0.ServerResponse, error) {
	return c.options.serverResponses[url.String()], nil
}

func (c *mockRegistryAPIClient) GetServerVersions(ctx context.Context, url *registryapi.ServerURL) (v0.ServerListResponse, error) {
	return c.options.serverListResponses[url.VersionsListURL()], nil
}
