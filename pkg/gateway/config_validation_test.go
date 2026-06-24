package gateway

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/workingset"
	"github.com/docker/mcp-gateway/test/mocks"
)

func TestValidateServerConfigReportsMissingRequiredConfig(t *testing.T) {
	server := catalog.Server{
		Config: []any{
			map[string]any{
				"properties": map[string]any{
					"config_path": map[string]any{
						"type": "string",
					},
				},
				"required": []any{"config_path"},
			},
		},
	}

	require.Equal(t, []string{"config_path (missing)"}, validateServerConfig(server, nil))
	require.Equal(t, []string{"config_path (missing)"}, validateServerConfig(server, map[string]any{"config_path": ""}))
	require.Empty(t, validateServerConfig(server, map[string]any{"config_path": "/home/user/.kube/config"}))
}

func TestValidateServerConfigAllowsEmptyObjectValue(t *testing.T) {
	server := catalog.Server{
		Config: []any{
			map[string]any{
				"properties": map[string]any{
					"headers": map[string]any{
						"type": "object",
					},
				},
				"required": []any{"headers"},
			},
		},
	}

	require.Empty(t, validateServerConfig(server, map[string]any{"headers": map[string]any{}}))
}

func TestValidateServerConfigRejectsWrongTypeForRequiredString(t *testing.T) {
	server := catalog.Server{
		Config: []any{
			map[string]any{
				"properties": map[string]any{
					"config_path": map[string]any{
						"type": "string",
					},
				},
				"required": []any{"config_path"},
			},
		},
	}

	for _, value := range []any{map[string]any{}, []any{}} {
		missingConfig := validateServerConfig(server, map[string]any{"config_path": value})
		require.Len(t, missingConfig, 1)
		require.Contains(t, missingConfig[0], "config_path (")
		require.NotContains(t, missingConfig[0], "missing")
	}
}

func TestActivateProfileRejectsMissingRequiredConfig(t *testing.T) {
	serveEmptySecretsEngine(t)

	server := catalog.Server{
		Name: "kubernetes",
		Type: "remote",
		Remote: catalog.Remote{
			URL: "https://mcp.example.com/mcp",
		},
		Secrets: []catalog.Secret{
			{
				Name: "KUBECONFIG_TOKEN",
				Env:  "KUBECONFIG_TOKEN",
			},
		},
		Config: []any{
			map[string]any{
				"name": "kubernetes",
				"properties": map[string]any{
					"config_path": map[string]any{
						"type": "string",
					},
				},
				"required": []any{"config_path"},
				"type":     "object",
			},
		},
	}

	g := &Gateway{}
	err := g.ActivateProfile(t.Context(), workingset.WorkingSet{
		ID:   "test",
		Name: "test",
		Servers: []workingset.Server{
			{
				Type:     workingset.ServerTypeRemote,
				Endpoint: server.Remote.URL,
				Config:   map[string]any{},
				Snapshot: &workingset.ServerSnapshot{
					Server: server,
				},
			},
		},
	})

	require.ErrorContains(t, err, "Cannot activate profile 'test'")
	require.ErrorContains(t, err, "Server 'kubernetes'")
	require.ErrorContains(t, err, "Missing secrets: KUBECONFIG_TOKEN")
	require.ErrorContains(t, err, "Missing/invalid config: config_path (missing)")
}

func TestWorkingSetConfigurationReadRejectsMissingRequiredConfig(t *testing.T) {
	dao, err := db.New(db.WithDatabaseFile(filepath.Join(t.TempDir(), "test.db")))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, dao.Close())
	})

	kubernetesServer := catalog.Server{
		Name:  "kubernetes",
		Type:  "server",
		Image: "mcp/kubernetes@sha256:ad0316b4ddcc61356d45abce4abf5b090c85cd20ffbff5dd0b6e743db11cb788",
		Volumes: []string{
			"{{kubernetes.config_path}}:/home/appuser/.kube/config",
		},
		Config: []any{
			map[string]any{
				"name": "kubernetes",
				"properties": map[string]any{
					"config_path": map[string]any{
						"type": "string",
					},
				},
				"required": []any{"config_path"},
				"type":     "object",
			},
		},
	}

	err = dao.CreateWorkingSet(t.Context(), db.WorkingSet{
		ID:   "test",
		Name: "test",
		Servers: db.ServerList{
			{
				Type:  string(workingset.ServerTypeImage),
				Image: kubernetesServer.Image,
				Snapshot: &db.ServerSnapshot{
					Server: kubernetesServer,
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	cfg := NewWorkingSetConfiguration(Config{WorkingSet: "test"}, mocks.NewMockOCIService(), nil)
	_, err = cfg.readOnce(t.Context(), dao)
	require.ErrorContains(t, err, "Cannot activate profile 'test'")
	require.ErrorContains(t, err, "Server 'kubernetes'")
	require.ErrorContains(t, err, "Missing/invalid config: config_path (missing)")
}

func serveEmptySecretsEngine(t *testing.T) {
	t.Helper()

	home, err := os.MkdirTemp("/tmp", "mcp-gateway-test-home-")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(home))
	})

	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	cacheDir, err := os.UserCacheDir()
	require.NoError(t, err)
	socketPath := filepath.Join(cacheDir, "docker-secrets-engine", "engine.sock")
	require.NoError(t, os.MkdirAll(filepath.Dir(socketPath), 0o755))

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}),
	}

	go func() {
		_ = server.Serve(listener)
	}()

	t.Cleanup(func() {
		require.NoError(t, server.Shutdown(context.Background()))
	})
}
