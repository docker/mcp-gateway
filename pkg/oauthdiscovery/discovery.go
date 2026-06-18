package oauthdiscovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	oauthhelpers "github.com/docker/mcp-gateway-oauth-helpers"

	"github.com/docker/mcp-gateway/pkg/desktop"
	"github.com/docker/mcp-gateway/pkg/remoteurl"
)

func DiscoverOAuthRequirements(ctx context.Context, serverURL string) (*oauthhelpers.Discovery, error) {
	if err := remoteurl.Validate(ctx, serverURL); err != nil {
		return nil, err
	}

	client := newGuardedHTTPClient(ctx, 30*time.Second)

	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}

	mcpPayload := `{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"mcp-gateway","version":"1.0.0"}},"id":1}`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serverURL, strings.NewReader(mcpPayload))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "docker-mcp-gateway/1.0.0")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connecting to server %s: %w", serverURL, err)
	}
	defer resp.Body.Close()

	wwwAuth := resp.Header.Get("WWW-Authenticate")
	var challenges []oauthhelpers.WWWAuthenticateChallenge
	if wwwAuth != "" {
		challenges, err = oauthhelpers.ParseWWWAuthenticate(wwwAuth)
		if err != nil {
			challenges = nil
		}
	}

	defaultAuthServerURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	var resourceMetadata *oauthhelpers.ProtectedResourceMetadata
	authServerURL := defaultAuthServerURL

	resourceMetadataURL := ""
	if challenges != nil {
		resourceMetadataURL = oauthhelpers.FindResourceMetadataURL(challenges)
	}

	if resourceMetadataURL != "" {
		resourceMetadata, err = fetchOAuthProtectedResourceMetadata(ctx, client, resourceMetadataURL)
		if err == nil && resourceMetadata != nil && resourceMetadata.AuthorizationServer != "" {
			authServerURL = resourceMetadata.AuthorizationServer
		}
	} else {
		wellKnownURL := fmt.Sprintf("%s/.well-known/oauth-protected-resource", defaultAuthServerURL)
		resourceMetadata, err = fetchOAuthProtectedResourceMetadata(ctx, client, wellKnownURL)
		if err == nil && resourceMetadata != nil && resourceMetadata.AuthorizationServer != "" {
			authServerURL = resourceMetadata.AuthorizationServer
		}
	}

	authServerMetadata, err := fetchAuthorizationServerMetadata(ctx, client, authServerURL)
	if err != nil {
		return nil, fmt.Errorf("fetching authorization server metadata from %s: %w", authServerURL, err)
	}

	discovery := &oauthhelpers.Discovery{
		RequiresOAuth: true,

		ResourceURL:         defaultAuthServerURL,
		ResourceServer:      defaultAuthServerURL,
		AuthorizationServer: authServerURL,

		Issuer:                            authServerMetadata.Issuer,
		AuthorizationEndpoint:             authServerMetadata.AuthorizationEndpoint,
		TokenEndpoint:                     authServerMetadata.TokenEndpoint,
		RegistrationEndpoint:              authServerMetadata.RegistrationEndpoint,
		JWKSUri:                           authServerMetadata.JWKSUri,
		ScopesSupported:                   authServerMetadata.ScopesSupported,
		ResponseTypesSupported:            authServerMetadata.ResponseTypesSupported,
		ResponseModesSupported:            authServerMetadata.ResponseModesSupported,
		GrantTypesSupported:               authServerMetadata.GrantTypesSupported,
		TokenEndpointAuthMethodsSupported: authServerMetadata.TokenEndpointAuthMethodsSupported,
		SupportsPKCE:                      slices.Contains(authServerMetadata.CodeChallengeMethodsSupported, "S256"),
		CodeChallengeMethod:               authServerMetadata.CodeChallengeMethodsSupported,
	}

	if resourceMetadata != nil {
		if resourceMetadata.Resource != "" {
			discovery.ResourceURL = resourceMetadata.Resource
			discovery.ResourceServer = resourceMetadata.Resource
		}
		if len(resourceMetadata.Scopes) > 0 {
			discovery.Scopes = resourceMetadata.Scopes
		}
	}
	if len(discovery.Scopes) == 0 {
		discovery.Scopes = oauthhelpers.FindRequiredScopes(challenges)
	}
	if err := ValidateDiscovery(ctx, discovery); err != nil {
		return nil, err
	}

	return discovery, nil
}

