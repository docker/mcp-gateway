package workingset

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/pkg/db"
)

func registryURL(serverName string) string {
	return fmt.Sprintf("https://mcp.docker.com/v0/servers/%s", url.PathEscape(serverName))
}

func TestNoWorkingSetsHumanReadable(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	output := captureStdout(func() {
		err := Servers(ctx, dao, "", "", OutputFormatHumanReadable)
		require.NoError(t, err)
	})

	assert.Contains(t, output, "No working sets found")
}

func TestNoWorkingSetsJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	output := captureStdout(func() {
		err := Servers(ctx, dao, "", "", OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err := json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestNoWorkingSetsYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	output := captureStdout(func() {
		err := Servers(ctx, dao, "", "", OutputFormatYAML)
		require.NoError(t, err)
	})

	var results []SearchResult
	err := yaml.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestOneWorkingSetJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set-1",
		Name: "Test Working Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "test-server-1:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "", "", OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "test-set-1", results[0].ID)
	assert.Equal(t, "Test Working Set 1", results[0].Name)
	assert.Len(t, results[0].Servers, 1)
	assert.Equal(t, ServerTypeImage, results[0].Servers[0].Type)
	assert.Equal(t, "test-server-1:latest", results[0].Servers[0].Image)
}

func TestOneWorkingSetYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set-1",
		Name: "Test Working Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "test-server-1:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "", "", OutputFormatYAML)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = yaml.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "test-set-1", results[0].ID)
	assert.Equal(t, "Test Working Set 1", results[0].Name)
	assert.Len(t, results[0].Servers, 1)
	assert.Equal(t, ServerTypeImage, results[0].Servers[0].Type)
	assert.Equal(t, "test-server-1:latest", results[0].Servers[0].Image)
}

