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

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/docker"
	"github.com/docker/mcp-gateway/pkg/gateway/proxies"
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
		"grafana.api_key": "se://docker/mcp/grafana.api_key",
	}

	args, env := argsAndEnv(t, "grafana", catalogYAML, configYAML, secrets)

	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"-l", "docker-mcp=true", "-l", "docker-mcp-tool-type=mcp", "-l", "docker-mcp-name=grafana", "-l", "docker-mcp-transport=stdio",
		"-e", "GRAFANA_API_KEY", "-e", "GRAFANA_URL",
	}, args)
	assert.Equal(t, []string{"GRAFANA_API_KEY=se://docker/mcp/grafana.api_key", "GRAFANA_URL=TEST"}, env)
}

func TestApplyConfigMongoDB(t *testing.T) {
	catalogYAML := `
secrets:
  - name: mongodb.connection_string
    env: MDB_MCP_CONNECTION_STRING
  `
	secrets := map[string]string{
		"mongodb.connection_string": "se://docker/mcp/mongodb.connection_string",
	}

	args, env := argsAndEnv(t, "mongodb", catalogYAML, "", secrets)

	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"-l", "docker-mcp=true", "-l", "docker-mcp-tool-type=mcp", "-l", "docker-mcp-name=mongodb", "-l", "docker-mcp-transport=stdio",
		"-e", "MDB_MCP_CONNECTION_STRING",
	}, args)
	assert.Equal(t, []string{"MDB_MCP_CONNECTION_STRING=se://docker/mcp/mongodb.connection_string"}, env)
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
		"notion.internal_integration_token": "se://docker/mcp/notion.internal_integration_token",
	}

	args, env := argsAndEnv(t, "notion", catalogYAML, "", secrets)

	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"-l", "docker-mcp=true", "-l", "docker-mcp-tool-type=mcp", "-l", "docker-mcp-name=notion", "-l", "docker-mcp-transport=stdio",
		"-e", "INTERNAL_INTEGRATION_TOKEN", "-e", "OPENAPI_MCP_HEADERS",
	}, args)
	assert.Equal(t, []string{"INTERNAL_INTEGRATION_TOKEN=se://docker/mcp/notion.internal_integration_token", `OPENAPI_MCP_HEADERS={"Authorization": "Bearer se://docker/mcp/notion.internal_integration_token", "Notion-Version": "2022-06-28"}`}, env)
}

func TestApplyConfigFileBasedSecrets(t *testing.T) {
	// Test that file-based secrets (actual values) pass through correctly
	catalogYAML := `
secrets:
  - name: db.password
    env: DB_PASSWORD
  - name: api.key
    env: API_KEY
`
	// File-based mode: secrets map contains actual values (not se:// URIs)
	secrets := map[string]string{
		"db.password": "my-actual-db-password",
		"api.key":     "my-actual-api-key",
	}

	args, env := argsAndEnv(t, "myserver", catalogYAML, "", secrets)

	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"-l", "docker-mcp=true", "-l", "docker-mcp-tool-type=mcp", "-l", "docker-mcp-name=myserver", "-l", "docker-mcp-transport=stdio",
		"-e", "DB_PASSWORD", "-e", "API_KEY",
	}, args)
	// File-based mode: actual values pass through unchanged
	assert.Equal(t, []string{"DB_PASSWORD=my-actual-db-password", "API_KEY=my-actual-api-key"}, env)
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

	args, env := argsAndEnv(t, "hub", catalogYAML, configYAML, nil)

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

	args, env := argsAndEnv(t, "hub", catalogYAML, "", nil)

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

	args, env := argsAndEnv(t, "hub", catalogYAML, configYAML, nil)

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

	args, env := argsAndEnv(t, "svc", catalogYAML, "", nil)

	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"-l", "docker-mcp=true", "-l", "docker-mcp-tool-type=mcp", "-l", "docker-mcp-name=svc", "-l", "docker-mcp-transport=stdio",
		"-u", "1001:2002",
	}, args)
	assert.Empty(t, env)
}

func TestApplyConfigExtraHosts(t *testing.T) {
	catalogYAML := `
description: Playwright MCP server.
title: Playwright
type: server
longLived: true
image: mcp/playwright@sha256:53da89d1da3dfbb61c10f707c1713cfee1f870f7fba5334e126c6c765e37db56
extraHosts:
  - "myhost:192.168.1.100"
  - "anotherhost:10.0.0.1"
  `

	args, env := argsAndEnv(t, "playwright", catalogYAML, "", nil)

	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"-l", "docker-mcp=true", "-l", "docker-mcp-tool-type=mcp", "-l", "docker-mcp-name=playwright", "-l", "docker-mcp-transport=stdio",
		"--add-host", "myhost:192.168.1.100",
		"--add-host", "anotherhost:10.0.0.1",
	}, args)
	assert.Empty(t, env)
}

