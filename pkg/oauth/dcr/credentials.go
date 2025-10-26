package dcr

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker-credential-helpers/credentials"

	"github.com/docker/mcp-gateway/pkg/log"
)

const (
	dcrKeySuffix = ".mcp-dcr"
	dcrKeyPrefix = "https://"
	dcrUsername  = "dcr_client"
)

// Client represents a dynamically registered OAuth client
// Simplified from Pinata - removed State field (not needed for CE mode)
type Client struct {
	AuthorizationEndpoint string    `json:"authorizationEndpoint,omitempty"`
	AuthorizationServer   string    `json:"authorizationServer,omitempty"`
	ClientID              string    `json:"clientId,omitempty"`
	ClientName            string    `json:"clientName,omitempty"`
	ProviderName          string    `json:"providerName"`
	RegisteredAt          time.Time `json:"registeredAt"`
	RequiredScopes        []string  `json:"requiredScopes,omitempty"`
	ResourceURL           string    `json:"resourceUrl,omitempty"`
	ScopesSupported       []string  `json:"scopesSupported,omitempty"`
	ServerName            string    `json:"serverName"`
	TokenEndpoint         string    `json:"tokenEndpoint,omitempty"`
}

// Credentials provides storage for DCR client metadata via credential helper
type Credentials struct {
	credentialHelper credentials.Helper
}

// NewCredentials creates a new DCR credentials store
func NewCredentials(credentialHelper credentials.Helper) *Credentials {
	return &Credentials{
		credentialHelper: credentialHelper,
	}
}

// getCredentialKey returns the credential helper key for a server
// Format: https://{serverName}.mcp-dcr
func getCredentialKey(serverName string) string {
	return dcrKeyPrefix + serverName + dcrKeySuffix
}

// SaveClient stores a DCR client in the credential helper
func (c *Credentials) SaveClient(serverName string, client Client) error {
	jsonData, err := json.Marshal(client)
	if err != nil {
		return fmt.Errorf("marshalling DCR client for %s: %w", serverName, err)
	}

	encodedData := base64.StdEncoding.EncodeToString(jsonData)

	cred := &credentials.Credentials{
		ServerURL: getCredentialKey(serverName),
		Username:  dcrUsername,
		Secret:    encodedData,
	}

	if err := c.credentialHelper.Add(cred); err != nil {
		return fmt.Errorf("storing DCR client for %s: %w", serverName, err)
	}

	log.Logf("- Stored DCR client for %s", serverName)
	return nil
}

// RetrieveClient retrieves a DCR client from the credential helper
func (c *Credentials) RetrieveClient(serverName string) (Client, error) {
	_, encodedData, err := c.credentialHelper.Get(getCredentialKey(serverName))
	if err != nil {
		if credentials.IsErrCredentialsNotFound(err) {
			return Client{}, fmt.Errorf("DCR client not found for %s", serverName)
		}
		return Client{}, fmt.Errorf("retrieving DCR client for %s: %w", serverName, err)
	}

	jsonData, err := base64.StdEncoding.DecodeString(encodedData)
	if err != nil {
		return Client{}, fmt.Errorf("decoding DCR client data for %s: %w", serverName, err)
	}

	var client Client
	if err := json.Unmarshal(jsonData, &client); err != nil {
		return Client{}, fmt.Errorf("unmarshalling DCR client for %s: %w", serverName, err)
	}

	return client, nil
}

// DeleteClient removes a DCR client from the credential helper
func (c *Credentials) DeleteClient(serverName string) error {
	if err := c.credentialHelper.Delete(getCredentialKey(serverName)); err != nil {
		return fmt.Errorf("deleting DCR client for %s: %w", serverName, err)
	}
	log.Logf("- Deleted DCR client for %s", serverName)
	return nil
}

// ListClients returns all stored DCR clients
func (c *Credentials) ListClients() (map[string]Client, error) {
	serverURLToUsername, err := c.credentialHelper.List()
	if err != nil {
		return nil, fmt.Errorf("listing credentials: %w", err)
	}

	clients := make(map[string]Client)

	// Filter for DCR credentials and retrieve them
	for serverURL := range serverURLToUsername {
		// Check if this is a DCR credential (format: https://<server-name>.mcp-dcr)
		if strings.HasSuffix(serverURL, dcrKeySuffix) {
			// Extract server name: "https://notion-remote.mcp-dcr" -> "notion-remote"
			serverName := strings.TrimPrefix(serverURL, dcrKeyPrefix)
			serverName = strings.TrimSuffix(serverName, dcrKeySuffix)

			client, err := c.RetrieveClient(serverName)
			if err != nil {
				log.Logf("! Failed to retrieve DCR client %s during list: %v", serverName, err)
				continue
			}

			clients[serverName] = client
		}
	}

	return clients, nil
}
