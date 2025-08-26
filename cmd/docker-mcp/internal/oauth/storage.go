package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
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

// DockerDesktopStorage implements CredentialStorage using Docker Desktop's API
type DockerDesktopStorage struct {
	client *desktop.RawClient
}

// NewDockerDesktopStorage creates a new credential storage interface
func NewDockerDesktopStorage() CredentialStorage {
	return &DockerDesktopStorage{
		client: desktop.ClientBackend,
	}
}

// StoreClientCredentials stores DCR client credentials via Docker Desktop
func (s *DockerDesktopStorage) StoreClientCredentials(ctx context.Context, serverName string, creds *ClientCredentials) error {
	if creds == nil {
		return fmt.Errorf("credentials cannot be nil")
	}

	// Marshal credentials to JSON
	data, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Create storage request for Docker Desktop
	credentialKey := formatCredentialKey(serverName, "dcr")
	
	req := struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}{
		Key:   credentialKey,
		Value: string(data),
	}

	// Send to Docker Desktop credential storage API
	if err := s.client.Post(ctx, "/oauth/credentials", req, nil); err != nil {
		return fmt.Errorf("failed to store credentials for %s: %w", serverName, err)
	}

	return nil
}

// GetClientCredentials retrieves DCR client credentials from Docker Desktop
func (s *DockerDesktopStorage) GetClientCredentials(ctx context.Context, serverName string) (*ClientCredentials, error) {
	credentialKey := formatCredentialKey(serverName, "dcr")
	
	// Query Docker Desktop for stored credentials
	var response struct {
		Value string `json:"value"`
	}

	queryParams := url.Values{}
	queryParams.Set("key", credentialKey)
	
	err := s.client.Get(ctx, "/oauth/credentials?"+queryParams.Encode(), &response)
	if err != nil {
		// Check if it's a "not found" error
		if isNotFoundError(err) {
			return nil, fmt.Errorf("no credentials found for %s", serverName)
		}
		return nil, fmt.Errorf("failed to retrieve credentials for %s: %w", serverName, err)
	}

	if response.Value == "" {
		return nil, fmt.Errorf("empty credentials for %s", serverName)
	}

	// Unmarshal the credentials
	var creds ClientCredentials
	if err := json.Unmarshal([]byte(response.Value), &creds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials for %s: %w", serverName, err)
	}

	return &creds, nil
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

// GetOrCreateDCRCredentials gets existing DCR credentials or creates new ones via DCR
func GetOrCreateDCRCredentials(ctx context.Context, storage CredentialStorage, discovery *OAuthDiscovery, serverName string) (*ClientCredentials, error) {
	// Check if we already have credentials
	if exists, err := storage.HasClientCredentials(ctx, serverName); err == nil && exists {
		creds, err := storage.GetClientCredentials(ctx, serverName)
		if err == nil {
			return creds, nil
		}
		// If we can't retrieve them, fall through to DCR
	}

	// No existing credentials, perform DCR
	creds, err := PerformDCR(ctx, discovery, serverName)
	if err != nil {
		return nil, fmt.Errorf("DCR failed for %s: %w", serverName, err)
	}

	// Store the new credentials
	if err := storage.StoreClientCredentials(ctx, serverName, creds); err != nil {
		// Log warning but don't fail - we still have the credentials
		fmt.Printf("Warning: failed to store credentials for %s: %v\n", serverName, err)
	}

	return creds, nil
}

// formatCredentialKey creates a consistent key format for storing credentials
// Format: "dcr_<server-name>" for DCR credentials
func formatCredentialKey(serverName, credType string) string {
	// Sanitize server name for use as a key
	sanitized := strings.ReplaceAll(serverName, "/", "_")
	sanitized = strings.ReplaceAll(sanitized, ":", "_")
	sanitized = strings.ReplaceAll(sanitized, ".", "_")
	
	return fmt.Sprintf("%s_%s", credType, sanitized)
}

// isNotFoundError checks if an error represents a "not found" condition
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "not found") || 
		   strings.Contains(errStr, "404") ||
		   strings.Contains(errStr, "no such") ||
		   strings.Contains(errStr, "does not exist")
}