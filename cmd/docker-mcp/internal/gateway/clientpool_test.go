package gateway

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/docker/cli/cli/command"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/docker"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/proxies"
)

func TestApplyConfigGrafana(t *testing.T) {
	catalogYAML := `
command:
  - --transport=stdio
secrets:
  - name: grafana.api_key
    env: GRAFANA_API_KEY
env:
  - name: GRAFANA_URL
    value: '{{grafana.url}}'
`
	configYAML := `
grafana:
  url: TEST
`
	secrets := map[string]string{
		"grafana.api_key": "API_KEY",
	}

	args, env := argsAndEnv(t, "grafana", catalogYAML, configYAML, secrets, nil)

	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"-l", "docker-mcp=true", "-l", "docker-mcp-tool-type=mcp", "-l", "docker-mcp-name=grafana", "-l", "docker-mcp-transport=stdio",
		"-e", "GRAFANA_API_KEY", "-e", "GRAFANA_URL",
	}, args)
	assert.Equal(t, []string{"GRAFANA_API_KEY=API_KEY", "GRAFANA_URL=TEST"}, env)
}

func TestApplyConfigMongoDB(t *testing.T) {
	catalogYAML := `
secrets:
  - name: mongodb.connection_string
    env: MDB_MCP_CONNECTION_STRING
  `
	secrets := map[string]string{
		"mongodb.connection_string": "HOST:PORT",
	}

	args, env := argsAndEnv(t, "mongodb", catalogYAML, "", secrets, nil)

	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"-l", "docker-mcp=true", "-l", "docker-mcp-tool-type=mcp", "-l", "docker-mcp-name=mongodb", "-l", "docker-mcp-transport=stdio",
		"-e", "MDB_MCP_CONNECTION_STRING",
	}, args)
	assert.Equal(t, []string{"MDB_MCP_CONNECTION_STRING=HOST:PORT"}, env)
}

func TestApplyConfigNotion(t *testing.T) {
	catalogYAML := `
secrets:
  - name: notion.internal_integration_token
    env: INTERNAL_INTEGRATION_TOKEN
    example: ntn_****
env:
  - name: OPENAPI_MCP_HEADERS
    value: '{"Authorization": "Bearer $INTERNAL_INTEGRATION_TOKEN", "Notion-Version": "2022-06-28"}'
  `
	secrets := map[string]string{
		"notion.internal_integration_token": "ntn_DUMMY",
	}

	args, env := argsAndEnv(t, "notion", catalogYAML, "", secrets, nil)

	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"-l", "docker-mcp=true", "-l", "docker-mcp-tool-type=mcp", "-l", "docker-mcp-name=notion", "-l", "docker-mcp-transport=stdio",
		"-e", "INTERNAL_INTEGRATION_TOKEN", "-e", "OPENAPI_MCP_HEADERS",
	}, args)
	assert.Equal(t, []string{"INTERNAL_INTEGRATION_TOKEN=ntn_DUMMY", `OPENAPI_MCP_HEADERS={"Authorization": "Bearer ntn_DUMMY", "Notion-Version": "2022-06-28"}`}, env)
}

func TestApplyConfigMountAs(t *testing.T) {
	catalogYAML := `
volumes:
  - '{{hub.log_path|mount_as:/logs:ro}}'
  `
	configYAML := `
hub:
  log_path: /local/logs
`

	args, env := argsAndEnv(t, "hub", catalogYAML, configYAML, nil, nil)

	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"-l", "docker-mcp=true", "-l", "docker-mcp-tool-type=mcp", "-l", "docker-mcp-name=hub", "-l", "docker-mcp-transport=stdio",
		"-v", "/local/logs:/logs:ro",
	}, args)
	assert.Empty(t, env)
}

func TestApplyConfigEmptyMountAs(t *testing.T) {
	catalogYAML := `
volumes:
  - '{{hub.log_path|mount_as:/logs:ro}}'
  `

	args, env := argsAndEnv(t, "hub", catalogYAML, "", nil, nil)

	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"-l", "docker-mcp=true", "-l", "docker-mcp-tool-type=mcp", "-l", "docker-mcp-name=hub", "-l", "docker-mcp-transport=stdio",
	}, args)
	assert.Empty(t, env)
}

