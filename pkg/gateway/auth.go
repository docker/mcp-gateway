package gateway

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"math/big"
	"net/http"
	"os"
)

const (
	tokenLength = 50
	// Characters to use for random token generation (lowercase letters and numbers)
	tokenCharset = "abcdefghijklmnopqrstuvwxyz0123456789"
)

// generateAuthToken generates a random 50-character string using lowercase letters and numbers
func generateAuthToken() (string, error) {
	token := make([]byte, tokenLength)
	charsetLen := big.NewInt(int64(len(tokenCharset)))

	for i := range tokenLength {
		num, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", fmt.Errorf("failed to generate random token: %w", err)
		}
		token[i] = tokenCharset[num.Int64()]
	}

	return string(token), nil
}

// getOrGenerateAuthToken retrieves the auth token from environment variable MCP_GATEWAY_AUTH_TOKEN
// or generates a new one if not set or empty
func getOrGenerateAuthToken() (string, bool, error) {
	envToken := os.Getenv("MCP_GATEWAY_AUTH_TOKEN")
	if envToken != "" {
		return envToken, false, nil // false indicates token was from environment
	}

	token, err := generateAuthToken()
	if err != nil {
		return "", false, err
	}
	return token, true, nil // true indicates token was generated
}

// authenticationMiddleware creates an HTTP middleware that validates requests using either:
// 1. Basic authentication (username can be anything, password must match the auth token)
// 2. Query parameter MCP_GATEWAY_AUTH_TOKEN that matches the auth token
//
// The /health endpoint is excluded from authentication.
func authenticationMiddleware(authToken string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication for health check endpoint
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		authenticated := false

		// Check for query parameter authentication
		if queryToken := r.URL.Query().Get("MCP_GATEWAY_AUTH_TOKEN"); queryToken != "" {
			// Use constant-time comparison to prevent timing attacks
			if subtle.ConstantTimeCompare([]byte(queryToken), []byte(authToken)) == 1 {
				authenticated = true
			}
		}

		// Check for basic authentication if query param auth failed
		if !authenticated {
			username, password, hasBasicAuth := r.BasicAuth()
			if hasBasicAuth {
				// Username can be anything, only password needs to match
				_ = username // Explicitly ignore username
				if subtle.ConstantTimeCompare([]byte(password), []byte(authToken)) == 1 {
					authenticated = true
				}
			}
		}

		if !authenticated {
			// Return 401 Unauthorized with WWW-Authenticate header
			w.Header().Set("WWW-Authenticate", `Basic realm="MCP Gateway"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Authentication successful, proceed to next handler
		next.ServeHTTP(w, r)
	})
}

// formatGatewayURL formats the gateway URL with the auth token as a query parameter
func formatGatewayURL(port int, endpoint string, authToken string) string {
	return fmt.Sprintf("http://localhost:%d%s?MCP_GATEWAY_AUTH_TOKEN=%s", port, endpoint, authToken)
}

// formatBasicAuthCredentials formats the basic auth credentials for display
func formatBasicAuthCredentials(authToken string) string {
	// Encode as "username:password" in base64 for convenience
	credentials := fmt.Sprintf("user:%s", authToken)
	encoded := base64.StdEncoding.EncodeToString([]byte(credentials))
	return fmt.Sprintf("Authorization: Basic %s", encoded)
}
