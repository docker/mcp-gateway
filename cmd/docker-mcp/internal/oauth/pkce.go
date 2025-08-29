package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
)

// GenerateCodeVerifier creates a cryptographically random code verifier (RFC 7636)
// Must be 43-128 characters long using [A-Z] / [a-z] / [0-9] / "-" / "." / "_" / "~"
func GenerateCodeVerifier() string {
	// Generate 96 bytes which will encode to exactly 128 base64url chars
	b := make([]byte, 96)
	_, err := rand.Read(b)
	if err != nil {
		// This should never happen, but fallback to deterministic generation
		panic(fmt.Sprintf("failed to generate random bytes for PKCE: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// GenerateS256Challenge creates SHA256 code challenge from verifier (RFC 7636)
func GenerateS256Challenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// GenerateState creates a random OAuth state parameter
func GenerateState() string {
	b := make([]byte, 32) // 32 bytes = 43 base64url chars
	_, err := rand.Read(b)
	if err != nil {
		panic(fmt.Sprintf("failed to generate random state: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// PKCEFlow contains all parameters needed for OAuth flow with PKCE
type PKCEFlow struct {
	State        string // OAuth state parameter
	CodeVerifier string // PKCE code verifier (kept secret)
	ResourceURL  string // MCP resource URL for token binding
	ServerName   string // MCP server name
}

// BuildAuthorizationURL builds a complete OAuth authorization URL with PKCE parameters
func BuildAuthorizationURL(discovery *OAuthDiscovery, clientID string, scopes []string, serverName string) (string, *PKCEFlow, error) {
	if discovery.AuthorizationEndpoint == "" {
		return "", nil, fmt.Errorf("no authorization endpoint found")
	}

	if clientID == "" {
		return "", nil, fmt.Errorf("client ID is required")
	}

	// Generate PKCE parameters
	verifier := GenerateCodeVerifier()
	challenge := GenerateS256Challenge(verifier)
	state := GenerateState()

	// Build OAuth parameters
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", "http://mcp.docker.com/oauth/callback") // mcp-oauth callback
	params.Set("state", state)

	// PKCE parameters (OAuth 2.1 MUST requirement for public clients)
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256") // Strongest available method

	// Resource parameter (RFC 8707 for token audience binding)
	if discovery.ResourceURL != "" {
		params.Set("resource", discovery.ResourceURL)
	}

	// Add scopes if provided
	if len(scopes) > 0 {
		params.Set("scope", strings.Join(scopes, " "))
	}

	// Build complete authorization URL
	authURL := discovery.AuthorizationEndpoint + "?" + params.Encode()

	// Create PKCE flow object
	pkceFlow := &PKCEFlow{
		State:        state,
		CodeVerifier: verifier,
		ResourceURL:  discovery.ResourceURL,
		ServerName:   serverName,
	}

	return authURL, pkceFlow, nil
}

// OpenBrowser opens the given URL in the user's default browser
func OpenBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)

	return exec.Command(cmd, args...).Start()
}
