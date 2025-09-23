package oauth

// Discovery contains OAuth configuration discovered from MCP server
//
// MCP SPEC COMPLIANCE:
// This struct aggregates discovery results from multiple OAuth/MCP specifications:
// - RFC 9728: OAuth 2.0 Protected Resource Metadata
// - RFC 8414: OAuth 2.0 Authorization Server Metadata
// - MCP Authorization Specification Section 4.1: Authorization Server Discovery
type Discovery struct {
	// From RFC 9728 - OAuth Protected Resource Metadata
	ResourceURL         string   // The protected resource URL
	ResourceServer      string   // Resource server identifier
	AuthorizationServer string   // Authorization server URL
	Scopes              []string // Required scopes for this resource

	// From RFC 8414 - Authorization Server Metadata
	AuthorizationEndpoint string   // OAuth authorization endpoint
	TokenEndpoint         string   // OAuth token endpoint
	RegistrationEndpoint  string   // Dynamic Client Registration endpoint (RFC 7591)
	JWKSUri               string   // JSON Web Key Set URI
	SupportsPKCE          bool     // Whether server supports PKCE (S256)
	CodeChallengeMethod   []string // Supported PKCE methods

	// Additional OAuth metadata
	Issuer                            string   // Authorization server issuer identifier
	ScopesSupported                   []string // All scopes supported by authorization server
	ResponseTypesSupported            []string // Supported OAuth response types
	ResponseModesSupported            []string // Supported OAuth response modes
	GrantTypesSupported               []string // Supported OAuth grant types
	TokenEndpointAuthMethodsSupported []string // Supported client authentication methods
}

// ProtectedResourceMetadata represents metadata from /.well-known/oauth-protected-resource
//
// RFC 9728 COMPLIANCE - OAuth 2.0 Protected Resource Metadata:
// - Section 3: Defines the Protected Resource Metadata structure
// - Section 3.2: Specifies required and optional fields
// - Section 5.1: Specifies WWW-Authenticate response inclusion
//
// COMPATIBILITY NOTE: Handles both authorization_server (singular) and authorization_servers (plural)
// formats since different servers implement RFC 9728 differently
type ProtectedResourceMetadata struct {
	Resource             string   `json:"resource"`                        // REQUIRED: Protected resource identifier
	AuthorizationServer  string   `json:"authorization_server,omitempty"`  // RFC 9728 standard (single server)
	AuthorizationServers []string `json:"authorization_servers,omitempty"` // Some servers use plural (array)
	Scopes               []string `json:"scopes,omitempty"`                // OPTIONAL: Required scopes
}

// AuthorizationServerMetadata represents metadata from /.well-known/oauth-authorization-server
//
// RFC 8414 COMPLIANCE - OAuth 2.0 Authorization Server Metadata:
// - Section 3: Defines Authorization Server Metadata structure
// - Section 3.2: Specifies REQUIRED fields (issuer, authorization_endpoint, token_endpoint)
// - Section 3.2: Validates issuer URL matches authorization server URL
//
// MCP SPEC REQUIREMENTS:
// - MCP clients MUST use this metadata per Section 4.2
// - Dynamic Client Registration endpoint support for Phase 2
type AuthorizationServerMetadata struct {
	Issuer                            string   `json:"issuer"`                                          // REQUIRED: Issuer identifier
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`                          // REQUIRED: Authorization endpoint
	TokenEndpoint                     string   `json:"token_endpoint"`                                  // REQUIRED: Token endpoint
	JWKSUri                           string   `json:"jwks_uri,omitempty"`                              // OPTIONAL: JSON Web Key Set
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`                 // OPTIONAL: DCR endpoint (RFC 7591)
	ScopesSupported                   []string `json:"scopes_supported,omitempty"`                      // OPTIONAL: Supported scopes
	ResponseTypesSupported            []string `json:"response_types_supported,omitempty"`              // OPTIONAL: Response types
	ResponseModesSupported            []string `json:"response_modes_supported,omitempty"`              // OPTIONAL: Response modes
	GrantTypesSupported               []string `json:"grant_types_supported,omitempty"`                 // OPTIONAL: Grant types
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"` // OPTIONAL: Auth methods
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported,omitempty"`      // OPTIONAL: PKCE methods
}

