package proxies

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"

	"github.com/docker/mcp-gateway/pkg/docker"
	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/sliceutil"
)

const l4Image = "docker/mcp-l4proxy:v1@sha256:121b87decc25cda901dbd4ffbd20b116fffbd0fbeecc827c228fa45094a9934c"

// runL4Proxies takes a list of L4 proxies and starts an L4 proxy container for
// each hostname. It updates the target config with the container links to add
// to the MCP tool. It returns a list of proxy container names, and an error if
// any.
func runL4Proxies(ctx context.Context, cli docker.Client, target *TargetConfig, extNwName string, proxies []Proxy, keepCtrs bool) (proxyNames []string, retErr error) {
	if len(proxies) == 0 {
		return nil, nil
	}

	if err := cli.PullImage(ctx, l4Image); err != nil {
		return nil, fmt.Errorf("pulling image %s: %w", l4Image, err)
	}

	defer func() {
		if retErr != nil && !keepCtrs {
			for _, name := range proxyNames {
				if err := cli.RemoveContainer(ctx, name, true); err != nil {
					log.Logf("failed to remove proxy container %s: %v", name, err)
				}
			}
		}
	}()

	// Sort proxies to group them by hostname.
	slices.SortFunc(proxies, cmpProxies)

	// toProxy is a set of ports to proxy through a common L4 proxy, for a
	// particular hostname.
	var toProxy []uint16
	for i, proxy := range proxies {
		toProxy = append(toProxy, proxy.Port)

		if i < len(proxies)-1 && proxy.Hostname == proxies[i+1].Hostname {
			// Same hostname as next Proxy, continue collecting ports to start
			// a common L4 proxy.
			continue
		}

		proxyName := "docker-mcp-l4proxy-" + randString()
		if err := runL4Proxy(ctx, cli, proxyName, proxy.Hostname, target.NetworkName, extNwName, toProxy, keepCtrs); err != nil {
			return nil, fmt.Errorf("running l4 proxy %s: %w", proxyName, err)
		}

		target.Links = append(target.Links, proxyName+":"+proxy.Hostname)
		proxyNames = append(proxyNames, proxyName)

		// Next proxy will be for a different hostname, reset the list of ports.
		toProxy = nil
	}

	return proxyNames, nil
}

// runL4Proxy starts an L4 proxy container for a given hostname and a list of
// ports. It returns an error if the container fails to start.
func runL4Proxy(ctx context.Context, cli docker.Client, proxyName, hostname, intNwName, extNwName string, ports []uint16, keepCtrs bool) error {
	portsStr := strings.Join(sliceutil.Map(ports, func(p uint16) string {
		return strconv.Itoa(int(p))
	}), ",")

	log.Logf("starting l4 proxy %s for %s, ports %s", proxyName, hostname, portsStr)

	err := cli.StartContainer(ctx, proxyName,
		container.Config{
			Image: l4Image,
			Env: []string{
				"PROXY_HOSTNAME=" + hostname,
				"PROXY_PORTS=" + portsStr,
			},
			Labels: map[string]string{
				"docker-mcp":            "true",
				"docker-mcp-proxy":      "true",
				"docker-mcp-proxy-type": "l4",
			},
		},
		container.HostConfig{
			NetworkMode: container.NetworkMode(intNwName),
			AutoRemove:  !keepCtrs,
		},
		network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				intNwName: {},
				extNwName: {},
			},
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func cmpProxies(a, b Proxy) int {
	// First compare by hostname
	if a.Hostname < b.Hostname {
		return -1
	}
	if a.Hostname > b.Hostname {
		return 1
	}
	// If hostnames are equal, compare by port
	return int(a.Port - b.Port)
}
