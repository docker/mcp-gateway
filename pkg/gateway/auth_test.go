package gateway

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestGenerateAuthToken(t *testing.T) {
	token, err := generateAuthToken()
	if err != nil {
		t.Fatalf("generateAuthToken() failed: %v", err)
	}

	if len(token) != tokenLength {
		t.Errorf("expected token length %d, got %d", tokenLength, len(token))
	}

	// Check that token only contains allowed characters
	for _, ch := range token {
		if !strings.ContainsRune(tokenCharset, ch) {
			t.Errorf("token contains invalid character: %c", ch)
		}
	}
}

func TestGetOrGenerateAuthToken_FromEnvironment(t *testing.T) {
	expectedToken := "test-token-from-env"
	os.Setenv("MCP_GATEWAY_AUTH_TOKEN", expectedToken)
	defer os.Unsetenv("MCP_GATEWAY_AUTH_TOKEN")

	token, wasGenerated, err := getOrGenerateAuthToken()
	if err != nil {
		t.Fatalf("getOrGenerateAuthToken() failed: %v", err)
	}

	if token != expectedToken {
		t.Errorf("expected token %q, got %q", expectedToken, token)
	}

	if wasGenerated {
		t.Error("expected wasGenerated to be false when token is from environment")
	}
}

func TestGetOrGenerateAuthToken_Generated(t *testing.T) {
	os.Unsetenv("MCP_GATEWAY_AUTH_TOKEN")

	token, wasGenerated, err := getOrGenerateAuthToken()
	if err != nil {
		t.Fatalf("getOrGenerateAuthToken() failed: %v", err)
	}

	if len(token) != tokenLength {
		t.Errorf("expected token length %d, got %d", tokenLength, len(token))
	}

	if !wasGenerated {
		t.Error("expected wasGenerated to be true when token is generated")
	}
}

func TestGetOrGenerateAuthToken_EmptyEnvironment(t *testing.T) {
	os.Setenv("MCP_GATEWAY_AUTH_TOKEN", "")
	defer os.Unsetenv("MCP_GATEWAY_AUTH_TOKEN")

	token, wasGenerated, err := getOrGenerateAuthToken()
	if err != nil {
		t.Fatalf("getOrGenerateAuthToken() failed: %v", err)
	}

	if len(token) != tokenLength {
		t.Errorf("expected token length %d, got %d", tokenLength, len(token))
	}

	if !wasGenerated {
		t.Error("expected wasGenerated to be true when environment token is empty")
	}
}

func TestAuthenticationMiddleware_HealthEndpoint(t *testing.T) {
	authToken := "test-token-123"
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("healthy"))
	})

	middleware := authenticationMiddleware(authToken, handler)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d for /health, got %d", http.StatusOK, w.Code)
	}
}

func TestAuthenticationMiddleware_BearerAuth_Valid(t *testing.T) {
	authToken := "test-token-123"
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	middleware := authenticationMiddleware(authToken, handler)

	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	// Set Bearer token in Authorization header
	req.Header.Set("Authorization", "Bearer "+authToken)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d with valid bearer auth, got %d", http.StatusOK, w.Code)
	}
}

func TestAuthenticationMiddleware_BearerAuth_Invalid(t *testing.T) {
	authToken := "test-token-123"
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	middleware := authenticationMiddleware(authToken, handler)

	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d with invalid bearer auth, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestAuthenticationMiddleware_NoAuth(t *testing.T) {
	authToken := "test-token-123"
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	middleware := authenticationMiddleware(authToken, handler)

	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d with no auth, got %d", http.StatusUnauthorized, w.Code)
	}

	// Check for WWW-Authenticate header
	if w.Header().Get("WWW-Authenticate") == "" {
		t.Error("expected WWW-Authenticate header to be set")
	}
}

func TestAuthenticationMiddleware_BearerAuth_MalformedHeader(t *testing.T) {
	authToken := "test-token-123"
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	middleware := authenticationMiddleware(authToken, handler)

	// Test with malformed Authorization headers - all should fail
	malformedHeaders := []string{
		"bearer " + authToken,  // lowercase bearer
		"Basic " + authToken,   // wrong auth type
		"Bearer",               // missing token
		authToken,              // missing Bearer prefix
		"Bearer  " + authToken, // extra space
	}

	for _, header := range malformedHeaders {
		req := httptest.NewRequest(http.MethodGet, "/sse", nil)
		req.Header.Set("Authorization", header)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d with malformed header %q, got %d", http.StatusUnauthorized, header, w.Code)
		}
	}
}

func TestFormatGatewayURL(t *testing.T) {
	tests := []struct {
		port     int
		endpoint string
		expected string
	}{
		{8811, "/sse", "http://localhost:8811/sse"},
		{3000, "/mcp", "http://localhost:3000/mcp"},
		{80, "/test", "http://localhost:80/test"},
	}

	for _, tt := range tests {
		result := formatGatewayURL(tt.port, tt.endpoint)
		if result != tt.expected {
			t.Errorf("formatGatewayURL(%d, %q) = %q, want %q", tt.port, tt.endpoint, result, tt.expected)
		}
	}
}

func TestFormatBearerToken(t *testing.T) {
	authToken := "test-token-123"
	result := formatBearerToken(authToken)

	expected := "Authorization: Bearer " + authToken
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}

	// Verify it has the correct prefix
	if !strings.HasPrefix(result, "Authorization: Bearer ") {
		t.Errorf("expected result to start with 'Authorization: Bearer ', got %q", result)
	}
}
