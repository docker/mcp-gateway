package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/docker"
	mcpclient "github.com/docker/mcp-gateway/cmd/docker-mcp/internal/mcp"
)

func thisIsAnIntegrationTest(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test.")
	}
}

func runDockerMCP(t *testing.T, args ...string) string {
	t.Helper()
	args = append([]string{"mcp"}, args...)
	fmt.Println(args)
	cmd := exec.CommandContext(t.Context(), "docker", args...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	return string(out)
}

func writeFile(t *testing.T, parent, name string, content string) {
	t.Helper()
	path := filepath.Join(parent, name)
	require.NoError(t, os.MkdirAll(filepath.Base(parent), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestIntegrationVersion(t *testing.T) {
	thisIsAnIntegrationTest(t)
	out := runDockerMCP(t, "version")
	assert.NotEmpty(t, out)
}

func TestIntegrationCatalogLs(t *testing.T) {
	thisIsAnIntegrationTest(t)
	out := runDockerMCP(t, "catalog", "ls")
	assert.Contains(t, out, "docker-mcp: Docker MCP Catalog")
}

func TestIntegrationCatalogShow(t *testing.T) {
	thisIsAnIntegrationTest(t)
	out := runDockerMCP(t, "catalog", "show")
	assert.Contains(t, out, "playwright:")
}

func TestIntegrationDryRunEmpty(t *testing.T) {
	thisIsAnIntegrationTest(t)
	out := runDockerMCP(t, "gateway", "run", "--dry-run", "--servers=")
	assert.Contains(t, out, "Initialized in")
}

func TestIntegrationDryRunFetch(t *testing.T) {
	thisIsAnIntegrationTest(t)
	out := runDockerMCP(t, "gateway", "run", "--dry-run", "--servers=fetch", "--catalog="+catalog.DockerCatalogURL)
	assert.Contains(t, out, "fetch: (1 tools)")
	assert.Contains(t, out, "Initialized in")
}

func TestIntegrationCallToolClickhouse(t *testing.T) {
	thisIsAnIntegrationTest(t)
	tmp := t.TempDir()
	writeFile(t, tmp, ".env", "clickhouse.password=")
	writeFile(t, tmp, "config.yaml", "clickhouse:\n  host: sql-clickhouse.clickhouse.com\n  user: demo\n")

	gatewayArgs := []string{
		"--servers=clickhouse",
		"--secrets=" + filepath.Join(tmp, ".env"),
		"--config=" + filepath.Join(tmp, "config.yaml"),
		"--catalog=" + catalog.DockerCatalogURL,
	}

	out := runDockerMCP(t, "tools", "call", "--gateway-arg="+strings.Join(gatewayArgs, ","), "list_databases")
	assert.Contains(t, out, "amazon")
	assert.Contains(t, out, "bluesky")
	assert.Contains(t, out, "country")
}

func TestIntegrationCallToolDuckDuckDb(t *testing.T) {
	thisIsAnIntegrationTest(t)
	gatewayArgs := []string{
		"--servers=duckduckgo",
		"--catalog=" + catalog.DockerCatalogURL,
	}

	out := runDockerMCP(t, "tools", "call", "--gateway-arg="+strings.Join(gatewayArgs, ","), "search", "query=Docker")
	assert.Contains(t, out, "Found 10 search results")
}

// ----------------------------------------------------------------------------
// Long lived container tests
// ----------------------------------------------------------------------------

func createDockerClient(t *testing.T) docker.Client {
	t.Helper()

	dockerCli, err := command.NewDockerCli()
	require.NoError(t, err)

	clientOptions := flags.ClientOptions{
		Hosts:     []string{"unix:///var/run/docker.sock"},
		TLS:       false,
		TLSVerify: false,
	}

	err = dockerCli.Initialize(&clientOptions)
	require.NoError(t, err)

	dockerClient := docker.NewClient(dockerCli)

	return dockerClient
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
		c.Close()
	})

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "docker",
		Version: "1.0.0",
	}

	_, err := c.Initialize(t.Context(), initRequest, false)
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

	fmt.Println(args)
	c := newTestGatewayClient(t, args)

	response, err := c.CallTool(t.Context(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "get_current_time",
			Arguments: map[string]any{
				"timezone": "UTC",
			},
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

	fmt.Println(args)
	c := newTestGatewayClient(t, args)

	response, err := c.CallTool(t.Context(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "get_current_time",
			Arguments: map[string]any{
				"timezone": "UTC",
			},
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

	fmt.Println(args)
	c := newTestGatewayClient(t, args)

	response, err := c.CallTool(t.Context(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "get_current_time",
			Arguments: map[string]any{
				"timezone": "UTC",
			},
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

	fmt.Println(args)
	c := newTestGatewayClient(t, args)

	response, err := c.CallTool(t.Context(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "get_current_time",
			Arguments: map[string]any{
				"timezone": "UTC",
			},
		},
	})
	require.NoError(t, err)
	require.False(t, response.IsError)

	// Shutdown
	err = c.Close()
	require.NoError(t, err)

	waitForCondition(t, func() bool {
		containerID, err := dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
		return err == nil && containerID == ""
	})

	containerID, err := dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
	require.NoError(t, err)
	require.Empty(t, containerID)
}
