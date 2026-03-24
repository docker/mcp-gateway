package gateway

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
)

type TokenInfo struct {
	Identity string
}

type TokenStore map[string]TokenInfo

type contextKey string

const authIdentityKey contextKey = "auth.identity"

const (
	tokenLength = 50
	// Characters to use for random token generation (lowercase letters and numbers)
	tokenCharset = "abcdefghijklmnopqrstuvwxyz0123456789"
)

func loadAuthTokens() (TokenStore, error) {
	raw := os.Getenv("MCP_GATEWAY_AUTH_TOKENS")
	store := make(TokenStore)

	if raw == "" {
		return store, nil
	}

	entries := strings.Split(raw, ",")
	for _, entry := range entries {
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid token entry: %s", entry)
		}

		store[parts[1]] = TokenInfo{Identity: parts[0]}
	}

	return store, nil
}

func IdentityFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(authIdentityKey).(string)
	return id, ok
}

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

// authenticationMiddlewareMulti creates an HTTP middleware that validates
// Bearer tokens from a TokenStore.
// The /health route is always public.
func authenticationMiddlewareMulti(store TokenStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip /health route
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		// Retrieve the Authorization header from the incoming request
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			unauthorized(w)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		// Constant-time comparison against each token in the store
		for t, info := range store {
			if subtle.ConstantTimeCompare([]byte(token), []byte(t)) == 1 {
				// Store identity in the request context
				ctx := r.Context()
				ctx = context.WithValue(ctx, authIdentityKey, info.Identity)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		unauthorized(w)
	})
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="MCP Gateway"`)
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

// formatGatewayURL formats the gateway URL without authentication info
func formatGatewayURL(port int, endpoint string) string {
	return fmt.Sprintf("http://localhost:%d%s", port, endpoint)
}

// formatBearerToken formats the Bearer token for display in the Authorization header
func formatBearerToken(authToken string) string {
	return fmt.Sprintf("Authorization: Bearer %s", authToken)
}
