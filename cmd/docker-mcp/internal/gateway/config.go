package gateway

type Config struct {
	Options
	ServerNames  []string
	CatalogPath  []string
	ConfigPath   []string
	RegistryPath []string
	ToolsPath    []string
	SecretsPath  string
}

type Options struct {
	Port                    int
	Transport               string
	ToolNames               []string
	Interceptors            []string
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
	Central                 bool
	OAuthInterceptorEnabled bool
	McpOAuthDcrEnabled      bool
}
