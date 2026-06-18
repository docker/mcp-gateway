package oauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Verify SDKHandler satisfies the auth.OAuthHandler interface at compile time.
var _ auth.OAuthHandler = (*SDKHandler)(nil)

func TestSDKHandler_TokenSource_NoToken(t *testing.T) {
	handler := &SDKHandler{
		serverName: "test-server",
		mode:       ModeCE,
		credHelper: &CredentialHelper{
			credentialHelper: newFakeCredentialHelper(),
			mode:             ModeCE,
		},
	}

	ts, err := handler.TokenSource(context.Background())
	require.NoError(t, err)
	assert.Nil(t, ts, "TokenSource should return nil when no token exists")
}

func TestSDKHandler_TokenSource_WithToken(t *testing.T) {
	fake := newFakeCredentialHelper()

	// Store a DCR client using the exact key format from dcr.Credentials:
	// key: "https://{serverName}.mcp-dcr", username: "dcr_client"
	dcrClientJSON, _ := json.Marshal(map[string]string{
		"serverName":            "test-server",
		"providerName":          "test-server",
		"clientId":              "client123",
		"authorizationEndpoint": "https://auth.example.com/authorize",
		"tokenEndpoint":         "https://auth.example.com/token",
	})
	_ = fake.Add(&credentials.Credentials{
		ServerURL: "https://test-server.mcp-dcr",
		Username:  "dcr_client",
		Secret:    base64.StdEncoding.EncodeToString(dcrClientJSON),
	})

	// Store a token at the key the CE path expects: {authorizationEndpoint}/{providerName}
	tokenJSON, _ := json.Marshal(map[string]string{
		"access_token": "my-access-token",
		"token_type":   "Bearer",
	})
	_ = fake.Add(&credentials.Credentials{
		ServerURL: "https://auth.example.com/authorize/test-server",
		Username:  "oauth2_test-server",
		Secret:    base64.StdEncoding.EncodeToString(tokenJSON),
	})

	handler := &SDKHandler{
		serverName: "test-server",
		mode:       ModeCE,
		credHelper: &CredentialHelper{
			credentialHelper: fake,
			mode:             ModeCE,
		},
	}

	ts, err := handler.TokenSource(context.Background())
	require.NoError(t, err)
	require.NotNil(t, ts, "TokenSource should return non-nil when token exists")

	tok, err := ts.Token()
	require.NoError(t, err)
	assert.Equal(t, "my-access-token", tok.AccessToken)
	assert.Equal(t, "Bearer", tok.TokenType)
}

func TestSDKHandler_Authorize_ReturnsError(t *testing.T) {
	handler := NewSDKHandler("my-server", ModeCE)

	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(strings.NewReader("Unauthorized")),
	}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://example.com/mcp", nil)

	err := handler.Authorize(context.Background(), req, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "my-server")
	assert.Contains(t, err.Error(), "docker mcp oauth authorize")
}

func TestSDKHandler_Authorize_NilBody(t *testing.T) {
	handler := NewSDKHandler("my-server", ModeCE)

	resp := &http.Response{StatusCode: http.StatusForbidden}
	err := handler.Authorize(context.Background(), nil, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "my-server")
}

func TestSDKHandler_Authorize_NilResponse(t *testing.T) {
	handler := NewSDKHandler("my-server", ModeCE)

	err := handler.Authorize(context.Background(), nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "my-server")
}
