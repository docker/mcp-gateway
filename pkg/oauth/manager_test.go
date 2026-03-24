package oauth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/oauth/dcr"
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

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	helper := newFakeCredentialHelper()
	manager := NewManager(helper)
	return manager
}

func setupTestDCRClient(t *testing.T, manager *Manager, serverName string) {
	t.Helper()

	client := dcr.Client{
		ServerName:            serverName,
		ProviderName:          serverName,
		ClientID:              "test-client-id-123",
		ClientName:            "MCP Gateway - " + serverName,
		AuthorizationEndpoint: "https://auth.example.com/authorize",
		TokenEndpoint:         "https://auth.example.com/token",
		ResourceURL:           "https://api.example.com",
		ScopesSupported:       []string{"read", "write"},
		RequiredScopes:        []string{"read"},
		RegisteredAt:          time.Now(),
	}

	err := manager.dcrManager.Credentials().SaveClient(serverName, client)
	require.NoError(t, err)
}

func TestManager_BuildAuthURL_StateFormat(t *testing.T) {
	manager := setupTestManager(t)
	serverName := "test-server"

	// Setup DCR client
	setupTestDCRClient(t, manager, serverName)

	tests := []struct {
		name         string
		callbackURL  string
		expectPrefix string
	}{
		{
			name:         "with callback URL",
			callbackURL:  "http://localhost:8080/callback",
			expectPrefix: "mcp-gateway:8080:",
		},
		{
			name:         "with different port",
			callbackURL:  "http://localhost:9000/callback",
			expectPrefix: "mcp-gateway:9000:",
		},
		{
			name:         "without callback URL",
			callbackURL:  "",
			expectPrefix: "", // Just UUID, no prefix
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authURL, baseState, verifier, err := manager.BuildAuthorizationURL(
				context.Background(),
				serverName,
				[]string{"read"},
				tt.callbackURL,
			)

			require.NoError(t, err)
			assert.NotEmpty(t, authURL)
			assert.NotEmpty(t, baseState)
			assert.NotEmpty(t, verifier)

			// Verify state format in URL
			if tt.expectPrefix != "" {
				// State is URL-encoded, so check for URL-encoded version
				// ":" becomes "%3A" in URL encoding
				expectedEncoded := strings.ReplaceAll(tt.expectPrefix, ":", "%3A")
				assert.Contains(t, authURL, "state="+expectedEncoded)
			} else {
				// Without callback URL, state should be just the UUID (baseState)
				assert.Contains(t, authURL, "state="+baseState)
			}
		})
	}
}

func TestManager_BuildAuthURL_PKCE(t *testing.T) {
	manager := setupTestManager(t)
	serverName := "test-server"

	// Setup DCR client
	setupTestDCRClient(t, manager, serverName)

	authURL, _, verifier, err := manager.BuildAuthorizationURL(
		context.Background(),
		serverName,
		[]string{"read"},
		"",
	)

	require.NoError(t, err)
	assert.NotEmpty(t, verifier)

	// Verify PKCE parameters in URL
	assert.Contains(t, authURL, "code_challenge=")
	assert.Contains(t, authURL, "code_challenge_method=S256")
}

func TestManager_BuildAuthURL_Resource(t *testing.T) {
	manager := setupTestManager(t)
	serverName := "test-server"

	// Setup DCR client
	setupTestDCRClient(t, manager, serverName)

	authURL, _, _, err := manager.BuildAuthorizationURL(
		context.Background(),
		serverName,
		[]string{"read"},
		"",
	)

	require.NoError(t, err)

	// Verify resource parameter in URL (RFC 8707)
	assert.Contains(t, authURL, "resource=https%3A%2F%2Fapi.example.com")
}

func TestManager_BuildAuthURL_NoDCRClient(t *testing.T) {
	manager := setupTestManager(t)

	// Try to build auth URL without DCR client
	_, _, _, err := manager.BuildAuthorizationURL(
		context.Background(),
		"non-existent-server",
		[]string{"read"},
		"",
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "DCR client not found")
}

func TestManager_BuildAuthURL_InvalidCallbackURL(t *testing.T) {
	manager := setupTestManager(t)
	serverName := "test-server"

	// Setup DCR client
	setupTestDCRClient(t, manager, serverName)

	// Try with invalid callback URL (no port)
	_, _, _, err := manager.BuildAuthorizationURL(
		context.Background(),
		serverName,
		[]string{"read"},
		"http://localhost/callback",
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "callback URL missing port")
}