func ValidateDiscovery(ctx context.Context, discovery *oauthhelpers.Discovery) error {
	if discovery == nil {
		return fmt.Errorf("OAuth discovery result is empty")
	}

	for _, endpoint := range []struct {
		name   string
		rawURL string
	}{
		{name: "resource", rawURL: discovery.ResourceURL},
		{name: "resource server", rawURL: discovery.ResourceServer},
		{name: "authorization server", rawURL: discovery.AuthorizationServer},
		{name: "authorization endpoint", rawURL: discovery.AuthorizationEndpoint},
		{name: "token endpoint", rawURL: discovery.TokenEndpoint},
		{name: "registration endpoint", rawURL: discovery.RegistrationEndpoint},
	} {
		if endpoint.rawURL == "" {
			continue
		}
		if err := remoteurl.Validate(ctx, endpoint.rawURL); err != nil {
			return fmt.Errorf("invalid OAuth %s: %w", endpoint.name, err)
		}
	}

	return nil
}

func PerformDCR(ctx context.Context, discovery *oauthhelpers.Discovery, serverName string, redirectURI string) (*oauthhelpers.ClientCredentials, error) {
	if discovery == nil || discovery.RegistrationEndpoint == "" {
		return nil, fmt.Errorf("no registration endpoint found for %s", serverName)
	}
	if err := ValidateDiscovery(ctx, discovery); err != nil {
		return nil, err
	}
	if err := validateRedirectURI(redirectURI); err != nil {
		return nil, fmt.Errorf("invalid redirect URI: %w", err)
	}
	if redirectURI == "" {
		redirectURI = oauthhelpers.DefaultRedirectURI
	}

	registration := oauthhelpers.DCRRequest{
		ClientName:              fmt.Sprintf("MCP Gateway - %s", serverName),
		RedirectURIs:            []string{redirectURI},
		TokenEndpointAuthMethod: "none",
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		ClientURI:               "https://github.com/docker/mcp-gateway",
		SoftwareID:              "mcp-gateway",
		SoftwareVersion:         "1.0.0",
		Contacts:                []string{"support@docker.com"},
	}
	if len(discovery.Scopes) > 0 {
		registration.Scope = strings.Join(discovery.Scopes, " ")
	}

	body, err := json.Marshal(registration)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal DCR request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, discovery.RegistrationEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create DCR request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "MCP-Gateway/1.0.0")

	client := newGuardedHTTPClient(ctx, 30*time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send DCR request to %s: %w", discovery.RegistrationEndpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		errorBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("DCR failed with status %d for %s", resp.StatusCode, serverName)
		}

		errorMsg := string(errorBody)
		var errorResp map[string]any
		if err := json.Unmarshal(errorBody, &errorResp); err == nil {
			if errDesc, ok := errorResp["error_description"].(string); ok {
				errorMsg = errDesc
			} else if errField, ok := errorResp["error"].(string); ok {
				errorMsg = errField
			} else if message, ok := errorResp["message"].(string); ok {
				errorMsg = message
			}
		}

		return nil, fmt.Errorf("DCR failed with status %d for %s: %s", resp.StatusCode, serverName, errorMsg)
	}

	var dcrResponse oauthhelpers.DCRResponse
	if err := json.NewDecoder(resp.Body).Decode(&dcrResponse); err != nil {
		return nil, fmt.Errorf("failed to decode DCR response: %w", err)
	}
	if dcrResponse.ClientID == "" {
		return nil, fmt.Errorf("DCR response missing client_id for %s", serverName)
	}

	return &oauthhelpers.ClientCredentials{
		ClientID:              dcrResponse.ClientID,
		ServerURL:             discovery.ResourceURL,
		IsPublic:              true,
		AuthorizationEndpoint: discovery.AuthorizationEndpoint,
		TokenEndpoint:         discovery.TokenEndpoint,
	}, nil
}

