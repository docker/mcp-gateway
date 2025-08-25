package gateway

import (
	"crypto/rand"
	"fmt"
	"strings"
)

const (
	// SessionIDPrefix is the prefix for all gateway session IDs
	SessionIDPrefix = "mcp-gateway"
	// SessionIDLength is the length of the random suffix (8 characters)
	SessionIDLength = 8
)

// GenerateSessionID creates a unique session ID for the gateway instance
// Format: "mcp-gateway-<8-char-random-hex>"
func GenerateSessionID() (string, error) {
	// Generate 4 random bytes (8 hex characters)
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random session ID: %w", err)
	}

	// Convert to hex string (lowercase)
	randomHex := fmt.Sprintf("%x", randomBytes)

	return fmt.Sprintf("%s-%s", SessionIDPrefix, randomHex), nil
}

// IsValidSessionID checks if a session ID has the correct format
func IsValidSessionID(sessionID string) bool {
	if !strings.HasPrefix(sessionID, SessionIDPrefix+"-") {
		return false
	}

	suffix := strings.TrimPrefix(sessionID, SessionIDPrefix+"-")
	return len(suffix) == SessionIDLength
}

// GetSessionIDLabels returns Kubernetes labels for session tracking
func GetSessionIDLabels(sessionID string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by":   "mcp-gateway",
		"app.kubernetes.io/instance":     sessionID,
		"app.kubernetes.io/component":    "mcp-server",
		"mcp-gateway.docker.com/session": sessionID,
	}
}
