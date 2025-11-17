package oauth

import (
	"os"
	"testing"

	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsCEMode(t *testing.T) {
	// Test CE mode detection
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{
			name:     "CE mode enabled",
			envValue: "true",
			expected: true,
		},
		{
			name:     "CE mode disabled",
			envValue: "false",
			expected: false,
		},
		{
			name:     "CE mode not set",
			envValue: "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldValue := os.Getenv("DOCKER_MCP_USE_CE")
			defer func() {
				if oldValue == "" {
					os.Unsetenv("DOCKER_MCP_USE_CE")
				} else {
					os.Setenv("DOCKER_MCP_USE_CE", oldValue)
				}
			}()

			if tt.envValue == "" {
				os.Unsetenv("DOCKER_MCP_USE_CE")
			} else {
				os.Setenv("DOCKER_MCP_USE_CE", tt.envValue)
			}

			result := IsCEMode()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReadWriteHelper_Operations(t *testing.T) {
	// Use fake helper for testing
	fakeHelper := newFakeCredentialHelper()

	// Test Add
	err := fakeHelper.Add(&credentials.Credentials{
		ServerURL: "https://test.example.com",
		Username:  "testuser",
		Secret:    "testsecret",
	})
	require.NoError(t, err)

	// Test Get
	username, secret, err := fakeHelper.Get("https://test.example.com")
	require.NoError(t, err)
	assert.Equal(t, "testuser", username)
	assert.Equal(t, "testsecret", secret)

	// Test List
	list, err := fakeHelper.List()
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Contains(t, list, "https://test.example.com")

	// Test Delete
	err = fakeHelper.Delete("https://test.example.com")
	require.NoError(t, err)

	// Verify deletion
	_, _, err = fakeHelper.Get("https://test.example.com")
	assert.Error(t, err)
}

func TestReadWriteHelper_GetNotFound(t *testing.T) {
	fakeHelper := newFakeCredentialHelper()

	// Try to get non-existent credential
	_, _, err := fakeHelper.Get("https://non-existent.example.com")
	require.Error(t, err)
	assert.True(t, credentials.IsErrCredentialsNotFound(err))
}

func TestReadWriteHelper_DeleteNotFound(t *testing.T) {
	fakeHelper := newFakeCredentialHelper()

	// Try to delete non-existent credential
	err := fakeHelper.Delete("https://non-existent.example.com")
	assert.Error(t, err)
}

func TestOAuthHelper_ReadOnlyOperations(t *testing.T) {
	helper := oauthHelper{
		program: nil, // Not testing actual program execution
	}

	// Add should fail (read-only)
	err := helper.Add(&credentials.Credentials{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read-only")

	// Delete should fail (read-only)
	err = helper.Delete("https://test.example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read-only")

	// List should return empty map
	list, err := helper.List()
	require.NoError(t, err)
	assert.Empty(t, list)
}
