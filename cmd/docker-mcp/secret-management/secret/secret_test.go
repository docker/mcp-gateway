package secret

import (
	"io"
	"testing"

	seclient "github.com/docker/secrets-engine/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDefaultSecretKey(t *testing.T) {
	result, err := GetDefaultSecretKey(seclient.MustParseID("mykey"))
	require.NoError(t, err)
	assert.Equal(t, "docker/mcp/mykey", result.String())
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

func TestDefaultSecretSetCommand(t *testing.T) {
	key, err := GetDefaultSecretKey(seclient.MustParseID("mykey"))
	require.NoError(t, err)

	const secretValue = "super-secret-value-123"
	c := defaultSecretSetCommand(t.Context(), key, secretValue)
	require.NotNil(t, c)

	// Verify command and arguments contain --force overwrite flag
	assert.Equal(t, []string{"docker", "pass", "set", "docker/mcp/mykey", "--force"}, c.Args)

	// Verify secret is NOT leaked into command arguments
	for _, arg := range c.Args {
		assert.NotContains(t, arg, secretValue)
	}

	// Verify secret value is properly delivered via stdin
	require.NotNil(t, c.Stdin)
	stdinBytes, err := io.ReadAll(c.Stdin)
	require.NoError(t, err)
	assert.Equal(t, secretValue, string(stdinBytes))
}
