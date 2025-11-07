package workingset

import (
	"testing"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var oneServerError = "at least one server must be specified"

func TestAddOneServerToWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	var servers = []string{
		"docker://myimage:latest",
	}

	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", servers)
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)
	assert.Equal(t, "myimage:latest", dbSet.Servers[0].Image)
}

func TestAddMultipleServersToWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	var servers = []string{
		"docker://myimage:latest",
		"docker://anotherimage:v1.0",
	}

	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", servers)
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)
	assert.Equal(t, "myimage:latest", dbSet.Servers[0].Image)
	assert.Equal(t, "anotherimage:v1.0", dbSet.Servers[1].Image)
}

func TestAddRegistryServerToWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	var servers = []string{
		"https://example.com/v0/servers/server1",
	}

	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", servers)
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)
	assert.Equal(t, "https://example.com/v0/servers/server1/versions/0.1.0", dbSet.Servers[0].Source)
}

func TestAddMixTypeServerToWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	var servers = []string{
		"docker://myimage:latest",
		"https://example.com/v0/servers/server1",
	}

	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", servers)
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)
	assert.Equal(t, "myimage:latest", dbSet.Servers[0].Image)
	assert.Equal(t, "https://example.com/v0/servers/server1/versions/0.1.0", dbSet.Servers[1].Source)
}

func TestAddNoServersToWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "test-set",
		Name:    "Test Working Set",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	var servers = []string{}

	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", servers)
	require.Error(t, err)
	assert.Contains(t, err.Error(), oneServerError)
}

func TestRemoveOneServerFromWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	var serverName = "https://example.com/v0/servers/server1"
	var setId = "test-set"

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", "test-set", []string{
		serverName,
	})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, setId)
	assert.Len(t, dbSet.Servers, 1)

	err = RemoveServers(ctx, dao, setId, []string{
		"https://example.com/v0/servers/server1/versions/0.1.0",
	})
	assert.NoError(t, err)

	dbSet, err = dao.GetWorkingSet(ctx, setId)
	assert.NoError(t, err)

	assert.Len(t, dbSet.Servers, 0)
}

func TestRemoveMultipleServersFromWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	var workingSetId = "test-set"

	var servers = []string{
		"docker://myimage:latest",
		"docker://anotherimage:v1.0",
	}

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), workingSetId, "My Test Set", servers)
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, workingSetId)
	assert.Len(t, dbSet.Servers, 2)

	err = RemoveServers(ctx, dao, workingSetId, servers)
	require.NoError(t, err)

	dbSet, err = dao.GetWorkingSet(ctx, workingSetId)
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 0)
}

func TestRemoveOneOfManyServerFromWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	var workingSetId = "test-set"

	var servers = []string{
		"docker://myimage:latest",
		"docker://anotherimage:v1.0",
	}

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), workingSetId, "My Test Set", servers)
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, workingSetId)
	assert.Len(t, dbSet.Servers, 2)

	err = RemoveServers(ctx, dao, workingSetId, []string{servers[0]})
	require.NoError(t, err)

	dbSet, err = dao.GetWorkingSet(ctx, workingSetId)
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 1)
	assert.Equal(t, "anotherimage:v1.0", dbSet.Servers[0].Image)
}

func TestRemoveMixTypeServerFromWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	var workingSetId = "test-set"

	var servers = []string{
		"docker://myimage:latest",
		"https://example.com/v0/servers/server1",
	}

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), workingSetId, "My Test Set", servers)
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, workingSetId)
	assert.Len(t, dbSet.Servers, 2)

	err = RemoveServers(ctx, dao, workingSetId, []string{
		"docker://myimage:latest",
		"https://example.com/v0/servers/server1/versions/0.1.0",
	})
	require.NoError(t, err)

	dbSet, err = dao.GetWorkingSet(ctx, workingSetId)
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 0)
}

func TestRemoveNoServersFromWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	var workingSetId = "test-set"

	var servers = []string{
		"docker://myimage:latest",
		"https://example.com/v0/servers/server1",
	}

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), workingSetId, "My Test Set", servers)
	require.NoError(t, err)

	err = RemoveServers(ctx, dao, workingSetId, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), oneServerError)
}
