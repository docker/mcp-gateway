package remoteurl

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeResolver map[string][]netip.Addr

func (r fakeResolver) LookupNetIP(_ context.Context, _ string, host string) ([]netip.Addr, error) {
	if ips, ok := r[host]; ok {
		return ips, nil
	}
	return nil, fmt.Errorf("unexpected lookup for %s", host)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestValidateRejectsUnsafeRemoteURLs(t *testing.T) {
	validator := NewValidator(Options{
		Resolver: fakeResolver{
			"private.example.test": {netip.MustParseAddr("10.0.0.5")},
			"mixed.example.test":   {netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("10.0.0.5")},
		},
	})

	tests := []struct {
		name   string
		rawURL string
	}{
		{name: "http scheme", rawURL: "http://example.com/mcp"},
		{name: "userinfo", rawURL: "https://user:pass@example.com/mcp"},
		{name: "non-http scheme", rawURL: "ftp://example.com/mcp"},
		{name: "localhost", rawURL: "https://localhost/mcp"},
		{name: "loopback IPv4", rawURL: "https://127.0.0.1/mcp"},
		{name: "loopback IPv6", rawURL: "https://[::1]/mcp"},
		{name: "IPv4-mapped loopback", rawURL: "https://[::ffff:127.0.0.1]/mcp"},
		{name: "NAT64 well-known prefix", rawURL: "https://[64:ff9b::a9fe:a9fe]/mcp"},
		{name: "link local", rawURL: "https://169.254.169.254/latest/meta-data"},
		{name: "private IP", rawURL: "https://10.0.0.1/mcp"},
		{name: "cluster suffix", rawURL: "https://api.default.svc.cluster.local/mcp"},
		{name: "metadata hostname", rawURL: "https://metadata.google.internal/computeMetadata/v1"},
		{name: "DNS private result", rawURL: "https://private.example.test/mcp"},
		{name: "DNS mixed result", rawURL: "https://mixed.example.test/mcp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(context.Background(), tt.rawURL)
			require.Error(t, err)
		})
	}
}

func TestValidateAllowsPublicHTTPS(t *testing.T) {
	validator := NewValidator(Options{
		Resolver: fakeResolver{
			"public.example.test": {netip.MustParseAddr("8.8.8.8")},
		},
	})

	require.NoError(t, validator.Validate(context.Background(), "https://public.example.test/mcp"))
}

func TestValidateAllowsInsecureDevURLsWhenOptedIn(t *testing.T) {
	validator := NewValidator(Options{AllowInsecure: true})

	require.NoError(t, validator.Validate(context.Background(), "http://localhost:3000/mcp"))
	require.Error(t, validator.Validate(context.Background(), "file://localhost/tmp/socket"))
	require.Error(t, validator.Validate(context.Background(), "https://user@example.com/mcp"))
}

func TestDirectTransportDisablesProxy(t *testing.T) {
	transport, ok := DirectTransport().(*http.Transport)

	require.True(t, ok)
	assert.Nil(t, transport.Proxy)
}

func TestGuardDialerRequiresRequestTargetHost(t *testing.T) {
	validator := NewValidator(Options{
		Resolver: fakeResolver{
			"public.example.test": {netip.MustParseAddr("8.8.8.8")},
		},
	})

	calledDialer := false
	base := &http.Transport{
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			calledDialer = true
			return nil, fmt.Errorf("unexpected dial")
		},
	}

	rt := validator.guardDialer(base, false)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://public.example.test/mcp", http.NoBody)
	require.NoError(t, err)

	_, err = rt.RoundTrip(req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target host missing")
	assert.False(t, calledDialer)
}

func TestGuardTransportRejectsUnexpectedProxyDial(t *testing.T) {
	validator := NewValidator(Options{
		Resolver: fakeResolver{
			"public.example.test": {netip.MustParseAddr("8.8.8.8")},
		},
	})

	calledDialer := false
	base := &http.Transport{
		Proxy: http.ProxyURL(&url.URL{
			Scheme: "http",
			Host:   "proxy.example.test:8080",
		}),
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			calledDialer = true
			return nil, fmt.Errorf("unexpected proxy dial")
		},
	}
	client := &http.Client{
		Transport: validator.GuardTransport(base),
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://public.example.test/mcp", http.NoBody)
	require.NoError(t, err)

	_, err = client.Do(req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match request host")
	assert.False(t, calledDialer)
}

func TestGuardTrustedProxyTransportAllowsExplicitProxyDial(t *testing.T) {
	validator := NewValidator(Options{
		Resolver: fakeResolver{
			"public.example.test": {netip.MustParseAddr("8.8.8.8")},
		},
	})

	calledDialer := false
	base := &http.Transport{
		Proxy: http.ProxyURL(&url.URL{
			Scheme: "http",
			Host:   "proxy.example.test:8080",
		}),
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			calledDialer = true
			return nil, fmt.Errorf("trusted proxy dial")
		},
	}
	client := &http.Client{
		Transport: validator.GuardTrustedProxyTransport(base),
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://public.example.test/mcp", http.NoBody)
	require.NoError(t, err)

	_, err = client.Do(req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trusted proxy dial")
	assert.True(t, calledDialer)
}

func TestGuardTransportRejectsUnsafeRedirects(t *testing.T) {
	validator := NewValidator(Options{
		Resolver: fakeResolver{
			"public.example.test": {netip.MustParseAddr("8.8.8.8")},
		},
	})

	calls := 0
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{
			StatusCode: http.StatusFound,
			Header: http.Header{
				"Location": []string{"https://127.0.0.1/mcp"},
			},
			Body:    http.NoBody,
			Request: req,
		}, nil
	})

	client := &http.Client{
		Transport: validator.GuardTransport(base),
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://public.example.test/mcp", http.NoBody)
	require.NoError(t, err)

	_, err = client.Do(req)
	require.Error(t, err)
	assert.Equal(t, 1, calls, "redirect destination should be rejected before the second request reaches the base transport")
}
