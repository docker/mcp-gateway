package oauth

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRefreshResourceRoundTripperAddsResourceToRefreshGrant(t *testing.T) {
	const resource = "https://mcp.example.com/api"

	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		values, err := url.ParseQuery(string(body))
		require.NoError(t, err)
		require.Equal(t, "refresh_token", values.Get("grant_type"))
		require.Equal(t, "refresh-token", values.Get("refresh_token"))
		require.Equal(t, resource, values.Get("resource"))

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"new-access-token","token_type":"Bearer","expires_in":3600}`)),
		}, nil
	})
	client := &http.Client{Transport: &refreshResourceRoundTripper{base: base, resource: resource}}
	ctx := context.WithValue(t.Context(), oauth2.HTTPClient, client)
	config := &oauth2.Config{
		ClientID: "client-id",
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://auth.example.com/token",
		},
	}
	expired := &oauth2.Token{
		AccessToken:  "old-access-token",
		RefreshToken: "refresh-token",
		Expiry:       time.Now().Add(-time.Minute),
	}

	refreshed, err := config.TokenSource(ctx, expired).Token()
	require.NoError(t, err)
	require.Equal(t, "new-access-token", refreshed.AccessToken)
}

func TestRefreshResourceRoundTripperLeavesOtherRequestsUnchanged(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		resource string
	}{
		{
			name: "empty resource",
			body: "grant_type=refresh_token&refresh_token=refresh-token",
		},
		{
			name:     "non-refresh grant",
			body:     "code=authorization-code&grant_type=authorization_code",
			resource: "https://mcp.example.com/api",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				body, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				require.Equal(t, test.body, string(body))
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("{}")),
				}, nil
			})
			transport := &refreshResourceRoundTripper{base: base, resource: test.resource}
			req, err := http.NewRequest(http.MethodPost, "https://auth.example.com/token", strings.NewReader(test.body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			_, err = transport.RoundTrip(req)
			require.NoError(t, err)
		})
	}
}