func TestApplyConfigMountAsReadOnly(t *testing.T) {
	catalogYAML := `
volumes:
  - '{{hub.log_path|mount_as:/logs:ro}}'
  `
	configYAML := `
hub:
  log_path: /local/logs
`

	args, env := argsAndEnv(t, "hub", catalogYAML, configYAML, nil, readOnly())

	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"-l", "docker-mcp=true", "-l", "docker-mcp-tool-type=mcp", "-l", "docker-mcp-name=hub", "-l", "docker-mcp-transport=stdio",
		"-v", "/local/logs:/logs:ro",
	}, args)
	assert.Empty(t, env)
}

func TestApplyConfigUser(t *testing.T) {
	catalogYAML := `
user: "1001:2002"
  `

	args, env := argsAndEnv(t, "svc", catalogYAML, "", nil, nil)

	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"-l", "docker-mcp=true", "-l", "docker-mcp-tool-type=mcp", "-l", "docker-mcp-name=svc", "-l", "docker-mcp-transport=stdio",
		"-u", "1001:2002",
	}, args)
	assert.Empty(t, env)
}

func argsAndEnv(t *testing.T, name, catalogYAML, configYAML string, secrets map[string]string, readOnly *bool) ([]string, []string) {
	t.Helper()

	clientPool := &clientPool{
		Options: Options{
			Cpus:   1,
			Memory: "2Gb",
		},
	}
	return clientPool.argsAndEnv(&catalog.ServerConfig{
		Name:    name,
		Spec:    parseSpec(t, catalogYAML),
		Config:  parseConfig(t, configYAML),
		Secrets: secrets,
	}, readOnly, proxies.TargetConfig{})
}

func parseSpec(t *testing.T, contentYAML string) catalog.Server {
	t.Helper()
	var spec catalog.Server
	err := yaml.Unmarshal([]byte(contentYAML), &spec)
	require.NoError(t, err)
	return spec
}

func parseConfig(t *testing.T, contentYAML string) map[string]any {
	t.Helper()
	var config map[string]any
	err := yaml.Unmarshal([]byte(contentYAML), &config)
	require.NoError(t, err)
	return config
}

func readOnly() *bool {
	return boolPtr(true)
}

func boolPtr(b bool) *bool {
	return &b
}

func TestStdioClientInitialization(t *testing.T) {
	// This is an integration test that requires Docker
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Also skip if INTEGRATION_TEST env var is not set
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test - set INTEGRATION_TEST=1 to run")
	}

	serverConfig := catalog.ServerConfig{
		Name: "test-server",
		Spec: catalog.Server{
			Image:   "mcp/brave-search@sha256:e13f4693a3421e2b316c8b6196c5c543c77281f9d8938850681e3613bba95115", // User should provide their image
			Command: []string{},
			Env:     []catalog.Env{{Name: "BRAVE_API_KEY", Value: "test_key"}},
		},
		Config:  map[string]any{},
		Secrets: map[string]string{},
	}

	// Create a real Docker CLI client
	dockerCli, err := command.NewDockerCli()
	require.NoError(t, err)

	dockerClient := docker.NewClient(dockerCli)
	clientPool := newClientPool(Options{
		Cpus:   1,
		Memory: "512m",
	}, dockerClient)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test client acquisition and initialization
	client, err := clientPool.AcquireClient(ctx, &serverConfig, &clientConfig{readOnly: boolPtr(false)})
	if err != nil {
		t.Fatalf("Failed to acquire client: %v", err)
	}
	defer clientPool.ReleaseClient(client)

	// Test ListTools to verify the client is working
	tools, err := client.Session().ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("Failed to list tools: %v", err)
	}

	// Basic assertions - user can customize based on expected behavior
	assert.NotNil(t, tools)
	assert.NotNil(t, tools.Tools)

	t.Logf("Successfully initialized stdio client and retrieved %d tools", len(tools.Tools))
}
