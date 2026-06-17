package oauthdiscovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/remoteurl"
)

func TestBuildRFC8414WellKnownURLRejectsLocalHTTPByDefault(t *testing.T) {
	_, err := buildRFC8414WellKnownURL(t.Context(), "http://localhost:8080/oauth")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "https")
}

func TestBuildRFC8414WellKnownURLHonorsInsecureDevOptIn(t *testing.T) {
	t.Setenv(remoteurl.AllowInsecureRemoteURLEnv, "1")

	metadataURL, err := buildRFC8414WellKnownURL(t.Context(), "http://localhost:8080/oauth")

	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080/.well-known/oauth-authorization-server/oauth", metadataURL)
}

func TestBuildRFC8414WellKnownURLAllowsPublicHTTPSIssuer(t *testing.T) {
	metadataURL, err := buildRFC8414WellKnownURL(t.Context(), "https://8.8.8.8/oauth")

	require.NoError(t, err)
	assert.Equal(t, "https://8.8.8.8/.well-known/oauth-authorization-server/oauth", metadataURL)
}
