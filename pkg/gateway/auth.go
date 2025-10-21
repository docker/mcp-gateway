package gateway

import (
	"crypto/rand"
	"crypto/subtle"
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

// authenticationMiddleware creates an HTTP middleware that validates requests using
// Bearer token in the Authorization header.
//
// The /health endpoint is excluded from authentication.
// Authentication is also disabled when DOCKER_MCP_IN_CONTAINER=1 for compose networking.
func authenticationMiddleware(authToken string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication in container environments (compose networking)
		if os.Getenv("DOCKER_MCP_IN_CONTAINER") == "1" {
			next.ServeHTTP(w, r)
			return
		}

		// Skip authentication for health check endpoint
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		authenticated := false

		// Check for Bearer token in Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			// Extract Bearer token from "Bearer <token>" format
			const bearerPrefix = "Bearer "
			if len(authHeader) > len(bearerPrefix) && authHeader[:len(bearerPrefix)] == bearerPrefix {
				bearerToken := authHeader[len(bearerPrefix):]
				// Use constant-time comparison to prevent timing attacks
				if subtle.ConstantTimeCompare([]byte(bearerToken), []byte(authToken)) == 1 {
					authenticated = true
				}
			}
		}

		if !authenticated {
			// Return 401 Unauthorized with WWW-Authenticate header
			w.Header().Set("WWW-Authenticate", `Bearer realm="MCP Gateway"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Authentication successful, proceed to next handler
		next.ServeHTTP(w, r)
	})
}

// formatGatewayURL formats the gateway URL without authentication info
func formatGatewayURL(port int, endpoint string) string {
	return fmt.Sprintf("http://localhost:%d%s", port, endpoint)
}

// formatBearerToken formats the Bearer token for display in the Authorization header
func formatBearerToken(authToken string) string {
	return fmt.Sprintf("Authorization: Bearer %s", authToken)
}
