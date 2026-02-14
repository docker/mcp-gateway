package mcp

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// roundTripFunc is an adapter to use functions as http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestHeaderRoundTripper_AttachesAuthorizationHeader(t *testing.T) {
	// Verifies that headerRoundTripper propagates Authorization headers to requests.
	// This is the mechanism through which OAuth tokens (both catalog and dynamic) reach
	// the remote MCP server.
	var capturedReq *http.Request
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedReq = req
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})

	rt := &headerRoundTripper{
		base: base,
		headers: map[string]string{
			"Authorization": "Bearer test-oauth-token",
		},
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/mcp", nil)
	require.NoError(t, err)

	_, err = rt.RoundTrip(req)
	require.NoError(t, err)

	require.NotNil(t, capturedReq)
	assert.Equal(t, "Bearer test-oauth-token", capturedReq.Header.Get("Authorization"))
}

func TestHeaderRoundTripper_DoesNotOverrideExistingAccept(t *testing.T) {
	var capturedReq *http.Request
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedReq = req
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})

	rt := &headerRoundTripper{
		base: base,
		headers: map[string]string{
			"Accept":        "application/json",
			"Authorization": "Bearer token",
		},
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/mcp", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/event-stream")

	_, err = rt.RoundTrip(req)
	require.NoError(t, err)

	require.NotNil(t, capturedReq)
	assert.Equal(t, "text/event-stream", capturedReq.Header.Get("Accept"),
		"Accept should not be overridden when already set")
	assert.Equal(t, "Bearer token", capturedReq.Header.Get("Authorization"),
		"Authorization should still be set")
}

func TestHeaderRoundTripper_DoesNotMutateOriginalRequest(t *testing.T) {
	base := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})

	rt := &headerRoundTripper{
		base: base,
		headers: map[string]string{
			"Authorization": "Bearer token",
		},
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/mcp", nil)
	require.NoError(t, err)

	_, err = rt.RoundTrip(req)
	require.NoError(t, err)

	assert.Empty(t, req.Header.Get("Authorization"),
		"original request should not be mutated")
}

func TestHeaderRoundTripper_MultipleCustomHeaders(t *testing.T) {
	var capturedReq *http.Request
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedReq = req
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})

	rt := &headerRoundTripper{
		base: base,
		headers: map[string]string{
			"Authorization": "Bearer dynamic-oauth-token",
			"X-Custom":      "custom-value",
		},
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/mcp", nil)
	require.NoError(t, err)

	_, err = rt.RoundTrip(req)
	require.NoError(t, err)

	require.NotNil(t, capturedReq)
	assert.Equal(t, "Bearer dynamic-oauth-token", capturedReq.Header.Get("Authorization"))
	assert.Equal(t, "custom-value", capturedReq.Header.Get("X-Custom"))
}

func TestHeaderRoundTripper_EmptyHeaders(t *testing.T) {
	var capturedReq *http.Request
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedReq = req
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})

	rt := &headerRoundTripper{
		base:    base,
		headers: map[string]string{},
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/mcp", nil)
	require.NoError(t, err)

	_, err = rt.RoundTrip(req)
	require.NoError(t, err)

	require.NotNil(t, capturedReq)
	assert.Empty(t, capturedReq.Header.Get("Authorization"),
		"no Authorization header when headers map is empty")
}
