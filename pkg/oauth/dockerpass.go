package oauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/oauth2"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/secret"
	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/oauth/dcr"
)

// Docker pass stores tokens and DCR clients in the OS Keychain at well-known
// key paths. Writes use the `docker pass set` CLI command (via the secret
// package). Reads go through the Secrets Engine API, which aggregates all
// providers including the docker-pass plugin.
//
// Plugin resolution: both the docker-pass plugin (pattern `**`) and the
// docker-desktop-mcp-oauth plugin (pattern `docker/mcp/oauth/**`) match the
// token key path. For community servers the token is written via `docker pass`,
// so only the docker-pass plugin has an entry; the Desktop OAuth plugin returns
// "not found" and the Secrets Engine falls through to docker-pass. For catalog
// servers written by Desktop's OAuth manager, the Desktop plugin responds first
// (more specific pattern). This asymmetry is what makes the same GetSecret call
// return the right value for both modes.
//
// Token format: base64-encoded JSON of oauth2.Token (same as CE mode).
// DCR format:   base64-encoded JSON of dcr.Client (same as CE mode).

// --- CredentialHelper methods for docker pass (community mode) ---

// getOAuthTokenDockerPass retrieves an OAuth access token for a community
// server stored in docker pass. The Secrets Engine's docker-pass plugin
// returns the raw stored value at docker/mcp/oauth/{server}.
func (h *CredentialHelper) getOAuthTokenDockerPass(ctx context.Context, serverName string) (string, error) {
	oauthID := secret.GetOAuthKey(serverName)
	env, err := secret.GetSecret(ctx, oauthID)
	if errors.Is(err, secret.ErrSecretNotFound) {
		return "", fmt.Errorf("OAuth token not found for %s. Run 'docker mcp oauth authorize %s' to authenticate", serverName, serverName)
	}
	if err != nil {
		return "", fmt.Errorf("failed to query Secrets Engine for %s: %w", serverName, err)
	}

	storedValue := string(env.Value)
	if storedValue == "" {
		return "", fmt.Errorf("empty OAuth token found for %s", serverName)
	}

	// Docker pass stores base64-encoded JSON of the full oauth2.Token.
	tokenJSON, err := base64.StdEncoding.DecodeString(storedValue)
	if err != nil {
		return "", fmt.Errorf("failed to decode OAuth token for %s: %w", serverName, err)
	}

	var tokenData struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal(tokenJSON, &tokenData); err != nil {
		return "", fmt.Errorf("failed to parse OAuth token JSON for %s: %w", serverName, err)
	}

	if tokenData.AccessToken == "" {
		return "", fmt.Errorf("empty OAuth access token found for %s", serverName)
	}

	return tokenData.AccessToken, nil
}

// tokenExistsDockerPass checks whether a token exists in docker pass for a
// community server. Reads via the Secrets Engine.
func (h *CredentialHelper) tokenExistsDockerPass(ctx context.Context, serverName string) (bool, error) {
	oauthID := secret.GetOAuthKey(serverName)
	env, err := secret.GetSecret(ctx, oauthID)
	if errors.Is(err, secret.ErrSecretNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to query Secrets Engine for %s: %w", serverName, err)
	}
	return string(env.Value) != "", nil
}

// getTokenStatusDockerPass retrieves token validity and expiry from docker
// pass for a community server. The stored value is base64-encoded JSON
// containing the expiry field.
func (h *CredentialHelper) getTokenStatusDockerPass(ctx context.Context, serverName string) (TokenStatus, error) {
	oauthID := secret.GetOAuthKey(serverName)
	env, err := secret.GetSecret(ctx, oauthID)
	if errors.Is(err, secret.ErrSecretNotFound) {
		return TokenStatus{Valid: false}, fmt.Errorf("OAuth token not found for %s", serverName)
	}
	if err != nil {
		return TokenStatus{Valid: false}, fmt.Errorf("failed to query Secrets Engine for %s: %w", serverName, err)
	}

	storedValue := string(env.Value)
	if storedValue == "" {
		return TokenStatus{Valid: false}, fmt.Errorf("empty OAuth token found for %s", serverName)
	}

	tokenJSON, err := base64.StdEncoding.DecodeString(storedValue)
	if err != nil {
		return TokenStatus{Valid: false}, fmt.Errorf("failed to decode OAuth token for %s: %w", serverName, err)
	}

	var tokenData struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token,omitempty"`
		Expiry       string `json:"expiry,omitempty"`
	}
	if err := json.Unmarshal(tokenJSON, &tokenData); err != nil {
		return TokenStatus{Valid: false}, fmt.Errorf("failed to parse OAuth token JSON for %s: %w", serverName, err)
	}

	if tokenData.AccessToken == "" {
		return TokenStatus{Valid: false}, fmt.Errorf("empty OAuth access token found for %s", serverName)
	}

	if tokenData.Expiry == "" {
		// No expiry -- token is valid but trigger immediate refresh check.
		return TokenStatus{
			Valid:        true,
			ExpiresAt:    time.Time{},
			NeedsRefresh: true,
		}, nil
	}

	expiresAt, err := time.Parse(time.RFC3339, tokenData.Expiry)
	if err != nil {
		return TokenStatus{Valid: false}, fmt.Errorf("failed to parse expiry time for %s: %w", serverName, err)
	}

	now := time.Now()
	timeUntilExpiry := expiresAt.Sub(now)
	needsRefresh := timeUntilExpiry <= 10*time.Second

	log.Logf("- Token status for %s (docker pass): valid=true, expires_at=%s, time_until_expiry=%v, needs_refresh=%v",
		serverName, expiresAt.Format(time.RFC3339), timeUntilExpiry.Round(time.Second), needsRefresh)

	return TokenStatus{
		Valid:        true,
		ExpiresAt:    expiresAt,
		NeedsRefresh: needsRefresh,
	}, nil
}

