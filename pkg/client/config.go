package client

import (
	"context"
	_ "embed"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"gopkg.in/yaml.v3"
)

//go:embed config.yml
var configYaml string

const (
	vendorCursor        = "cursor"
	vendorVSCode        = "vscode"
	vendorClaudeDesktop = "claude-desktop"
	vendorContinueDev   = "continue"
	VendorGordon        = "gordon"
	vendorZed           = "zed"
	VendorCodex         = "codex"
	vendorKiro          = "kiro"
)

var (
	getProjectRoot  = FindGitProjectRoot
	ErrNotInGitRepo = errors.New("could not find root project root (use --global flag to update global configuration)")
)

type Config struct {
	System  map[string]globalCfg `yaml:"system"`
	Project map[string]localCfg  `yaml:"project"`
}

func ReadConfig() *Config {
	var result Config
	// We know it parses since it's embedded and covered by tests.
	if err := yaml.Unmarshal([]byte(configYaml), &result); err != nil {
		panic("Failed to parse config")
	}
	return &result
}

func FindGitProjectRoot(dir string) string {
	for {
		gitPath := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func GetSupportedMCPClients(cfg Config) []string {
	tmp := map[string]struct{}{
		// Gordon is not supported for profiles yet
		VendorCodex: {},
	}
	for k := range cfg.System {
		tmp[k] = struct{}{}
	}
	for k := range cfg.Project {
		tmp[k] = struct{}{}
	}
	return slices.Sorted(maps.Keys(tmp))
}

type ErrVendorNotFound struct {
	global bool
	vendor string
	config Config
}

func (e *ErrVendorNotFound) Error() string {
	var alternative string
	if e.global {
		if _, ok := e.config.Project[e.vendor]; ok {
			alternative = " Did you mean to not use the --global flag?"
		}
	} else {
		if _, ok := e.config.System[e.vendor]; ok {
			alternative = " Did you mean to use the --global flag?"
		}
	}
	return "Vendor not found: " + e.vendor + "." + alternative
}

type Updater func(key string, server *MCPServerSTDIO) error

func newMCPGatewayServer() *MCPServerSTDIO {
	var env map[string]string
	if runtime.GOOS == "windows" {
		// As of 0.9.3, Claude Desktop locks down environment variables that CLI plugins need.
		env = map[string]string{
			"LOCALAPPDATA": os.Getenv("LOCALAPPDATA"),
			"ProgramFiles": os.Getenv("ProgramFiles"),
			"ProgramData":  os.Getenv("ProgramData"),
		}
	}
	return &MCPServerSTDIO{
		Command: "docker",
		Args:    []string{"mcp", "gateway", "run"},
		Env:     env,
	}
}

func newMcpGatewayServerWithWorkingSet(workingSet string) *MCPServerSTDIO {
	server := newMCPGatewayServer()
	server.Args = append(server.Args, "--profile", workingSet)
	return server
}

func getUpdater(vendor string, global bool, cwd string, config Config) (Updater, error) {
	if global {
		cfg, ok := config.System[vendor]
		if !ok {
			return nil, &ErrVendorNotFound{vendor: vendor, global: global, config: config}
		}
		processor, err := NewGlobalCfgProcessor(cfg)
		if err != nil {
			return nil, err
		}
		return processor.Update, nil
	}
	projectRoot := getProjectRoot(cwd)
	if projectRoot == "" {
		return nil, ErrNotInGitRepo
	}
	cfg, ok := config.Project[vendor]
	if !ok {
		return nil, &ErrVendorNotFound{vendor: vendor, global: global, config: config}
	}
	processor, err := NewLocalCfgProcessor(cfg, projectRoot)
	if err != nil {
		return nil, err
	}
	return processor.Update, nil
}

func IsSupportedMCPClient(cfg Config, vendor string, global bool) bool {
	if vendor == VendorCodex {
		return global // only global codex is supported
	}
	if global && vendor == VendorGordon {
		return true // global gordon is supported
	}
	if global {
		_, ok := cfg.System[vendor]
		return ok
	}
	_, ok := cfg.Project[vendor]
	return ok
}

type MCPClientCfgBase struct {
	DisplayName           string    `json:"displayName"`
	Source                string    `json:"source"`
	Icon                  string    `json:"icon"`
	ConfigName            string    `json:"configName"`
	IsMCPCatalogConnected bool      `json:"dockerMCPCatalogConnected"`
	WorkingSet            string    `json:"profile"`
	Err                   *CfgError `json:"error"`

	Cfg *MCPJSONLists
}

func (c *MCPClientCfgBase) setParseResult(lists *MCPJSONLists, err error) {
	c.Err = classifyError(err)
	if lists != nil {
		server := containsMCPDocker(lists.STDIOServers)
		if server.Name != "" {
			c.IsMCPCatalogConnected = true
			c.WorkingSet = server.GetWorkingSet()
		}
	}
	c.Cfg = lists
}

func FindClientsByProfile(ctx context.Context, profileID string) map[string]any {
	clients := make(map[string]any)
	cfg := ReadConfig()

	for vendor, pathCfg := range cfg.System {
		processor, err := NewGlobalCfgProcessor(pathCfg)
		if err != nil {
			continue
		}
		clientCfg := processor.ParseConfig()
		if clientCfg.WorkingSet == profileID {
			clients[vendor] = clientCfg
		}
	}

	// TODO: Add support for Gordon with flags
	// gordonCfg := GetGordonSetup(ctx)
	// if gordonCfg.WorkingSet == profileID {
	// 	clients[VendorGordon] = gordonCfg
	// }

	codexCfg := GetCodexSetup(ctx)
	if codexCfg.WorkingSet == profileID {
		clients[VendorCodex] = codexCfg
	}

	return clients
}
