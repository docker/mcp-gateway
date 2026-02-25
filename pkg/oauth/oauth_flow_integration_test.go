package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/docker/mcp-gateway/pkg/oauth/dcr"
)

// tokenExchangeRecord captures the form values sent to the mock token endpoint.
type tokenExchangeRecord struct {
	mu           sync.Mutex
	GrantType    string
	Code         string
	CodeVerifier string
	RedirectURI  string
	Resource     string
	Called       bool
	CalledCount  int
}

func (r *tokenExchangeRecord) record(req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_ = req.ParseForm()
	r.GrantType = req.Form.Get("grant_type")
	r.Code = req.Form.Get("code")
	r.CodeVerifier = req.Form.Get("code_verifier")
	r.RedirectURI = req.Form.Get("redirect_uri")
	r.Resource = req.Form.Get("resource")
	r.Called = true
	r.CalledCount++
}

// newMockTokenServer creates a mock OAuth token endpoint that returns a valid token.
// The returned tokenExchangeRecord captures the request for assertions.
func newMockTokenServer(t *testing.T) (*httptest.Server, *tokenExchangeRecord) {
	t.Helper()
	rec := &tokenExchangeRecord{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "mock-access-token",
			"refresh_token": "mock-refresh-token",
			"token_type":    "bearer",
			"expires_in":    3600,
		})
	}))
	t.Cleanup(server.Close)
	return server, rec
}

// newMockTokenServerError creates a mock token endpoint that returns an error.
func newMockTokenServerError(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "Authorization code expired",
		})
	}))
	t.Cleanup(server.Close)
	return server
}

// setupIntegrationManager creates a Manager with a fake credential helper and a
// DCR client whose TokenEndpoint and AuthorizationEndpoint point to mockServerURL.
func setupIntegrationManager(t *testing.T, serverName, tokenEndpointURL string) (*Manager, dcr.Client) {
	t.Helper()
	helper := newFakeCredentialHelper()
	manager := NewManager(helper)

	client := dcr.Client{
		ServerName:            serverName,
		ProviderName:          serverName,
		ClientID:              "test-client-id-integration",
		ClientName:            "MCP Gateway - " + serverName,
		AuthorizationEndpoint: "https://auth.example.com/authorize",
		TokenEndpoint:         tokenEndpointURL,
		ResourceURL:           "https://api.example.com",
		ScopesSupported:       []string{"read", "write"},
		RequiredScopes:        []string{"read"},
		RegisteredAt:          time.Now(),
	}

	err := manager.dcrManager.Credentials().SaveClient(serverName, client)
	require.NoError(t, err)

	return manager, client
}

// TestIntegration_FullCEOAuthFlow_HappyPath tests the full CE OAuth flow:
// DCR client setup -> build auth URL -> callback server receives code -> token exchange -> token stored.
func TestIntegration_FullCEOAuthFlow_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	getTestPort(t)

	// Create mock token endpoint
	tokenServer, rec := newMockTokenServer(t)

	// Setup manager with DCR client pointing to mock token endpoint
	serverName := "integration-test-server"
	manager, dcrClient := setupIntegrationManager(t, serverName, tokenServer.URL)

	// Start callback server
	callbackServer, err := NewCallbackServer()
	require.NoError(t, err)
	defer func() { _ = callbackServer.Shutdown(context.Background()) }()

	go func() { _ = callbackServer.Start() }()
	time.Sleep(50 * time.Millisecond)

	// Build authorization URL
	callbackURL := callbackServer.URL()
	authURL, baseState, verifier, err := manager.BuildAuthorizationURL(
		context.Background(),
		serverName,
		[]string{"read"},
		callbackURL,
	)
	require.NoError(t, err)
	assert.NotEmpty(t, authURL)
	assert.NotEmpty(t, baseState)
	assert.NotEmpty(t, verifier)

	// Verify auth URL contains expected parameters
	assert.Contains(t, authURL, "response_type=code")
	assert.Contains(t, authURL, "client_id=test-client-id-integration")
	assert.Contains(t, authURL, "code_challenge=")
	assert.Contains(t, authURL, "code_challenge_method=S256")
	assert.Contains(t, authURL, "scope=read")
	assert.Contains(t, authURL, "resource=")

	// Simulate OAuth provider redirect to callback server
	callbackState := fmt.Sprintf("mcp-gateway:%d:%s", callbackServer.Port(), baseState)
	callbackReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, fmt.Sprintf("http://localhost:%d/callback?code=mock-auth-code&state=%s", callbackServer.Port(), callbackState), nil)
	require.NoError(t, err)
	callbackResp, err := http.DefaultClient.Do(callbackReq)
	require.NoError(t, err)
	defer callbackResp.Body.Close()
	assert.Equal(t, http.StatusOK, callbackResp.StatusCode)

	// Wait for callback
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	code, state, err := callbackServer.Wait(ctx)
	require.NoError(t, err)
	assert.Equal(t, "mock-auth-code", code)
	assert.Equal(t, callbackState, state)

	// Exchange code for token -- manager.ExchangeCode validates against baseState
	err = manager.ExchangeCode(context.Background(), code, baseState)
	require.NoError(t, err)

	// Verify mock token endpoint was called correctly
	assert.True(t, rec.Called)
	assert.Equal(t, "authorization_code", rec.GrantType)
	assert.Equal(t, "mock-auth-code", rec.Code)
	assert.Equal(t, verifier, rec.CodeVerifier, "PKCE verifier should match")
	assert.Equal(t, "https://api.example.com", rec.Resource, "RFC 8707 resource param should be sent")

	// Verify token was stored in credential helper
	token, err := manager.tokenStore.Retrieve(dcrClient)
	require.NoError(t, err)
	assert.Equal(t, "mock-access-token", token.AccessToken)
	assert.Equal(t, "mock-refresh-token", token.RefreshToken)
}

