package dcr

import (
	"fmt"
	"testing"
	"time"

	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCredentialHelper is a fake in-memory credential helper for testing
type fakeCredentialHelper struct {
	store map[string]*credentials.Credentials
}

func newFakeCredentialHelper() *fakeCredentialHelper {
	return &fakeCredentialHelper{
		store: make(map[string]*credentials.Credentials),
	}
}

func (f *fakeCredentialHelper) Add(creds *credentials.Credentials) error {
	f.store[creds.ServerURL] = creds
	return nil
}

func (f *fakeCredentialHelper) Delete(serverURL string) error {
	if _, exists := f.store[serverURL]; !exists {
		return credentials.NewErrCredentialsNotFound()
	}
	delete(f.store, serverURL)
	return nil
}

func (f *fakeCredentialHelper) Get(serverURL string) (string, string, error) {
	cred, exists := f.store[serverURL]
	if !exists {
		return "", "", credentials.NewErrCredentialsNotFound()
	}
	return cred.Username, cred.Secret, nil
}

func (f *fakeCredentialHelper) List() (map[string]string, error) {
	result := make(map[string]string)
	for url, cred := range f.store {
		result[url] = cred.Username
	}
	return result, nil
}

var _ credentials.Helper = &fakeCredentialHelper{}

func TestDCRCredentials_SaveRetrieve(t *testing.T) {
	helper := newFakeCredentialHelper()
	creds := NewCredentials(helper)

	client := Client{
		ServerName:            "notion-remote",
		ProviderName:          "notion-remote",
		ClientID:              "test-client-id-123",
		ClientName:            "MCP Gateway - notion-remote",
		AuthorizationEndpoint: "https://auth.example.com/authorize",
		TokenEndpoint:         "https://auth.example.com/token",
		ResourceURL:           "https://api.example.com",
		ScopesSupported:       []string{"read", "write"},
		RequiredScopes:        []string{"read"},
		RegisteredAt:          time.Now(),
	}

	// Save client
	err := creds.SaveClient("notion-remote", client)
	require.NoError(t, err)

	// Retrieve client
	retrieved, err := creds.RetrieveClient("notion-remote")
	require.NoError(t, err)

	// Verify round-trip
	assert.Equal(t, client.ServerName, retrieved.ServerName)
	assert.Equal(t, client.ProviderName, retrieved.ProviderName)
	assert.Equal(t, client.ClientID, retrieved.ClientID)
	assert.Equal(t, client.ClientName, retrieved.ClientName)
	assert.Equal(t, client.AuthorizationEndpoint, retrieved.AuthorizationEndpoint)
	assert.Equal(t, client.TokenEndpoint, retrieved.TokenEndpoint)
	assert.Equal(t, client.ResourceURL, retrieved.ResourceURL)
	assert.Equal(t, client.ScopesSupported, retrieved.ScopesSupported)
	assert.Equal(t, client.RequiredScopes, retrieved.RequiredScopes)
}

func TestDCRCredentials_KeyFormat(t *testing.T) {
	helper := newFakeCredentialHelper()
	creds := NewCredentials(helper)

	serverName := "test-server"
	client := Client{
		ServerName:   serverName,
		ProviderName: serverName,
		ClientID:     "test-client-id",
		RegisteredAt: time.Now(),
	}

	// Save client
	err := creds.SaveClient(serverName, client)
	require.NoError(t, err)

	// Verify key format in underlying store
	expectedKey := fmt.Sprintf("https://%s.mcp-dcr", serverName)
	_, exists := helper.store[expectedKey]
	assert.True(t, exists, "credential should be stored with key: %s", expectedKey)

	// Verify username
	storedCred := helper.store[expectedKey]
	assert.Equal(t, "dcr_client", storedCred.Username)
}

func TestDCRCredentials_NotFound(t *testing.T) {
	helper := newFakeCredentialHelper()
	creds := NewCredentials(helper)

	// Try to retrieve non-existent client
	_, err := creds.RetrieveClient("non-existent-server")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DCR client not found")
}

func TestDCRCredentials_List(t *testing.T) {
	helper := newFakeCredentialHelper()
	creds := NewCredentials(helper)

	// Add multiple DCR clients
	servers := []string{"notion-remote", "github-server", "slack-api"}
	for _, serverName := range servers {
		client := Client{
			ServerName:   serverName,
			ProviderName: serverName,
			ClientID:     fmt.Sprintf("client-id-%s", serverName),
			RegisteredAt: time.Now(),
		}
		err := creds.SaveClient(serverName, client)
		require.NoError(t, err)
	}

	// Add non-DCR credential to verify filtering
	helper.store["https://docker.io"] = &credentials.Credentials{
		ServerURL: "https://docker.io",
		Username:  "testuser",
		Secret:    "testsecret",
	}

	// List clients
	clients, err := creds.ListClients()
	require.NoError(t, err)

	// Should only return DCR clients (3 items)
	assert.Len(t, clients, 3)

	// Verify all DCR clients are present
	for _, serverName := range servers {
		client, exists := clients[serverName]
		assert.True(t, exists, "client %s should be in list", serverName)
		assert.Equal(t, serverName, client.ServerName)
	}

	// Verify non-DCR credential is not included
	_, exists := clients["docker.io"]
	assert.False(t, exists, "non-DCR credential should not be in list")
}

func TestDCRCredentials_Delete(t *testing.T) {
	helper := newFakeCredentialHelper()
	creds := NewCredentials(helper)

	serverName := "test-server"
	client := Client{
		ServerName:   serverName,
		ProviderName: serverName,
		ClientID:     "test-client-id",
		RegisteredAt: time.Now(),
	}

	// Save client
	err := creds.SaveClient(serverName, client)
	require.NoError(t, err)

	// Verify it exists
	_, err = creds.RetrieveClient(serverName)
	require.NoError(t, err)

	// Delete client
	err = creds.DeleteClient(serverName)
	require.NoError(t, err)

	// Verify it's gone
	_, err = creds.RetrieveClient(serverName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DCR client not found")
}

func TestDCRCredentials_DeleteNonExistent(t *testing.T) {
	helper := newFakeCredentialHelper()
	creds := NewCredentials(helper)

	// Try to delete non-existent client
	err := creds.DeleteClient("non-existent-server")
	require.Error(t, err)
}
