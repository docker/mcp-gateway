package fetch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUntrustedRejectsUnsafeURL(t *testing.T) {
	_, err := Untrusted(t.Context(), "https://127.0.0.1/readme.md")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not allowed")
}

func TestUntrustedRejectsUnknownHTTPURL(t *testing.T) {
	_, err := Untrusted(t.Context(), "http://example.com/readme.md")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remote URL must use https")
}
