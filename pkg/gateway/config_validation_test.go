package gateway

import (
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
