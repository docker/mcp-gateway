package workingset

import (
	"fmt"
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

	ociService := getMockOciService()

	err = UpdateTools(ctx, dao, ociService, "test-set", []string{"test-server.create_issue"}, []string{})
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

	ociService := getMockOciService()

	err = UpdateTools(ctx, dao, ociService, "test-set", []string{"test-server-1.create_issue", "test-server-2.update_issue"}, []string{})
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

	ociService := getMockOciService()

	err = UpdateTools(ctx, dao, ociService, "test-set", []string{}, []string{"test-server.create_issue"})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	assert.Empty(t, dbSet.Servers[0].Tools)
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

	ociService := getMockOciService()

	err = UpdateTools(ctx, dao, ociService, "test-set", []string{}, []string{
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

	ociService := getMockOciService()

	err = UpdateTools(ctx, dao, ociService, "test-set", []string{
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

	ociService := getMockOciService()

	err = UpdateTools(ctx, dao, ociService, "test-set", []string{}, []string{})
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

	ociService := getMockOciService()

	setid := "bogus"

	err = UpdateTools(ctx, dao, ociService, setid, []string{"test-server-1.tool"}, []string{})
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

	ociService := getMockOciService()

	err = UpdateTools(ctx, dao, ociService, "test-set", []string{"bogus"}, []string{})
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

	ociService := getMockOciService()

	err = UpdateTools(ctx, dao, ociService, "test-set", []string{"bogus.tool"}, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("server %s not found in working set for argument %s", "bogus", "bogus.tool"))
}
