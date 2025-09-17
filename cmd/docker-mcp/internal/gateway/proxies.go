package gateway

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/proxies"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/mcp"
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

func newClientWithCleanup(client mcp.Client, cleanup func(context.Context) error) mcp.Client {
	return &clientWithCleanup{
		Client:  client,
		cleanup: cleanup,
	}
}

func (c *clientWithCleanup) Close() error {
	return errors.Join(c.Client.Session().Close(), c.cleanup(context.TODO()))
}

type clientWithCleanup struct {
	mcp.Client
	cleanup func(context.Context) error
}
