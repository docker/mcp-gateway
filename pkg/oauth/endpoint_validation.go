package oauth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/docker/mcp-gateway/pkg/desktop"
	"github.com/docker/mcp-gateway/pkg/oauth/dcr"
	"github.com/docker/mcp-gateway/pkg/remoteurl"
)

func guardedOAuthHTTPClient(ctx context.Context, timeout time.Duration) *http.Client {
	if proxyTransport := desktop.DockerDesktopProxySocketTransport(ctx); proxyTransport != nil {
		return remoteurl.NewTrustedProxyHTTPClient(timeout, proxyTransport)
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: remoteurl.GuardDirectTransport(),
	}
}

func validateOutboundDCRClientEndpoints(ctx context.Context, client dcr.Client) error {
	for _, endpoint := range []struct {
		name   string
		rawURL string
	}{
		{name: "token endpoint", rawURL: client.TokenEndpoint},
		{name: "resource URL", rawURL: client.ResourceURL},
	} {
		if endpoint.rawURL == "" {
			continue
		}
		if err := remoteurl.Validate(ctx, endpoint.rawURL); err != nil {
			return fmt.Errorf("invalid OAuth %s for %s: %w", endpoint.name, client.ServerName, err)
		}
	}
	return nil
}
