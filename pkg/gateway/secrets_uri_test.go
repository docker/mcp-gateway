package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/docker/mcp-gateway/pkg/catalog"
)

func TestBuildFallbackURIsSkipsLongLivedLocalSecrets(t *testing.T) {
	uris := buildFallbackURIs([]ServerSecretConfig{
		{
			Secrets: []catalog.Secret{
				{Name: "apify-mcp-server.apify_token", Env: "APIFY_TOKEN"},
			},
			RequireVerifiedSecrets: true,
		},
	})

	assert.Empty(t, uris)
}

func TestLocalLongLivedServerTreatsGlobalFlagAsLongLived(t *testing.T) {
	assert.True(t, localLongLivedServer(catalog.Server{Image: "mcp/test:latest"}, true))
}

func TestLocalLongLivedServerExcludesRemoteServers(t *testing.T) {
	assert.False(t, localLongLivedServer(catalog.Server{
		Remote: catalog.Remote{URL: "https://mcp.example.com/mcp"},
	}, true))
}

func TestBuildFallbackURIsKeepsShortLivedLocalSecrets(t *testing.T) {
	uris := buildFallbackURIs([]ServerSecretConfig{
		{
			Secrets: []catalog.Secret{
				{Name: "short.api_key", Env: "API_KEY"},
			},
		},
	})

	assert.Equal(t, "se://docker/mcp/short.api_key", uris["short.api_key"])
}

func TestBuildFallbackURIsDoesNotBackfillSharedLongLivedSecrets(t *testing.T) {
	uris := buildFallbackURIs([]ServerSecretConfig{
		{
			Secrets: []catalog.Secret{
				{Name: "shared.api_key", Env: "API_KEY"},
			},
			RequireVerifiedSecrets: true,
		},
		{
			Secrets: []catalog.Secret{
				{Name: "shared.api_key", Env: "API_KEY"},
			},
		},
	})

	assert.NotContains(t, uris, "shared.api_key")
}
