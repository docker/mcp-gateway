package fetch

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/remoteurl"
)

func TestUntrustedRejectsUnsafeURL(t *testing.T) {
	_, err := Untrusted(t.Context(), "https://127.0.0.1/readme.md")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not allowed")
}

func TestUntrustedAllowsLocalHTTPWithOptIn(t *testing.T) {
	t.Setenv(remoteurl.AllowInsecureRemoteURLEnv, "1")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("readme"))
	}))
	t.Cleanup(server.Close)

	got, err := Untrusted(t.Context(), server.URL)
	require.NoError(t, err)
	assert.Equal(t, []byte("readme"), got)
}
