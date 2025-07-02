package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/catalog"
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
