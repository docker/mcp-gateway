package dcr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDCRManager_BasicOperations(t *testing.T) {
	helper := newFakeCredentialHelper()
	manager := NewManager(helper, "http://localhost:8080/callback")

	serverName := "test-server"
	client := Client{
		ServerName:   serverName,
		ProviderName: serverName,
		ClientID:     "test-client-id",
	}

	// Save via credentials
	err := manager.Credentials().SaveClient(serverName, client)
	require.NoError(t, err)

	// Get via manager
	retrieved, err := manager.GetDCRClient(serverName)
	require.NoError(t, err)
	assert.Equal(t, client.ClientID, retrieved.ClientID)

	// List via manager
	clients, err := manager.ListDCRClients()
	require.NoError(t, err)
	assert.Len(t, clients, 1)
	assert.Contains(t, clients, serverName)

	// Delete via manager
	err = manager.DeleteDCRClient(serverName)
	require.NoError(t, err)

	// Verify deletion
	_, err = manager.GetDCRClient(serverName)
	assert.Error(t, err)
}

func TestMergeScopes(t *testing.T) {
	tests := []struct {
		name           string
		requiredScopes []string
		userScopes     string
		expected       []string
	}{
		{
			name:           "no user scopes",
			requiredScopes: []string{"read", "write"},
			userScopes:     "",
			expected:       []string{"read", "write"},
		},
		{
			name:           "empty user scopes",
			requiredScopes: []string{"read", "write"},
			userScopes:     " ",
			expected:       []string{"read", "write"},
		},
		{
			name:           "user scopes with no overlap",
			requiredScopes: []string{"read", "write"},
			userScopes:     "admin delete",
			expected:       []string{"read", "write", "admin", "delete"},
		},
		{
			name:           "user scopes with partial overlap",
			requiredScopes: []string{"read", "write"},
			userScopes:     "write delete",
			expected:       []string{"read", "write", "delete"},
		},
		{
			name:           "user scopes with full overlap",
			requiredScopes: []string{"read", "write"},
			userScopes:     "read write",
			expected:       []string{"read", "write"},
		},
		{
			name:           "single user scope",
			requiredScopes: []string{"read"},
			userScopes:     "admin",
			expected:       []string{"read", "admin"},
		},
		{
			name:           "empty required scopes",
			requiredScopes: []string{},
			userScopes:     "read write",
			expected:       []string{"read", "write"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeScopes(tt.requiredScopes, tt.userScopes)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestDCRManager_RedirectURI(t *testing.T) {
	helper := newFakeCredentialHelper()
	redirectURI := "http://localhost:9000/oauth/callback"
	manager := NewManager(helper, redirectURI)

	// Verify redirect URI is stored
	assert.Equal(t, redirectURI, manager.redirectURI)
}
