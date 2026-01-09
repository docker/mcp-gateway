// Package plugins provides interfaces and registry for MCP Gateway plugins.
//
// The plugin architecture supports two provider types:
//   - In-Memory Provider: Direct Go code execution (in-process)
//   - MCP Provider: JSON-RPC over HTTP to MCP servers (local containerized or remote)
//
// This design allows the same plugin interfaces to work across Desktop and Kubernetes
// deployments, with different provider implementations for each environment.
package plugins

import (
	"context"
)

// Plugin Code Interfaces (The Stable Contract)
// These interfaces define what plugins must implement. Gateway code depends on these
// interfaces, not on specific implementations.

// AuthProvider validates credentials and returns user principals.
type AuthProvider interface {
	ValidateCredential(ctx context.Context, creds Credentials) (*UserPrincipal, error)
}

// Credentials represents authentication credentials to validate.
type Credentials struct {
	Type  string `json:"type"`  // e.g., "api_key", "oauth_token", "jwt"
	Value string `json:"value"` // The credential value
}

// UserPrincipal represents an authenticated user.
type UserPrincipal struct {
	UserID   string            `json:"user_id"`
	TenantID string            `json:"tenant_id,omitempty"`
	Roles    []string          `json:"roles,omitempty"`
	Groups   []string          `json:"groups,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// CredentialStorage stores and retrieves credentials for users.
type CredentialStorage interface {
	Store(ctx context.Context, userID, server, credType, value string) error
	Retrieve(ctx context.Context, userID, server, credType string) (string, error)
	Delete(ctx context.Context, userID, server, credType string) error
	List(ctx context.Context, userID string) ([]CredentialInfo, error)
}

// CredentialInfo contains metadata about a stored credential.
type CredentialInfo struct {
	Server     string `json:"server"`
	CredType   string `json:"cred_type"`
	CreatedAt  string `json:"created_at,omitempty"`
	ExpiresAt  string `json:"expires_at,omitempty"`
}

// AuthProxy injects credentials into outgoing requests.
type AuthProxy interface {
	InjectCredentials(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error)
}

// ProxyRequest represents a request that needs credential injection.
type ProxyRequest struct {
	UserID    string            `json:"user_id"`
	TenantID  string            `json:"tenant_id,omitempty"`
	MCPServer string            `json:"mcp_server"`
	TargetURL string            `json:"target_url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      string            `json:"body,omitempty"`
}

// ProxyResponse contains the modified request with injected credentials.
type ProxyResponse struct {
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body,omitempty"`
}

// AuditSink logs audit events.
type AuditSink interface {
	LogEvent(ctx context.Context, event *AuditEvent) error
}

// AuditEvent represents an audit log entry.
type AuditEvent struct {
	Timestamp string            `json:"timestamp"`
	EventType string            `json:"event_type"`
	TenantID  string            `json:"tenant_id,omitempty"`
	UserID    string            `json:"user_id,omitempty"`
	MCPServer string            `json:"mcp_server,omitempty"`
	Tool      string            `json:"tool,omitempty"`
	Result    string            `json:"result,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// PolicyEvaluator checks access permissions.
type PolicyEvaluator interface {
	CheckAccess(ctx context.Context, principal *UserPrincipal, mcpServer string) error
}

// MCPProvisioner provisions and manages MCP server instances.
type MCPProvisioner interface {
	Provision(ctx context.Context, server *ServerDef, userID string) (*ProvisionedServer, error)
	Deprovision(ctx context.Context, serverID string) error
	List(ctx context.Context, userID string) ([]*ProvisionedServer, error)
}

// ServerDef defines an MCP server to provision.
type ServerDef struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"`   // "image", "remote", "registry"
	Image  string            `json:"image,omitempty"`
	Source string            `json:"source,omitempty"`
	Endpoint string          `json:"endpoint,omitempty"`
	Env    map[string]string `json:"env,omitempty"`
}

// ProvisionedServer represents a running MCP server instance.
type ProvisionedServer struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Endpoint  string `json:"endpoint"`
	Status    string `json:"status"`
	UserID    string `json:"user_id"`
}

// TelemetryPlugin defines the interface for telemetry plugins.
// This interface mirrors OpenTelemetry's metric instruments, allowing
// plugins to receive telemetry data in a generic format.
type TelemetryPlugin interface {
	RecordCounter(ctx context.Context, name string, value int64, attrs map[string]string)
	RecordHistogram(ctx context.Context, name string, value float64, attrs map[string]string)
	RecordGauge(ctx context.Context, name string, value int64, attrs map[string]string)
	Close() error
}

