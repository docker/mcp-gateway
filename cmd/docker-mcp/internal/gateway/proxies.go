package gateway

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/proxies"
	mcpclient "github.com/docker/mcp-gateway/cmd/docker-mcp/internal/mcp"
)

func (cp *clientPool) runProxies(ctx context.Context, allowedHosts []string, longRunning bool) (proxies.TargetConfig, func(context.Context) error, error) {
	var nwProxies []proxies.Proxy
	for _, spec := range allowedHosts {
		proxy, err := proxies.ParseProxySpec(spec)
		if err != nil {
			return proxies.TargetConfig{}, nil, fmt.Errorf("invalid proxy spec %q: %w", spec, err)
		}
		nwProxies = append(nwProxies, proxy)
	}

	return proxies.RunNetworkProxies(ctx, cp.docker, nwProxies, cp.LongLived || longRunning, cp.DebugDNS)
}

// RunProxies implements the ProxyRunner interface for use by provisioners
func (cp *clientPool) RunProxies(ctx context.Context, allowedHosts []string, longRunning bool) (proxies.TargetConfig, func(context.Context) error, error) {
	return cp.runProxies(ctx, allowedHosts, longRunning)
}

func newClientWithCleanup(client mcpclient.Client, cleanup func(context.Context) error) mcpclient.Client {
	return &clientWithCleanup{
		Client:  client,
		cleanup: cleanup,
	}
}

func (c *clientWithCleanup) Close() error {
	return errors.Join(c.Session().Close(), c.cleanup(context.TODO()))
}

// GetCleanup returns the cleanup function - used by ReleaseClient
func (c *clientWithCleanup) GetCleanup() func(context.Context) error {
	return c.cleanup
}

type clientWithCleanup struct {
	mcpclient.Client
	cleanup func(context.Context) error
}
