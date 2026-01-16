package oauth

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	pkgoauth "github.com/docker/mcp-gateway/pkg/oauth"
	"github.com/docker/mcp-gateway/pkg/oauth/dcr"
)

// RegisterOptions contains configuration for manually registering OAuth credentials
type RegisterOptions struct {
	ClientID              string
	ClientSecret          string
	AuthorizationEndpoint string
	TokenEndpoint         string
	Scopes                string
	Provider              string
	ResourceURL           string
}

// Register manually registers OAuth client credentials for a server
// This is used when the OAuth provider does not support Dynamic Client Registration (DCR)
func Register(_ context.Context, serverName string, opts RegisterOptions) error {
	// Validate required fields
	if err := validateRegisterOptions(serverName, opts); err != nil {
		return err
	}

	// Parse scopes
	var scopesList []string
	if opts.Scopes != "" {
		scopesList = strings.Split(opts.Scopes, ",")
		// Trim whitespace from each scope
		for i, scope := range scopesList {
			scopesList[i] = strings.TrimSpace(scope)
		}
	}

	// Use server name as provider if not specified
	provider := opts.Provider
	if provider == "" {
		provider = serverName
	}

	// Use authorization endpoint as resource URL if not specified
	resourceURL := opts.ResourceURL
	if resourceURL == "" {
		// Try to extract base URL from authorization endpoint
		if u, err := url.Parse(opts.AuthorizationEndpoint); err == nil {
			resourceURL = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
		}
	}

	// Create DCR client struct
	client := dcr.Client{
		ServerName:            serverName,
		ClientID:              opts.ClientID,
		ClientSecret:          opts.ClientSecret,
		ProviderName:          provider,
		AuthorizationEndpoint: opts.AuthorizationEndpoint,
		TokenEndpoint:         opts.TokenEndpoint,
		RequiredScopes:        scopesList,
		ResourceURL:           resourceURL,
		RegisteredAt:          time.Now(),
		ClientName:            fmt.Sprintf("MCP Gateway - %s (manual)", serverName),
	}

	// Store in credential helper
	credHelper := pkgoauth.NewReadWriteCredentialHelper()
	credentials := dcr.NewCredentials(credHelper)

	if err := credentials.SaveClient(serverName, client); err != nil {
		return fmt.Errorf("failed to store OAuth credentials: %w", err)
	}

	fmt.Printf("Successfully registered OAuth client for server: %s\n", serverName)
	fmt.Printf("  Provider: %s\n", provider)
	fmt.Printf("  Client ID: %s\n", opts.ClientID)
	if opts.ClientSecret != "" {
		fmt.Printf("  Client Secret: [configured]\n")
	} else {
		fmt.Printf("  Client Secret: [none - public client]\n")
	}
	fmt.Printf("  Authorization Endpoint: %s\n", opts.AuthorizationEndpoint)
	fmt.Printf("  Token Endpoint: %s\n", opts.TokenEndpoint)
	if len(scopesList) > 0 {
		fmt.Printf("  Scopes: %s\n", strings.Join(scopesList, ", "))
	}
	fmt.Printf("\nYou can now authorize with: docker mcp oauth authorize %s\n", serverName)

	return nil
}

// validateRegisterOptions validates the registration options
func validateRegisterOptions(serverName string, opts RegisterOptions) error {
	if serverName == "" {
		return fmt.Errorf("server name is required")
	}

	if opts.ClientID == "" {
		return fmt.Errorf("client-id is required")
	}

	if opts.AuthorizationEndpoint == "" {
		return fmt.Errorf("auth-endpoint is required")
	}

	if opts.TokenEndpoint == "" {
		return fmt.Errorf("token-endpoint is required")
	}

	// Validate URLs
	if _, err := url.Parse(opts.AuthorizationEndpoint); err != nil {
		return fmt.Errorf("invalid auth-endpoint URL: %w", err)
	}

	if _, err := url.Parse(opts.TokenEndpoint); err != nil {
		return fmt.Errorf("invalid token-endpoint URL: %w", err)
	}

	if opts.ResourceURL != "" {
		if _, err := url.Parse(opts.ResourceURL); err != nil {
			return fmt.Errorf("invalid resource-url: %w", err)
		}
	}

	return nil
}
