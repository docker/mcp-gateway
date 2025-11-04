package secret

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSecretKey(t *testing.T) {
	result := getSecretKey("mykey")
	assert.Equal(t, "sm_mykey", result)
}

func TestParseArg(t *testing.T) {
	// Test key=value parsing
	secret, err := ParseArg("key=value", SetOpts{Provider: Credstore})
	require.NoError(t, err)
	assert.Equal(t, "key", secret.key)
	assert.Equal(t, "value", secret.val)

	// Test key-only for non-direct providers
	secret, err = ParseArg("keyname", SetOpts{Provider: "oauth/github"})
	require.NoError(t, err)
	assert.Equal(t, "keyname", secret.key)
	assert.Empty(t, secret.val)

	// Test error on key=value with non-direct provider
	_, err = ParseArg("key=value", SetOpts{Provider: "oauth/github"})
	assert.Error(t, err)
}

func TestIsDirectValueProvider(t *testing.T) {
	assert.True(t, isDirectValueProvider(""))
	assert.True(t, isDirectValueProvider(Credstore))
	assert.False(t, isDirectValueProvider("oauth/github"))
}
