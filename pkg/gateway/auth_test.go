package gateway

import (
	"encoding/base64"
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
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy"))
	})

	middleware := authenticationMiddleware(authToken, handler)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d for /health, got %d", http.StatusOK, w.Code)
	}
}

func TestAuthenticationMiddleware_QueryParamAuth_Valid(t *testing.T) {
	authToken := "test-token-123"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	middleware := authenticationMiddleware(authToken, handler)

	req := httptest.NewRequest("GET", "/sse?MCP_GATEWAY_AUTH_TOKEN=test-token-123", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d with valid query param token, got %d", http.StatusOK, w.Code)
	}
}

func TestAuthenticationMiddleware_QueryParamAuth_Invalid(t *testing.T) {
	authToken := "test-token-123"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	middleware := authenticationMiddleware(authToken, handler)

	req := httptest.NewRequest("GET", "/sse?MCP_GATEWAY_AUTH_TOKEN=wrong-token", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d with invalid query param token, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestAuthenticationMiddleware_BasicAuth_Valid(t *testing.T) {
	authToken := "test-token-123"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	middleware := authenticationMiddleware(authToken, handler)

	req := httptest.NewRequest("GET", "/sse", nil)
	// Set basic auth with any username and the token as password
	req.SetBasicAuth("user", authToken)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d with valid basic auth, got %d", http.StatusOK, w.Code)
	}
}

func TestAuthenticationMiddleware_BasicAuth_Invalid(t *testing.T) {
	authToken := "test-token-123"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	middleware := authenticationMiddleware(authToken, handler)

	req := httptest.NewRequest("GET", "/sse", nil)
	req.SetBasicAuth("user", "wrong-password")
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d with invalid basic auth, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestAuthenticationMiddleware_NoAuth(t *testing.T) {
	authToken := "test-token-123"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	middleware := authenticationMiddleware(authToken, handler)

	req := httptest.NewRequest("GET", "/sse", nil)
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

func TestAuthenticationMiddleware_BasicAuth_UsernameIgnored(t *testing.T) {
	authToken := "test-token-123"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	middleware := authenticationMiddleware(authToken, handler)

	// Test with different usernames - all should work
	usernames := []string{"user", "admin", "anything", ""}
	for _, username := range usernames {
		req := httptest.NewRequest("GET", "/sse", nil)
		req.SetBasicAuth(username, authToken)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d with username %q, got %d", http.StatusOK, username, w.Code)
		}
	}
}

func TestFormatGatewayURL(t *testing.T) {
	tests := []struct {
		port      int
		endpoint  string
		authToken string
		expected  string
	}{
		{8811, "/sse", "abc123", "http://localhost:8811/sse?MCP_GATEWAY_AUTH_TOKEN=abc123"},
		{3000, "/mcp", "xyz789", "http://localhost:3000/mcp?MCP_GATEWAY_AUTH_TOKEN=xyz789"},
		{80, "/test", "token", "http://localhost:80/test?MCP_GATEWAY_AUTH_TOKEN=token"},
	}

	for _, tt := range tests {
		result := formatGatewayURL(tt.port, tt.endpoint, tt.authToken)
		if result != tt.expected {
			t.Errorf("formatGatewayURL(%d, %q, %q) = %q, want %q", tt.port, tt.endpoint, tt.authToken, result, tt.expected)
		}
	}
}

func TestFormatBasicAuthCredentials(t *testing.T) {
	authToken := "test-token-123"
	result := formatBasicAuthCredentials(authToken)

	if !strings.HasPrefix(result, "Authorization: Basic ") {
		t.Errorf("expected result to start with 'Authorization: Basic ', got %q", result)
	}

	// Decode and verify the base64 encoded credentials
	encoded := strings.TrimPrefix(result, "Authorization: Basic ")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("failed to decode base64: %v", err)
	}

	expected := "user:" + authToken
	if string(decoded) != expected {
		t.Errorf("expected decoded credentials to be %q, got %q", expected, string(decoded))
	}
}