// DCRRequest represents a Dynamic Client Registration request
//
// RFC 7591 COMPLIANCE - OAuth 2.0 Dynamic Client Registration Protocol:
// - Section 2: Defines Client Registration Request structure
// - Section 3: Specifies Client Information fields
// - Uses JSON format as specified in Section 2
//
// PUBLIC CLIENT IMPLEMENTATION:
// - Uses token_endpoint_auth_method="none" for public clients
// - Includes redirect_uris pointing to mcp-oauth proxy
// - Requests authorization_code and refresh_token grant types
type DCRRequest struct {
	ClientName              string   `json:"client_name"`                // Human-readable client name
	RedirectURIs            []string `json:"redirect_uris"`              // Callback URLs (mcp-oauth proxy)
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"` // "none" for public clients
	GrantTypes              []string `json:"grant_types"`                // OAuth grant types requested
	ResponseTypes           []string `json:"response_types"`             // OAuth response types requested
	Scope                   string   `json:"scope,omitempty"`            // Space-separated scopes

	// Additional metadata for better client identification
	ClientURI       string   `json:"client_uri,omitempty"`       // Client information URL
	SoftwareID      string   `json:"software_id,omitempty"`      // Software identifier
	SoftwareVersion string   `json:"software_version,omitempty"` // Software version
	Contacts        []string `json:"contacts,omitempty"`         // Contact information
}

// DCRResponse represents a Dynamic Client Registration response
//
// RFC 7591 COMPLIANCE:
// - Section 3.2.1: Defines Client Registration Response structure
// - client_id is REQUIRED in successful responses
// - client_secret is omitted for public clients (token_endpoint_auth_method="none")
type DCRResponse struct {
	ClientID                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret,omitempty"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at,omitempty"`
	ClientSecretExpiresAt   int64    `json:"client_secret_expires_at,omitempty"`
	RedirectURIs            []string `json:"redirect_uris,omitempty"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
	GrantTypes              []string `json:"grant_types,omitempty"`
	ResponseTypes           []string `json:"response_types,omitempty"`
	ClientName              string   `json:"client_name,omitempty"`
	ClientURI               string   `json:"client_uri,omitempty"`
	LogoURI                 string   `json:"logo_uri,omitempty"`
	Scope                   string   `json:"scope,omitempty"`
	Contacts                []string `json:"contacts,omitempty"`
	TosURI                  string   `json:"tos_uri,omitempty"`
	PolicyURI               string   `json:"policy_uri,omitempty"`
	JwksURI                 string   `json:"jwks_uri,omitempty"`
	SoftwareID              string   `json:"software_id,omitempty"`
	SoftwareVersion         string   `json:"software_version,omitempty"`
	RegistrationAccessToken string   `json:"registration_access_token,omitempty"`
	RegistrationClientURI   string   `json:"registration_client_uri,omitempty"`
}

// WWWAuthenticateChallenge represents a parsed WWW-Authenticate challenge
//
// RFC 6750 COMPLIANCE - OAuth 2.0 Bearer Token Usage:
// - Section 3: Defines WWW-Authenticate Header Field format
// - Section 3.1: Specifies Bearer challenge format
//
// MCP SPEC SUPPORT:
// - MCP Authorization Specification Section 4.1: WWW-Authenticate header parsing
// - RFC 9728 Section 5.1: resource_metadata parameter support
type WWWAuthenticateChallenge struct {
	Scheme     string            // Authentication scheme (e.g., "Bearer")
	Parameters map[string]string // Challenge parameters (realm, scope, resource_metadata, etc.)
}
