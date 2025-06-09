package gateway

type Config struct {
	Options
	ServerNames  []string
	CatalogPath  string
	ConfigPath   string
	RegistryPath string
	SecretsPath  string
}

type Options struct {
	Port             int
	Transport        string
	ToolNames        []string
	Verbose          bool
	KeepContainers   bool
	LogCalls         bool
	BlockSecrets     bool
	VerifySignatures bool
	DryRun           bool
	Watch            bool
}
