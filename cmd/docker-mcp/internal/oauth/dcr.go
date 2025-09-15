package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
func PerformDCR(ctx context.Context, discovery *Discovery, serverName string) (*ClientCredentials, error) {
	if discovery.RegistrationEndpoint == "" {
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
	}

	// Marshal the registration request
	body, err := json.Marshal(registration)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal DCR request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, discovery.RegistrationEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create DCR request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "MCP-Gateway/1.0.0")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send DCR request to %s: %w", discovery.RegistrationEndpoint, err)
	}
	defer resp.Body.Close()

	// Check response status (201 Created or 200 OK are acceptable)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		// Read error response body to understand why DCR failed
		errorBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("DCR failed with status %d for %s", resp.StatusCode, serverName)
		}

		errorMsg := string(errorBody)

		// Try to parse as JSON for structured error
		var errorResp map[string]any
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

		return nil, fmt.Errorf("DCR failed with status %d for %s: %s", resp.StatusCode, serverName, errorMsg)
	}

	// Parse the response
	var dcrResponse DCRResponse
	if err := json.NewDecoder(resp.Body).Decode(&dcrResponse); err != nil {
		return nil, fmt.Errorf("failed to decode DCR response: %w", err)
	}

	if dcrResponse.ClientID == "" {
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

	return creds, nil
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
