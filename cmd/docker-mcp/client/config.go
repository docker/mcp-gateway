package client

import (
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

var (
	getProjectRoot  = findGitProjectRoot
	errNotInGitRepo = errors.New("could not find root project root (use --global flag to update global configuration)")
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

func findGitProjectRoot(dir string) string {
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
		vendorGordon: {},
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

func GetUpdater(vendor string, global bool, cwd string, config Config) (Updater, error) {
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
		return nil, errNotInGitRepo
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

type MCPClientCfgBase struct {
	DisplayName           string    `json:"displayName"`
	Source                string    `json:"source"`
	Icon                  string    `json:"icon"`
	ConfigName            string    `json:"configName"`
	IsMCPCatalogConnected bool      `json:"dockerMCPCatalogConnected"`
	Err                   *CfgError `json:"error"`

	cfg *MCPJSONLists
}

func (c *MCPClientCfgBase) setParseResult(lists *MCPJSONLists, err error) {
	c.Err = classifyError(err)
	if lists != nil {
		if containsMCPDocker(lists.STDIOServers) {
			c.IsMCPCatalogConnected = true
		}
	}
	c.cfg = lists
}
