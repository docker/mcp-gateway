package mocks

import (
	"context"

	v0 "github.com/modelcontextprotocol/registry/pkg/api/v0"

	"github.com/docker/mcp-gateway/pkg/registryapi"
)

type mockRegistryAPIClient struct {
	options MockRegistryAPIClientOptions
}

type MockRegistryAPIClientOptions struct {
	serverResponses     map[string]v0.ServerResponse
	serverListResponses map[string]v0.ServerListResponse
	listServersResponse []v0.ServerResponse
	listServersError    error
}

type MockRegistryAPIClientOption func(*MockRegistryAPIClientOptions)

func WithServerResponses(serverResponses map[string]v0.ServerResponse) MockRegistryAPIClientOption {
	return func(o *MockRegistryAPIClientOptions) {
		o.serverResponses = serverResponses
	}
}

func WithServerListResponses(serverListResponses map[string]v0.ServerListResponse) MockRegistryAPIClientOption {
	return func(o *MockRegistryAPIClientOptions) {
		o.serverListResponses = serverListResponses
	}
}

func WithListServersResponse(servers []v0.ServerResponse) MockRegistryAPIClientOption {
	return func(o *MockRegistryAPIClientOptions) {
		o.listServersResponse = servers
	}
}

func WithListServersError(err error) MockRegistryAPIClientOption {
	return func(o *MockRegistryAPIClientOptions) {
		o.listServersError = err
	}
}

func NewMockRegistryAPIClient(opts ...MockRegistryAPIClientOption) registryapi.Client {
	options := &MockRegistryAPIClientOptions{
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

func (c *mockRegistryAPIClient) GetServer(_ context.Context, url *registryapi.ServerURL) (v0.ServerResponse, error) {
	return c.options.serverResponses[url.String()], nil
}

func (c *mockRegistryAPIClient) GetServerVersions(_ context.Context, url *registryapi.ServerURL) (v0.ServerListResponse, error) {
	return c.options.serverListResponses[url.VersionsListURL()], nil
}

func (c *mockRegistryAPIClient) ListServers(_ context.Context, _ string, _ string) ([]v0.ServerResponse, error) {
	if c.options.listServersError != nil {
		return nil, c.options.listServersError
	}
	return c.options.listServersResponse, nil
}
