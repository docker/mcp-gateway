package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// ClientCredentials represents stored client credentials for a public client
// For public clients, only the client_id is stored (no client_secret)
type ClientCredentials struct {
	ClientID              string `json:"client_id"`
	ServerURL             string `json:"server_url"` // The resource server URL
	IsPublic              bool   `json:"is_public"`  // Always true for our implementation
	AuthorizationEndpoint string `json:"authorization_endpoint,omitempty"`
	TokenEndpoint         string `json:"token_endpoint,omitempty"`
	// No ClientSecret field - public clients don't have secrets
}

// PerformDCR performs Dynamic Client Registration with the authorization server
// Returns client credentials for the registered public client
//
// RFC 7591 COMPLIANCE:
// - Uses token_endpoint_auth_method="none" for public clients
// - Includes redirect_uris pointing to mcp-oauth proxy
// - Requests authorization_code and refresh_token grant types
func PerformDCR(ctx context.Context, discovery *OAuthDiscovery, serverName string) (*ClientCredentials, error) {
	fmt.Printf("DEBUG: Starting DCR for server: %s\n", serverName)
	fmt.Printf("DEBUG: Registration endpoint: %s\n", discovery.RegistrationEndpoint)

	if discovery.RegistrationEndpoint == "" {
		fmt.Printf("DEBUG: No registration endpoint found for %s\n", serverName)
		return nil, fmt.Errorf("no registration endpoint found for %s", serverName)
	}

	// Build DCR request for PUBLIC client
	registration := DCRRequest{
		ClientName: fmt.Sprintf("MCP Gateway - %s", serverName),
		RedirectURIs: []string{
			"https://mcp.docker.com/oauth/callback", // mcp-oauth proxy callback only
		},
		TokenEndpointAuthMethod: "none", // PUBLIC client (no client secret)
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},

		// Additional metadata for better client identification
		ClientURI:       "https://github.com/docker/mcp-gateway",
		SoftwareID:      "mcp-gateway",
		SoftwareVersion: "1.0.0",
		Contacts:        []string{"support@docker.com"},
	}

	// Add requested scopes if provided
	if len(discovery.Scopes) > 0 {
		registration.Scope = joinScopes(discovery.Scopes)
		fmt.Printf("DEBUG: Added scopes: %s\n", registration.Scope)
	}

	// Marshal the registration request
	body, err := json.Marshal(registration)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal DCR request: %w", err)
	}

	fmt.Printf("DEBUG: DCR request body: %s\n", string(body))

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, discovery.RegistrationEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create DCR request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "MCP-Gateway/1.0.0")

	// Send the request
	fmt.Printf("DEBUG: Sending DCR request to %s\n", discovery.RegistrationEndpoint)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("DEBUG: DCR request failed: %v\n", err)
		return nil, fmt.Errorf("failed to send DCR request to %s: %w", discovery.RegistrationEndpoint, err)
	}
	defer resp.Body.Close()

	fmt.Printf("DEBUG: DCR response status: %d\n", resp.StatusCode)

	// Check response status (201 Created or 200 OK are acceptable)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		// Read error response body to understand why DCR failed
		errorBody, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("DEBUG: DCR failed with status %d for %s (could not read error body: %v)\n", resp.StatusCode, serverName, err)
			return nil, fmt.Errorf("DCR failed with status %d for %s", resp.StatusCode, serverName)
		}
		
		errorMsg := string(errorBody)
		
		// Try to parse as JSON for structured error
		var errorResp map[string]interface{}
		if err := json.Unmarshal(errorBody, &errorResp); err == nil {
			// Successfully parsed as JSON - look for common error fields
			if errDesc, ok := errorResp["error_description"].(string); ok {
				errorMsg = errDesc
			} else if errField, ok := errorResp["error"].(string); ok {
				errorMsg = errField
			} else if message, ok := errorResp["message"].(string); ok {
				errorMsg = message
			}
		}
		
		fmt.Printf("DEBUG: DCR failed with status %d for %s\n", resp.StatusCode, serverName)
		fmt.Printf("DEBUG: DCR error response: %s\n", errorMsg)
		
		return nil, fmt.Errorf("DCR failed with status %d for %s: %s", resp.StatusCode, serverName, errorMsg)
	}

	// Parse the response
	var dcrResponse DCRResponse
	if err := json.NewDecoder(resp.Body).Decode(&dcrResponse); err != nil {
		fmt.Printf("DEBUG: Failed to decode DCR response: %v\n", err)
		return nil, fmt.Errorf("failed to decode DCR response: %w", err)
	}

	fmt.Printf("DEBUG: DCR response client_id: %s\n", dcrResponse.ClientID)
	fmt.Printf("DEBUG: Full DCR response: %+v\n", dcrResponse)

	if dcrResponse.ClientID == "" {
		fmt.Printf("DEBUG: DCR response missing client_id for %s\n", serverName)
		return nil, fmt.Errorf("DCR response missing client_id for %s", serverName)
	}

	// Create client credentials (public client - no secret)
	creds := &ClientCredentials{
		ClientID:              dcrResponse.ClientID,
		ServerURL:             discovery.ResourceURL,
		IsPublic:              true,
		AuthorizationEndpoint: discovery.AuthorizationEndpoint,
		TokenEndpoint:         discovery.TokenEndpoint,
		// No ClientSecret for public clients
	}

	fmt.Printf("DEBUG: Created credentials with client_id: %s\n", creds.ClientID)
	return creds, nil
}

