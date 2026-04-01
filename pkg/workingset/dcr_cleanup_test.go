package workingset

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/desktop"
)

// mockDCRClient tracks GetOAuthApp, GetDCRClient, and DeleteDCRClient calls for testing.
type mockDCRClient struct {
	// registered holds the set of apps that have DCR entries
	registered map[string]bool
	// authorized holds the set of apps where the user is still authorized
	authorized map[string]bool
	// deleted tracks which apps had DeleteDCRClient called
	deleted []string
}

func newMockDCRClient(apps ...string) *mockDCRClient {
	m := &mockDCRClient{
		registered: make(map[string]bool),
		authorized: make(map[string]bool),
	}
	for _, app := range apps {
		m.registered[app] = true
	}
	return m
}

func (m *mockDCRClient) withAuthorized(apps ...string) *mockDCRClient {
	for _, app := range apps {
		m.authorized[app] = true
	}
	return m
}

func (m *mockDCRClient) GetOAuthApp(_ context.Context, app string) (*desktop.OAuthApp, error) {
	if m.registered[app] {
		return &desktop.OAuthApp{
			App:        app,
			Authorized: m.authorized[app],
		}, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockDCRClient) GetDCRClient(_ context.Context, app string) (*desktop.DCRClient, error) {
	if m.registered[app] {
		return &desktop.DCRClient{ServerName: app, State: "registered"}, nil
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockDCRClient) DeleteDCRClient(_ context.Context, app string) error {
	delete(m.registered, app)
	m.deleted = append(m.deleted, app)
	return nil
}

func TestCleanupOrphanedDCREntries_DeletesOrphanedAndRevoked(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create a profile with one server
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "profile-1",
		Name: "Profile 1",
		Servers: db.ServerList{
			{
				Type: "remote",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{Name: "server-a"},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	// DCR entries exist for both; server-b is not authorized
	mock := newMockDCRClient("server-a", "server-b")

	// Remove server-b (not in any profile, not authorized) → should be deleted
	doCleanupOrphanedDCREntries(ctx, dao, mock, []string{"server-b"})

	assert.Equal(t, []string{"server-b"}, mock.deleted)
	assert.True(t, mock.registered["server-a"], "server-a should not be deleted")
}

func TestCleanupOrphanedDCREntries_SkipsAuthorized(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Empty profile (server was removed)
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "profile-1",
		Name:    "Profile 1",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	// server-a has DCR entry and is still authorized
	mock := newMockDCRClient("server-a").withAuthorized("server-a")

	// server-a is not in any profile but IS authorized → should NOT be deleted
	doCleanupOrphanedDCREntries(ctx, dao, mock, []string{"server-a"})

	assert.Empty(t, mock.deleted)
	assert.True(t, mock.registered["server-a"])
}

func TestCleanupOrphanedDCREntries_SkipsInUse(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Create two profiles, both containing server-a
	for _, id := range []string{"profile-1", "profile-2"} {
		err := dao.CreateWorkingSet(ctx, db.WorkingSet{
			ID:   id,
			Name: id,
			Servers: db.ServerList{
				{
					Type: "remote",
					Snapshot: &db.ServerSnapshot{
						Server: catalog.Server{Name: "server-a"},
					},
				},
			},
			Secrets: db.SecretMap{},
		})
		require.NoError(t, err)
	}

	mock := newMockDCRClient("server-a")

	// server-a is still in profile-2, should NOT be deleted
	doCleanupOrphanedDCREntries(ctx, dao, mock, []string{"server-a"})

	assert.Empty(t, mock.deleted)
	assert.True(t, mock.registered["server-a"])
}

func TestCleanupOrphanedDCREntries_SkipsNoDCREntry(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Empty profile
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "profile-1",
		Name:    "Profile 1",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	// No DCR entries registered
	mock := newMockDCRClient()

	// server-x has no DCR entry → GetDCRClient returns error → skip delete
	doCleanupOrphanedDCREntries(ctx, dao, mock, []string{"server-x"})

	assert.Empty(t, mock.deleted)
}

func TestCleanupOrphanedDCREntries_MultipleServers(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Profile with server-a only
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "profile-1",
		Name: "Profile 1",
		Servers: db.ServerList{
			{
				Type: "remote",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{Name: "server-a"},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	// DCR entries for all three; server-c is still authorized
	mock := newMockDCRClient("server-a", "server-b", "server-c").withAuthorized("server-c")

	// Remove server-a (still in profile), server-b (revoked), server-c (authorized)
	doCleanupOrphanedDCREntries(ctx, dao, mock, []string{"server-a", "server-b", "server-c"})

	// Only server-b should be deleted (server-a in profile, server-c authorized)
	assert.Equal(t, []string{"server-b"}, mock.deleted)
	assert.True(t, mock.registered["server-a"])
	assert.True(t, mock.registered["server-c"])
}

func TestCleanupOrphanedDCREntries_ServerInDifferentProfile(t *testing.T) {
	dao := setupTestDB(t)
	ctx := t.Context()

	// Profile 1 is now empty (server-a and server-b were already removed by RemoveServers)
	err := dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:      "profile-1",
		Name:    "Profile 1",
		Servers: db.ServerList{},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	// Profile 2 still has server-b
	err = dao.CreateWorkingSet(ctx, db.WorkingSet{
		ID:   "profile-2",
		Name: "Profile 2",
		Servers: db.ServerList{
			{
				Type: "remote",
				Snapshot: &db.ServerSnapshot{
					Server: catalog.Server{Name: "server-b"},
				},
			},
		},
		Secrets: db.SecretMap{},
	})
	require.NoError(t, err)

	mock := newMockDCRClient("server-a", "server-b")

	// Cleanup after removing server-a and server-b from profile-1
	// server-b is still in profile-2, so only server-a should be deleted
	doCleanupOrphanedDCREntries(ctx, dao, mock, []string{"server-a", "server-b"})

	assert.Equal(t, []string{"server-a"}, mock.deleted)
	assert.True(t, mock.registered["server-b"])
}
