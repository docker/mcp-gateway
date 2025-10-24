package gateway

import "github.com/docker/mcp-gateway/pkg/catalog"

type Config struct {
	Options
	WorkingSet         string
	ServerNames        []string
	CatalogPath        []string
	ConfigPath         []string
	RegistryPath       []string
	ToolsPath          []string
	SecretsPath        string
	SessionName        string           // Session name for persisting configuration
	MCPRegistryServers []catalog.Server // catalog.Server objects from MCP registries
}

type Options struct {
	Port                    int
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
}