func fetchOAuthProtectedResourceMetadata(ctx context.Context, client *http.Client, metadataURL string) (*oauthhelpers.ProtectedResourceMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching metadata from %s: %w", metadataURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metadata endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var metadata oauthhelpers.ProtectedResourceMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return nil, fmt.Errorf("parsing JSON response: %w", err)
	}
	if metadata.Resource == "" {
		return nil, fmt.Errorf("resource field missing in protected resource metadata")
	}
	if metadata.AuthorizationServer == "" {
		if len(metadata.AuthorizationServers) == 0 {
			return nil, fmt.Errorf("authorization_server or authorization_servers field missing in protected resource metadata")
		}
		metadata.AuthorizationServer = metadata.AuthorizationServers[0]
	}

	return &metadata, nil
}

func fetchAuthorizationServerMetadata(ctx context.Context, client *http.Client, authServerURL string) (*oauthhelpers.AuthorizationServerMetadata, error) {
	metadataURL, err := buildRFC8414WellKnownURL(ctx, authServerURL)
	if err != nil {
		return nil, fmt.Errorf("building well-known URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching metadata from %s: %w", metadataURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("authorization server metadata endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var metadata oauthhelpers.AuthorizationServerMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return nil, fmt.Errorf("parsing JSON response: %w", err)
	}
	if metadata.Issuer == "" {
		return nil, fmt.Errorf("issuer field missing in authorization server metadata")
	}
	if metadata.AuthorizationEndpoint == "" {
		return nil, fmt.Errorf("authorization_endpoint field missing in authorization server metadata")
	}
	if metadata.TokenEndpoint == "" {
		return nil, fmt.Errorf("token_endpoint field missing in authorization server metadata")
	}
	if _, err := url.Parse(metadata.Issuer); err != nil {
		return nil, fmt.Errorf("invalid issuer URL: %w", err)
	}

	return &metadata, nil
}

func buildRFC8414WellKnownURL(ctx context.Context, issuerURL string) (string, error) {
	parsed, err := url.Parse(issuerURL)
	if err != nil {
		return "", fmt.Errorf("invalid issuer URL: %w", err)
	}
	if err := remoteurl.Validate(ctx, issuerURL); err != nil {
		return "", err
	}
	if parsed.RawQuery != "" {
		return "", fmt.Errorf("issuer URL must not contain query parameters")
	}
	if parsed.Fragment != "" {
		return "", fmt.Errorf("issuer URL must not contain fragment")
	}

	host := strings.ToLower(parsed.Host)
	path := parsed.Path
	if path == "/" {
		path = ""
	}

	return fmt.Sprintf("%s://%s/.well-known/oauth-authorization-server%s", parsed.Scheme, host, path), nil
}

func validateRedirectURI(redirectURI string) error {
	if redirectURI == "" {
		return nil
	}

	parsed, err := url.Parse(redirectURI)
	if err != nil {
		return fmt.Errorf("invalid redirect URI format: %w", err)
	}

	switch parsed.Hostname() {
	case "localhost", "127.0.0.1", "::1", "mcp.docker.com":
		return nil
	default:
		return fmt.Errorf("redirect URI host %q not allowed - must be localhost or mcp.docker.com", parsed.Hostname())
	}
}

func newGuardedHTTPClient(ctx context.Context, timeout time.Duration) *http.Client {
	if proxyDialer := desktop.DockerDesktopProxySocketDialer(ctx); proxyDialer != nil {
		return remoteurl.NewTrustedProxyHTTPClient(timeout, proxyDialer)
	}
	return remoteurl.NewDirectHTTPClient(timeout)
}
