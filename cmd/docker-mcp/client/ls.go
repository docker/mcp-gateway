package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

const (
	vendorCursor        = "cursor"
	vendorVSCode        = "vscode"
	vendorClaudeDesktop = "claude-desktop"
	vendorContinueDev   = "continue"
	vendorGordon        = "gordon"
	vendorZed           = "zed"
	vendorCodex         = "codex"
)

const (
	orangeYellowColor = "\033[38;5;208m"
	redColor          = "\033[31m"
	greenColor        = "\033[32m"
	resetColor        = "\033[0m"
)

var (
	greenCircle  = fmt.Sprintf("%s\u25CF%s", greenColor, resetColor)
	redCircle    = fmt.Sprintf("%s\u25CF%s", redColor, resetColor)
	orangeCircle = fmt.Sprintf("%s\u25CF%s", orangeYellowColor, resetColor)
)

func List(ctx context.Context, cwd string, config Config, global, outputJSON bool) error {
	var result Configs
	if global {
		result = parseGlobalConfigs(ctx, config)
	} else {
		projectRoot := findGitProjectRoot(cwd)
		if projectRoot == "" {
			return errNotInGitRepo
		}
		result = parseLocalProjectConfigs(projectRoot, config)
	}
	if outputJSON {
		jsonData, err := json.MarshalIndent(result.GetData(), "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(jsonData))
		return nil
	}
	result.HumanPrint()
	return nil
}

type Configs interface {
	HumanPrint()
	GetData() any
}

func prettifyCommand(name, cmd string) string {
	if name == "MCP_DOCKER" {
		return "Docker MCP Catalog (gateway server)"
	}
	return cmd
}

type ProjectConfigs struct {
	root string
	data map[string]ProjectMCPClientCfg
}

func (cfg ProjectConfigs) HumanPrint() {
	// Make sure to always display things with the same order.
	var vendors []string
	for vendor := range cfg.data {
		vendors = append(vendors, vendor)
	}
	sort.Strings(vendors)

	fmt.Printf("=== Project-wide MCP Configurations (%s) ===\n", cfg.root)
	for _, vendor := range vendors {
		data := cfg.data[vendor]
		if !data.IsConfigured {
			fmt.Printf(" %s %s: no mcp configured\n", redCircle, vendor)
			continue
		}
		prettyPrintBaseData(vendor, data.MCPClientCfgBase)
	}
}

func (cfg ProjectConfigs) GetData() any {
	return cfg.data
}

func parseLocalProjectConfigs(projectRoot string, config Config) ProjectConfigs {
	result := ProjectConfigs{root: projectRoot, data: make(map[string]ProjectMCPClientCfg)}
	for v, pathCfg := range config.Project {
		processor, err := NewLocalCfgProcessor(pathCfg, projectRoot)
		if err != nil {
			continue
		}
		cfg := processor.Parse()
		cfg.ConfigName = v
		result.data[v] = cfg
	}
	return result
}

type GlobalConfig map[string]MCPClientCfg

func (cfg GlobalConfig) HumanPrint() {
	// Make sure to always display things with the same order.
	var vendors []string
	for vendor := range cfg {
		vendors = append(vendors, vendor)
	}
	sort.Strings(vendors)

	fmt.Printf("=== System-wide MCP Configurations ===\n")
	for _, vendor := range vendors {
		data := cfg[vendor]
		if !data.IsInstalled || !data.IsOsSupported {
			continue
		}
		prettyPrintBaseData(vendor, data.MCPClientCfgBase)
	}
}

func prettyPrintBaseData(vendor string, data MCPClientCfgBase) {
	if data.Err != nil {
		fmt.Printf(" %s %s: %s\n", redCircle, vendor, data.Err.Err)
		return
	}
	circle := redCircle
	nrServers := 0
	if data.cfg != nil {
		nrServers = len(data.cfg.STDIOServers) + len(data.cfg.SSEServers) + len(data.cfg.HTTPServers)
	}
	if nrServers > 0 {
		circle = orangeCircle
	}
	connected := "disconnected"
	if data.IsMCPCatalogConnected {
		circle = greenCircle
		connected = "connected"
	}
	fmt.Printf(" %s %s: %s\n", circle, vendor, connected)
	if data.cfg == nil {
		return
	}
	for _, server := range data.cfg.STDIOServers {
		fmt.Printf("   %s: %s (stdio)\n", server.Name, prettifyCommand(server.Name, server.String()))
	}
	for _, server := range data.cfg.SSEServers {
		fmt.Printf("   %s: %s (sse)\n", server.Name, server.String())
	}
	for _, server := range data.cfg.HTTPServers {
		fmt.Printf("   %s: %s (http)\n", server.Name, server.String())
	}
}

func (cfg GlobalConfig) GetData() any {
	return cfg
}

func parseGlobalConfigs(ctx context.Context, config Config) GlobalConfig {
	result := make(map[string]MCPClientCfg)
	for v, pathCfg := range config.System {
		processor, err := NewGlobalCfgProcessor(pathCfg)
		if err != nil {
			continue
		}
		cfg := processor.ParseConfig()
		cfg.ConfigName = v
		result[v] = cfg
	}
	err := desktop.CheckFeatureIsEnabled(ctx, "enableDockerAI", "Docker AI")
	if err == nil {
		result[vendorGordon] = getGordonSetup(ctx)
	}
	result[vendorCodex] = getCodexSetup(ctx)
	return result
}
