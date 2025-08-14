package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// GitHubToken represents the OAuth token response from GitHub
type GitHubToken struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// ExchangeCodeForToken exchanges an authorization code for an access token
func ExchangeCodeForToken(ctx context.Context, code string) (*GitHubToken, error) {
	// Get client credentials from environment or use test values
	clientID := os.Getenv("GITHUB_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("GitHub OAuth credentials not configured. Set GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET environment variables")
	}
	
	// GitHub's token endpoint
	tokenURL := "https://github.com/login/oauth/access_token"
	
	// Prepare the request
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("code", code)
	
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating token request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	
	// Make the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exchanging code for token: %w", err)
	}
	defer resp.Body.Close()
	
	// Parse the response
	var tokenResp GitHubToken
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}
	
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("no access token in response")
	}
	
	fmt.Printf("Successfully obtained GitHub access token (scopes: %s)\n", tokenResp.Scope)
	return &tokenResp, nil
}

// CompleteOAuthFlow handles the complete OAuth flow after starting the callback server
func CompleteOAuthFlow(ctx context.Context) (*GitHubToken, error) {
	fmt.Println("Waiting for OAuth callback...")
	
	// Wait for authorization code (5 minute timeout)
	code, err := WaitForAuthCode(5 * time.Minute)
	if err != nil {
		return nil, fmt.Errorf("waiting for auth code: %w", err)
	}
	
	fmt.Printf("Received authorization code: %s\n", code)
	
	// Exchange code for token
	token, err := ExchangeCodeForToken(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchanging code for token: %w", err)
	}
	
	// TODO: Store token using Docker Desktop's credential helper
	// For now, we just return it
	
	return token, nil
}