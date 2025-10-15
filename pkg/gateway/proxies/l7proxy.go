package proxies

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"

	"github.com/docker/mcp-gateway/pkg/docker"
	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/sliceutil"
)

const l7Image = "docker/mcp-l7proxy:v1@sha256:ef8fd775fdf8ad060af897018c0db3c52229c493cfde437e86c754f3fcd59233"

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

	log.Logf("    - Starting l7 proxy %s for %s", proxyName, allowedHosts)

	err := cli.StartContainer(ctx, proxyName,
		container.Config{
			Image: l7Image,
			Env: []string{
				"ALLOWED_HOSTS=" + allowedHosts,
			},
			Labels: map[string]string{
				"docker-mcp":            "true",
				"docker-mcp-proxy":      "true",
				"docker-mcp-proxy-type": "l7",
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
	if err != nil {
		return "", err
	}

	return proxyName, nil
}
