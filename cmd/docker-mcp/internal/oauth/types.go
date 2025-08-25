package oauth

import "time"

// OAuthDiscovery contains OAuth configuration discovered from MCP server
type OAuthDiscovery struct {
	// OAuth requirements detected
	RequiresOAuth bool

	// OAuth Protected Resource Metadata (RFC 9728)
	ResourceURL         string
	ResourceServer      string
	AuthorizationServer string
	Scopes              []string

	// Authorization Server Metadata (RFC 8414)
	AuthorizationEndpoint string
	TokenEndpoint         string
	RegistrationEndpoint  string
	JWKSUri              string
	SupportsPKCE         bool
	CodeChallengeMethod  []string

	// Additional metadata
	Issuer                        string
	ScopesSupported               []string
	ResponseTypesSupported        []string
	ResponseModesSupported        []string
	GrantTypesSupported           []string
	TokenEndpointAuthMethodsSupported []string
}

// OAuthProtectedResourceMetadata represents metadata from /.well-known/oauth-protected-resource
// Based on RFC 9728 - OAuth 2.0 Protected Resource Metadata
type OAuthProtectedResourceMetadata struct {
	Resource             string   `json:"resource"`
	AuthorizationServer  string   `json:"authorization_server,omitempty"`  // RFC 9728 standard (single)
	AuthorizationServers []string `json:"authorization_servers,omitempty"` // Some servers use plural (array)
	Scopes               []string `json:"scopes,omitempty"`
}

// OAuthAuthorizationServerMetadata represents metadata from /.well-known/oauth-authorization-server  
// Based on RFC 8414 - OAuth 2.0 Authorization Server Metadata
type OAuthAuthorizationServerMetadata struct {
	Issuer                                string   `json:"issuer"`
	AuthorizationEndpoint                 string   `json:"authorization_endpoint"`
	TokenEndpoint                         string   `json:"token_endpoint"`
	JWKSUri                              string   `json:"jwks_uri,omitempty"`
	RegistrationEndpoint                  string   `json:"registration_endpoint,omitempty"`
	ScopesSupported                      []string `json:"scopes_supported,omitempty"`
	ResponseTypesSupported               []string `json:"response_types_supported,omitempty"`
	ResponseModesSupported               []string `json:"response_modes_supported,omitempty"`
	GrantTypesSupported                  []string `json:"grant_types_supported,omitempty"`
	TokenEndpointAuthMethodsSupported    []string `json:"token_endpoint_auth_methods_supported,omitempty"`
	CodeChallengeMethodsSupported        []string `json:"code_challenge_methods_supported,omitempty"`
}

// WWWAuthenticateChallenge represents a parsed WWW-Authenticate header challenge
type WWWAuthenticateChallenge struct {
	Scheme     string
	Parameters map[string]string
}

// DCRRequest represents an OAuth 2.0 Dynamic Client Registration request (RFC 7591)
type DCRRequest struct {
	RedirectURIs             []string `json:"redirect_uris"`
	TokenEndpointAuthMethod  string   `json:"token_endpoint_auth_method"`
	GrantTypes               []string `json:"grant_types,omitempty"`
	ResponseTypes            []string `json:"response_types,omitempty"`
	ClientName               string   `json:"client_name,omitempty"`
	ClientURI                string   `json:"client_uri,omitempty"`
	LogoURI                  string   `json:"logo_uri,omitempty"`
	Scope                    string   `json:"scope,omitempty"`
	Contacts                 []string `json:"contacts,omitempty"`
	TosURI                   string   `json:"tos_uri,omitempty"`
	PolicyURI                string   `json:"policy_uri,omitempty"`
	JwksURI                  string   `json:"jwks_uri,omitempty"`
	SoftwareID               string   `json:"software_id,omitempty"`
	SoftwareVersion          string   `json:"software_version,omitempty"`
}

// DCRResponse represents an OAuth 2.0 Dynamic Client Registration response (RFC 7591)
type DCRResponse struct {
	ClientID                 string    `json:"client_id"`
	ClientSecret             string    `json:"client_secret,omitempty"`
	ClientIDIssuedAt         int64     `json:"client_id_issued_at,omitempty"`
	ClientSecretExpiresAt    int64     `json:"client_secret_expires_at,omitempty"`
	RedirectURIs             []string  `json:"redirect_uris,omitempty"`
	TokenEndpointAuthMethod  string    `json:"token_endpoint_auth_method,omitempty"`
	GrantTypes               []string  `json:"grant_types,omitempty"`
	ResponseTypes            []string  `json:"response_types,omitempty"`
	ClientName               string    `json:"client_name,omitempty"`
	ClientURI                string    `json:"client_uri,omitempty"`
	LogoURI                  string    `json:"logo_uri,omitempty"`
	Scope                    string    `json:"scope,omitempty"`
	Contacts                 []string  `json:"contacts,omitempty"`
	TosURI                   string    `json:"tos_uri,omitempty"`
	PolicyURI                string    `json:"policy_uri,omitempty"`
	JwksURI                  string    `json:"jwks_uri,omitempty"`
	SoftwareID               string    `json:"software_id,omitempty"`
	SoftwareVersion          string    `json:"software_version,omitempty"`
	RegistrationAccessToken  string    `json:"registration_access_token,omitempty"`
	RegistrationClientURI    string    `json:"registration_client_uri,omitempty"`
}

// OAuthToken represents an OAuth access token with metadata
type OAuthToken struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Scope        string    `json:"scope,omitempty"`
	
	// Internal metadata
	IssuedAt   time.Time `json:"issued_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	Resource   string    `json:"resource,omitempty"`
}

// IsExpired checks if the token is expired
func (t *OAuthToken) IsExpired() bool {
	if t.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*t.ExpiresAt)
}

// WillExpireSoon checks if the token will expire within the given duration
func (t *OAuthToken) WillExpireSoon(within time.Duration) bool {
	if t.ExpiresAt == nil {
		return false
	}
	return time.Now().Add(within).After(*t.ExpiresAt)
}