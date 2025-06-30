package gateway

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"

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

	args, env := argsAndEnv(t, "grafana", catalogYAML, configYAML, secrets)

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

	args, env := argsAndEnv(t, "mongodb", catalogYAML, "", secrets)

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

	args, env := argsAndEnv(t, "notion", catalogYAML, "", secrets)

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

func argsAndEnv(t *testing.T, name, catalogYAML, configYAML string, secrets map[string]string) ([]string, []string) {
	t.Helper()

	clientPool := &clientPool{
		Options: Options{
			Cpus:   1,
			Memory: "2Gb",
		},
	}
	return clientPool.argsAndEnv(ServerConfig{
		Name:    name,
		Spec:    parseSpec(t, catalogYAML),
		Config:  parseConfig(t, configYAML),
		Secrets: secrets,
	}, nil, proxies.TargetConfig{})
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

var pullTimeImageOnce sync.Once

func pullTimeImage(t *testing.T) {
	t.Helper()
	pullTimeImageOnce.Do(func() {
		dockerClient := createDockerClient(t)
		err := dockerClient.PullImage(t.Context(), "mcp/time@sha256:9c46a918633fb474bf8035e3ee90ebac6bcf2b18ccb00679ac4c179cba0ebfcf")
		require.NoError(t, err)
	})
}

func TestClientClosesShortLivedContainer(t *testing.T) {
	dockerClient := createDockerClient(t)

	pullTimeImage(t)

	clientPool := newClientPool(Options{
		Cpus:   1,
		Memory: "256Mb",
	}, dockerClient)

	client, err := clientPool.AcquireClient(t.Context(), ServerConfig{
		Name: "time",
		Spec: catalog.Server{
			Image: "mcp/time@sha256:9c46a918633fb474bf8035e3ee90ebac6bcf2b18ccb00679ac4c179cba0ebfcf",
		},
	}, nil)

	require.NoError(t, err)
	require.NotNil(t, client)

	toolResult, err := client.ListTools(t.Context(), mcp.ListToolsRequest{})
	require.NoError(t, err)
	require.NotNil(t, toolResult)

	containerID, err := dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
	require.NoError(t, err)
	require.NotEmpty(t, containerID)

	// Releasing client will remove it
	clientPool.ReleaseClient(client)

	waitForCondition(t, func() bool {
		containerID, err = dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
		return err == nil && containerID == ""
	})

	containerID, err = dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
	require.NoError(t, err)
	require.Empty(t, containerID)

	clientPool.Close()
}

func TestClientLeavesSingletonRunning(t *testing.T) {
	dockerClient := createDockerClient(t)

	pullTimeImage(t)

	clientPool := newClientPool(Options{
		Cpus:   1,
		Memory: "256Mb",
	}, dockerClient)

	client, err := clientPool.AcquireClient(t.Context(), ServerConfig{
		Name: "time",
		Spec: catalog.Server{
			Image:     "mcp/time@sha256:9c46a918633fb474bf8035e3ee90ebac6bcf2b18ccb00679ac4c179cba0ebfcf",
			Singleton: true,
		},
	}, nil)

	require.NoError(t, err)
	require.NotNil(t, client)

	toolResult, err := client.ListTools(t.Context(), mcp.ListToolsRequest{})
	require.NoError(t, err)
	require.NotNil(t, toolResult)

	containerID, err := dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
	require.NoError(t, err)
	require.NotEmpty(t, containerID)

	// Releasing client won't remove it
	clientPool.ReleaseClient(client)

	// Condition will never be true, that's ok
	waitForCondition(t, func() bool {
		containerID, err = dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
		return err == nil && containerID == ""
	})

	containerID, err = dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
	require.NoError(t, err)
	require.NotEmpty(t, containerID)

	clientPool.Close()

	// Condition should become true now that it's closed
	waitForCondition(t, func() bool {
		containerID, err = dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
		return err == nil && containerID == ""
	})

	containerID, err = dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
	require.NoError(t, err)
	require.Empty(t, containerID)
}

func TestClientLeavesRunningWithSingletonFlag(t *testing.T) {
	dockerClient := createDockerClient(t)

	pullTimeImage(t)

	clientPool := newClientPool(Options{
		Cpus:          1,
		Memory:        "256Mb",
		AllSingletons: true,
	}, dockerClient)

	client, err := clientPool.AcquireClient(t.Context(), ServerConfig{
		Name: "time",
		Spec: catalog.Server{
			Image: "mcp/time@sha256:9c46a918633fb474bf8035e3ee90ebac6bcf2b18ccb00679ac4c179cba0ebfcf",
		},
	}, nil)

	require.NoError(t, err)
	require.NotNil(t, client)

	toolResult, err := client.ListTools(t.Context(), mcp.ListToolsRequest{})
	require.NoError(t, err)
	require.NotNil(t, toolResult)

	containerID, err := dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
	require.NoError(t, err)
	require.NotEmpty(t, containerID)

	// Releasing client won't remove it
	clientPool.ReleaseClient(client)

	// Condition will never be true, that's ok
	waitForCondition(t, func() bool {
		containerID, err = dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
		return err == nil && containerID == ""
	})

	containerID, err = dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
	require.NoError(t, err)
	require.NotEmpty(t, containerID)

	clientPool.Close()

	// Condition should become true now that it's closed
	waitForCondition(t, func() bool {
		containerID, err = dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
		return err == nil && containerID == ""
	})

	containerID, err = dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
	require.NoError(t, err)
	require.Empty(t, containerID)
}

func TestClientReusesRunningSingleton(t *testing.T) {
	dockerClient := createDockerClient(t)

	pullTimeImage(t)

	clientPool := newClientPool(Options{
		Cpus:   1,
		Memory: "256Mb",
	}, dockerClient)

	config := ServerConfig{
		Name: "time",
		Spec: catalog.Server{
			Image:     "mcp/time@sha256:9c46a918633fb474bf8035e3ee90ebac6bcf2b18ccb00679ac4c179cba0ebfcf",
			Singleton: true,
		},
	}

	client, err := clientPool.AcquireClient(t.Context(), config, nil)

	require.NoError(t, err)
	require.NotNil(t, client)

	containerID, err := dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
	require.NoError(t, err)
	require.NotEmpty(t, containerID)

	// Releasing client won't remove it
	clientPool.ReleaseClient(client)

	// Condition will never be true, that's ok
	waitForCondition(t, func() bool {
		containerID, err = dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
		return err == nil && containerID == ""
	})

	containerID, err = dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
	require.NoError(t, err)
	require.NotEmpty(t, containerID)

	// Acquire it again
	client, err = clientPool.AcquireClient(t.Context(), config, nil)

	require.NoError(t, err)
	require.NotNil(t, client)

	containerID, err = dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
	require.NoError(t, err)
	require.NotEmpty(t, containerID)

	clientPool.Close()

	// Condition should become true now that it's closed
	waitForCondition(t, func() bool {
		containerID, err = dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
		return err == nil && containerID == ""
	})

	containerID, err = dockerClient.FindContainerByLabel(t.Context(), "docker-mcp-name=time")
	require.NoError(t, err)
	require.Empty(t, containerID)
}