// Plugin Provider Types
// Providers are responsible for creating plugin instances based on configuration.

// PluginProvider creates plugin instances from configuration.
type PluginProvider interface {
	// Name returns the provider name (e.g., "in-memory", "mcp").
	Name() string

	// CreateAuthProvider creates an AuthProvider from configuration.
	CreateAuthProvider(ctx context.Context, config PluginConfig) (AuthProvider, error)

	// CreateCredentialStorage creates a CredentialStorage from configuration.
	CreateCredentialStorage(ctx context.Context, config PluginConfig) (CredentialStorage, error)

	// CreateAuthProxy creates an AuthProxy from configuration.
	CreateAuthProxy(ctx context.Context, config PluginConfig) (AuthProxy, error)

	// CreateAuditSink creates an AuditSink from configuration.
	CreateAuditSink(ctx context.Context, config PluginConfig) (AuditSink, error)

	// CreatePolicyEvaluator creates a PolicyEvaluator from configuration.
	CreatePolicyEvaluator(ctx context.Context, config PluginConfig) (PolicyEvaluator, error)

	// CreateMCPProvisioner creates an MCPProvisioner from configuration.
	CreateMCPProvisioner(ctx context.Context, config PluginConfig) (MCPProvisioner, error)

	// CreateTelemetryPlugin creates a TelemetryPlugin from configuration.
	CreateTelemetryPlugin(ctx context.Context, config PluginConfig) (TelemetryPlugin, error)
}

// Configuration Types

// PluginConfig defines configuration for a plugin.
type PluginConfig struct {
	// Provider is the provider type: "in-memory" or "mcp".
	Provider string `json:"provider" yaml:"provider"`

	// Implementation is the implementation name (for in-memory provider).
	// Examples: "desktop-implicit", "stdout", "always-allow"
	Implementation string `json:"implementation,omitempty" yaml:"implementation,omitempty"`

	// Server is the MCP server reference (for mcp provider).
	// Can be a catalog reference string or inline server definition.
	// Examples:
	//   - "catalog://docker.io/docker/mcp-plugins:v1/auth-k8s-secret"
	//   - ServerConfig object for inline definition
	Server any `json:"server,omitempty" yaml:"server,omitempty"`
}

// ServerConfig defines an inline MCP server configuration.
type ServerConfig struct {
	// Type is the server type: "image", "remote", or "registry".
	Type string `json:"type" yaml:"type"`

	// Image is the container image (for type: image).
	Image string `json:"image,omitempty" yaml:"image,omitempty"`

	// Endpoint is the HTTP endpoint (for type: remote).
	Endpoint string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`

	// Source is the registry source URL (for type: registry).
	Source string `json:"source,omitempty" yaml:"source,omitempty"`

	// Port is the port the server listens on.
	Port int `json:"port,omitempty" yaml:"port,omitempty"`

	// Env are environment variables for the server.
	Env map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
}

// Config represents the plugins section of gateway configuration.
type Config struct {
	AuthProvider      *PluginConfig `json:"auth_provider,omitempty" yaml:"auth_provider,omitempty"`
	CredentialStorage *PluginConfig `json:"credential_storage,omitempty" yaml:"credential_storage,omitempty"`
	AuthProxy         *PluginConfig `json:"auth_proxy,omitempty" yaml:"auth_proxy,omitempty"`
	AuditSink         *PluginConfig `json:"audit_sink,omitempty" yaml:"audit_sink,omitempty"`
	PolicyEvaluator   *PluginConfig `json:"policy_evaluator,omitempty" yaml:"policy_evaluator,omitempty"`
	MCPProvisioner    *PluginConfig `json:"mcp_provisioner,omitempty" yaml:"mcp_provisioner,omitempty"`
	Telemetry         *PluginConfig `json:"telemetry,omitempty" yaml:"telemetry,omitempty"`
}

// ContainerManager manages plugin containers.
// Desktop uses Docker Engine API, Kubernetes uses noop (sidecars pre-started).
type ContainerManager interface {
	// EnsureRunning ensures the container is running and returns its endpoint.
	EnsureRunning(ctx context.Context, config ServerConfig) (endpoint string, err error)

	// Stop stops a container.
	Stop(ctx context.Context, endpoint string) error
}