func TestMultipleWorkingSetsSpecifyWorkingSetJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set-1",
		Name: "Test Working Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "test-1:latest"},
		},
	})
	require.NoError(t, err)

	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set-2",
		Name: "Test Working Set 2",
		Servers: db.ServerList{
			{Type: "image", Image: "test-2:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "", "test-set-2", OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "test-set-2", results[0].ID)
	assert.Equal(t, "Test Working Set 2", results[0].Name)
	assert.Len(t, results[0].Servers, 1)
	assert.Equal(t, ServerTypeImage, results[0].Servers[0].Type)
	assert.Equal(t, "test-2:latest", results[0].Servers[0].Image)
}

func TestMultipleWorkingSetsSpecifyWorkingSetYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set-1",
		Name: "Test Working Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "test-1:latest"},
		},
	})
	require.NoError(t, err)

	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set-2",
		Name: "Test Working Set 2",
		Servers: db.ServerList{
			{Type: "image", Image: "test-2:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "", "test-set-2", OutputFormatYAML)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = yaml.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "test-set-2", results[0].ID)
	assert.Equal(t, "Test Working Set 2", results[0].Name)
	assert.Len(t, results[0].Servers, 1)
	assert.Equal(t, ServerTypeImage, results[0].Servers[0].Type)
	assert.Equal(t, "test-2:latest", results[0].Servers[0].Image)
}

func TestMultipleWorkingSetsWithQueryJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set-1",
		Name: "Test Working Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "test-server-1:latest"},
		},
	})
	require.NoError(t, err)

	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set-2",
		Name: "Test Working Set 2",
		Servers: db.ServerList{
			{Type: "image", Image: "test-server-2:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "test-server", "", OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.Equal(t, "test-set-1", results[0].ID)
	assert.Equal(t, "Test Working Set 1", results[0].Name)
	assert.Len(t, results[0].Servers, 1)
	assert.Equal(t, ServerTypeImage, results[0].Servers[0].Type)
	assert.Equal(t, "test-server-1:latest", results[0].Servers[0].Image)

	assert.Equal(t, "test-set-2", results[1].ID)
	assert.Equal(t, "Test Working Set 2", results[1].Name)
	assert.Len(t, results[1].Servers, 1)
	assert.Equal(t, ServerTypeImage, results[1].Servers[0].Type)
	assert.Equal(t, "test-server-2:latest", results[1].Servers[0].Image)
}

func TestMultipleWorkingSetsWithQueryYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set-1",
		Name: "Test Working Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "test-server-1:latest"},
		},
	})
	require.NoError(t, err)

	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set-2",
		Name: "Test Working Set 2",
		Servers: db.ServerList{
			{Type: "image", Image: "test-server-2:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "test-server", "", OutputFormatYAML)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = yaml.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.Equal(t, "test-set-1", results[0].ID)
	assert.Equal(t, "Test Working Set 1", results[0].Name)
	assert.Len(t, results[0].Servers, 1)
	assert.Equal(t, ServerTypeImage, results[0].Servers[0].Type)
	assert.Equal(t, "test-server-1:latest", results[0].Servers[0].Image)

	assert.Equal(t, "test-set-2", results[1].ID)
	assert.Equal(t, "Test Working Set 2", results[1].Name)
	assert.Len(t, results[1].Servers, 1)
	assert.Equal(t, ServerTypeImage, results[1].Servers[0].Type)
	assert.Equal(t, "test-server-2:latest", results[1].Servers[0].Image)
}

func TestMultipleWorkingSetsWithQueryAndWorkingSetJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set-1",
		Name: "Test Working Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "test-server-1:latest"},
		},
	})
	require.NoError(t, err)

	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set-2",
		Name: "Test Working Set 2",
		Servers: db.ServerList{
			{Type: "image", Image: "test-server-2:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "test-server", "test-set-2", OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Equal(t, "test-set-2", results[0].ID)
	assert.Equal(t, "Test Working Set 2", results[0].Name)
	assert.Len(t, results[0].Servers, 1)
	assert.Equal(t, ServerTypeImage, results[0].Servers[0].Type)
	assert.Equal(t, "test-server-2:latest", results[0].Servers[0].Image)
}

func TestMultipleWorkingSetsWithQueryAndWorkingSetYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set-1",
		Name: "Test Working Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "test-server-1:latest"},
		},
	})
	require.NoError(t, err)

	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set-2",
		Name: "Test Working Set 2",
		Servers: db.ServerList{
			{Type: "image", Image: "test-server-2:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "test-server", "test-set-2", OutputFormatYAML)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = yaml.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Equal(t, "test-set-2", results[0].ID)
	assert.Equal(t, "Test Working Set 2", results[0].Name)
	assert.Len(t, results[0].Servers, 1)
	assert.Equal(t, ServerTypeImage, results[0].Servers[0].Type)
	assert.Equal(t, "test-server-2:latest", results[0].Servers[0].Image)
}

func TestQueryMatchesNoResultsJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set-1",
		Name: "Test Working Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:latest"},
			{Type: "image", Image: "redis:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "nonexistent-server", "", OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestQueryMatchesNoResultsYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set-1",
		Name: "Test Working Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:latest"},
			{Type: "image", Image: "redis:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "nonexistent-server", "", OutputFormatYAML)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = yaml.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestQueryMatchesOneResultJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set-1",
		Name: "Test Working Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:latest"},
			{Type: "image", Image: "redis:latest"},
			{Type: "image", Image: "mongodb:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "postgres", "", OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "test-set-1", results[0].ID)
	assert.Len(t, results[0].Servers, 1)
	assert.Equal(t, "postgres:latest", results[0].Servers[0].Image)
}

func TestQueryMatchesOneResultYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set-1",
		Name: "Test Working Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:latest"},
			{Type: "image", Image: "redis:latest"},
			{Type: "image", Image: "mongodb:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "postgres", "", OutputFormatYAML)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = yaml.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "test-set-1", results[0].ID)
	assert.Len(t, results[0].Servers, 1)
	assert.Equal(t, "postgres:latest", results[0].Servers[0].Image)
}