// GetDCREndpoint discovers the DCR registration endpoint from authorization server metadata
// This is a fallback method when discovery.RegistrationEndpoint is not already populated
//
// RFC 8414 COMPLIANCE:
// - Fetches from /.well-known/oauth-authorization-server
// - Falls back to /.well-known/openid-configuration (for OIDC compatibility)
func GetDCREndpoint(ctx context.Context, authorizationServer string) (string, error) {
	// Parse the authorization server URL
	baseURL, err := url.Parse(authorizationServer)
	if err != nil {
		return "", fmt.Errorf("invalid authorization server URL: %w", err)
	}

	// Try OAuth 2.0 Authorization Server Metadata (RFC 8414) first
	wellKnownURL := baseURL.ResolveReference(&url.URL{Path: "/.well-known/oauth-authorization-server"})

	endpoint, err := fetchRegistrationEndpoint(ctx, wellKnownURL.String())
	if err == nil && endpoint != "" {
		return endpoint, nil
	}

	// Fallback to OpenID Connect Discovery
	oidcURL := baseURL.ResolveReference(&url.URL{Path: "/.well-known/openid-configuration"})

	endpoint, err = fetchRegistrationEndpoint(ctx, oidcURL.String())
	if err != nil {
		return "", fmt.Errorf("failed to discover registration endpoint from %s: %w", authorizationServer, err)
	}

	if endpoint == "" {
		return "", fmt.Errorf("registration_endpoint not found in discovery document for %s", authorizationServer)
	}

	return endpoint, nil
}

// fetchRegistrationEndpoint fetches the registration endpoint from a well-known URL
func fetchRegistrationEndpoint(ctx context.Context, wellKnownURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnownURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "MCP-Gateway/1.0.0")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("discovery request failed with status %d", resp.StatusCode)
	}

	var metadata struct {
		RegistrationEndpoint string `json:"registration_endpoint"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return "", fmt.Errorf("failed to decode discovery document: %w", err)
	}

	return metadata.RegistrationEndpoint, nil
}

// joinScopes joins a slice of scopes into a space-separated string
// per OAuth 2.0 specification (RFC 6749 Section 3.3)
func joinScopes(scopes []string) string {
	if len(scopes) == 0 {
		return ""
	}

	// Use simple string concatenation for small arrays
	result := scopes[0]
	for i := 1; i < len(scopes); i++ {
		result += " " + scopes[i]
	}
	return result
}
