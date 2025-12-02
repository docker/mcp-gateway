package client

import (
	"context"
	"encoding/json"
	"os/exec"
)

func GetGordonSetup(ctx context.Context) MCPClientCfg {
	result := MCPClientCfg{
		MCPClientCfgBase: MCPClientCfgBase{
			DisplayName: "Ask Gordon",
			Source:      "https://docs.docker.com/ai/gordon/",
			Icon:        "https://raw.githubusercontent.com/docker/mcp-gateway/main/img/client/gordon.png",
			ConfigName:  VendorGordon,
			Err:         nil,
		},
		IsInstalled:   true,
		IsOsSupported: true,
	}
	workingSet, err := ReadGordonProfile()
	if err != nil {
		result.Err = classifyError(err)
		return result
	}
	result.WorkingSet = workingSet
	out, err := exec.CommandContext(ctx, "docker", "ai", "config", "get").Output()
	if err != nil {
		result.Err = classifyError(err)
		return result
	}
	temp := struct {
		Features []struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		} `json:"features"`
	}{}
	if err := json.Unmarshal(out, &temp); err != nil {
		result.Err = classifyError(err)
		return result
	}
	for _, feature := range temp.Features {
		if feature.Name == "MCP Catalog" && feature.Enabled {
			result.IsMCPCatalogConnected = true
			result.Cfg = &MCPJSONLists{STDIOServers: []MCPServerSTDIO{{Name: DockerMCPCatalog}}}
			if workingSet != "" {
				// Hacky way to make it say there is a profile attached
				result.Cfg.STDIOServers[0].Args = append(result.Cfg.STDIOServers[0].Args, "--profile", workingSet)
			}
			break
		}
	}
	return result
}

func ConnectGordon(ctx context.Context, workingSet string) error {
	if workingSet != "" {
		if err := writeGordonProfile(workingSet); err != nil {
			return err
		}
	}
	return exec.CommandContext(ctx, "docker", "ai", "config", "set-feature", "MCP Catalog", "true").Run()
}

func DisconnectGordon(ctx context.Context) error {
	return exec.CommandContext(ctx, "docker", "ai", "config", "set-feature", "MCP Catalog", "false").Run()
}
