package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/docker"
	mcpclient "github.com/docker/mcp-gateway/cmd/docker-mcp/internal/mcp"
)

func createDockerClient(t *testing.T) docker.Client {
	t.Helper()

	dockerCli, err := command.NewDockerCli()
	require.NoError(t, err)

	err = dockerCli.Initialize(&flags.ClientOptions{
		Hosts:     []string{"unix:///var/run/docker.sock"},
		TLS:       false,
		TLSVerify: false,
	})
	require.NoError(t, err)

	return docker.NewClient(dockerCli)
}

func waitForCondition(t *testing.T, condition func() bool) {
	t.Helper()

	timeoutCtx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	for {
		if condition() {
			return
		}

		select {
		case <-timeoutCtx.Done():
			return
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func newTestGatewayClient(t *testing.T, args []string) mcpclient.Client {
	t.Helper()

	c := mcpclient.NewStdioCmdClient("mcp-test", "docker", os.Environ(), args...)
	t.Cleanup(func() {
		c.Session().Close()
	})

	initParams := &mcp.InitializeParams{
		ProtocolVersion: "2024-11-05",
		ClientInfo: &mcp.Implementation{
			Name:    "docker",
			Version: "1.0.0",
		},
	}

	err := c.Initialize(t.Context(), initParams, false, nil, nil)
	require.NoError(t, err)

	return c
}

func TestIntegrationShortLivedContainerCloses(t *testing.T) {
	thisIsAnIntegrationTest(t)

	dockerClient := createDockerClient(t)
	tmp := t.TempDir()
	writeFile(t, tmp, "catalog.yaml", "name: docker-test\nregistry:\n  time:\n    image: mcp/time@sha256:9c46a918633fb474bf8035e3ee90ebac6bcf2b18ccb00679ac4c179cba0ebfcf")

	args := []string{
		"mcp",
		"gateway",
		"run",
		"--catalog=" + filepath.Join(tmp, "catalog.yaml"),
		"--servers=time",
	}

	c := newTestGatewayClient(t, args)

	response, err := c.Session().CallTool(t.Context(), &mcp.CallToolParams{
		Name: "get_current_time",
		Arguments: map[string]any{
			"timezone": "UTC",
		},
	})
	require.NoError(t, err)
	require.False(t, response.IsError)

	waitForCondition(t, func() bool {
		containerID, err := dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
		return err == nil && containerID == ""
	})

	containerID, err := dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
	require.NoError(t, err)
	require.Empty(t, containerID)
}

func TestIntegrationLongLivedServerStaysRunning(t *testing.T) {
	thisIsAnIntegrationTest(t)

	dockerClient := createDockerClient(t)
	tmp := t.TempDir()
	writeFile(t, tmp, "catalog.yaml", "name: docker-test\nregistry:\n  time:\n    longLived: true\n    image: mcp/time@sha256:9c46a918633fb474bf8035e3ee90ebac6bcf2b18ccb00679ac4c179cba0ebfcf")

	args := []string{
		"mcp",
		"gateway",
		"run",
		"--catalog=" + filepath.Join(tmp, "catalog.yaml"),
		"--servers=time",
	}

	c := newTestGatewayClient(t, args)

	response, err := c.Session().CallTool(t.Context(), &mcp.CallToolParams{
		Name: "get_current_time",
		Arguments: map[string]any{
			"timezone": "UTC",
		},
	})
	require.NoError(t, err)
	require.False(t, response.IsError)

	// Not great, but at least if it's going to try to shut down the container falsely, this test should normally fail with the short wait added.
	time.Sleep(3 * time.Second)

	containerID, err := dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
	require.NoError(t, err)
	require.NotEmpty(t, containerID)
}

func TestIntegrationLongLivedServerWithFlagStaysRunning(t *testing.T) {
	thisIsAnIntegrationTest(t)

	dockerClient := createDockerClient(t)
	tmp := t.TempDir()
	writeFile(t, tmp, "catalog.yaml", "name: docker-test\nregistry:\n  time:\n    image: mcp/time@sha256:9c46a918633fb474bf8035e3ee90ebac6bcf2b18ccb00679ac4c179cba0ebfcf")

	args := []string{
		"mcp",
		"gateway",
		"run",
		"--catalog=" + filepath.Join(tmp, "catalog.yaml"),
		"--servers=time",
		"--long-lived",
	}

	c := newTestGatewayClient(t, args)

	response, err := c.Session().CallTool(t.Context(), &mcp.CallToolParams{
		Name: "get_current_time",
		Arguments: map[string]any{
			"timezone": "UTC",
		},
	})
	require.NoError(t, err)
	require.False(t, response.IsError)

	// Not great, but at least if it's going to try to shut down the container falsely, this test should normally fail with the short wait added.
	time.Sleep(3 * time.Second)

	containerID, err := dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
	require.NoError(t, err)
	require.NotEmpty(t, containerID)
}

func TestIntegrationLongLivedShouldCleanupContainerBeforeShutdown(t *testing.T) {
	thisIsAnIntegrationTest(t)

	dockerClient := createDockerClient(t)
	tmp := t.TempDir()
	writeFile(t, tmp, "catalog.yaml", "name: docker-test\nregistry:\n  time:\n    image: mcp/time@sha256:9c46a918633fb474bf8035e3ee90ebac6bcf2b18ccb00679ac4c179cba0ebfcf")

	args := []string{
		"mcp",
		"gateway",
		"run",
		"--catalog=" + filepath.Join(tmp, "catalog.yaml"),
		"--servers=time",
		"--long-lived",
	}

	c := newTestGatewayClient(t, args)

	response, err := c.Session().CallTool(t.Context(), &mcp.CallToolParams{
		Name: "get_current_time",
		Arguments: map[string]any{
			"timezone": "UTC",
		},
	})
	require.NoError(t, err)
	require.False(t, response.IsError)

	// Shutdown
	err = c.Session().Close()
	require.NoError(t, err)

	waitForCondition(t, func() bool {
		containerID, err := dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
		return err == nil && containerID == ""
	})

	containerID, err := dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
	require.NoError(t, err)
	require.Empty(t, containerID)
}
