package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestIsAllowedOrigin tests the isAllowedOrigin helper function with various inputs.
func TestIsAllowedOrigin(t *testing.T) {
	tests := []struct {
		name     string
		origin   string
		expected bool
	}{
		// Valid localhost origins
		{"http localhost no port", "http://localhost", true},
		{"https localhost no port", "https://localhost", true},
		{"http localhost with port", "http://localhost:3000", true},
		{"https localhost with port", "https://localhost:8080", true},
		{"http 127.0.0.1 no port", "http://127.0.0.1", true},
		{"https 127.0.0.1 no port", "https://127.0.0.1", true},
		{"http 127.0.0.1 with port", "http://127.0.0.1:8080", true},
		{"https 127.0.0.1 with port", "https://127.0.0.1:5000", true},
		{"http IPv6 localhost", "http://[::1]", true},
		{"https IPv6 localhost", "https://[::1]", true},
		{"http IPv6 localhost with port", "http://[::1]:8080", true},
		{"https IPv6 localhost with port", "https://[::1]:3000", true},

		// Invalid origins - malicious domains
		{"evil domain", "https://evil.com", false},
		{"evil domain with port", "https://evil.com:8080", false},
		{"subdomain attack", "http://localhost.evil.com", false},
		{"subdomain with 127", "http://127.0.0.1.evil.com", false},

		// Invalid origins - RFC 1122 prohibits 0.0.0.0 as destination
		{"0.0.0.0 address", "http://0.0.0.0:8080", false},
		{"0.0.0.0 no port", "http://0.0.0.0", false},
		{"all zeros IPv6", "http://[::]:8080", false},

		// Invalid schemes
		{"ftp scheme", "ftp://localhost", false},
		{"ws scheme", "ws://localhost", false},
		{"file scheme", "file://localhost", false},

		// Malformed URLs
		{"not a URL", "not-a-url", false},
		{"missing scheme", "localhost:8080", false},
		{"single slash", "http:/localhost", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isAllowedOrigin(tt.origin)
			if result != tt.expected {
				t.Errorf("isAllowedOrigin(%q) = %v, expected %v", tt.origin, result, tt.expected)
			}
		})
	}
}

func TestOriginSecurityHandler(t *testing.T) {
	// Create a simple handler that always succeeds if reached
	successHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	// Wrap it with our security handler
	secureHandler := originSecurityHandler(successHandler)

	tests := []struct {
		name           string
		origin         string
		expectedStatus int
		reason         string
	}{
		{
			name:           "No Origin header (non-browser clients)",
			origin:         "",
			expectedStatus: http.StatusOK,
			reason:         "CRITICAL: curl, SDKs, and same-origin browser requests must work. Browsers don't send Origin for same-origin requests.",
		},
		{
			name:           "Malicious origin (the actual attack)",
			origin:         "https://evil.com",
			expectedStatus: http.StatusForbidden,
			reason:         "CRITICAL: This is the vulnerability we're fixing. Block cross-origin requests from non-localhost origins.",
		},
		{
			name:           "Localhost origin (legitimate browser client)",
			origin:         "http://localhost:3000",
			expectedStatus: http.StatusOK,
			reason:         "CRITICAL: Developer running local frontend on different port must work. Common development scenario.",
		},
		{
			name:           "Non-localhost IP origin",
			origin:         "http://0.0.0.0:8080",
			expectedStatus: http.StatusForbidden,
			reason:         "Block non-localhost IPs. Note: In DNS rebinding, evil.com resolves to 0.0.0.0 but Origin would be http://evil.com",
		},
		{
			name:           "Subdomain bypass attempt",
			origin:         "http://localhost.evil.com",
			expectedStatus: http.StatusForbidden,
			reason:         "IMPORTANT: Prevent validation bypass using subdomain that contains 'localhost'. Common attack technique.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			rr := httptest.NewRecorder()
			secureHandler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d\nReason: %s", tt.expectedStatus, rr.Code, tt.reason)
			}

			// Verify response body for blocked requests contains helpful error message
			if tt.expectedStatus == http.StatusForbidden {
				body := rr.Body.String()
				if !strings.Contains(body, "Forbidden") || !strings.Contains(body, "Origin") {
					t.Errorf("Expected error body to contain 'Forbidden' and 'Origin', got %q", body)
				}
			}
		})
	}
}

// TestCombinedSecurityLayers verifies that both Origin validation and authentication work together.
// This ensures defense-in-depth: both layers must pass for a request to succeed.
func TestCombinedSecurityLayers(t *testing.T) {
	authToken := "test-token-secure-123"

	successHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	// Wrap with both layers (same as production code)
	withOriginCheck := originSecurityHandler(successHandler)
	withBothLayers := authenticationMiddleware(authToken, withOriginCheck)

	tests := []struct {
		name           string
		origin         string
		authHeader     string
		expectedStatus int
		reason         string
	}{
		{
			name:           "Valid origin + valid token",
			origin:         "http://localhost:3000",
			authHeader:     "Bearer test-token-secure-123",
			expectedStatus: http.StatusOK,
			reason:         "Both security layers pass - request should succeed",
		},
		{
			name:           "Valid origin + invalid token",
			origin:         "http://localhost:3000",
			authHeader:     "Bearer wrong-token",
			expectedStatus: http.StatusUnauthorized,
			reason:         "Origin is valid but auth fails - should block at auth layer",
		},
		{
			name:           "Invalid origin + valid token",
			origin:         "https://evil.com",
			authHeader:     "Bearer test-token-secure-123",
			expectedStatus: http.StatusForbidden,
			reason:         "Token is valid but origin is malicious - should block at origin layer",
		},
		{
			name:           "Invalid origin + no token",
			origin:         "https://evil.com",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			reason:         "Both layers fail - auth middleware (outer) checks first, blocks with 401",
		},
		{
			name:           "No origin + valid token (CLI client)",
			origin:         "",
			authHeader:     "Bearer test-token-secure-123",
			expectedStatus: http.StatusOK,
			reason:         "CLI tools with valid auth should work",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			rr := httptest.NewRecorder()
			withBothLayers.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d\nReason: %s", tt.expectedStatus, rr.Code, tt.reason)
			}
		})
	}
}

func authenticationMiddleware(token string, next http.Handler) http.Handler {
	store := TokenStore{
		token: {Identity: "test"},
	}
	return authenticationMiddlewareMulti(store, next)
}
