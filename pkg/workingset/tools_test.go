package workingset

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
)

func TestEnableATool(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	err = UpdateTools(ctx, dao, "test-set", []string{"test-server.create_issue"}, []string{}, []string{}, []string{})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Contains(t, dbSet.Servers[0].Tools, "create_issue")
	assert.Len(t, dbSet.Servers[0].Tools, 1)
}

func TestEnableMultipleTools(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server-1",
					},
				},
			},
			{
				Type:  "image",
				Image: "anotherimage:v1.0",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server-2",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	err = UpdateTools(ctx, dao, "test-set", []string{"test-server-1.create_issue", "test-server-2.update_issue"}, []string{}, []string{}, []string{})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Contains(t, dbSet.Servers[0].Tools, "create_issue")
	assert.Contains(t, dbSet.Servers[1].Tools, "update_issue")
}

func TestDisableATool(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
				Tools: []string{
					"create_issue",
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	err = UpdateTools(ctx, dao, "test-set", []string{}, []string{"test-server.create_issue"}, []string{}, []string{})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Empty(t, dbSet.Servers[0].Tools)
}

func TestDisableMultipleTools(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server-1",
					},
				},
				Tools: []string{
					"tool-1",
					"tool-2",
				},
			},
			{
				Type:  "image",
				Image: "anotherimage:v1.0",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server-2",
					},
				},
				Tools: []string{
					"tool-1",
					"tool-2",
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	err = UpdateTools(ctx, dao, "test-set", []string{}, []string{
		"test-server-1.tool-1",
		"test-server-1.tool-2",
		"test-server-2.tool-1",
		"test-server-2.tool-2",
	}, []string{}, []string{})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Empty(t, dbSet.Servers[0].Tools)
	assert.Empty(t, dbSet.Servers[1].Tools)
}

func TestEnableAndDisableMultipleTools(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server-1",
					},
				},
			},
			{
				Type:  "image",
				Image: "anotherimage:v1.0",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server-2",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	err = UpdateTools(ctx, dao, "test-set", []string{
		"test-server-1.create_issue", "test-server-2.update_issue",
	}, []string{
		"test-server-1.create_issue", "test-server-2.update_issue",
	}, []string{}, []string{})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Empty(t, dbSet.Servers[0].Tools)
	assert.Empty(t, dbSet.Servers[1].Tools)
}

func TestErrorNoOperation(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server-1",
					},
				},
			},
			{
				Type:  "image",
				Image: "anotherimage:v1.0",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server-2",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	err = UpdateTools(ctx, dao, "test-set", []string{}, []string{}, []string{}, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must provide at least one flag")
}

func TestErrorWorkingSetNotFound(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server-1",
					},
				},
			},
			{
				Type:  "image",
				Image: "anotherimage:v1.0",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server-2",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	setid := "bogus"

	err = UpdateTools(ctx, dao, setid, []string{"test-server-1.tool"}, []string{}, []string{}, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("profile %s not found", setid))
}

func TestErrorInvalidToolFormat(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server-1",
					},
				},
			},
			{
				Type:  "image",
				Image: "anotherimage:v1.0",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server-2",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	err = UpdateTools(ctx, dao, "test-set", []string{"bogus"}, []string{}, []string{}, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("invalid tool argument: %s, expected <serverName>.<toolName>", "bogus"))
}

func TestErrorServerNotFound(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server-1",
					},
				},
			},
			{
				Type:  "image",
				Image: "anotherimage:v1.0",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server-2",
					},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	err = UpdateTools(ctx, dao, "test-set", []string{"bogus.tool"}, []string{}, []string{}, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("server %s not found in profile for argument %s", "bogus", "bogus.tool"))
}

func TestEnableAllTools(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
				Tools: []string{"create_issue", "update_issue"},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	err = UpdateTools(ctx, dao, "test-set", []string{}, []string{}, []string{"test-server"}, []string{})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Nil(t, dbSet.Servers[0].Tools)
}

func TestEnableAllToolsMultipleServers(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server-1",
					},
				},
				Tools: []string{"tool-1"},
			},
			{
				Type:  "image",
				Image: "anotherimage:v1.0",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server-2",
					},
				},
				Tools: []string{"tool-2"},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	err = UpdateTools(ctx, dao, "test-set", []string{}, []string{}, []string{"test-server-1", "test-server-2"}, []string{})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Nil(t, dbSet.Servers[0].Tools)
	assert.Nil(t, dbSet.Servers[1].Tools)
}

