package oauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtractProviderFromMessage tests provider name extraction from messages that Docker Desktop sends via SSE stream
func TestExtractProviderFromMessage(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		// Message formats (with 'for' pattern)
		{
			name:     "code-received event",
			message:  "Login code received for github-remote",
			expected: "github-remote",
		},
		{
			name:     "login-success event",
			message:  "Login successful for linear-remote",
			expected: "linear-remote",
		},
		{
			name:     "login-start event",
			message:  "Login started for notion-remote",
			expected: "notion-remote",
		},
		{
			name:     "token-refresh event",
			message:  "Token refreshed for asana-remote",
			expected: "asana-remote",
		},
		// Message formats (with 'of' pattern)
		{
			name:     "logout-success event",
			message:  "Successfully logged out of slack-remote",
			expected: "slack-remote",
		},
		// Error event has different format - uses 'during' pattern which we don't extract
		// This is OK because error events don't trigger server reloads
		{
			name:     "error event",
			message:  "An error occurred during github-remote login flow: token expired",
			expected: "",
		},
		// Edge cases
		{
			name:     "no pattern match",
			message:  "Unknown event format",
			expected: "",
		},
		{
			name:     "empty string",
			message:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractProviderFromMessage(tt.message)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestParseOAuthEvent tests parsing of the JSON notification structure that Docker Desktop sends via SSE stream
func TestParseOAuthEvent(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		expected    Event
		expectError bool
	}{
		{
			name: "login-start event with full structure",
			jsonData: `{
				"id": "550e8400-e29b-41d4-a716-446655440000",
				"operation": "mcp-oauth-login-start",
				"type": "ActionStatusType",
				"title": "Login flow started",
				"message": "Login started for github-remote",
				"ephemeral": true,
				"icon": "loading"
			}`,
			expected: Event{
				Type:     EventLoginStart,
				Provider: "github-remote",
				Message:  "Login started for github-remote",
			},
			expectError: false,
		},
		{
			name: "code-received event",
			jsonData: `{
				"id": "550e8400-e29b-41d4-a716-446655440001",
				"operation": "mcp-oauth-code-received",
				"type": "ActionStatusType",
				"title": "Login code received",
				"message": "Login code received for linear-remote",
				"ephemeral": true,
				"icon": "info"
			}`,
			expected: Event{
				Type:     EventCodeReceived,
				Provider: "linear-remote",
				Message:  "Login code received for linear-remote",
			},
			expectError: false,
		},
		{
			name: "login-success event",
			jsonData: `{
				"id": "550e8400-e29b-41d4-a716-446655440002",
				"operation": "mcp-oauth-login-success",
				"type": "ActionStatusType",
				"title": "Login successful",
				"message": "Login successful for notion-remote",
				"ephemeral": false,
				"icon": "success"
			}`,
			expected: Event{
				Type:     EventLoginSuccess,
				Provider: "notion-remote",
				Message:  "Login successful for notion-remote",
			},
			expectError: false,
		},
		{
			name: "token-refresh event",
			jsonData: `{
				"id": "550e8400-e29b-41d4-a716-446655440003",
				"operation": "mcp-oauth-token-refresh",
				"type": "ActionStatusType",
				"title": "Token refreshed",
				"message": "Token refreshed for asana-remote",
				"ephemeral": true,
				"icon": "success"
			}`,
			expected: Event{
				Type:     EventTokenRefresh,
				Provider: "asana-remote",
				Message:  "Token refreshed for asana-remote",
			},
			expectError: false,
		},
		{
			name: "logout-success event",
			jsonData: `{
				"id": "550e8400-e29b-41d4-a716-446655440004",
				"operation": "mcp-oauth-logout-success",
				"type": "ActionStatusType",
				"title": "Logout successful",
				"message": "Successfully logged out of slack-remote",
				"ephemeral": false,
				"icon": "success"
			}`,
			expected: Event{
				Type:     EventLogoutSuccess,
				Provider: "slack-remote",
				Message:  "Successfully logged out of slack-remote",
			},
			expectError: false,
		},
		{
			name: "error event with error field",
			jsonData: `{
				"id": "550e8400-e29b-41d4-a716-446655440005",
				"operation": "mcp-oauth-error",
				"type": "ActionStatusType",
				"title": "Login error",
				"message": "An error occurred during github-remote login flow: token expired",
				"ephemeral": false,
				"icon": "error",
				"error": "token expired"
			}`,
			expected: Event{
				Type:     EventError,
				Provider: "", // Error messages use 'during' pattern which we don't extract
				Message:  "An error occurred during github-remote login flow: token expired",
				Error:    "token expired",
			},
			expectError: false,
		},
		{
			name:        "invalid JSON",
			jsonData:    `{"operation": invalid}`,
			expected:    Event{},
			expectError: true,
		},
		{
			name:        "missing operation field",
			jsonData:    `{"id": "test", "message": "Some message", "type": "ActionStatusType"}`,
			expected:    Event{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseOAuthEvent(tt.jsonData)

			if tt.expectError {
				require.Error(t, err, "expected an error but got none")
			} else {
				require.NoError(t, err, "unexpected error: %v", err)
				assert.Equal(t, tt.expected.Type, result.Type, "event type mismatch")
				assert.Equal(t, tt.expected.Provider, result.Provider, "provider mismatch")
				assert.Equal(t, tt.expected.Message, result.Message, "message mismatch")
				assert.Equal(t, tt.expected.Error, result.Error, "error field mismatch")
			}
		})
	}
}
