package dcr

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker-credential-helpers/credentials"
	oauth "github.com/docker/mcp-gateway-oauth-helpers"
	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/log"
)

// Manager orchestrates Dynamic Client Registration flows
type Manager struct {
	credentials *Credentials
	redirectURI string
}

// NewManager creates a new DCR manager with the specified redirect URI
func NewManager(credentialHelper credentials.Helper, redirectURI string) *Manager {
	return &Manager{
		credentials: NewCredentials(credentialHelper),
		redirectURI: redirectURI,
	}
}

// Credentials returns the credentials store
func (m *Manager) Credentials() *Credentials {
	return m.credentials
}

// GetDCRClient retrieves a DCR client from storage
func (m *Manager) GetDCRClient(serverName string) (Client, error) {
	return m.credentials.RetrieveClient(serverName)
}

// PerformDiscoveryAndRegistration executes OAuth discovery and DCR for a server
// This is called when no DCR client exists or when it needs re-registration
func (m *Manager) PerformDiscoveryAndRegistration(ctx context.Context, serverName string, scopes string) error {
	log.Logf("- Performing OAuth discovery and DCR for: %s", serverName)

	// Get server URL from catalog
	serverURL, err := getServerURL(ctx, serverName)
	if err != nil {
		return fmt.Errorf("getting server URL: %w", err)
	}

	// Perform OAuth discovery (RFC 9728, RFC 8414)
	log.Logf("- Starting OAuth discovery for: %s at: %s", serverName, serverURL)
	ctx = oauth.WithLogger(ctx, &logger{})
	discovery, err := oauth.DiscoverOAuthRequirements(ctx, serverURL)
	if err != nil {
		return fmt.Errorf("discovering OAuth requirements for %s: %w", serverName, err)
	}
	log.Logf("- Discovery successful for: %s", serverName)

	// Merge user-provided scopes with resource-required scopes
	mergedScopes := mergeScopes(discovery.Scopes, scopes)
	if len(mergedScopes) > len(discovery.Scopes) {
		discovery.Scopes = mergedScopes
		log.Logf("- Merged scopes for DCR registration: %v", mergedScopes)
	}

	// Perform Dynamic Client Registration (RFC 7591) with our redirect URI
	creds, err := oauth.PerformDCR(ctx, discovery, serverName, m.redirectURI)
	if err != nil {
		return fmt.Errorf("registering DCR client for %s: %w", serverName, err)
	}
	log.Logf("- Registration successful for: %s, clientID: %s", serverName, creds.ClientID)

	// Create and save DCR client
	dcrClient := Client{
		ServerName:            serverName,
		ProviderName:          serverName, // For DCR, provider name = server name
		ClientID:              creds.ClientID,
		ClientName:            fmt.Sprintf("MCP Gateway - %s", serverName),
		AuthorizationEndpoint: creds.AuthorizationEndpoint,
		TokenEndpoint:         creds.TokenEndpoint,
		ResourceURL:           creds.ServerURL,
		ScopesSupported:       discovery.ScopesSupported,
		RequiredScopes:        discovery.Scopes,
		RegisteredAt:          time.Now(),
	}

	if err := m.credentials.SaveClient(serverName, dcrClient); err != nil {
		return fmt.Errorf("saving DCR client for %s: %w", serverName, err)
	}

	log.Logf("- Completed DCR for: %s", serverName)
	return nil
}

// DeleteDCRClient removes a DCR client from storage
func (m *Manager) DeleteDCRClient(serverName string) error {
	return m.credentials.DeleteClient(serverName)
}

// ListDCRClients returns all stored DCR clients
func (m *Manager) ListDCRClients() (map[string]Client, error) {
	return m.credentials.ListClients()
}

// getServerURL retrieves the server URL from the catalog
func getServerURL(ctx context.Context, serverName string) (string, error) {
	cat, err := catalog.GetWithOptions(ctx, true, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get catalog: %w", err)
	}

	server, found := cat.Servers[serverName]
	if !found {
		return "", fmt.Errorf("server %s not found in catalog", serverName)
	}

	if server.Remote.URL == "" {
		return "", fmt.Errorf("server %s is not a remote server or missing URL", serverName)
	}

	return server.Remote.URL, nil
}

// mergeScopes combines resource-required scopes with user-provided scopes
func mergeScopes(requiredScopes []string, userScopes string) []string {
	if userScopes == "" || userScopes == " " {
		return requiredScopes
	}

	userScopesList := strings.Fields(userScopes)
	merged := make([]string, len(requiredScopes))
	copy(merged, requiredScopes)

	// Add user scopes if not already present
	for _, userScope := range userScopesList {
		found := false
		for _, existingScope := range merged {
			if existingScope == userScope {
				found = true
				break
			}
		}
		if !found {
			merged = append(merged, userScope)
		}
	}

	return merged
}

// logger adapter for oauth-helpers library
type logger struct{}

func (l *logger) Infof(format string, args ...interface{}) {
	log.Logf(format, args...)
}

func (l *logger) Warnf(format string, args ...interface{}) {
	log.Logf("! "+format, args...)
}

func (l *logger) Debugf(format string, args ...interface{}) {
	log.Logf(format, args...)
}
