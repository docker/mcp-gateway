package gateway

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/logs"
	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/mcp"
)

const image = "davidgageot135/http-proxy@sha256:e021c46e5201ab824b846c956252f13e4bb5612ca30bc51e3fbbdff5d5273db6"

func (g *Gateway) runProxySideCar(ctx context.Context, allowedHosts []string) (func(context.Context) error, string, error) {
	log("  - Running proxy sidecar for hosts", allowedHosts)

	if err := g.docker.PullImage(ctx, image); err != nil {
		return nil, "", fmt.Errorf("pulling image %s: %w", image, err)
	}

	// Start the proxy.
	// TODO: use the docker api
	ctxRun, cancel := context.WithCancel(ctx)
	name := "docker-mcp-proxy-" + randString(11)
	args := []string{"run", "--name", name, "--label", "docker-mcp=true", "-e", "ALLOWED_HOSTS"}
	if !g.KeepContainers {
		args = append(args, "--rm")
	}
	args = append(args, image)

	cmd := exec.CommandContext(ctxRun, "docker", args...)
	cmd.Env = []string{"ALLOWED_HOSTS=" + strings.Join(allowedHosts, ",")}
	cmd.Stderr = logs.NewPrefixer(os.Stderr, "  > http_proxy: ")
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, "", fmt.Errorf("starting http proxy: %w", err)
	}

	// Wait for the container to be started.
	if err := g.waitForContainer(ctx, name); err != nil {
		cancel()
		return nil, "", fmt.Errorf("waiting for proxy container %s to start: %w", name, err)
	}

	// Create internal network, for the MCP Servers.
	network := "docker-mcp-internal" + randString(11)
	if err := g.docker.CreateNetwork(ctx, network, true, map[string]string{"docker-mcp": "true"}); err != nil {
		cancel()
		return nil, "", fmt.Errorf("creating internal network %s: %w", network, err)
	}

	cleanup := func(ctx context.Context) error {
		cancel()
		return errors.Join(
			g.docker.RemoveNetwork(ctx, network),
		)
	}

	// Connect the proxy to the internal network, in addition to the default bridge network.
	if err := g.docker.ConnectNetwork(ctx, network, name, "proxy"); err != nil {
		_ = cleanup(ctx)
		return nil, "", fmt.Errorf("attaching proxy to internal network %s: %w", name, err)
	}

	return cleanup, network, nil
}

func (g *Gateway) waitForContainer(ctx context.Context, name string) error {
	var lastErr error
	for range 100 {
		ok, _, err := g.docker.ContainerExists(ctx, name)
		if ok {
			return nil
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

func randString(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}

	return string(b)
}

func newClientWithCleanup(client mcp.Client, cleanup func(context.Context) error) mcp.Client {
	return &clientWithCleanup{
		Client:  client,
		cleanup: cleanup,
	}
}

func (c *clientWithCleanup) Close() error {
	return errors.Join(c.Client.Close(), c.cleanup(context.TODO()))
}

type clientWithCleanup struct {
	mcp.Client
	cleanup func(context.Context) error
}
