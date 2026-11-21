package gateway

import (
	"time"

	"github.com/docker/mcp-gateway/pkg/catalog"
)

// DefaultSessionTimeout is how long a streaming client session may sit idle
// before the gateway closes it and releases the upstream connections it kept
// open. Clients are expected to send a request (a ping is enough) to keep a
// session alive.
const DefaultSessionTimeout = 30 * time.Minute

type Config struct {
	Options
	WorkingSet         string
	ServerNames        []string
	CatalogPath        []string
	ConfigPath         []string
	RegistryPath       []string
	ToolsPath          []string
	SecretsPath        string
	MCPRegistryServers []catalog.Server // catalog.Server objects from MCP registries
}

type Options struct {
	Port                    int
	Host                    string
	Transport               string
	ToolNames               []string
	Interceptors            []string
	OciRef                  []string
	Verbose                 bool
	LongLived               bool
	DebugDNS                bool
	LogCalls                bool
	BlockSecrets            bool
	BlockNetwork            bool
	VerifySignatures        bool
	DryRun                  bool
	Watch                   bool
	Cpus                    int
	Memory                  string
	Static                  bool
	OAuthInterceptorEnabled bool
	McpOAuthDcrEnabled      bool
	DynamicTools            bool
	ToolNamePrefix          bool
	LogFilePath             string
	UseEmbeddings           bool
	UseProfiles             bool
	AllowUnauthenticated    bool
	// SessionTimeout is how long a streaming (streamable HTTP) client session
	// may sit idle before it is closed. Closing the session releases the
	// remote MCP connections cached for it (see clientPool). Zero disables
	// the timeout and lets idle sessions live until the gateway stops.
	SessionTimeout time.Duration
	// PreserveToolSchemaDialect relays downstream tool schemas with the JSON
	// Schema dialect they were declared in, instead of translating pre-2020-12
	// dialects that 2020-12-only clients reject.
	PreserveToolSchemaDialect bool
}