// TestIntegration_CEOAuthFlow_TokenExchangeFailure tests that a token endpoint error
// is propagated and no token is stored.
func TestIntegration_CEOAuthFlow_TokenExchangeFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	getTestPort(t)

	// Create mock token endpoint that returns an error
	tokenServer := newMockTokenServerError(t)

	serverName := "exchange-error-server"
	manager, dcrClient := setupIntegrationManager(t, serverName, tokenServer.URL)

	// Build auth URL
	authURL, baseState, _, err := manager.BuildAuthorizationURL(
		context.Background(),
		serverName,
		[]string{"read"},
		"",
	)
	require.NoError(t, err)
	assert.NotEmpty(t, authURL)

	// Exchange code -- should fail because mock returns error
	err = manager.ExchangeCode(context.Background(), "mock-auth-code", baseState)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token exchange failed")

	// Verify no token was stored
	_, err = manager.tokenStore.Retrieve(dcrClient)
	require.Error(t, err)
}

// TestIntegration_CEOAuthFlow_CallbackError tests that an OAuth error callback
// (error=access_denied) is correctly propagated through the callback server.
func TestIntegration_CEOAuthFlow_CallbackError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	getTestPort(t)

	callbackServer, err := NewCallbackServer()
	require.NoError(t, err)
	defer func() { _ = callbackServer.Shutdown(context.Background()) }()

	go func() { _ = callbackServer.Start() }()
	time.Sleep(50 * time.Millisecond)

	// Simulate OAuth provider returning error
	errReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, fmt.Sprintf(
		"http://localhost:%d/callback?error=access_denied&error_description=User+denied+access",
		callbackServer.Port(),
	), nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(errReq)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Wait should return the error
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, err = callbackServer.Wait(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OAuth error: access_denied")
	assert.Contains(t, err.Error(), "User denied access")
}

// TestIntegration_CEOAuthFlow_CallbackTimeout tests that the callback server
// times out when no callback arrives.
func TestIntegration_CEOAuthFlow_CallbackTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	getTestPort(t)

	callbackServer, err := NewCallbackServer()
	require.NoError(t, err)
	defer func() { _ = callbackServer.Shutdown(context.Background()) }()

	go func() { _ = callbackServer.Start() }()
	time.Sleep(50 * time.Millisecond)

	// Wait with a short timeout -- no callback will arrive
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, _, err = callbackServer.Wait(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "callback timeout")
}