func TestApplyConfigLongLivedIgnoresReadOnly(t *testing.T) {
	catalogYAML := `
longLived: true
volumes:
  - '/local/data:/data'
  `

	args, env := argsAndEnv(t, "longlived", catalogYAML, "", nil)

	// Volumes should NOT have :ro appended regardless of readOnly flag
	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"-l", "docker-mcp=true", "-l", "docker-mcp-tool-type=mcp", "-l", "docker-mcp-name=longlived", "-l", "docker-mcp-transport=stdio",
		"-v", "/local/data:/data",
	}, args)
	assert.Empty(t, env)
}

func TestApplyConfigShortLivedRespectsReadOnly(t *testing.T) {
	catalogYAML := `
volumes:
  - '/local/data:/data'
  `

	args, env := argsAndEnv(t, "shortlived", catalogYAML, "", nil)

	// Short-lived servers no longer apply read-only mounts automatically
	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"-l", "docker-mcp=true", "-l", "docker-mcp-tool-type=mcp", "-l", "docker-mcp-name=shortlived", "-l", "docker-mcp-transport=stdio",
		"-v", "/local/data:/data",
	}, args)
	assert.Empty(t, env)
}

func argsAndEnv(t *testing.T, name, catalogYAML, configYAML string, secrets map[string]string) ([]string, []string) {
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
	}, proxies.TargetConfig{})
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
	}, dockerClient, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test client acquisition and initialization
	client, err := clientPool.AcquireClient(ctx, &serverConfig, &clientConfig{})
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

func TestGlobalDisableNetwork(t *testing.T) {
	catalogYAML := `
command:
  - --transport=stdio
`

	// Test that global DisableNetwork=true adds --network none
	args, env := argsAndEnvWithGlobalDisableNetwork(t, catalogYAML, "", nil, nil, true)

	assert.Contains(t, args, "--network")
	assert.Contains(t, args, "none")
	assert.Empty(t, env)
}

func TestGlobalDisableNetworkOverridesServerSetting(t *testing.T) {
	// Server has disableNetwork: false, but global flag should override
	catalogYAML := `
command:
  - --transport=stdio
disableNetwork: false
`

	// Test that global DisableNetwork=true overrides server setting
	args, env := argsAndEnvWithGlobalDisableNetwork(t, catalogYAML, "", nil, nil, true)

	assert.Contains(t, args, "--network")
	assert.Contains(t, args, "none")
	assert.Empty(t, env)
}

func TestServerDisableNetworkWhenGlobalFalse(t *testing.T) {
	// Server has disableNetwork: true, global is false - should still disable network
	catalogYAML := `
command:
  - --transport=stdio
disableNetwork: true
`

	// Test that server DisableNetwork=true works when global is false
	args, env := argsAndEnvWithGlobalDisableNetwork(t, catalogYAML, "", nil, nil, false)

	assert.Contains(t, args, "--network")
	assert.Contains(t, args, "none")
	assert.Empty(t, env)
}

func TestNoNetworkDisabledWhenBothFalse(t *testing.T) {
	// Both server and global disable network are false - should enable networks
	catalogYAML := `
command:
  - --transport=stdio
disableNetwork: false
`

	// Test that when both global and server DisableNetwork are false, networks are enabled
	args, env := argsAndEnvWithGlobalDisableNetwork(t, catalogYAML, "", nil, nil, false)

	// Should not contain --network none
	networkNoneFound := false
	for i, arg := range args {
		if arg == "--network" && i+1 < len(args) && args[i+1] == "none" {
			networkNoneFound = true
			break
		}
	}
	assert.False(t, networkNoneFound, "Should not disable network when both global and server settings are false")
	assert.Empty(t, env)
}

func argsAndEnvWithGlobalDisableNetwork(t *testing.T, catalogYAML, configYAML string, secrets map[string]string, readOnly *bool, globalDisableNetwork bool) ([]string, []string) {
	t.Helper()

	clientPool := &clientPool{
		Options: Options{
			Cpus:           1,
			Memory:         "2Gb",
			DisableNetwork: globalDisableNetwork,
		},
	}
	return clientPool.argsAndEnv(&catalog.ServerConfig{
		Name:    "test-server",
		Spec:    parseSpec(t, catalogYAML),
		Config:  parseConfig(t, configYAML),
		Secrets: secrets,
	}, readOnly, proxies.TargetConfig{})
}
