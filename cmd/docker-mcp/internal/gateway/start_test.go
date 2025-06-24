package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/catalog"
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

	gateway := &Gateway{}
	args, env := gateway.argsAndEnv(ServerConfig{
		Name:   "grafana",
		Spec:   parseSpec(t, catalogYAML),
		Config: parseConfig(t, configYAML),
		Secrets: map[string]string{
			"grafana.api_key": "API_KEY",
		},
	}, nil, "")

	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"--label", "docker-mcp=true", "--label", "docker-mcp-tool-type=mcp", "--label", "docker-mcp-name=grafana", "--label", "docker-mcp-transport=stdio",
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

	gateway := &Gateway{}
	args, env := gateway.argsAndEnv(ServerConfig{
		Name:   "mongodb",
		Spec:   parseSpec(t, catalogYAML),
		Config: map[string]any{},
		Secrets: map[string]string{
			"mongodb.connection_string": "HOST:PORT",
		},
	}, nil, "")

	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"--label", "docker-mcp=true", "--label", "docker-mcp-tool-type=mcp", "--label", "docker-mcp-name=mongodb", "--label", "docker-mcp-transport=stdio",
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

	gateway := &Gateway{}
	args, env := gateway.argsAndEnv(ServerConfig{
		Name:   "notion",
		Spec:   parseSpec(t, catalogYAML),
		Config: map[string]any{},
		Secrets: map[string]string{
			"notion.internal_integration_token": "ntn_DUMMY",
		},
	}, nil, "")

	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"--label", "docker-mcp=true", "--label", "docker-mcp-tool-type=mcp", "--label", "docker-mcp-name=notion", "--label", "docker-mcp-transport=stdio",
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
	gateway := &Gateway{}
	args, env := gateway.argsAndEnv(ServerConfig{
		Name:    "hub",
		Spec:    parseSpec(t, catalogYAML),
		Config:  parseConfig(t, configYAML),
		Secrets: map[string]string{},
	}, nil, "")

	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"--label", "docker-mcp=true", "--label", "docker-mcp-tool-type=mcp", "--label", "docker-mcp-name=hub", "--label", "docker-mcp-transport=stdio",
		"-v", "/local/logs:/logs:ro",
	}, args)
	assert.Empty(t, env)
}

func TestApplyConfigEmptyMountAs(t *testing.T) {
	catalogYAML := `
volumes:
  - '{{hub.log_path|mount_as:/logs:ro}}'
  `

	gateway := &Gateway{}
	args, env := gateway.argsAndEnv(ServerConfig{
		Name:    "hub",
		Spec:    parseSpec(t, catalogYAML),
		Config:  map[string]any{},
		Secrets: map[string]string{},
	}, nil, "")

	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"--label", "docker-mcp=true", "--label", "docker-mcp-tool-type=mcp", "--label", "docker-mcp-name=hub", "--label", "docker-mcp-transport=stdio",
	}, args)
	assert.Empty(t, env)
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