func TestEnableAllToolsServerNotFound(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
				Tools: []string{"tool-1"},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	err = UpdateTools(ctx, dao, "test-set", []string{}, []string{}, []string{"bogus"}, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server bogus not found in profile")
}

func TestDisableAllTools(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
				Tools: []string{"create_issue", "update_issue"},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	err = UpdateTools(ctx, dao, "test-set", []string{}, []string{}, []string{}, []string{"test-server"})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Empty(t, dbSet.Servers[0].Tools)
	assert.NotNil(t, dbSet.Servers[0].Tools)
}

func TestDisableAllToolsMultipleServers(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server-1",
					},
				},
				Tools: []string{"tool-1"},
			},
			{
				Type:  "image",
				Image: "anotherimage:v1.0",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server-2",
					},
				},
				Tools: []string{"tool-2"},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	err = UpdateTools(ctx, dao, "test-set", []string{}, []string{}, []string{}, []string{"test-server-1", "test-server-2"})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Empty(t, dbSet.Servers[0].Tools)
	assert.NotNil(t, dbSet.Servers[0].Tools)
	assert.Empty(t, dbSet.Servers[1].Tools)
	assert.NotNil(t, dbSet.Servers[1].Tools)
}

func TestDisableAllToolsServerNotFound(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Working Set",
		Servers: db.ServerList{
			{
				Type:  "image",
				Image: "myimage:latest",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{
						Name: "test-server",
					},
				},
				Tools: []string{"tool-1"},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	err = UpdateTools(ctx, dao, "test-set", []string{}, []string{}, []string{}, []string{"bogus"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server bogus not found in profile")
}

func TestOutputMessages(t *testing.T) {
	tests := []struct {
		name           string
		initialTools   []string
		enableTools    []string
		disableTools   []string
		expectedOutput string
	}{
		{
			name:           "enable tools",
			initialTools:   []string{},
			enableTools:    []string{"test-server.create_issue", "test-server.update_issue"},
			disableTools:   []string{},
			expectedOutput: "Updated profile test-set: 2 tool(s) enabled, 0 tool(s) disabled\n",
		},
		{
			name:           "disable tools",
			initialTools:   []string{"create_issue", "update_issue"},
			enableTools:    []string{},
			disableTools:   []string{"test-server.create_issue", "test-server.update_issue"},
			expectedOutput: "Updated profile test-set: 0 tool(s) enabled, 2 tool(s) disabled\n",
		},
		{
			name:           "enable and disable different tools",
			initialTools:   []string{"create_issue"},
			enableTools:    []string{"test-server.update_issue"},
			disableTools:   []string{"test-server.create_issue"},
			expectedOutput: "Updated profile test-set: 1 tool(s) enabled, 1 tool(s) disabled\n",
		},
		{
			name:           "no changes - enable existing tool",
			initialTools:   []string{"create_issue"},
			enableTools:    []string{"test-server.create_issue"},
			disableTools:   []string{},
			expectedOutput: "No changes made to profile test-set\n",
		},
		{
			name:           "no changes - disable non-existent tool",
			initialTools:   []string{"create_issue"},
			enableTools:    []string{},
			disableTools:   []string{"test-server.update_issue"},
			expectedOutput: "No changes made to profile test-set\n",
		},
		{
			name:           "overlap - enable and disable same tool",
			initialTools:   []string{},
			enableTools:    []string{"test-server.create_issue"},
			disableTools:   []string{"test-server.create_issue"},
			expectedOutput: "Updated profile test-set: 1 tool(s) enabled, 1 tool(s) disabled\nWarning: The following tool(s) were both enabled and disabled in the same operation: test-server.create_issue\n",
		},
		{
			name:           "overlap - enable and disable with partial overlap",
			initialTools:   []string{"create_issue"},
			enableTools:    []string{"test-server.update_issue", "test-server.delete_issue"},
			disableTools:   []string{"test-server.create_issue", "test-server.update_issue"},
			expectedOutput: "Updated profile test-set: 2 tool(s) enabled, 2 tool(s) disabled\nWarning: The following tool(s) were both enabled and disabled in the same operation: test-server.update_issue\n",
		},
		{
			name:           "overlap - multiple overlapping tools",
			initialTools:   []string{},
			enableTools:    []string{"test-server.create_issue", "test-server.update_issue", "test-server.delete_issue"},
			disableTools:   []string{"test-server.create_issue", "test-server.update_issue"},
			expectedOutput: "Updated profile test-set: 3 tool(s) enabled, 2 tool(s) disabled\nWarning: The following tool(s) were both enabled and disabled in the same operation: test-server.create_issue, test-server.update_issue\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dao := setupTestDB(t)
			ctx := t.Context()

			err := dao.CreateWorkingSet(ctx, db.WorkingSet{
				ID:   "test-set",
				Name: "Test Working Set",
				Servers: db.ServerList{
					{
						Type:  "image",
						Image: "myimage:latest",
						Snapshot: &db.ServerSnapshot{
							Server: catalog.Server{
								Name: "test-server",
							},
						},
						Tools: tt.initialTools,
					},
				},
				Secrets: db.SecretMap{},
			})
			require.NoError(t, err)

			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err = UpdateTools(ctx, dao, "test-set", tt.enableTools, tt.disableTools, []string{}, []string{})
			require.NoError(t, err)

			w.Close()
			os.Stdout = old

			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
			output := buf.String()

			assert.Equal(t, tt.expectedOutput, output)
		})
	}
}
