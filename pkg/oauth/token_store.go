package oauth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/docker/docker-credential-helpers/credentials"
	"golang.org/x/oauth2"

	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/oauth/dcr"
)

// TokenStore provides storage for OAuth tokens via credential helper
type TokenStore struct {
	credentialHelper credentials.Helper
}

// NewTokenStore creates a new token store
func NewTokenStore(credentialHelper credentials.Helper) *TokenStore {
	return &TokenStore{
		credentialHelper: credentialHelper,
	}
}

// Save stores an OAuth token in the credential helper
// Key format: {authorizationEndpoint}/{providerName}
func (t *TokenStore) Save(dcrClient dcr.Client, token *oauth2.Token) error {
	// Marshal token to JSON
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("marshalling token: %w", err)
	}

	// Base64 encode
	encoded := base64.StdEncoding.EncodeToString(tokenJSON)

	// Credential key format: {authorizationEndpoint}/{providerName}
	key := fmt.Sprintf("%s/%s", dcrClient.AuthorizationEndpoint, dcrClient.ProviderName)

	cred := &credentials.Credentials{
		ServerURL: key,
		Username:  fmt.Sprintf("oauth2_%s", dcrClient.ProviderName),
		Secret:    encoded,
	}

	if err := t.credentialHelper.Add(cred); err != nil {
		return fmt.Errorf("storing token for %s: %w", dcrClient.ServerName, err)
	}

	log.Logf("- Stored OAuth token for %s", dcrClient.ServerName)
	return nil
}

// Retrieve retrieves an OAuth token from the credential helper
func (t *TokenStore) Retrieve(dcrClient dcr.Client) (*oauth2.Token, error) {
	// Credential key format: {authorizationEndpoint}/{providerName}
	key := fmt.Sprintf("%s/%s", dcrClient.AuthorizationEndpoint, dcrClient.ProviderName)

	_, encoded, err := t.credentialHelper.Get(key)
	if err != nil {
		if credentials.IsErrCredentialsNotFound(err) {
			return nil, fmt.Errorf("token not found for %s", dcrClient.ServerName)
		}
		return nil, fmt.Errorf("retrieving token for %s: %w", dcrClient.ServerName, err)
	}

	// Base64 decode
	tokenJSON, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decoding token for %s: %w", dcrClient.ServerName, err)
	}

	// Unmarshal token
	var token oauth2.Token
	if err := json.Unmarshal(tokenJSON, &token); err != nil {
		return nil, fmt.Errorf("unmarshalling token for %s: %w", dcrClient.ServerName, err)
	}

	return &token, nil
}

// Delete removes an OAuth token from the credential helper
func (t *TokenStore) Delete(dcrClient dcr.Client) error {
	key := fmt.Sprintf("%s/%s", dcrClient.AuthorizationEndpoint, dcrClient.ProviderName)

	if err := t.credentialHelper.Delete(key); err != nil {
		return fmt.Errorf("deleting token for %s: %w", dcrClient.ServerName, err)
	}

	log.Logf("- Deleted OAuth token for %s", dcrClient.ServerName)
	return nil
}
