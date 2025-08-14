package desktop

import (
	"context"
	"fmt"
	"net/url"
	"os"
)

// MockAuthClient provides a mock implementation for testing without Docker Desktop changes
type MockAuthClient struct {
	*Tools
}

// NewMockAuthClient creates a mock auth client for testing
func NewMockAuthClient() *MockAuthClient {
	return &MockAuthClient{
		Tools: NewAuthClient(),
	}
}

// PostOAuthAppMCPGateway simulates the new Docker Desktop endpoint that:
// 1. Doesn't open a browser
// 2. Uses proper state parameter with mcp-gateway prefix
func (c *MockAuthClient) PostOAuthAppMCPGateway(ctx context.Context, provider, scopes string) (AuthResponse, error) {
	// Get OAuth service URL from environment
	oauthServiceURL := os.Getenv("DOCKER_MCP_AUTH_SERVICE_URL")
	if oauthServiceURL == "" {
		oauthServiceURL = "http://localhost:3278"
	}
	
	// Get GitHub client ID from environment
	clientID := os.Getenv("GITHUB_CLIENT_ID")
	if clientID == "" {
		return AuthResponse{}, fmt.Errorf("GITHUB_CLIENT_ID environment variable not set")
	}
	
	// Build the OAuth URL with proper state
	// State format: "mcp-gateway:http://localhost:3278/oauth/callback"
	callbackURL := "http://localhost:3278/oauth/callback"
	state := fmt.Sprintf("mcp-gateway:%s", url.QueryEscape(callbackURL))
	
	// Construct GitHub OAuth URL
	authURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&state=%s",
		clientID,
		url.QueryEscape(fmt.Sprintf("%s/oauth/callback", oauthServiceURL)),
		state,
	)
	
	// Add scopes if provided
	if scopes != "" {
		authURL += "&scope=" + url.QueryEscape(scopes)
	}
	
	fmt.Printf("[Mock] Generated OAuth URL with state: %s\n", state)
	fmt.Printf("[Mock] OAuth URL: %s\n", authURL)
	
	// Return the URL WITHOUT opening browser
	return AuthResponse{
		BrowserURL: authURL,
		AuthType:   "authorization_code",
	}, nil
}