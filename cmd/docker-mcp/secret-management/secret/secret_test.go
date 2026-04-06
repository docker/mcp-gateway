package secret

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDefaultSecretKey(t *testing.T) {
	result := GetDefaultSecretKey("mykey")
	assert.Equal(t, "docker/mcp/mykey", result)
}

func TestValidateSecretName(t *testing.T) {
	valid := []string{"mykey", "postgres_password", "GITHUB_TOKEN", "some-key", "key.with.dots"}
	for _, name := range valid {
		require.NoError(t, ValidateSecretName(name), "expected %q to be valid", name)
	}

	invalid := []string{"**", "*", "docker/mcp/**", "key*", "key?", "key[0]", "key{a}"}
	for _, name := range invalid {
		assert.Error(t, ValidateSecretName(name), "expected %q to be invalid", name)
	}
}

func TestParseArg(t *testing.T) {
	// Test key=value parsing
	secret, err := ParseArg("key=value", SetOpts{})
	require.NoError(t, err)
	assert.Equal(t, "key", secret.key)
	assert.Equal(t, "value", secret.val)

	// Test invalid format (no = sign)
	_, err = ParseArg("just-a-key", SetOpts{})
	assert.Error(t, err, "should error when no = sign is present")
}

func TestIsDirectValueProvider(t *testing.T) {
	assert.True(t, isDirectValueProvider(""))
	assert.True(t, isDirectValueProvider(Credstore))
	assert.False(t, isDirectValueProvider("oauth/github"))
}