func TestQueryMatchesOneResultWithMultipleSetsJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-1",
		Name: "Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:latest"},
			{Type: "image", Image: "redis:latest"},
		},
	})
	require.NoError(t, err)

	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-2",
		Name: "Set 2",
		Servers: db.ServerList{
			{Type: "image", Image: "nginx:latest"},
			{Type: "image", Image: "mongodb:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "postgres", "", OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "set-1", results[0].ID)
	assert.Len(t, results[0].Servers, 1)
	assert.Equal(t, "postgres:latest", results[0].Servers[0].Image)
}

func TestQueryMatchesOneResultWithMultipleSetsYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-1",
		Name: "Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:latest"},
			{Type: "image", Image: "redis:latest"},
		},
	})
	require.NoError(t, err)

	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-2",
		Name: "Set 2",
		Servers: db.ServerList{
			{Type: "image", Image: "nginx:latest"},
			{Type: "image", Image: "mongodb:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "postgres", "", OutputFormatYAML)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = yaml.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "set-1", results[0].ID)
	assert.Len(t, results[0].Servers, 1)
	assert.Equal(t, "postgres:latest", results[0].Servers[0].Image)
}

func TestQueryMatchesNoResultsWithMultipleSetsJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-1",
		Name: "Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:latest"},
			{Type: "image", Image: "redis:latest"},
		},
	})
	require.NoError(t, err)

	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-2",
		Name: "Set 2",
		Servers: db.ServerList{
			{Type: "image", Image: "nginx:latest"},
			{Type: "image", Image: "mongodb:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "nonexistent", "", OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestQueryMatchesNoResultsWithMultipleSetsYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-1",
		Name: "Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:latest"},
			{Type: "image", Image: "redis:latest"},
		},
	})
	require.NoError(t, err)

	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-2",
		Name: "Set 2",
		Servers: db.ServerList{
			{Type: "image", Image: "nginx:latest"},
			{Type: "image", Image: "mongodb:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "nonexistent", "", OutputFormatYAML)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = yaml.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestQueryMatchesMultipleResultsSameSetJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set-1",
		Name: "Test Working Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "my-app-frontend:latest"},
			{Type: "image", Image: "my-app-backend:latest"},
			{Type: "image", Image: "redis:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "my-app", "", OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "test-set-1", results[0].ID)
	assert.Len(t, results[0].Servers, 2)
	assert.Equal(t, "my-app-backend:latest", results[0].Servers[0].Image)
	assert.Equal(t, "my-app-frontend:latest", results[0].Servers[1].Image)
}

func TestQueryMatchesMultipleResultsSameSetYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set-1",
		Name: "Test Working Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "my-app-frontend:latest"},
			{Type: "image", Image: "my-app-backend:latest"},
			{Type: "image", Image: "redis:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "my-app", "", OutputFormatYAML)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = yaml.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "test-set-1", results[0].ID)
	assert.Len(t, results[0].Servers, 2)
	assert.Equal(t, "my-app-backend:latest", results[0].Servers[0].Image)
	assert.Equal(t, "my-app-frontend:latest", results[0].Servers[1].Image)
}

func TestQueryMatchesMultipleResultsSameSetWithMultipleSetsJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-1",
		Name: "Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "my-app-frontend:latest"},
			{Type: "image", Image: "my-app-backend:latest"},
			{Type: "image", Image: "redis:latest"},
		},
	})
	require.NoError(t, err)

	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-2",
		Name: "Set 2",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:latest"},
			{Type: "image", Image: "nginx:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "my-app", "", OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "set-1", results[0].ID)
	assert.Len(t, results[0].Servers, 2)
	assert.Equal(t, "my-app-backend:latest", results[0].Servers[0].Image)
	assert.Equal(t, "my-app-frontend:latest", results[0].Servers[1].Image)
}

func TestQueryMatchesMultipleResultsSameSetWithMultipleSetsYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-1",
		Name: "Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "my-app-frontend:latest"},
			{Type: "image", Image: "my-app-backend:latest"},
			{Type: "image", Image: "redis:latest"},
		},
	})
	require.NoError(t, err)

	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-2",
		Name: "Set 2",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:latest"},
			{Type: "image", Image: "nginx:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "my-app", "", OutputFormatYAML)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = yaml.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "set-1", results[0].ID)
	assert.Len(t, results[0].Servers, 2)
	assert.Equal(t, "my-app-backend:latest", results[0].Servers[0].Image)
	assert.Equal(t, "my-app-frontend:latest", results[0].Servers[1].Image)
}

