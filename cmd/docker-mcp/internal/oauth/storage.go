package oauth

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
)

// CredentialStorage provides an interface for storing and retrieving OAuth credentials
// via Docker Desktop's credential helper system
type CredentialStorage interface {
	// StoreClientCredentials stores DCR client credentials for a server
	StoreClientCredentials(ctx context.Context, serverName string, creds *ClientCredentials) error
	
	// GetClientCredentials retrieves stored DCR client credentials for a server
	GetClientCredentials(ctx context.Context, serverName string) (*ClientCredentials, error)
	
	// HasClientCredentials checks if client credentials exist for a server
	HasClientCredentials(ctx context.Context, serverName string) (bool, error)
	
}

// DockerDesktopStorage implements CredentialStorage using Docker Desktop's DCR API
type DockerDesktopStorage struct {
	client *desktop.Tools
}

// NewDockerDesktopStorage creates a new credential storage interface
func NewDockerDesktopStorage() CredentialStorage {
	return &DockerDesktopStorage{
		client: desktop.NewAuthClient(),
	}
}

// StoreClientCredentials stores DCR client credentials via Docker Desktop DCR API
func (s *DockerDesktopStorage) StoreClientCredentials(ctx context.Context, serverName string, creds *ClientCredentials) error {
	
	if creds == nil {
		return fmt.Errorf("credentials cannot be nil")
	}

	// Create DCR registration request
	req := desktop.RegisterDCRRequest{
		ClientID:              creds.ClientID,
		ClientName:            fmt.Sprintf("MCP Gateway - %s", serverName),
		AuthorizationEndpoint: creds.AuthorizationEndpoint,
		TokenEndpoint:         creds.TokenEndpoint,
	}

	// Register with Docker Desktop DCR API
	if err := s.client.RegisterDCRClient(ctx, serverName, req); err != nil {
		return fmt.Errorf("failed to store credentials for %s: %w", serverName, err)
	}
	return nil
}

// GetClientCredentials retrieves DCR client credentials from Docker Desktop
func (s *DockerDesktopStorage) GetClientCredentials(ctx context.Context, serverName string) (*ClientCredentials, error) {
	
	// Get DCR client from Docker Desktop DCR API
	dcrClient, err := s.client.GetDCRClient(ctx, serverName)
	if err != nil {
		return nil, fmt.Errorf("no credentials found for %s: %w", serverName, err)
	}
	
	creds := &ClientCredentials{
		ClientID: dcrClient.ClientID,
	}
	return creds, nil
}

// HasClientCredentials checks if client credentials exist for a server
func (s *DockerDesktopStorage) HasClientCredentials(ctx context.Context, serverName string) (bool, error) {
	_, err := s.GetClientCredentials(ctx, serverName)
	if err != nil {
		if strings.Contains(err.Error(), "no credentials found") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}