func TestManager_ExchangeCode_InvalidState(t *testing.T) {
	manager := setupTestManager(t)

	// Try to exchange code with invalid state
	err := manager.ExchangeCode(
		context.Background(),
		"test-code",
		"invalid-state-uuid",
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid state parameter")
}

func TestManager_SetRedirectURI(t *testing.T) {
	manager := setupTestManager(t)

	customURI := "http://custom.example.com/callback"
	manager.SetRedirectURI(customURI)

	assert.Equal(t, customURI, manager.redirectURI)
}

func TestNewManagerWithRedirectURI(t *testing.T) {
	helper := newFakeCredentialHelper()
	localhostURI := "http://localhost:5001/callback"

	manager := NewManagerWithRedirectURI(helper, localhostURI)

	assert.Equal(t, localhostURI, manager.redirectURI)
	assert.NotNil(t, manager.dcrManager)
	assert.NotNil(t, manager.tokenStore)
	assert.NotNil(t, manager.stateManager)
}

func TestManager_EnsureDCRClient_AlreadyExists(t *testing.T) {
	manager := setupTestManager(t)
	serverName := "test-server"

	// Setup DCR client
	setupTestDCRClient(t, manager, serverName)

	// EnsureDCRClient should succeed without re-registration
	err := manager.EnsureDCRClient(context.Background(), serverName, "read")
	require.NoError(t, err)
}

func TestManager_StateValidation(t *testing.T) {
	manager := setupTestManager(t)

	// Generate state
	state := manager.stateManager.Generate("test-server", "test-verifier")
	assert.NotEmpty(t, state)

	// Validate state
	serverName, verifier, err := manager.stateManager.Validate(state)
	require.NoError(t, err)
	assert.Equal(t, "test-server", serverName)
	assert.Equal(t, "test-verifier", verifier)

	// State should be single-use
	_, _, err = manager.stateManager.Validate(state)
	assert.Error(t, err)
}

func TestManager_BuildAuthURL_ScopeOverride(t *testing.T) {
	manager := setupTestManager(t)
	serverName := "test-server"

	// Setup DCR client with default scopes
	setupTestDCRClient(t, manager, serverName)

	customScopes := []string{"admin", "delete", "write"}

	authURL, _, _, err := manager.BuildAuthorizationURL(
		context.Background(),
		serverName,
		customScopes,
		"",
	)

	require.NoError(t, err)

	// Verify custom scopes are in URL
	for _, scope := range customScopes {
		assert.Contains(t, authURL, scope)
	}
}

func TestManager_CallbackURLParsing(t *testing.T) {
	manager := setupTestManager(t)
	serverName := "test-server"

	setupTestDCRClient(t, manager, serverName)

	tests := []struct {
		name        string
		callbackURL string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid URL with port",
			callbackURL: "http://localhost:8080/callback",
			expectError: false,
		},
		{
			name:        "invalid URL",
			callbackURL: "://invalid",
			expectError: true,
			errorMsg:    "invalid callback URL",
		},
		{
			name:        "URL without port",
			callbackURL: "http://localhost/callback",
			expectError: true,
			errorMsg:    "callback URL missing port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := manager.BuildAuthorizationURL(
				context.Background(),
				serverName,
				[]string{"read"},
				tt.callbackURL,
			)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestManager_StateFormatWithPort(t *testing.T) {
	manager := setupTestManager(t)
	serverName := "test-server"

	setupTestDCRClient(t, manager, serverName)

	authURL, baseState, _, err := manager.BuildAuthorizationURL(
		context.Background(),
		serverName,
		[]string{"read"},
		"http://localhost:8080/callback",
	)

	require.NoError(t, err)

	// Extract state from URL
	parts := strings.Split(authURL, "state=")
	require.Len(t, parts, 2)

	statePart := strings.Split(parts[1], "&")[0]

	// State should have format: mcp-gateway:8080:UUID
	assert.True(t, strings.HasPrefix(statePart, "mcp-gateway%3A8080%3A") || strings.HasPrefix(statePart, "mcp-gateway:8080:"))

	// Base state should be just the UUID (no prefix)
	assert.NotContains(t, baseState, "mcp-gateway")
	assert.NotContains(t, baseState, ":")
}
