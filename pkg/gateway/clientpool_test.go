package gateway

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	hostPath := t.TempDir()
	expectedHostPath, err := cleanDockerHostPath(hostPath)
	require.NoError(t, err)
	catalogYAML := `
volumes:
  - '{{hub.log_path|mount_as:/logs:ro}}'
  `
	configYAML := fmt.Sprintf(`
hub:
  log_path: %s
`, hostPath)

	args, env := argsAndEnv(t, "hub", catalogYAML, configYAML, nil)

	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"-l", "docker-mcp=true", "-l", "docker-mcp-tool-type=mcp", "-l", "docker-mcp-name=hub", "-l", "docker-mcp-transport=stdio",
		"-v", expectedHostPath + ":/logs:ro",
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

func TestApplyConfigVolumeFilterDefaultsHostBindsToReadOnly(t *testing.T) {
	hostPath := t.TempDir()
	expectedHostPath, err := cleanDockerHostPath(hostPath)
	require.NoError(t, err)
	catalogYAML := `
volumes:
  - '{{filesystem.paths|volume|into}}'
  `
	configYAML := fmt.Sprintf(`
filesystem:
  paths:
    - %s
`, hostPath)

	args, env := argsAndEnv(t, "filesystem", catalogYAML, configYAML, nil)

	mountIndex := -1
	for i, arg := range args {
		if arg == "-v" {
			mountIndex = i + 1
			break
		}
	}
	require.NotEqual(t, -1, mountIndex)
	require.Less(t, mountIndex, len(args))
	assert.True(t, strings.HasPrefix(args[mountIndex], expectedHostPath+":"))
	assert.True(t, strings.HasSuffix(args[mountIndex], ":ro"))
	assert.Empty(t, env)
}

