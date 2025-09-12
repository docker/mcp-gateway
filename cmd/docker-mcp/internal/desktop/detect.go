package desktop

import (
	"context"
	"errors"
	"os"
)

// IsDockerDesktopAvailable checks if Docker Desktop is available and functional.
// It verifies that the necessary sockets exist and are accessible.
func IsDockerDesktopAvailable(ctx context.Context) bool {
	paths := Paths()
	
	// Check if JFS socket exists and is accessible
	if _, err := os.Stat(paths.JFSSocket); err != nil {
		return false
	}
	
	// Try to connect to the secrets service to verify it's functional
	client := NewSecretsClient()
	_, err := client.ListJfsSecrets(ctx)
	
	// If we can list secrets without error, Desktop is available
	// Note: An empty list is still a successful response
	return err == nil
}

// MustHaveDockerDesktop checks if Docker Desktop is available and panics if not.
// This is used in contexts where Desktop is absolutely required.
func MustHaveDockerDesktop(ctx context.Context) error {
	if !IsDockerDesktopAvailable(ctx) {
		return errors.New("Docker Desktop is not available. This operation requires Docker Desktop or an alternative secret provider")
	}
	return nil
}

// DesktopAvailabilityInfo provides detailed information about Desktop availability.
type DesktopAvailabilityInfo struct {
	Available    bool
	JFSSocket    string
	SocketExists bool
	ServiceError error
}

// GetDesktopAvailabilityInfo returns detailed information about Docker Desktop availability.
func GetDesktopAvailabilityInfo(ctx context.Context) DesktopAvailabilityInfo {
	paths := Paths()
	info := DesktopAvailabilityInfo{
		JFSSocket: paths.JFSSocket,
	}
	
	// Check socket existence
	if _, err := os.Stat(paths.JFSSocket); err != nil {
		info.SocketExists = false
		info.ServiceError = err
		return info
	}
	info.SocketExists = true
	
	// Test service functionality
	client := NewSecretsClient()
	_, err := client.ListJfsSecrets(ctx)
	if err != nil {
		info.ServiceError = err
		return info
	}
	
	info.Available = true
	return info
}