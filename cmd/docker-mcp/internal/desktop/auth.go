package desktop

import (
	"context"
	"fmt"
	"net/url"
	"reflect"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/contextkeys"
)

type OAuthApp struct {
	App         string        `json:"app"`
	Authorized  bool          `json:"authorized"`
	Provider    string        `json:"provider"`
	Scopes      []OAuthScopes `json:"scopes,omitempty"`
	Tools       []string      `json:"tools"`
	AccessToken string        `json:"accessToken,omitempty"`
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

func (c *Tools) GetOAuthApp(ctx context.Context, app string) (OAuthApp, error) {
	AvoidResourceSaverMode(ctx)

	var result OAuthApp
	err := c.rawClient.Get(ctx, fmt.Sprintf("/apps/%v", app), &result)
	return result, err
}

// DCR (Dynamic Client Registration) Methods

type RegisterDCRRequest struct {
	ClientID            string `json:"clientId"`
	ClientName          string `json:"clientName,omitempty"`
	AuthorizationServer string `json:"authorizationServer,omitempty"`
	AuthorizationEndpoint string `json:"authorizationEndpoint,omitempty"`
	TokenEndpoint        string `json:"tokenEndpoint,omitempty"`
}

type DCRClient struct {
	ServerName           string `json:"serverName"`
	ClientID            string `json:"clientId"`
	ClientName          string `json:"clientName,omitempty"`
	RegisteredAt        string `json:"registeredAt"` // ISO timestamp
	AuthorizationServer string `json:"authorizationServer,omitempty"`
	AuthorizationEndpoint string `json:"authorizationEndpoint,omitempty"`
	TokenEndpoint        string `json:"tokenEndpoint,omitempty"`
}

func (c *Tools) RegisterDCRClient(ctx context.Context, app string, req RegisterDCRRequest) error {
	AvoidResourceSaverMode(ctx)

	var result map[string]string
	return c.rawClient.Post(ctx, fmt.Sprintf("/apps/%s/dcr", app), req, &result)
}

func (c *Tools) GetDCRClient(ctx context.Context, app string) (*DCRClient, error) {
	AvoidResourceSaverMode(ctx)

	var result DCRClient
	err := c.rawClient.Get(ctx, fmt.Sprintf("/apps/%s/dcr", app), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Tools) DeleteDCRClient(ctx context.Context, app string) error {
	AvoidResourceSaverMode(ctx)

	return c.rawClient.Delete(ctx, fmt.Sprintf("/apps/%s/dcr", app))
}



// Authorization URL Generation

type AuthorizationURLRequest struct {
	Scopes []string `json:"scopes,omitempty"`
}

type AuthorizationURLResponse struct {
	AuthorizationURL string `json:"authorizationUrl"`
	State           string `json:"state"`
	BrowserOpened   bool   `json:"browserOpened"`
}

func (c *Tools) GetAuthorizationURL(ctx context.Context, serverName string, scopes []string) (*AuthorizationURLResponse, error) {
	AvoidResourceSaverMode(ctx)

	req := AuthorizationURLRequest{
		Scopes: scopes,
	}

	var result AuthorizationURLResponse
	err := c.rawClient.Post(ctx, fmt.Sprintf("/apps/%s/authorization-url", serverName), req, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
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
