package oauth

import "time"

// OAuthDiscovery contains OAuth configuration discovered from MCP server
//
// MCP SPEC COMPLIANCE:  
// This struct aggregates discovery results from multiple OAuth/MCP specifications:
// - RFC 9728: OAuth 2.0 Protected Resource Metadata
// - RFC 8414: OAuth 2.0 Authorization Server Metadata
// - MCP Authorization Specification Section 4.1: Authorization Server Discovery
type OAuthDiscovery struct {
	// Discovery result
	RequiresOAuth bool

	// From RFC 9728 - OAuth Protected Resource Metadata
	ResourceURL         string   // The protected resource URL
	ResourceServer      string   // Resource server identifier
	AuthorizationServer string   // Authorization server URL
	Scopes              []string // Required scopes for this resource

	// From RFC 8414 - Authorization Server Metadata
	AuthorizationEndpoint string   // OAuth authorization endpoint
	TokenEndpoint         string   // OAuth token endpoint
	RegistrationEndpoint  string   // Dynamic Client Registration endpoint (RFC 7591)
	JWKSUri              string   // JSON Web Key Set URI
	SupportsPKCE         bool     // Whether server supports PKCE (S256)
	CodeChallengeMethod  []string // Supported PKCE methods

	// Additional OAuth metadata
	Issuer                        string   // Authorization server issuer identifier
	ScopesSupported               []string // All scopes supported by authorization server
	ResponseTypesSupported        []string // Supported OAuth response types
	ResponseModesSupported        []string // Supported OAuth response modes  
	GrantTypesSupported           []string // Supported OAuth grant types
	TokenEndpointAuthMethodsSupported []string // Supported client authentication methods
}

// OAuthProtectedResourceMetadata represents metadata from /.well-known/oauth-protected-resource
//
// RFC 9728 COMPLIANCE - OAuth 2.0 Protected Resource Metadata:
// - Section 3: Defines the Protected Resource Metadata structure
// - Section 3.2: Specifies required and optional fields
// - Section 5.1: Specifies WWW-Authenticate response inclusion
//
// COMPATIBILITY NOTE: Handles both authorization_server (singular) and authorization_servers (plural)
// formats since different servers implement RFC 9728 differently
type OAuthProtectedResourceMetadata struct {
	Resource             string   `json:"resource"`                        // REQUIRED: Protected resource identifier
	AuthorizationServer  string   `json:"authorization_server,omitempty"`  // RFC 9728 standard (single server)
	AuthorizationServers []string `json:"authorization_servers,omitempty"` // Some servers use plural (array)
	Scopes               []string `json:"scopes,omitempty"`                // OPTIONAL: Required scopes
}

// OAuthAuthorizationServerMetadata represents metadata from /.well-known/oauth-authorization-server  
//
// RFC 8414 COMPLIANCE - OAuth 2.0 Authorization Server Metadata:
// - Section 3: Defines Authorization Server Metadata structure
// - Section 3.2: Specifies REQUIRED fields (issuer, authorization_endpoint, token_endpoint)
// - Section 3.2: Validates issuer URL matches authorization server URL
//
// MCP SPEC REQUIREMENTS:
// - MCP clients MUST use this metadata per Section 4.2
// - Dynamic Client Registration endpoint support for Phase 2
type OAuthAuthorizationServerMetadata struct {
	Issuer                                string   `json:"issuer"`                                        // REQUIRED: Issuer identifier
	AuthorizationEndpoint                 string   `json:"authorization_endpoint"`                        // REQUIRED: Authorization endpoint
	TokenEndpoint                         string   `json:"token_endpoint"`                               // REQUIRED: Token endpoint
	JWKSUri                              string   `json:"jwks_uri,omitempty"`                           // OPTIONAL: JSON Web Key Set
	RegistrationEndpoint                  string   `json:"registration_endpoint,omitempty"`              // OPTIONAL: DCR endpoint (RFC 7591)
	ScopesSupported                      []string `json:"scopes_supported,omitempty"`                   // OPTIONAL: Supported scopes
	ResponseTypesSupported               []string `json:"response_types_supported,omitempty"`           // OPTIONAL: Response types
	ResponseModesSupported               []string `json:"response_modes_supported,omitempty"`           // OPTIONAL: Response modes
	GrantTypesSupported                  []string `json:"grant_types_supported,omitempty"`              // OPTIONAL: Grant types
	TokenEndpointAuthMethodsSupported    []string `json:"token_endpoint_auth_methods_supported,omitempty"` // OPTIONAL: Auth methods
	CodeChallengeMethodsSupported        []string `json:"code_challenge_methods_supported,omitempty"`   // OPTIONAL: PKCE methods
}

// WWWAuthenticateChallenge represents a parsed WWW-Authenticate header challenge
//
// RFC 7235 COMPLIANCE - HTTP Authentication:
// - Section 4.1: WWW-Authenticate header structure
// - Supports multiple authentication schemes and parameters
// - Used by MCP discovery to extract resource_metadata parameter
type WWWAuthenticateChallenge struct {
	Scheme     string            // Authentication scheme (e.g., "Bearer")
	Parameters map[string]string // Challenge parameters (realm, scope, resource_metadata, etc.)
}

// DCRRequest represents an OAuth 2.0 Dynamic Client Registration request (RFC 7591)
//
// RFC 7591 COMPLIANCE - Dynamic Client Registration:
// - Section 2: Client Registration Request structure
// - Section 3.1: Client Metadata fields
//
// MCP PHASE 2 USAGE:
// - Will be used for automatic client registration with authorization servers
// - Supports public clients (token_endpoint_auth_method = "none")
// - PKCE-enabled clients for enhanced security
type DCRRequest struct {
	RedirectURIs             []string `json:"redirect_uris"`                // REQUIRED: Client redirect URIs
	TokenEndpointAuthMethod  string   `json:"token_endpoint_auth_method"`   // REQUIRED: "none" for public clients
	GrantTypes               []string `json:"grant_types,omitempty"`        // OPTIONAL: ["authorization_code"]
	ResponseTypes            []string `json:"response_types,omitempty"`     // OPTIONAL: ["code"]
	ClientName               string   `json:"client_name,omitempty"`        // OPTIONAL: Human-readable name
	ClientURI                string   `json:"client_uri,omitempty"`         // OPTIONAL: Client information URI
	LogoURI                  string   `json:"logo_uri,omitempty"`           // OPTIONAL: Client logo URI
	Scope                    string   `json:"scope,omitempty"`              // OPTIONAL: Requested scope
	Contacts                 []string `json:"contacts,omitempty"`           // OPTIONAL: Contact information
	TosURI                   string   `json:"tos_uri,omitempty"`            // OPTIONAL: Terms of Service URI
	PolicyURI                string   `json:"policy_uri,omitempty"`         // OPTIONAL: Privacy Policy URI
	JwksURI                  string   `json:"jwks_uri,omitempty"`           // OPTIONAL: JSON Web Key Set URI
	SoftwareID               string   `json:"software_id,omitempty"`        // OPTIONAL: Software identifier
	SoftwareVersion          string   `json:"software_version,omitempty"`   // OPTIONAL: Software version
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