// TestIntegration_CEOAuthFlow_StateValidation tests that state is single-use
// and that a mismatched state is rejected.
func TestIntegration_CEOAuthFlow_StateValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tokenServer, _ := newMockTokenServer(t)

	serverName := "state-validation-server"
	manager, _ := setupIntegrationManager(t, serverName, tokenServer.URL)

	// Generate two states
	_, baseState1, _, err := manager.BuildAuthorizationURL(
		context.Background(),
		serverName,
		[]string{"read"},
		"",
	)
	require.NoError(t, err)

	_, baseState2, _, err := manager.BuildAuthorizationURL(
		context.Background(),
		serverName,
		[]string{"read"},
		"",
	)
	require.NoError(t, err)

	assert.NotEqual(t, baseState1, baseState2, "each call should generate a unique state")

	// Exchange with state1 -- should succeed
	err = manager.ExchangeCode(context.Background(), "code1", baseState1)
	require.NoError(t, err)

	// Exchange with state1 again -- should fail (single-use)
	err = manager.ExchangeCode(context.Background(), "code2", baseState1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid state parameter")

	// Exchange with bogus state -- should fail
	err = manager.ExchangeCode(context.Background(), "code3", "completely-bogus-state-uuid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid state parameter")

	// Exchange with state2 -- should still work
	err = manager.ExchangeCode(context.Background(), "code4", baseState2)
	require.NoError(t, err)
}

// TestIntegration_CEOAuthFlow_PKCEVerification tests that the PKCE verifier
// sent during token exchange matches the verifier generated during URL building.
func TestIntegration_CEOAuthFlow_PKCEVerification(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tokenServer, rec := newMockTokenServer(t)

	serverName := "pkce-test-server"
	manager, _ := setupIntegrationManager(t, serverName, tokenServer.URL)

	// Build auth URL and capture verifier
	_, baseState, verifier, err := manager.BuildAuthorizationURL(
		context.Background(),
		serverName,
		[]string{"read"},
		"",
	)
	require.NoError(t, err)
	assert.NotEmpty(t, verifier)

	// Verifier should be 43+ characters (per RFC 7636)
	assert.GreaterOrEqual(t, len(verifier), 43)

	// Exchange code -- this sends verifier to token endpoint
	err = manager.ExchangeCode(context.Background(), "auth-code", baseState)
	require.NoError(t, err)

	// Verify the verifier sent to token endpoint matches what was generated
	assert.True(t, rec.Called)
	assert.Equal(t, verifier, rec.CodeVerifier, "PKCE code_verifier must match the generated verifier")
}

// TestIntegration_CEOAuthFlow_TokenStoreRoundTrip tests the save/retrieve/delete
// cycle on the token store.
func TestIntegration_CEOAuthFlow_TokenStoreRoundTrip(t *testing.T) {
	helper := newFakeCredentialHelper()
	tokenStore := NewTokenStore(helper)

	dcrClient := dcr.Client{
		ServerName:            "roundtrip-server",
		ProviderName:          "roundtrip-server",
		ClientID:              "roundtrip-client-id",
		AuthorizationEndpoint: "https://auth.example.com/authorize",
		TokenEndpoint:         "https://auth.example.com/token",
	}

	now := time.Now().Truncate(time.Second)
	token := &oauth2.Token{
		AccessToken:  "save-access-token",
		RefreshToken: "save-refresh-token",
		TokenType:    "bearer",
		Expiry:       now.Add(1 * time.Hour),
	}

	// Save
	err := tokenStore.Save(dcrClient, token)
	require.NoError(t, err)

	// Retrieve
	retrieved, err := tokenStore.Retrieve(dcrClient)
	require.NoError(t, err)
	assert.Equal(t, "save-access-token", retrieved.AccessToken)
	assert.Equal(t, "save-refresh-token", retrieved.RefreshToken)

	// Delete
	err = tokenStore.Delete(dcrClient)
	require.NoError(t, err)

	// Retrieve after delete -- should fail
	_, err = tokenStore.Retrieve(dcrClient)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestIntegration_CEOAuthFlow_RevokeToken tests that RevokeToken removes the token
// from the credential helper.
func TestIntegration_CEOAuthFlow_RevokeToken(t *testing.T) {
	tokenServer, _ := newMockTokenServer(t)

	serverName := "revoke-test-server"
	manager, dcrClient := setupIntegrationManager(t, serverName, tokenServer.URL)

	// Build auth URL and exchange to get a stored token
	_, baseState, _, err := manager.BuildAuthorizationURL(
		context.Background(),
		serverName,
		[]string{"read"},
		"",
	)
	require.NoError(t, err)
	err = manager.ExchangeCode(context.Background(), "auth-code", baseState)
	require.NoError(t, err)

	// Verify token exists
	token, err := manager.tokenStore.Retrieve(dcrClient)
	require.NoError(t, err)
	assert.Equal(t, "mock-access-token", token.AccessToken)

	// Revoke
	err = manager.RevokeToken(context.Background(), serverName)
	require.NoError(t, err)

	// Verify token is gone
	_, err = manager.tokenStore.Retrieve(dcrClient)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