func TestApplyConfigMountAsReadOnly(t *testing.T) {
	hostPath := t.TempDir()
	expectedHostPath, err := cleanDockerHostPath(hostPath)
	require.NoError(t, err)
	catalogYAML := `
volumes:
  - '{{hub.log_path|mount_as:/logs:ro}}'
  `
	configYAML := fmt.Sprintf(`
hub:
  log_path: %s
`, hostPath)

	args, env := argsAndEnv(t, "hub", catalogYAML, configYAML, nil)

	assert.Equal(t, []string{
		"run", "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never",
		"-l", "docker-mcp=true", "-l", "docker-mcp-tool-type=mcp", "-l", "docker-mcp-name=hub", "-l", "docker-mcp-transport=stdio",
		"-v", expectedHostPath + ":/logs:ro",
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

func TestApplyConfigLongLivedRejectsWritableHostBind(t *testing.T) {
	catalogYAML := `
longLived: true
volumes:
  - '/local/data:/data:rw'
  `

	_, _, err := argsAndEnvErr(t, "longlived", catalogYAML, "", nil)
	require.ErrorContains(t, err, "host path bind mounts must be read-only")
}

func TestApplyConfigShortLivedRejectsWritableHostBind(t *testing.T) {
	catalogYAML := `
volumes:
  - '/local/data:/data:rw'
  `

	_, _, err := argsAndEnvErr(t, "shortlived", catalogYAML, "", nil)
	require.ErrorContains(t, err, "host path bind mounts must be read-only")
}

// TestArgsAndEnv_SkipsFlagShapedValues exercises the argv-sink guard:
// volume / user / extra-host values that start with '-' would be reparsed
// by docker as further flags, so they must be dropped from the argv.
func TestArgsAndEnv_SkipsFlagShapedValues(t *testing.T) {
	tests := []struct {
		name        string
		catalogYAML string
		wantMissing []string
	}{
		{
			name: "flag-shaped volume",
			catalogYAML: `
volumes:
  - "--privileged"
  - "legit-volume:/data"
`,
			wantMissing: []string{"--privileged"},
		},
		{
			name: "flag-shaped user",
			catalogYAML: `
user: "--privileged"
`,
			wantMissing: []string{"--privileged"},
		},
		{
			name: "flag-shaped extra host",
			catalogYAML: `
extraHosts:
  - "--privileged"
  - "ok.example:127.0.0.1"
`,
			wantMissing: []string{"--privileged"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, _ := argsAndEnv(t, "svc", tt.catalogYAML, "", nil)
			for _, missing := range tt.wantMissing {
				assert.NotContains(t, args, missing, "flag-shaped value must be skipped")
			}
		})
	}
}

func TestIsSafeFlagValue(t *testing.T) {
	for _, tc := range []struct {
		v    string
		want bool
	}{
		{"", false},
		{"-x", false},
		{"--privileged", false},
		{"/legit:/data", true},
		{"1001:2002", true},
		{"host.example:127.0.0.1", true},
	} {
		assert.Equal(t, tc.want, isSafeFlagValue(tc.v), "input=%q", tc.v)
	}
}

func TestValidateDockerVolumeBinds(t *testing.T) {
	t.Run("allows named volumes", func(t *testing.T) {
		require.NoError(t, validateDockerVolumeBinds([]string{"mcp-cache_1.cache:/cache"}))
	})

	t.Run("allows read-only binds from allowed roots", func(t *testing.T) {
		require.NoError(t, validateDockerVolumeBinds([]string{t.TempDir() + ":/data:ro"}))
	})

	t.Run("defaults host binds without mode to read-only", func(t *testing.T) {
		hostPath := t.TempDir()
		expectedSource, err := cleanDockerHostPath(hostPath)
		require.NoError(t, err)

		normalized, err := normalizeDockerVolumeBind(hostPath + ":/data")
		require.NoError(t, err)
		require.Equal(t, expectedSource+":/data:ro", normalized)
	})

	t.Run("allows read-only binds from configured roots", func(t *testing.T) {
		t.Setenv(dockerBindAllowedPathsEnv, "/opt/mcp-data")
		require.NoError(t, validateDockerVolumeBinds([]string{"/opt/mcp-data/project:/data:ro"}))
	})

	t.Run("allows home paths from configured roots", func(t *testing.T) {
		home := t.TempDir()
		project := filepath.Join(home, "trusted", "project")
		require.NoError(t, os.MkdirAll(project, 0o755))
		t.Setenv("HOME", home)
		t.Setenv(dockerBindAllowedPathsEnv, "~/trusted")

		require.NoError(t, validateDockerVolumeBinds([]string{"~/trusted/project:/data:ro"}))
		expectedSource, err := cleanDockerHostPath("~/trusted/project")
		require.NoError(t, err)
		normalized, err := normalizeDockerVolumeBind("~/trusted/project:/data:ro")
		require.NoError(t, err)
		require.Equal(t, expectedSource+":/data:ro", normalized)
	})

	t.Run("rejects host paths outside allowed roots", func(t *testing.T) {
		err := validateDockerVolumeBinds([]string{"/opt/mcp-data:/data:ro"})
		require.ErrorContains(t, err, "outside allowed roots")
		require.ErrorContains(t, err, "MCP_GATEWAY_DOCKER_BIND_ALLOWED_PATHS=/opt/mcp-data")
	})

	t.Run("rejects relative host paths with slash", func(t *testing.T) {
		err := validateDockerVolumeBinds([]string{"foo/bar:/data"})
		require.ErrorContains(t, err, "outside allowed roots")

		err = validateDockerVolumeBinds([]string{"foo/bar:/data:ro"})
		require.ErrorContains(t, err, "outside allowed roots")
	})

	t.Run("rejects relative host paths escaping upward", func(t *testing.T) {
		err := validateDockerVolumeBinds([]string{"subdir/../../../etc:/data"})
		require.ErrorContains(t, err, "outside allowed roots")

		err = validateDockerVolumeBinds([]string{"subdir/../../../etc:/data:ro"})
		require.ErrorContains(t, err, "outside allowed roots")
	})

	t.Run("rejects explicitly writable host path binds", func(t *testing.T) {
		err := validateDockerVolumeBinds([]string{t.TempDir() + ":/data:rw"})
		require.ErrorContains(t, err, "host path bind mounts must be read-only")
	})

	t.Run("rejects docker socket binds", func(t *testing.T) {
		err := validateDockerVolumeBinds([]string{"/var/run/docker.sock:/var/run/docker.sock:ro"})
		require.ErrorContains(t, err, "blocked")
	})

	t.Run("rejects symlink host paths escaping allowed roots", func(t *testing.T) {
		allowedRoot := t.TempDir()
		link := filepath.Join(allowedRoot, "etc-link")
		if err := os.Symlink("/etc", link); err != nil {
			t.Skipf("cannot create symlink: %v", err)
		}

		err := validateDockerVolumeBinds([]string{filepath.ToSlash(link) + ":/data:ro"})
		require.ErrorContains(t, err, "sensitive system path")
	})

	t.Run("rejects credential paths even under allowed roots", func(t *testing.T) {
		err := validateDockerVolumeBinds([]string{filepath.Join(t.TempDir(), ".ssh") + ":/ssh:ro"})
		require.ErrorContains(t, err, "credential path")
	})
}

func argsAndEnv(t *testing.T, name, catalogYAML, configYAML string, secrets map[string]string) ([]string, []string) {
	t.Helper()
	args, env, err := argsAndEnvErr(t, name, catalogYAML, configYAML, secrets)
	require.NoError(t, err)
	return args, env
}

func argsAndEnvErr(t *testing.T, name, catalogYAML, configYAML string, secrets map[string]string) ([]string, []string, error) {
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

func TestInvalidateOAuthClients_MatchesCommunityServer(t *testing.T) {
	// Community server: remote URL set, but no Spec.OAuth metadata.
	// This verifies Gap 3: InvalidateOAuthClients matches community servers
	// that use dynamic OAuth discovery without explicit OAuth config.
	cp := &clientPool{
		keptClients: make(map[clientKey]keptClient),
	}

	getter := &clientGetter{}
	getter.once.Do(func() {}) // mark as executed
	getter.err = fmt.Errorf("mock: no real client")

	key := clientKey{serverName: "com-notion-mcp"}
	cp.keptClients[key] = keptClient{
		Name:   "com-notion-mcp",
		Getter: getter,
		Config: &catalog.ServerConfig{
			Name: "com-notion-mcp",
			Spec: catalog.Server{
				Type: "remote",
				Remote: catalog.Remote{
					URL:       "https://mcp.notion.so/mcp",
					Transport: "streamable-http",
				},
				// No OAuth field - community server
			},
		},
	}

	cp.InvalidateOAuthClients("com-notion-mcp")

	assert.Empty(t, cp.keptClients, "community server should be invalidated by name")
}

func TestInvalidateOAuthClients_MatchesCatalogServer(t *testing.T) {
	// Catalog server: remote URL set WITH Spec.OAuth metadata.
	// Verifies backward compatibility: catalog servers still get invalidated.
	cp := &clientPool{
		keptClients: make(map[clientKey]keptClient),
	}

	getter := &clientGetter{}
	getter.once.Do(func() {})
	getter.err = fmt.Errorf("mock: no real client")

	key := clientKey{serverName: "notion-remote"}
	cp.keptClients[key] = keptClient{
		Name:   "notion-remote",
		Getter: getter,
		Config: &catalog.ServerConfig{
			Name: "notion-remote",
			Spec: catalog.Server{
				Type: "remote",
				Remote: catalog.Remote{
					URL:       "https://mcp.notion.so/mcp",
					Transport: "streamable-http",
				},
				OAuth: &catalog.OAuth{
					Providers: []catalog.OAuthProvider{{Provider: "notion"}},
				},
			},
		},
	}

	cp.InvalidateOAuthClients("notion-remote")

	assert.Empty(t, cp.keptClients, "catalog server should be invalidated by name")
}

func TestInvalidateOAuthClients_SkipsNonRemoteServer(t *testing.T) {
	// Docker container server: not remote, should NOT be invalidated.
	cp := &clientPool{
		keptClients: make(map[clientKey]keptClient),
	}

	getter := &clientGetter{}
	getter.once.Do(func() {})
	getter.err = fmt.Errorf("mock: no real client")

	key := clientKey{serverName: "my-container-server"}
	cp.keptClients[key] = keptClient{
		Name:   "my-container-server",
		Getter: getter,
		Config: &catalog.ServerConfig{
			Name: "my-container-server",
			Spec: catalog.Server{
				Type:  "server",
				Image: "mcp/my-server:latest",
				// Not remote - no URL
			},
		},
	}

	cp.InvalidateOAuthClients("my-container-server")

	assert.Len(t, cp.keptClients, 1, "non-remote server should NOT be invalidated")
}

func TestInvalidateOAuthClients_SkipsMismatchedName(t *testing.T) {
	// Remote server with different name: should NOT be invalidated.
	cp := &clientPool{
		keptClients: make(map[clientKey]keptClient),
	}

	getter := &clientGetter{}
	getter.once.Do(func() {})
	getter.err = fmt.Errorf("mock: no real client")

	key := clientKey{serverName: "other-server"}
	cp.keptClients[key] = keptClient{
		Name:   "other-server",
		Getter: getter,
		Config: &catalog.ServerConfig{
			Name: "other-server",
			Spec: catalog.Server{
				Type: "remote",
				Remote: catalog.Remote{
					URL: "https://other.example.com/mcp",
				},
			},
		},
	}

	cp.InvalidateOAuthClients("com-notion-mcp")

	assert.Len(t, cp.keptClients, 1, "server with different name should NOT be invalidated")
}

func TestInvalidateOAuthClients_OnlyMatchingRemoved(t *testing.T) {
	// Multiple clients: only the matching remote server should be removed.
	cp := &clientPool{
		keptClients: make(map[clientKey]keptClient),
	}

	makeGetter := func() *clientGetter {
		g := &clientGetter{}
		g.once.Do(func() {})
		g.err = fmt.Errorf("mock: no real client")
		return g
	}

	// Community OAuth server (should be invalidated)
	cp.keptClients[clientKey{serverName: "com-notion-mcp"}] = keptClient{
		Name:   "com-notion-mcp",
		Getter: makeGetter(),
		Config: &catalog.ServerConfig{
			Name: "com-notion-mcp",
			Spec: catalog.Server{
				Type:   "remote",
				Remote: catalog.Remote{URL: "https://mcp.notion.so/mcp"},
			},
		},
	}

	// Different remote server (should NOT be invalidated)
	cp.keptClients[clientKey{serverName: "github-remote"}] = keptClient{
		Name:   "github-remote",
		Getter: makeGetter(),
		Config: &catalog.ServerConfig{
			Name: "github-remote",
			Spec: catalog.Server{
				Type:   "remote",
				Remote: catalog.Remote{URL: "https://mcp.github.com/mcp"},
			},
		},
	}

	// Docker container server (should NOT be invalidated)
	cp.keptClients[clientKey{serverName: "local-server"}] = keptClient{
		Name:   "local-server",
		Getter: makeGetter(),
		Config: &catalog.ServerConfig{
			Name: "local-server",
			Spec: catalog.Server{
				Type:  "server",
				Image: "mcp/local:latest",
			},
		},
	}

	cp.InvalidateOAuthClients("com-notion-mcp")

	assert.Len(t, cp.keptClients, 2, "only the matching remote server should be removed")
	_, hasNotion := cp.keptClients[clientKey{serverName: "com-notion-mcp"}]
	assert.False(t, hasNotion, "com-notion-mcp should have been removed")
	_, hasGithub := cp.keptClients[clientKey{serverName: "github-remote"}]
	assert.True(t, hasGithub, "github-remote should remain")
	_, hasLocal := cp.keptClients[clientKey{serverName: "local-server"}]
	assert.True(t, hasLocal, "local-server should remain")
}

func TestLongLivedFlaggedServer(t *testing.T) {
	session := &mcp.ServerSession{}
	cp := &clientPool{}

	server := &catalog.ServerConfig{
		Name: "github",
		Spec: catalog.Server{
			Type:      "server",
			Image:     "ghcr.io/github/github-mcp-server:latest",
			LongLived: true,
		},
	}
	cfg := &clientConfig{serverSession: session}

	assert.True(t, cp.longLived(server, cfg), "LongLived=true with session")
}

func TestLongLivedRequiresSession(t *testing.T) {
	cp := &clientPool{}

	server := &catalog.ServerConfig{
		Name: "github",
		Spec: catalog.Server{
			Type:      "server",
			Image:     "ghcr.io/github/github-mcp-server:latest",
			LongLived: true,
		},
	}

	assert.False(t, cp.longLived(server, nil), "nil config")
	assert.False(t, cp.longLived(server, &clientConfig{}), "nil serverSession")
}

func TestLongLivedRemoteServer(t *testing.T) {
	session := &mcp.ServerSession{}
	cp := &clientPool{}

	remoteServer := &catalog.ServerConfig{
		Name: "remote-svc",
		Spec: catalog.Server{
			Type:   "remote",
			Remote: catalog.Remote{URL: "https://mcp.example.com/mcp"},
		},
	}
	cfg := &clientConfig{serverSession: session}

	assert.True(t, cp.longLived(remoteServer, cfg), "Remote server must always be long-lived")
}

func TestReleaseClientsForSession(t *testing.T) {
	sess1 := &mcp.ServerSession{}
	sess2 := &mcp.ServerSession{}

	makeGetter := func() *clientGetter {
		g := &clientGetter{}
		g.once.Do(func() {})
		g.err = fmt.Errorf("mock: no real client")
		return g
	}

	cp := &clientPool{
		keptClients: map[clientKey]keptClient{
			{serverName: "server-a", session: sess1}: {
				Name:   "server-a",
				Getter: makeGetter(),
				Config: &catalog.ServerConfig{Name: "server-a"},
			},
			{serverName: "server-b", session: sess1}: {
				Name:   "server-b",
				Getter: makeGetter(),
				Config: &catalog.ServerConfig{Name: "server-b"},
			},
			{serverName: "server-a", session: sess2}: {
				Name:   "server-a",
				Getter: makeGetter(),
				Config: &catalog.ServerConfig{Name: "server-a"},
			},
		},
	}

	cp.ReleaseClientsForSession(sess1)

	assert.Len(t, cp.keptClients, 1, "only sess2's client should remain")
	_, hasSess2 := cp.keptClients[clientKey{serverName: "server-a", session: sess2}]
	assert.True(t, hasSess2, "sess2's server-a client must still be present")
}

func TestAcquireClientNoDuplicatesUnderConcurrency(t *testing.T) {
	// Verify no map races under concurrent long-lived AcquireClient calls (run with -race)
	session := &mcp.ServerSession{}
	serverConfig := &catalog.ServerConfig{
		Name: "test-server",
		Spec: catalog.Server{
			Type:      "server",
			Image:     "mcp/test:latest",
			LongLived: true,
		},
		Config:  map[string]any{},
		Secrets: map[string]string{},
	}
	cfg := &clientConfig{serverSession: session}
	cp := &clientPool{
		Options:     Options{},
		keptClients: make(map[clientKey]keptClient),
	}

	const n = 10
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			_, _ = cp.AcquireClient(context.Background(), serverConfig, cfg)
		}()
	}
	wg.Wait()

	// GetClient fails without a running container; AcquireClient must remove the entry
	assert.Empty(t, cp.keptClients)
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
