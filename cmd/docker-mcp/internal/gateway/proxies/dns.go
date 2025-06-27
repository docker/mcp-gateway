package proxies

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/docker"
)

const dnsImage = "docker/mcp-dns-forwarder"

func runDNSForwarder(ctx context.Context, cli docker.Client, target *TargetConfig, extNwName string, keepCtrs bool) (_ string, _ io.ReadCloser, retErr error) {
	logf("running dns forwarder")

	if err := cli.PullImage(ctx, dnsImage); err != nil {
		return "", nil, fmt.Errorf("pulling image %s: %w", dnsImage, err)
	}

	ctrName := "docker-mcp-dns-forwarder-" + randString()
	// Remove the container if it fails to start (it might be left dangling
	// in "created" state gotherwise).
	defer func() {
		if retErr != nil && !keepCtrs {
			_ = cli.RemoveContainer(ctx, ctrName, true)
		}
	}()

	hostsEntries := map[string]string{} // proxyName -> hosts entries
	for _, link := range target.Links {
		parts := strings.Split(link, ":")
		proxyName := parts[0]
		// No need to re-resolve the proxy IP address if there's already an
		// entry in the hosts file.
		if v, ok := hostsEntries[proxyName]; ok {
			hostsEntries[proxyName] = v + " " + parts[1]
			continue
		}

		inspect, err := cli.InspectContainer(ctx, proxyName)
		if err != nil {
			return "", nil, fmt.Errorf("inspecting container %s: %w", proxyName, err)
		}

		hostsEntries[proxyName] = inspect.NetworkSettings.Networks[target.NetworkName].IPAddress + " " + parts[1]
	}

	if err := cli.StartContainer(ctx, ctrName,
		container.Config{
			Image: dnsImage,
			Env:   []string{"HOSTS_ENTRIES=" + strings.Join(slices.Collect(maps.Values(hostsEntries)), "\n")},
		},
		container.HostConfig{},
		network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				target.NetworkName: {},
				extNwName:          {},
			},
		},
	); err != nil {
		return "", nil, fmt.Errorf("starting container %s: %w", ctrName, err)
	}

	// We need to wait for the container to start before we can fetch its IP
	// address.
	if err := waitForContainer(ctx, cli, ctrName); err != nil {
		return "", nil, fmt.Errorf("waiting for container %s: %w", ctrName, err)
	}

	inspect, err := cli.InspectContainer(ctx, ctrName)
	if err != nil {
		return "", nil, fmt.Errorf("inspecting container %s: %w", ctrName, err)
	}
	target.DNS = inspect.NetworkSettings.Networks[target.NetworkName].IPAddress
	// All DNS queries need to be forwarded to the DNS forwarder, so remove
	// all links to proxies.
	target.Links = nil

	// Read logs with an uncancellable context otherwise logs might be lost.
	logReader, err := cli.ReadLogs(context.WithoutCancel(ctx), ctrName, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return "", nil, fmt.Errorf("reading logs for container %s: %w", ctrName, err)
	}

	go func() {
		scanner := bufio.NewScanner(logReader)
		for scanner.Scan() {
			log := scanner.Text()
			if strings.HasPrefix(log, "[INFO] REQ:") {
				logf("> dns forwarder: %s", strings.TrimPrefix(log, "[INFO] REQ: "))
			}
		}
	}()

	return ctrName, logReader, nil
}