// --- Exported helpers for docker pass token and DCR operations ---

// GetTokenFromDockerPass retrieves the full oauth2.Token from docker pass via
// the Secrets Engine. Used by the refresh loop to get the current token
// (including refresh_token) before refreshing.
func GetTokenFromDockerPass(ctx context.Context, serverName string) (*oauth2.Token, error) {
	oauthID := secret.GetOAuthKey(serverName)
	env, err := secret.GetSecret(ctx, oauthID)
	if errors.Is(err, secret.ErrSecretNotFound) {
		return nil, fmt.Errorf("OAuth token not found for %s", serverName)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query Secrets Engine for %s: %w", serverName, err)
	}

	storedValue := string(env.Value)
	if storedValue == "" {
		return nil, fmt.Errorf("empty OAuth token found for %s", serverName)
	}

	tokenJSON, err := base64.StdEncoding.DecodeString(storedValue)
	if err != nil {
		return nil, fmt.Errorf("failed to decode OAuth token for %s: %w", serverName, err)
	}

	var token oauth2.Token
	if err := json.Unmarshal(tokenJSON, &token); err != nil {
		return nil, fmt.Errorf("failed to parse OAuth token for %s: %w", serverName, err)
	}

	return &token, nil
}

// SaveTokenToDockerPass stores an oauth2.Token in docker pass as base64-encoded
// JSON at docker/mcp/oauth/{serverName}. Used by the authorize flow and the
// refresh loop for community servers in Desktop mode.
func SaveTokenToDockerPass(ctx context.Context, serverName string, token *oauth2.Token) error {
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("marshalling token for %s: %w", serverName, err)
	}

	encoded := base64.StdEncoding.EncodeToString(tokenJSON)

	if err := secret.SetOAuthToken(ctx, serverName, encoded); err != nil {
		return fmt.Errorf("storing OAuth token for %s: %w", serverName, err)
	}

	log.Logf("- Stored OAuth token for %s (docker pass)", serverName)
	return nil
}

// GetDCRClientFromDockerPass retrieves a DCR client from docker pass via the
// Secrets Engine. The value is base64-encoded JSON of dcr.Client stored at
// docker/mcp/oauth-dcr/{serverName}.
func GetDCRClientFromDockerPass(ctx context.Context, serverName string) (dcr.Client, error) {
	dcrID := secret.GetDCRKey(serverName)
	env, err := secret.GetSecret(ctx, dcrID)
	if errors.Is(err, secret.ErrSecretNotFound) {
		return dcr.Client{}, fmt.Errorf("DCR client not found for %s", serverName)
	}
	if err != nil {
		return dcr.Client{}, fmt.Errorf("failed to query Secrets Engine for DCR client %s: %w", serverName, err)
	}

	storedValue := string(env.Value)
	if storedValue == "" {
		return dcr.Client{}, fmt.Errorf("empty DCR client found for %s", serverName)
	}

	jsonData, err := base64.StdEncoding.DecodeString(storedValue)
	if err != nil {
		return dcr.Client{}, fmt.Errorf("failed to decode DCR client for %s: %w", serverName, err)
	}

	var client dcr.Client
	if err := json.Unmarshal(jsonData, &client); err != nil {
		return dcr.Client{}, fmt.Errorf("failed to parse DCR client for %s: %w", serverName, err)
	}

	return client, nil
}

// SaveDCRClientToDockerPass stores a DCR client in docker pass as base64-encoded
// JSON at docker/mcp/oauth-dcr/{serverName}. Used by the authorize flow for
// community servers in Desktop mode.
func SaveDCRClientToDockerPass(ctx context.Context, serverName string, client dcr.Client) error {
	jsonData, err := json.Marshal(client)
	if err != nil {
		return fmt.Errorf("marshalling DCR client for %s: %w", serverName, err)
	}

	encoded := base64.StdEncoding.EncodeToString(jsonData)

	if err := secret.SetDCRClient(ctx, serverName, encoded); err != nil {
		return fmt.Errorf("storing DCR client for %s: %w", serverName, err)
	}

	log.Logf("- Stored DCR client for %s (docker pass)", serverName)
	return nil
}
