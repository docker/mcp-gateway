package proxies

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/docker"
	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/sliceutil"
)

// TargetConfig represents the config that should be set on a target container
// to get all its traffic proxied through L4/L7 proxies.
type TargetConfig struct {
	NetworkName string
	Links       []string
	Env         []string
}

// RunNetworkProxies starts a set of Proxy and returns a TargetConfig that
// should be applied to a target container to get all its traffic proxied, a
// cleanup function to remove the network and proxies, and an error if any.
func RunNetworkProxies(ctx context.Context, cli docker.Client, proxies []Proxy, keepCtrs bool) (_ TargetConfig, _ func(context.Context) error, retErr error) {
	if len(proxies) == 0 {
		return TargetConfig{}, nil, nil
	}

	log("  - Running proxy sidecars for hosts", proxies)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create a network for connecting the MCP tool with proxies.
	target := TargetConfig{
		NetworkName: "docker-mcp-proxies-" + randString(),
	}
	if err := cli.CreateNetwork(ctx, target.NetworkName, true, map[string]string{"docker-mcp": "true"}); err != nil {
		return TargetConfig{}, nil, fmt.Errorf("creating internal network: %w", err)
	}
	defer func() {
		if retErr != nil {
			if err := cli.RemoveNetwork(ctx, target.NetworkName); err != nil {
				logf("failed to remove network %s: %v", target.NetworkName, err)
			}
		}
	}()

	// Create a network for proxies to connect to external services.
	extNwName := "docker-mcp-proxies-" + randString()
	if err := cli.CreateNetwork(ctx, extNwName, false, map[string]string{"docker-mcp": "true"}); err != nil {
		return TargetConfig{}, nil, fmt.Errorf("creating internal network: %w", err)
	}
	defer func() {
		if retErr != nil {
			if err := cli.RemoveNetwork(ctx, extNwName); err != nil {
				logf("failed to remove network %s: %v", extNwName, err)
			}
		}
	}()

	var proxyNames []string
	var err error

	// Start L4 proxies.
	l4Proxies := sliceutil.Filter(proxies, func(p Proxy) bool { return p.Protocol == TCP })
	proxyNames, err = runL4Proxies(ctx, cli, &target, extNwName, l4Proxies, keepCtrs)
	if err != nil {
		return TargetConfig{}, nil, fmt.Errorf("running l4 proxies: %w", err)
	}

	// Cleanup running proxies if anything goes wrong beyond that point. (Note
	// that there's no dedicated defer for l7proxy since l7ProxyName is appended
	// to proxyNames once it's started -- so it'll be cleaned up automatically
	// by this defer.)
	defer func() {
		if retErr != nil && !keepCtrs {
			for _, name := range proxyNames {
				if err := cli.RemoveContainer(ctx, name, true); err != nil {
					logf("failed to remove proxy container %s: %v", name, err)
				}
			}
		}
	}()

	// Start L7 proxy.
	l7Proxies := sliceutil.Filter(proxies, func(p Proxy) bool { return p.Protocol == HTTP })
	l7ProxyName, err := runL7Proxy(ctx, cli, &target, extNwName, l7Proxies, keepCtrs)
	if err != nil {
		return TargetConfig{}, nil, fmt.Errorf("running l7 proxy: %w", err)
	}

	if l7ProxyName != "" {
		proxyNames = append(proxyNames, l7ProxyName)
	}

	// Make sure all proxies are running.
	g, groupCtx := errgroup.WithContext(ctx)
	for _, name := range proxyNames {
		g.Go(func() error {
			return waitForContainer(groupCtx, cli, name)
		})
	}

	if err := g.Wait(); err != nil {
		return TargetConfig{}, nil, fmt.Errorf("waiting for proxies to start: %w", err)
	}

	// Cleanup function to remove the network and proxies.
	cleanup := func(ctx context.Context) error {
		if keepCtrs {
			return shutdownProxies(ctx, cli, proxyNames)
		}
		return removeProxies(ctx, cli, target.NetworkName, proxyNames)
	}

	return target, cleanup, nil
}

func shutdownProxies(ctx context.Context, cli docker.Client, proxyNames []string) error {
	logf("shutting down proxies (%s)", proxyNames)

	var errs []error
	for _, name := range proxyNames {
		if err := cli.StopContainer(ctx, name, 1); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop proxy container %s: %w", name, err))
		}
	}

	return errors.Join(errs...)
}

func removeProxies(ctx context.Context, cli docker.Client, nwName string, proxyNames []string) error {
	logf("removing proxies (%s) and network (%s)", proxyNames, nwName)

	var errs []error
	for _, name := range proxyNames {
		if err := cli.RemoveContainer(ctx, name, true); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove proxy container %s: %w", name, err))
		}
	}
	if err := cli.RemoveNetwork(ctx, nwName); err != nil {
		errs = append(errs, fmt.Errorf("failed to remove network %s: %w", nwName, err))
	}

	return errors.Join(errs...)
}

func randString() string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, 11)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}

	return string(b)
}

func waitForContainer(ctx context.Context, cli docker.Client, name string) error {
	var lastErr error
	for range 100 {
		ok, inspect, err := cli.ContainerExists(ctx, name)
		if ok {
			if inspect.State.Running {
				return nil
			}
			err = fmt.Errorf("container %s not running", name)
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	return lastErr
}
