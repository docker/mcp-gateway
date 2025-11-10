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

func TestAddATool(t *testing.T) {
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

	err = UpdateTools(ctx, dao, "test-set", []string{"test-server.create_issue"}, []string{})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Contains(t, dbSet.Servers[0].Tools, "create_issue")
	assert.Len(t, dbSet.Servers[0].Tools, 1)
}

func TestAddMultipleTools(t *testing.T) {
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

	err = UpdateTools(ctx, dao, "test-set", []string{"test-server-1.create_issue", "test-server-2.update_issue"}, []string{})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Contains(t, dbSet.Servers[0].Tools, "create_issue")
	assert.Contains(t, dbSet.Servers[1].Tools, "update_issue")
}

func TestRemoveATool(t *testing.T) {
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

	err = UpdateTools(ctx, dao, "test-set", []string{}, []string{"test-server.create_issue"})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Nil(t, dbSet.Servers[0].Tools)
}

func TestRemoveMultipleTools(t *testing.T) {
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
	})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Empty(t, dbSet.Servers[0].Tools)
	assert.Empty(t, dbSet.Servers[1].Tools)
}

func TestAddAndRemoveMultipleTools(t *testing.T) {
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
	})
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

	err = UpdateTools(ctx, dao, "test-set", []string{}, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must provide a flag either --add or --remove")
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

	err = UpdateTools(ctx, dao, setid, []string{"test-server-1.tool"}, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("working set %s not found", setid))
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

	err = UpdateTools(ctx, dao, "test-set", []string{"bogus"}, []string{})
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

	err = UpdateTools(ctx, dao, "test-set", []string{"bogus.tool"}, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("server %s not found in working set for argument %s", "bogus", "bogus.tool"))
}

func TestOutputMessages(t *testing.T) {
	tests := []struct {
		name           string
		initialTools   []string
		addTools       []string
		removeTools    []string
		expectedOutput string
	}{
		{
			name:           "add tools",
			initialTools:   []string{},
			addTools:       []string{"test-server.create_issue", "test-server.update_issue"},
			removeTools:    []string{},
			expectedOutput: "Updated working set test-set: 2 tool(s) added, 0 tool(s) removed\n",
		},
		{
			name:           "remove tools",
			initialTools:   []string{"create_issue", "update_issue"},
			addTools:       []string{},
			removeTools:    []string{"test-server.create_issue", "test-server.update_issue"},
			expectedOutput: "Updated working set test-set: 0 tool(s) added, 2 tool(s) removed\n",
		},
		{
			name:           "add and remove different tools",
			initialTools:   []string{"create_issue"},
			addTools:       []string{"test-server.update_issue"},
			removeTools:    []string{"test-server.create_issue"},
			expectedOutput: "Updated working set test-set: 1 tool(s) added, 1 tool(s) removed\n",
		},
		{
			name:           "no changes - add existing tool",
			initialTools:   []string{"create_issue"},
			addTools:       []string{"test-server.create_issue"},
			removeTools:    []string{},
			expectedOutput: "No tools were added or removed from working set test-set\n",
		},
		{
			name:           "no changes - remove non-existent tool",
			initialTools:   []string{"create_issue"},
			addTools:       []string{},
			removeTools:    []string{"test-server.update_issue"},
			expectedOutput: "No tools were added or removed from working set test-set\n",
		},
		{
			name:           "overlap - add and remove same tool",
			initialTools:   []string{},
			addTools:       []string{"test-server.create_issue"},
			removeTools:    []string{"test-server.create_issue"},
			expectedOutput: "Updated working set test-set: 1 tool(s) added, 1 tool(s) removed\nWarning: The following tool(s) were both added and removed in the same operation: test-server.create_issue\n",
		},
		{
			name:           "overlap - add and remove with partial overlap",
			initialTools:   []string{"create_issue"},
			addTools:       []string{"test-server.update_issue", "test-server.delete_issue"},
			removeTools:    []string{"test-server.create_issue", "test-server.update_issue"},
			expectedOutput: "Updated working set test-set: 2 tool(s) added, 2 tool(s) removed\nWarning: The following tool(s) were both added and removed in the same operation: test-server.update_issue\n",
		},
		{
			name:           "overlap - multiple overlapping tools",
			initialTools:   []string{},
			addTools:       []string{"test-server.create_issue", "test-server.update_issue", "test-server.delete_issue"},
			removeTools:    []string{"test-server.create_issue", "test-server.update_issue"},
			expectedOutput: "Updated working set test-set: 3 tool(s) added, 2 tool(s) removed\nWarning: The following tool(s) were both added and removed in the same operation: test-server.create_issue, test-server.update_issue\n",
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

			err = UpdateTools(ctx, dao, "test-set", tt.addTools, tt.removeTools)
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
