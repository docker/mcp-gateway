package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestOriginSecurityHandler verifies that the Origin header validation prevents DNS rebinding attacks
// as required by the MCP specification:
// https://modelcontextprotocol.io/specification/2024-11-05/basic/transports#security-warning
//
// Attack Scenario:
//  1. Developer runs: docker mcp gateway run --transport streaming --port 8080
//  2. Developer visits malicious website (https://evil.com)
//  3. JavaScript on evil.com tries: fetch('http://0.0.0.0:8080/mcp', {...})
//  4. Browser automatically adds: Origin: https://evil.com
//  5. Our validation MUST block this request
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
			name:           "DNS rebinding via 0.0.0.0",
			origin:         "http://0.0.0.0:8080",
			expectedStatus: http.StatusForbidden,
			reason:         "IMPORTANT: Specifically mentioned in vulnerability report. 0.0.0.0 bypasses browser CORS protections.",
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

			// Verify response body for blocked requests
			if tt.expectedStatus == http.StatusForbidden {
				expectedBody := "Forbidden: Invalid Origin header\n"
				if rr.Body.String() != expectedBody {
					t.Errorf("Expected body %q, got %q", expectedBody, rr.Body.String())
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
