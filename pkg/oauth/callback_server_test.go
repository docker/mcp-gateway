package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallbackServer_PortAssignment(t *testing.T) {
	server, err := NewCallbackServer()
	require.NoError(t, err)
	defer func() {
		_ = server.Shutdown(context.Background())
	}()

	// Should get a valid port
	assert.Positive(t, server.Port())
	assert.NotEmpty(t, server.URL())
	assert.Contains(t, server.URL(), "http://localhost:")
	assert.Contains(t, server.URL(), "/callback")
}

func TestCallbackServer_Success(t *testing.T) {
	server, err := NewCallbackServer()
	require.NoError(t, err)
	defer func() {
		_ = server.Shutdown(context.Background())
	}()

	// Start server in background
	go func() {
		_ = server.Start()
	}()

	// Wait briefly for server to start
	time.Sleep(50 * time.Millisecond)

	// Make request with code and state
	testCode := "test-auth-code-123"
	testState := "test-state-456"

	req := httptest.NewRequest(http.MethodGet, "/callback?code="+testCode+"&state="+testState, nil)
	w := httptest.NewRecorder()

	server.handleCallback(w, req)

	// Should return success
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Authorization Successful")

	// Should receive callback data
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	code, state, err := server.Wait(ctx)
	require.NoError(t, err)
	assert.Equal(t, testCode, code)
	assert.Equal(t, testState, state)
}

func TestCallbackServer_MissingCode(t *testing.T) {
	server, err := NewCallbackServer()
	require.NoError(t, err)
	defer func() {
		_ = server.Shutdown(context.Background())
	}()

	// Make request with only state
	req := httptest.NewRequest(http.MethodGet, "/callback?state=test-state", nil)
	w := httptest.NewRecorder()

	server.handleCallback(w, req)

	// Should return error
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Missing authorization code")

	// Should receive error
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, _, err = server.Wait(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Missing authorization code")
}

func TestCallbackServer_MissingState(t *testing.T) {
	server, err := NewCallbackServer()
	require.NoError(t, err)
	defer func() {
		_ = server.Shutdown(context.Background())
	}()

	// Make request with only code
	req := httptest.NewRequest(http.MethodGet, "/callback?code=test-code", nil)
	w := httptest.NewRecorder()

	server.handleCallback(w, req)

	// Should return error
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Missing state parameter")

	// Should receive error
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, _, err = server.Wait(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Missing state parameter")
}

func TestCallbackServer_OAuthError(t *testing.T) {
	tests := []struct {
		name        string
		queryString string
		expectedErr string
	}{
		{
			name:        "error with description",
			queryString: "error=access_denied&error_description=User+denied+access",
			expectedErr: "OAuth error: access_denied - User denied access",
		},
		{
			name:        "error without description",
			queryString: "error=invalid_request",
			expectedErr: "OAuth error: invalid_request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewCallbackServer()
			require.NoError(t, err)
			defer func() {
				_ = server.Shutdown(context.Background())
			}()

			// Make request with error parameters
			req := httptest.NewRequest(http.MethodGet, "/callback?"+tt.queryString, nil)
			w := httptest.NewRecorder()

			server.handleCallback(w, req)

			// Should return error
			assert.Equal(t, http.StatusBadRequest, w.Code)

			// Should receive error
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			_, _, err = server.Wait(ctx)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestCallbackServer_Timeout(t *testing.T) {
	server, err := NewCallbackServer()
	require.NoError(t, err)
	defer func() {
		_ = server.Shutdown(context.Background())
	}()

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Wait should timeout
	_, _, err = server.Wait(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "callback timeout")
}
