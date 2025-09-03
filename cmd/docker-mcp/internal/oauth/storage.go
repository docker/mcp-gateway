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
	
	// StorePKCEParameters stores PKCE parameters for OAuth callback
	StorePKCEParameters(ctx context.Context, flow *PKCEFlow) error
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
	fmt.Printf("DEBUG: StoreClientCredentials called for %s with client_id: %s\n", serverName, creds.ClientID)
	
	if creds == nil {
		fmt.Printf("DEBUG: Credentials are nil for %s\n", serverName)
		return fmt.Errorf("credentials cannot be nil")
	}

	// Create DCR registration request
	req := desktop.RegisterDCRRequest{
		ClientID:              creds.ClientID,
		ClientName:            fmt.Sprintf("MCP Gateway - %s", serverName),
		AuthorizationEndpoint: creds.AuthorizationEndpoint,
		TokenEndpoint:         creds.TokenEndpoint,
	}

	fmt.Printf("DEBUG: Calling RegisterDCRClient with request: %+v\n", req)

	// Register with Docker Desktop DCR API
	if err := s.client.RegisterDCRClient(ctx, serverName, req); err != nil {
		fmt.Printf("DEBUG: RegisterDCRClient failed: %v\n", err)
		return fmt.Errorf("failed to store credentials for %s: %w", serverName, err)
	}

	fmt.Printf("DEBUG: RegisterDCRClient succeeded for %s\n", serverName)
	return nil
}

// GetClientCredentials retrieves DCR client credentials from Docker Desktop
func (s *DockerDesktopStorage) GetClientCredentials(ctx context.Context, serverName string) (*ClientCredentials, error) {
	fmt.Printf("DEBUG: GetClientCredentials called for %s\n", serverName)
	
	// Get DCR client from Docker Desktop DCR API
	dcrClient, err := s.client.GetDCRClient(ctx, serverName)
	if err != nil {
		fmt.Printf("DEBUG: GetDCRClient failed for %s: %v\n", serverName, err)
		return nil, fmt.Errorf("no credentials found for %s: %w", serverName, err)
	}

	fmt.Printf("DEBUG: Retrieved DCR client: %+v\n", dcrClient)
	
	creds := &ClientCredentials{
		ClientID: dcrClient.ClientID,
	}
	
	fmt.Printf("DEBUG: Returning credentials with client_id: %s\n", creds.ClientID)
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

// GetOrCreateDCRCredentials gets existing DCR credentials or creates new ones via DCR
func GetOrCreateDCRCredentials(ctx context.Context, storage CredentialStorage, discovery *OAuthDiscovery, serverName string) (*ClientCredentials, error) {
	fmt.Printf("DEBUG: Getting or creating DCR credentials for %s\n", serverName)
	
	// Check if we already have credentials
	if exists, err := storage.HasClientCredentials(ctx, serverName); err == nil && exists {
		fmt.Printf("DEBUG: Found existing credentials for %s\n", serverName)
		creds, err := storage.GetClientCredentials(ctx, serverName)
		if err == nil {
			fmt.Printf("DEBUG: Successfully retrieved existing credentials: %s\n", creds.ClientID)
			return creds, nil
		}
		fmt.Printf("DEBUG: Failed to retrieve existing credentials, falling through to DCR: %v\n", err)
		// If we can't retrieve them, fall through to DCR
	} else {
		fmt.Printf("DEBUG: No existing credentials found (exists=%v, err=%v)\n", exists, err)
	}

	// No existing credentials, perform DCR
	fmt.Printf("DEBUG: Performing DCR for %s\n", serverName)
	creds, err := PerformDCR(ctx, discovery, serverName)
	if err != nil {
		fmt.Printf("DEBUG: DCR failed for %s: %v\n", serverName, err)
		return nil, fmt.Errorf("DCR failed for %s: %w", serverName, err)
	}

	fmt.Printf("DEBUG: DCR successful, got client_id: %s\n", creds.ClientID)

	// Store the new credentials
	fmt.Printf("DEBUG: Storing credentials for %s\n", serverName)
	if err := storage.StoreClientCredentials(ctx, serverName, creds); err != nil {
		// Log warning but don't fail - we still have the credentials
		fmt.Printf("Warning: failed to store credentials for %s: %v\n", serverName, err)
	} else {
		fmt.Printf("DEBUG: Successfully stored credentials for %s\n", serverName)
	}

	// Register the DCR client as an OAuth provider immediately
	// This ensures it's available in /apps for UI visibility and token retrieval
	fmt.Printf("DEBUG: Registering DCR provider for %s\n", serverName)
	client := desktop.NewAuthClient()
	if err := client.RegisterDCRProvider(ctx, serverName); err != nil {
		fmt.Printf("Warning: failed to register DCR provider for %s: %v\n", serverName, err)
		// Don't fail - the credentials are stored, provider registration can be done later
	} else {
		fmt.Printf("DEBUG: Successfully registered DCR provider for %s\n", serverName)
	}

	return creds, nil
}

// StorePKCEParameters stores PKCE parameters in Docker Desktop for OAuth callback
func (s *DockerDesktopStorage) StorePKCEParameters(ctx context.Context, flow *PKCEFlow) error {
	fmt.Printf("DEBUG: StorePKCEParameters called for state: %s\n", flow.State)
	
	if flow == nil {
		fmt.Printf("DEBUG: PKCEFlow is nil\n")
		return fmt.Errorf("PKCE flow cannot be nil")
	}

	// Create PKCE storage request
	req := desktop.StorePKCERequest{
		State:        flow.State,
		CodeVerifier: flow.CodeVerifier,
		ResourceURL:  flow.ResourceURL,
		ServerName:   flow.ServerName,
	}

	fmt.Printf("DEBUG: Calling StorePKCE with request: %+v\n", req)

	// Store with Docker Desktop PKCE API
	if err := s.client.StorePKCE(ctx, req); err != nil {
		fmt.Printf("DEBUG: StorePKCE failed: %v\n", err)
		return fmt.Errorf("failed to store PKCE parameters for state %s: %w", flow.State, err)
	}

	fmt.Printf("DEBUG: StorePKCE succeeded for state: %s\n", flow.State)
	return nil
}