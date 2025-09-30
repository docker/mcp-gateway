package client

import (
	"context"
	"os/exec"
	"strings"
)

// isCodexInstalled checks if the codex binary is installed and working
func isCodexInstalled(ctx context.Context) bool {
	err := exec.CommandContext(ctx, "codex", "--version").Run()
	return err == nil
}

// getCodexSetup returns the configuration status for Codex
func getCodexSetup(ctx context.Context) MCPClientCfg {
	result := MCPClientCfg{
		MCPClientCfgBase: MCPClientCfgBase{
			DisplayName: "Codex",
			Source:      "https://openai.com/codex/",
			Icon:        "https://www.svgrepo.com/show/306500/openai.svg",
			ConfigName:  vendorCodex,
			Err:         nil,
		},
		IsInstalled:   true,
		IsOsSupported: true,
	}

	// Check if docker mcp gateway is configured in codex
	out, err := exec.CommandContext(ctx, "codex", "mcp", "list").Output()
	if err != nil {
		result.Err = classifyError(err)
		return result
	}

	// Parse the output to check if docker mcp gateway is configured
	// The output format is expected to be lines containing server configurations
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		// Look for docker mcp gateway in the output
		if strings.Contains(line, "docker mcp gateway run") || strings.Contains(line, DockerMCPCatalog) {
			result.IsMCPCatalogConnected = true
			result.cfg = &MCPJSONLists{STDIOServers: []MCPServerSTDIO{{Name: DockerMCPCatalog}}}
			break
		}
	}

	return result
}

// connectCodex configures docker mcp gateway in Codex
func connectCodex(ctx context.Context) error {
	return exec.CommandContext(ctx, "codex", "mcp", "add", DockerMCPCatalog, "--", "docker", "mcp", "gateway", "run").Run()
}

// disconnectCodex removes docker mcp gateway from Codex
func disconnectCodex(ctx context.Context) error {
	return exec.CommandContext(ctx, "codex", "mcp", "remove", DockerMCPCatalog).Run()
}
