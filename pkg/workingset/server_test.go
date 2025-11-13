package workingset

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/db"
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

	servers := []string{
		"docker://myimage:latest",
	}

	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", servers)
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)
	assert.Equal(t, "My Image", dbSet.Servers[0].Snapshot.Server.Name)
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

	servers := []string{
		"docker://myimage:latest",
		"docker://anotherimage:v1.0",
	}

	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", servers)
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, "test-set")
	require.NoError(t, err)
	require.NotNil(t, dbSet)
	assert.Equal(t, "My Image", dbSet.Servers[0].Snapshot.Server.Name)
	assert.Equal(t, "Another Image", dbSet.Servers[1].Snapshot.Server.Name)
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

	servers := []string{}

	err = AddServers(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", servers)
	require.Error(t, err)
	assert.Contains(t, err.Error(), oneServerError)
}

func TestRemoveOneServerFromWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	serverURI := "docker://myimage:latest"
	setID := "test-set"

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), "test-set", "test-set", []string{
		serverURI,
	})
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, setID)
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 1)

	err = RemoveServers(ctx, dao, setID, []string{
		"My Image",
	})
	require.NoError(t, err)

	dbSet, err = dao.GetWorkingSet(ctx, setID)
	require.NoError(t, err)

	assert.Empty(t, dbSet.Servers)
}

func TestRemoveMultipleServersFromWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	workingSetID := "test-set"

	servers := []string{
		"docker://myimage:latest",
		"docker://anotherimage:v1.0",
	}

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), workingSetID, "My Test Set", servers)
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, workingSetID)
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 2)

	err = RemoveServers(ctx, dao, workingSetID, []string{"My Image", "Another Image"})
	require.NoError(t, err)

	dbSet, err = dao.GetWorkingSet(ctx, workingSetID)
	require.NoError(t, err)
	assert.Empty(t, dbSet.Servers)
}

func TestRemoveOneOfManyServerFromWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	workingSetID := "test-set"

	servers := []string{
		"docker://myimage:latest",
		"docker://anotherimage:v1.0",
	}

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), workingSetID, "My Test Set", servers)
	require.NoError(t, err)

	dbSet, err := dao.GetWorkingSet(ctx, workingSetID)
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 2)

	err = RemoveServers(ctx, dao, workingSetID, []string{"My Image"})
	require.NoError(t, err)

	dbSet, err = dao.GetWorkingSet(ctx, workingSetID)
	require.NoError(t, err)
	assert.Len(t, dbSet.Servers, 1)
	assert.Equal(t, "Another Image", dbSet.Servers[0].Snapshot.Server.Name)
}

func TestRemoveNoServersFromWorkingSet(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	workingSetID := "test-set"

	servers := []string{
		"docker://myimage:latest",
	}

	err := Create(ctx, dao, getMockRegistryClient(), getMockOciService(), workingSetID, "My Test Set", servers)
	require.NoError(t, err)

	err = RemoveServers(ctx, dao, workingSetID, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), oneServerError)
}
