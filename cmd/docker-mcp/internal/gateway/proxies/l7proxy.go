package proxies

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/docker"
	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/sliceutil"
)

const l7Image = "docker/mcp-http-proxy:v1@sha256:d57daf14d7097ff7e59740311d27c482e560cff2f35f8aff1d5a7cc3b5289b18"

// runL7Proxy starts a single L7 proxy for all the allowed hosts. It returns
// the proxy container name and a list of links to add to the MCP tool.
func runL7Proxy(ctx context.Context, cli docker.Client, target *TargetConfig, extNwName string, proxies []Proxy, keepCtrs bool) (string, error) {
	if len(proxies) == 0 {
		return "", nil
	}

	if err := cli.PullImage(ctx, l7Image); err != nil {
		return "", fmt.Errorf("pulling image %s: %w", l7Image, err)
	}

	proxyName := "docker-mcp-l7proxy-" + randString()
	allowedHosts := strings.Join(sliceutil.Map(proxies, func(p Proxy) string {
		return net.JoinHostPort(p.Hostname, strconv.Itoa(int(p.Port)))
	}), ",")

	target.Links = append(target.Links, sliceutil.Map(proxies, func(p Proxy) string {
		return proxyName + ":" + p.Hostname
	})...)
	target.Env = append(target.Env, "http_proxy="+proxyName+":8080", "https_proxy="+proxyName+":8080")

	logf("starting l7 proxy %s for %s", proxyName, allowedHosts)

	return proxyName, cli.StartContainer(ctx, proxyName,
		container.Config{
			Image: l7Image,
			Env: []string{
				"ALLOWED_HOSTS=" + allowedHosts,
			},
			Labels: map[string]string{
				"docker-mcp": "true",
			},
		},
		container.HostConfig{
			NetworkMode: container.NetworkMode(target.NetworkName),
			AutoRemove:  !keepCtrs,
		},
		network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				target.NetworkName: {},
				extNwName:          {},
			},
		},
	)
}
