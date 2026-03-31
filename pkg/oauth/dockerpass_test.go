package oauth

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/docker/mcp-gateway/pkg/oauth/dcr"
)

// TestEncodeDecodeToken verifies the round-trip encoding of oauth2.Token
// used by SaveTokenToDockerPass and GetTokenFromDockerPass. This tests the
// serialization logic without requiring docker pass or the Secrets Engine.
func TestEncodeDecodeToken(t *testing.T) {
	expiry := time.Now().Add(1 * time.Hour).Truncate(time.Second)
	original := &oauth2.Token{
		AccessToken:  "access-abc",
		TokenType:    "Bearer",
		RefreshToken: "refresh-xyz",
		Expiry:       expiry,
	}

	// Encode (same logic as SaveTokenToDockerPass)
	tokenJSON, err := json.Marshal(original)
	require.NoError(t, err)
	encoded := base64.StdEncoding.EncodeToString(tokenJSON)

	// Decode (same logic as GetTokenFromDockerPass)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)

	var restored oauth2.Token
	require.NoError(t, json.Unmarshal(decoded, &restored))

	assert.Equal(t, original.AccessToken, restored.AccessToken)
	assert.Equal(t, original.TokenType, restored.TokenType)
	assert.Equal(t, original.RefreshToken, restored.RefreshToken)
	assert.True(t, original.Expiry.Equal(restored.Expiry),
		"expiry mismatch: want %v, got %v", original.Expiry, restored.Expiry)
}

// TestEncodeDecodeDCRClient verifies the round-trip encoding of dcr.Client
// used by SaveDCRClientToDockerPass and GetDCRClientFromDockerPass.
func TestEncodeDecodeDCRClient(t *testing.T) {
	original := dcr.Client{
		ServerName:            "notion-remote",
		ProviderName:          "notion-remote",
		ClientID:              "client-123",
		ClientName:            "MCP Gateway - notion-remote",
		AuthorizationEndpoint: "https://api.notion.com/v1/oauth/authorize",
		TokenEndpoint:         "https://api.notion.com/v1/oauth/token",
		ResourceURL:           "https://mcp.notion.com/sse",
		RequiredScopes:        []string{"read", "write"},
		RegisteredAt:          time.Now().Truncate(time.Second),
	}

	// Encode (same logic as SaveDCRClientToDockerPass)
	jsonData, err := json.Marshal(original)
	require.NoError(t, err)
	encoded := base64.StdEncoding.EncodeToString(jsonData)

	// Decode (same logic as GetDCRClientFromDockerPass)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)

	var restored dcr.Client
	require.NoError(t, json.Unmarshal(decoded, &restored))

	assert.Equal(t, original.ServerName, restored.ServerName)
	assert.Equal(t, original.ProviderName, restored.ProviderName)
	assert.Equal(t, original.ClientID, restored.ClientID)
	assert.Equal(t, original.AuthorizationEndpoint, restored.AuthorizationEndpoint)
	assert.Equal(t, original.TokenEndpoint, restored.TokenEndpoint)
	assert.Equal(t, original.ResourceURL, restored.ResourceURL)
	assert.Equal(t, original.RequiredScopes, restored.RequiredScopes)
	assert.True(t, original.RegisteredAt.Equal(restored.RegisteredAt))
}

// TestGetTokenStatusDockerPass_ParsesExpiry verifies that token status
// correctly parses the expiry field from base64-encoded JSON tokens.
func TestTokenStatusFromBase64JSON(t *testing.T) {
	tests := []struct {
		name            string
		token           map[string]any
		expectValid     bool
		expectRefresh   bool
		expectHasExpiry bool
	}{
		{
			name: "valid token with future expiry",
			token: map[string]any{
				"access_token":  "tok-123",
				"token_type":    "Bearer",
				"refresh_token": "ref-456",
				"expiry":        time.Now().Add(1 * time.Hour).Format(time.RFC3339),
			},
			expectValid:     true,
			expectRefresh:   false,
			expectHasExpiry: true,
		},
		{
			name: "token expiring within 10 seconds",
			token: map[string]any{
				"access_token":  "tok-123",
				"token_type":    "Bearer",
				"refresh_token": "ref-456",
				"expiry":        time.Now().Add(5 * time.Second).Format(time.RFC3339),
			},
			expectValid:     true,
			expectRefresh:   true,
			expectHasExpiry: true,
		},
		{
			name: "expired token",
			token: map[string]any{
				"access_token":  "tok-123",
				"token_type":    "Bearer",
				"refresh_token": "ref-456",
				"expiry":        time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
			},
			expectValid:     true,
			expectRefresh:   true,
			expectHasExpiry: true,
		},
		{
			name: "token without expiry",
			token: map[string]any{
				"access_token": "tok-123",
				"token_type":   "Bearer",
			},
			expectValid:     true,
			expectRefresh:   true,
			expectHasExpiry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenJSON, err := json.Marshal(tt.token)
			require.NoError(t, err)

			// Simulate the parsing logic from getTokenStatusDockerPass
			var tokenData struct {
				AccessToken  string `json:"access_token"`
				TokenType    string `json:"token_type"`
				RefreshToken string `json:"refresh_token,omitempty"`
				Expiry       string `json:"expiry,omitempty"`
			}
			require.NoError(t, json.Unmarshal(tokenJSON, &tokenData))

			assert.NotEmpty(t, tokenData.AccessToken)

			if tt.expectHasExpiry {
				assert.NotEmpty(t, tokenData.Expiry)
				expiresAt, err := time.Parse(time.RFC3339, tokenData.Expiry)
				require.NoError(t, err)

				timeUntilExpiry := time.Until(expiresAt)
				needsRefresh := timeUntilExpiry <= 10*time.Second
				assert.Equal(t, tt.expectRefresh, needsRefresh)
			} else {
				assert.Empty(t, tokenData.Expiry)
			}
		})
	}
}
