package secret

import (
	"errors"
	"testing"

	"github.com/docker/secrets-engine/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretFromProvider(t *testing.T) {
	id, err := client.ParseID("docker/mcp/oauth-dcr/law-mcp")
	require.NoError(t, err)
	envelopes := []client.Envelope{
		{ID: id, Provider: "docker-desktop-mcp-dcr", Value: []byte("desktop")},
		{ID: id, Provider: "docker-pass", Value: []byte("community")},
	}

	selected, err := secretFromProvider(envelopes, id, "docker-pass")
	require.NoError(t, err)
	assert.Equal(t, "docker-pass", selected.Provider)
	assert.Equal(t, []byte("community"), selected.Value)
}

func TestSecretFromProviderMissing(t *testing.T) {
	id, err := client.ParseID("docker/mcp/oauth-dcr/law-mcp")
	require.NoError(t, err)

	_, err = secretFromProvider(nil, id, "docker-pass")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSecretNotFound))
}
