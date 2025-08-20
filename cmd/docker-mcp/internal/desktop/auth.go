package desktop

import (
	"context"
	"fmt"
	"net/url"
	"reflect"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/contextkeys"
)

type OAuthApp struct {
	App        string        `json:"app"`
	Authorized bool          `json:"authorized"`
	Provider   string        `json:"provider"`
	Scopes     []OAuthScopes `json:"scopes,omitempty"`
	Tools      []string      `json:"tools"`
}

type OAuthScopes struct {
	Description string   `json:"description,omitempty"`
	Metadata    []string `json:"metadata,omitempty"`
	Name        string   `json:"name,omitempty"`
}

type AuthResponse struct {
	AuthType   string `json:"authType,omitempty"`
	BrowserURL string `json:"browserUrl,omitempty"`
}

func NewAuthClient() *Tools {
	return &Tools{
		rawClient: newRawClient(dialAuth),
	}
}

type Tools struct {
	rawClient *RawClient
}

func (c *Tools) DeleteOAuthApp(ctx context.Context, app string) error {
	AvoidResourceSaverMode(ctx)

	return c.rawClient.Delete(ctx, fmt.Sprintf("/apps/%v", app))
}

func (c *Tools) ListOAuthApps(ctx context.Context) ([]OAuthApp, error) {
	AvoidResourceSaverMode(ctx)

	var result []OAuthApp
	err := c.rawClient.Get(ctx, "/apps", &result)
	return result, err
}

func (c *Tools) PostOAuthApp(ctx context.Context, app, scopes string, disableAutoOpen bool) (AuthResponse, error) {
	AvoidResourceSaverMode(ctx)

	q := ""
	q = addQueryParam(q, "scopes", scopes, false)

	// Only add disableAutoOpen parameter if oauth-interceptor feature is enabled
	// This is indicated by the presence of the feature flag in the context
	if oauthEnabled, ok := ctx.Value(contextkeys.OAuthInterceptorEnabledKey).(bool); ok && oauthEnabled {
		q = addQueryParam(q, "disableAutoOpen", disableAutoOpen, false)
	}

	if q != "" {
		q = "?" + q
	}
	var result AuthResponse
	err := c.rawClient.Post(ctx, fmt.Sprintf("/apps/%v", app)+q, nil, &result)
	return result, err
}

func addQueryParam[T any](q, name string, value T, required bool) string {
	if !required && reflect.DeepEqual(value, reflect.Zero(reflect.TypeOf(value)).Interface()) {
		return ""
	}
	p := name + "=" + url.QueryEscape(fmt.Sprint(value))
	if q == "" {
		return p
	}
	return q + "&" + p
}