func TestQueryMatchesMultipleResultsAcrossSetsJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "dev-set",
		Name: "Development",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:14"},
			{Type: "image", Image: "redis:latest"},
		},
	})
	require.NoError(t, err)

	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "prod-set",
		Name: "Production",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:15"},
			{Type: "image", Image: "nginx:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "postgres", "", OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.Equal(t, "dev-set", results[0].ID)
	assert.Len(t, results[0].Servers, 1)
	assert.Equal(t, "postgres:14", results[0].Servers[0].Image)

	assert.Equal(t, "prod-set", results[1].ID)
	assert.Len(t, results[1].Servers, 1)
	assert.Equal(t, "postgres:15", results[1].Servers[0].Image)
}

func TestQueryMatchesMultipleResultsAcrossSetsYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "dev-set",
		Name: "Development",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:14"},
			{Type: "image", Image: "redis:latest"},
		},
	})
	require.NoError(t, err)

	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "prod-set",
		Name: "Production",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:15"},
			{Type: "image", Image: "nginx:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "postgres", "", OutputFormatYAML)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = yaml.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.Equal(t, "dev-set", results[0].ID)
	assert.Len(t, results[0].Servers, 1)
	assert.Equal(t, "postgres:14", results[0].Servers[0].Image)

	assert.Equal(t, "prod-set", results[1].ID)
	assert.Len(t, results[1].Servers, 1)
	assert.Equal(t, "postgres:15", results[1].Servers[0].Image)
}

func TestNoQueryListsAllServersJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-1",
		Name: "Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "server-1:latest"},
			{Type: "image", Image: "server-2:latest"},
		},
	})
	require.NoError(t, err)

	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-2",
		Name: "Set 2",
		Servers: db.ServerList{
			{Type: "image", Image: "server-3:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "", "", OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.Equal(t, "set-1", results[0].ID)
	assert.Len(t, results[0].Servers, 2)
	assert.Equal(t, "server-1:latest", results[0].Servers[0].Image)
	assert.Equal(t, "server-2:latest", results[0].Servers[1].Image)

	assert.Equal(t, "set-2", results[1].ID)
	assert.Len(t, results[1].Servers, 1)
	assert.Equal(t, "server-3:latest", results[1].Servers[0].Image)
}

func TestNoQueryListsAllServersYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-1",
		Name: "Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "server-1:latest"},
			{Type: "image", Image: "server-2:latest"},
		},
	})
	require.NoError(t, err)

	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-2",
		Name: "Set 2",
		Servers: db.ServerList{
			{Type: "image", Image: "server-3:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "", "", OutputFormatYAML)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = yaml.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.Equal(t, "set-1", results[0].ID)
	assert.Len(t, results[0].Servers, 2)
	assert.Equal(t, "server-1:latest", results[0].Servers[0].Image)
	assert.Equal(t, "server-2:latest", results[0].Servers[1].Image)

	assert.Equal(t, "set-2", results[1].ID)
	assert.Len(t, results[1].Servers, 1)
	assert.Equal(t, "server-3:latest", results[1].Servers[0].Image)
}

func TestNoQueryWithSpecificWorkingSetJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-1",
		Name: "Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "server-1:latest"},
			{Type: "image", Image: "server-2:latest"},
		},
	})
	require.NoError(t, err)

	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-2",
		Name: "Set 2",
		Servers: db.ServerList{
			{Type: "image", Image: "server-3:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "", "set-1", OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "set-1", results[0].ID)
	assert.Len(t, results[0].Servers, 2)
}

func TestNoQueryWithSpecificWorkingSetYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-1",
		Name: "Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "server-1:latest"},
			{Type: "image", Image: "server-2:latest"},
		},
	})
	require.NoError(t, err)

	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-2",
		Name: "Set 2",
		Servers: db.ServerList{
			{Type: "image", Image: "server-3:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "", "set-1", OutputFormatYAML)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = yaml.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "set-1", results[0].ID)
	assert.Len(t, results[0].Servers, 2)
}

func TestQueryWithSpecificWorkingSetNoMatchJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-1",
		Name: "Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "redis", "set-1", OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestQueryWithSpecificWorkingSetNoMatchYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "set-1",
		Name: "Set 1",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "redis", "set-1", OutputFormatYAML)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = yaml.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestQueryCaseInsensitiveJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Set",
		Servers: db.ServerList{
			{Type: "image", Image: "PostgreSQL:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "postgresql", "", OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "PostgreSQL:latest", results[0].Servers[0].Image)
}

func TestQueryCaseInsensitiveYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Set",
		Servers: db.ServerList{
			{Type: "image", Image: "PostgreSQL:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "postgresql", "", OutputFormatYAML)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = yaml.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "PostgreSQL:latest", results[0].Servers[0].Image)
}

func TestQueryWithRegistryTypeServersJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Set",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:latest"},
			{Type: "registry", Source: registryURL("com.docker.mcp/copilot-mcp")},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "copilot", "", OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "test-set", results[0].ID)
	assert.Len(t, results[0].Servers, 1)
	assert.Equal(t, ServerTypeRegistry, results[0].Servers[0].Type)
	assert.Equal(t, registryURL("com.docker.mcp/copilot-mcp"), results[0].Servers[0].Source)
}

func TestQueryWithRegistryTypeServersYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Set",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:latest"},
			{Type: "registry", Source: registryURL("com.docker.mcp/copilot-mcp")},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "copilot", "", OutputFormatYAML)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = yaml.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "test-set", results[0].ID)
	assert.Len(t, results[0].Servers, 1)
	assert.Equal(t, ServerTypeRegistry, results[0].Servers[0].Type)
	assert.Equal(t, registryURL("com.docker.mcp/copilot-mcp"), results[0].Servers[0].Source)
}

func TestQueryWithHeterogeneousMixJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "mixed-set",
		Name: "Mixed Set",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:latest"},
			{Type: "registry", Source: registryURL("com.docker.mcp/filesystem-mcp")},
			{Type: "image", Image: "redis:latest"},
			{Type: "registry", Source: registryURL("com.docker.mcp/github-mcp")},
			{Type: "image", Image: "nginx:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "mcp", "", OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "mixed-set", results[0].ID)
	assert.Len(t, results[0].Servers, 2)
	assert.Equal(t, ServerTypeRegistry, results[0].Servers[0].Type)
	assert.Equal(t, registryURL("com.docker.mcp/filesystem-mcp"), results[0].Servers[0].Source)
	assert.Equal(t, ServerTypeRegistry, results[0].Servers[1].Type)
	assert.Equal(t, registryURL("com.docker.mcp/github-mcp"), results[0].Servers[1].Source)
}

func TestQueryWithHeterogeneousMixYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "mixed-set",
		Name: "Mixed Set",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:latest"},
			{Type: "registry", Source: registryURL("com.docker.mcp/filesystem-mcp")},
			{Type: "image", Image: "redis:latest"},
			{Type: "registry", Source: registryURL("com.docker.mcp/github-mcp")},
			{Type: "image", Image: "nginx:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "mcp", "", OutputFormatYAML)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = yaml.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "mixed-set", results[0].ID)
	assert.Len(t, results[0].Servers, 2)
	assert.Equal(t, ServerTypeRegistry, results[0].Servers[0].Type)
	assert.Equal(t, registryURL("com.docker.mcp/filesystem-mcp"), results[0].Servers[0].Source)
	assert.Equal(t, ServerTypeRegistry, results[0].Servers[1].Type)
	assert.Equal(t, registryURL("com.docker.mcp/github-mcp"), results[0].Servers[1].Source)
}

func TestWorkingSetWithNoServersJSON(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "empty-set",
		Name:    "Empty Set",
		Servers: db.ServerList{},
	})
	require.NoError(t, err)

	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "non-empty-set",
		Name: "Non Empty Set",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "", "", OutputFormatJSON)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = json.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "non-empty-set", results[0].ID)
}

