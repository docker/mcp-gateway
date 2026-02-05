package oauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/docker/docker-credential-helpers/client"
	"github.com/docker/docker-credential-helpers/credentials"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/secret"
	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/oauth/dcr"
)

// CredentialHelper provides secure access to OAuth tokens via credential helpers
type CredentialHelper struct {
	credentialHelper credentials.Helper
}

// GetHelper returns the underlying credential helper
func (h *CredentialHelper) GetHelper() credentials.Helper {
	return h.credentialHelper
}

// NewOAuthCredentialHelper creates a new OAuth credential helper
func NewOAuthCredentialHelper() *CredentialHelper {
	return &CredentialHelper{
		credentialHelper: newOAuthHelper(),
	}
}

// TokenStatus represents the validity status of an OAuth token
type TokenStatus struct {
	Valid        bool
	ExpiresAt    time.Time
	NeedsRefresh bool
}

// GetOAuthToken retrieves an OAuth token for the specified server
func (h *CredentialHelper) GetOAuthToken(ctx context.Context, serverName string) (string, error) {
	if IsCEMode() {
		return h.getOAuthTokenCE(serverName)
	}
	return h.getOAuthTokenDesktop(ctx, serverName)
}

// getOAuthTokenCE retrieves OAuth token in CE mode using credential helper.
// The credential helper returns base64-encoded JSON containing the access token.
func (h *CredentialHelper) getOAuthTokenCE(serverName string) (string, error) {
	dcrMgr := dcr.NewManager(h.credentialHelper, "")
	client, err := dcrMgr.GetDCRClient(serverName)
	if err != nil {
		return "", fmt.Errorf("no DCR client found for %s: %w", serverName, err)
	}
	credentialKey := fmt.Sprintf("%s/%s", client.AuthorizationEndpoint, client.ProviderName)

	_, tokenSecret, err := h.credentialHelper.Get(credentialKey)
	if err != nil {
		if credentials.IsErrCredentialsNotFound(err) {
			return "", fmt.Errorf("OAuth token not found for %s. Run 'docker mcp oauth authorize %s' to authenticate", serverName, serverName)
		}
		return "", fmt.Errorf("failed to retrieve OAuth token for %s: %w", serverName, err)
	}

	if tokenSecret == "" {
		return "", fmt.Errorf("empty OAuth token found for %s", serverName)
	}

	// Decode base64-encoded JSON
	tokenJSON, err := base64.StdEncoding.DecodeString(tokenSecret)
	if err != nil {
		return "", fmt.Errorf("failed to decode OAuth token for %s: %w", serverName, err)
	}

	// Parse JSON to extract the access token
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

// getOAuthTokenDesktop retrieves OAuth token in Desktop mode using Secrets Engine.
// The Secrets Engine returns the raw access token directly (Go auto-decodes base64 into []byte).
func (h *CredentialHelper) getOAuthTokenDesktop(ctx context.Context, serverName string) (string, error) {
	oauthID := secret.GetOAuthKey(serverName)
	env, err := secret.GetSecret(ctx, oauthID)
	if errors.Is(err, secret.ErrSecretNotFound) {
		return "", fmt.Errorf("OAuth token not found for %s. Run 'docker mcp oauth authorize %s' to authenticate", serverName, serverName)
	}
	if err != nil {
		return "", fmt.Errorf("failed to query Secrets Engine: %w", err)
	}

	tokenSecret := string(env.Value)
	if tokenSecret == "" {
		return "", fmt.Errorf("empty OAuth token found for %s", serverName)
	}
	return tokenSecret, nil
}

// TokenExists checks if an OAuth token exists for the specified server.
// This is the appropriate check for Desktop mode where Secrets Engine returns
// raw tokens without validity or expiry metadata - we can only verify existence.
// For CE mode, this also works but GetTokenStatus provides more detail.
func (h *CredentialHelper) TokenExists(ctx context.Context, serverName string) (bool, error) {
	if IsCEMode() {
		// CE mode: check credential helper
		dcrMgr := dcr.NewManager(h.credentialHelper, "")
		client, err := dcrMgr.GetDCRClient(serverName)
		if err != nil {
			return false, nil // No DCR client = no token
		}
		credentialKey := fmt.Sprintf("%s/%s", client.AuthorizationEndpoint, client.ProviderName)
		_, tokenSecret, err := h.credentialHelper.Get(credentialKey)
		if err != nil || tokenSecret == "" {
			return false, nil
		}
		return true, nil
	}

	// Desktop mode: check Secrets Engine
	oauthID := secret.GetOAuthKey(serverName)
	env, err := secret.GetSecret(ctx, oauthID)
	if errors.Is(err, secret.ErrSecretNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to query Secrets Engine: %w", err)
	}
	return string(env.Value) != "", nil
}

// GetTokenStatus checks token validity and expiry for refresh scheduling.
// CE mode only - requires JSON token format with expiry metadata.
// In Desktop mode, use TokenExists() instead since Secrets Engine returns
// raw tokens without expiry information.
func (h *CredentialHelper) GetTokenStatus(_ context.Context, serverName string) (TokenStatus, error) {
	if !IsCEMode() {
		return TokenStatus{}, fmt.Errorf("GetTokenStatus is only available in CE mode; use TokenExists() for Desktop mode")
	}

	// CE mode: Use credential helper directly
	dcrMgr := dcr.NewManager(h.credentialHelper, "")
	client, err := dcrMgr.GetDCRClient(serverName)
	if err != nil {
		return TokenStatus{Valid: false}, fmt.Errorf("no DCR client found for %s: %w", serverName, err)
	}
	credentialKey := fmt.Sprintf("%s/%s", client.AuthorizationEndpoint, client.ProviderName)

	_, tokenSecret, err := h.credentialHelper.Get(credentialKey)
	if err != nil {
		if credentials.IsErrCredentialsNotFound(err) {
			return TokenStatus{Valid: false}, fmt.Errorf("OAuth token not found for %s", serverName)
		}
		return TokenStatus{Valid: false}, fmt.Errorf("failed to retrieve OAuth token for %s: %w", serverName, err)
	}

	if tokenSecret == "" {
		return TokenStatus{Valid: false}, fmt.Errorf("empty OAuth token found for %s", serverName)
	}

	// CE mode: credential helper returns base64-encoded JSON with expiry
	tokenJSON, err := base64.StdEncoding.DecodeString(tokenSecret)
	if err != nil {
		return TokenStatus{Valid: false}, fmt.Errorf("failed to decode OAuth token for %s: %w", serverName, err)
	}

	// Parse the JSON to extract token data including expiry
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

	// Parse expiry time from RFC3339 string
	var expiresAt time.Time
	if tokenData.Expiry != "" {
		var err error
		expiresAt, err = time.Parse(time.RFC3339, tokenData.Expiry)
		if err != nil {
			return TokenStatus{Valid: false}, fmt.Errorf("failed to parse expiry time for %s: %w", serverName, err)
		}
	} else {
		// No expiry information - assume token is valid but check immediately
		return TokenStatus{
			Valid:        true,
			ExpiresAt:    time.Time{},
			NeedsRefresh: true,
		}, nil
	}

	// Check if token needs refresh (expires within 10 seconds)
	// Otherwise attempting to refresh earlier will return cached token
	now := time.Now()
	timeUntilExpiry := expiresAt.Sub(now)
	needsRefresh := timeUntilExpiry <= 10*time.Second

	log.Logf("- Token status for %s: valid=true, expires_at=%s, time_until_expiry=%v, needs_refresh=%v",
		serverName, expiresAt.Format(time.RFC3339), timeUntilExpiry.Round(time.Second), needsRefresh)

	return TokenStatus{
		Valid:        true,
		ExpiresAt:    expiresAt,
		NeedsRefresh: needsRefresh,
	}, nil
}

// newOAuthHelper creates a READ-ONLY credential helper for OAuth token access
// This is used by existing Gateway code that only reads tokens
func newOAuthHelper() credentials.Helper {
	helperName := getCredentialHelperName()
	if helperName == "" {
		log.Logf("! No credential helper found")
		log.Logf("! Install a credential helper from: https://github.com/docker/docker-credential-helpers")
		log.Logf("! Then configure Docker to use it (see repo for platform-specific instructions)")
		// Return a helper that will fail with clear error messages
		helperName = "notfound"
	}

	log.Logf("- Using credential helper: docker-credential-%s", helperName)
	return oauthHelper{
		program: newShellProgramFunc("docker-credential-" + helperName),
	}
}

// NewReadWriteCredentialHelper creates a READ-WRITE credential helper for CE mode
// This is used for DCR client storage and token storage operations
func NewReadWriteCredentialHelper() credentials.Helper {
	helperName := getCredentialHelperName()
	if helperName == "" {
		// Return a helper that will fail with clear errors
		helperName = "notfound"
	}

	// Return the actual client implementation (read-write)
	program := newShellProgramFunc("docker-credential-" + helperName)
	return &readWriteHelper{program: program}
}

// readWriteHelper is a full read-write credential helper
type readWriteHelper struct {
	program client.ProgramFunc
}

func (h *readWriteHelper) Add(creds *credentials.Credentials) error {
	return client.Store(h.program, creds)
}

func (h *readWriteHelper) Delete(serverURL string) error {
	return client.Erase(h.program, serverURL)
}

func (h *readWriteHelper) Get(serverURL string) (string, string, error) {
	creds, err := client.Get(h.program, serverURL)
	if err != nil {
		return "", "", err
	}
	return creds.Username, creds.Secret, nil
}

func (h *readWriteHelper) List() (map[string]string, error) {
	return client.List(h.program)
}

var _ credentials.Helper = &readWriteHelper{}

// getCredentialHelperName returns the credential helper to use
func getCredentialHelperName() string {
	resolver := NewResolver()
	helperName, err := resolver.Resolve()
	if err != nil {
		log.Logf("! %v", err)
		return ""
	}

	if helperName == "" {
		log.Logf("! No credential helper found")
		return ""
	}

	log.Logf("- Using credential helper: docker-credential-%s", helperName)
	return helperName
}

// newShellProgramFunc creates programs that are executed in a Shell.
func newShellProgramFunc(name string) client.ProgramFunc {
	return func(args ...string) client.Program {
		return &shell{cmd: exec.CommandContext(context.Background(), name, args...)}
	}
}

// shell invokes shell commands to talk with a remote credentials-helper.
type shell struct {
	cmd *exec.Cmd
}

// Output returns responses from the remote credentials-helper.
func (s *shell) Output() ([]byte, error) {
	return s.cmd.Output()
}

// Input sets the input to send to a remote credentials-helper.
func (s *shell) Input(in io.Reader) {
	s.cmd.Stdin = in
}

// oauthHelper wraps credential helper program for OAuth token access.
type oauthHelper struct {
	program client.ProgramFunc
}

func (h oauthHelper) List() (map[string]string, error) {
	return map[string]string{}, nil
}

// Add stores new credentials (not used for OAuth token retrieval)
func (h oauthHelper) Add(_ *credentials.Credentials) error {
	return fmt.Errorf("OAuth credential helper is read-only")
}

// Delete removes credentials (not used for OAuth token retrieval)
func (h oauthHelper) Delete(_ string) error {
	return fmt.Errorf("OAuth credential helper is read-only")
}

// Get returns the OAuth token for a given credential key
func (h oauthHelper) Get(credentialKey string) (string, string, error) {
	creds, err := client.Get(h.program, credentialKey)
	if err != nil {
		return "", "", err
	}
	return creds.Username, creds.Secret, nil
}

var _ credentials.Helper = oauthHelper{}