func TestWorkingSetWithNoServersYAML(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "empty-set",
		Name:    "Empty Set",
		Servers: db.ServerList{},
	})
	require.NoError(t, err)

	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "non-empty-set",
		Name: "Non Empty Set",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:latest"},
		},
	})
	require.NoError(t, err)

	output := captureStdout(func() {
		err := Servers(ctx, dao, "", "", OutputFormatYAML)
		require.NoError(t, err)
	})

	var results []SearchResult
	err = yaml.Unmarshal([]byte(output), &results)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "non-empty-set", results[0].ID)
}

type mockErrorDAO struct {
	searchError error
}

func (m *mockErrorDAO) SearchWorkingSets(_ context.Context, _ string, _ string) ([]db.WorkingSet, error) {
	return nil, m.searchError
}

func (m *mockErrorDAO) GetWorkingSet(_ context.Context, _ string) (*db.WorkingSet, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockErrorDAO) FindWorkingSetsByIDPrefix(_ context.Context, _ string) ([]db.WorkingSet, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockErrorDAO) ListWorkingSets(_ context.Context) ([]db.WorkingSet, error) {
	return nil, nil
}

func (m *mockErrorDAO) CreateWorkingSet(_ context.Context, _ db.WorkingSet) error {
	return nil
}

func (m *mockErrorDAO) UpdateWorkingSet(_ context.Context, _ db.WorkingSet) error {
	return nil
}

func (m *mockErrorDAO) RemoveWorkingSet(_ context.Context, _ string) error {
	return nil
}

func (m *mockErrorDAO) CreateCatalog(_ context.Context, _ db.Catalog) error {
	return nil
}

func (m *mockErrorDAO) DeleteCatalog(_ context.Context, _ string) error {
	return nil
}

func (m *mockErrorDAO) GetCatalog(_ context.Context, _ string) (*db.Catalog, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockErrorDAO) ListCatalogs(_ context.Context) ([]db.Catalog, error) {
	return nil, nil
}

func TestSearchDatabaseError(t *testing.T) {
	ctx := t.Context()
	mockDAO := &mockErrorDAO{
		searchError: fmt.Errorf("database connection failed"),
	}

	err := Servers(ctx, mockDAO, "test", "", OutputFormatJSON)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to search working sets")
	assert.Contains(t, err.Error(), "database connection failed")
}

func TestSearchInvalidFormat(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "test-set",
		Name: "Test Set",
		Servers: db.ServerList{
			{Type: "image", Image: "postgres:latest"},
		},
	})
	require.NoError(t, err)

	err = Servers(ctx, dao, "", "", OutputFormat("invalid"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}

func TestPrintSearchResultsHumanEmpty(t *testing.T) {
	output := captureStdout(func() {
		printSearchResultsHuman([]SearchResult{})
	})

	assert.Contains(t, output, "No working sets found")
}

func TestPrintSearchResultsHumanSingleServer(t *testing.T) {
	results := []SearchResult{
		{
			ID:   "test-set",
			Name: "Test Set",
			Servers: []Server{
				{Type: ServerTypeImage, Image: "postgres:latest"},
			},
		},
	}

	output := captureStdout(func() {
		printSearchResultsHuman(results)
	})

	assert.Contains(t, output, "test-set")
	assert.Contains(t, output, "postgres:latest")
	assert.Contains(t, output, "image")
}

func TestPrintSearchResultsHumanMultipleServersMultipleSets(t *testing.T) {
	results := []SearchResult{
		{
			ID:   "set-1",
			Name: "First Set",
			Servers: []Server{
				{Type: ServerTypeImage, Image: "postgres:latest"},
				{Type: ServerTypeRegistry, Source: registryURL("com.docker.mcp/filesystem-mcp")},
			},
		},
		{
			ID:   "set-2",
			Name: "Second Set",
			Servers: []Server{
				{Type: ServerTypeImage, Image: "redis:latest"},
			},
		},
	}

	output := captureStdout(func() {
		printSearchResultsHuman(results)
	})

	assert.Contains(t, output, "set-1")
	assert.Contains(t, output, "set-2")
	assert.Contains(t, output, "postgres:latest")
	assert.Contains(t, output, "redis:latest")
}
