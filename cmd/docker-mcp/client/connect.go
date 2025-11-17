package client

import (
	"context"
	"fmt"

	"github.com/docker/cli/cli/command"

	"github.com/docker/mcp-gateway/pkg/client"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/hints"
)

func Connect(ctx context.Context, dockerCli command.Cli, cwd string, config client.Config, vendor string, global, quiet bool, workingSet string) error {
	if vendor == client.VendorCodex {
		if !global {
			return fmt.Errorf("codex only supports global configuration. Re-run with --global or -g")
		}
		if err := client.ConnectCodex(ctx); err != nil {
			return err
		}
	} else if vendor == client.VendorGordon && global {
		if err := client.ConnectGordon(ctx); err != nil {
			return err
		}
	} else {
		updater, err := client.GetUpdater(vendor, global, cwd, config)
		if err != nil {
			return err
		}
		if workingSet != "" {
			if err := updater(client.DockerMCPCatalog, client.NewMcpGatewayServerWithWorkingSet(workingSet)); err != nil {
				return err
			}
		} else {
			if err := updater(client.DockerMCPCatalog, client.NewMCPGatewayServer()); err != nil {
				return err
			}
		}
	}
	if quiet {
		return nil
	}
	if err := List(ctx, cwd, config, global, false); err != nil {
		return err
	}
	fmt.Printf("You might have to restart '%s'.\n", vendor)
	if hints.Enabled(dockerCli) {
		hints.TipCyan.Print("Tip: Your client is now connected! Use ")
		hints.TipCyanBoldItalic.Print("docker mcp tools ls")
		hints.TipCyan.Println(" to see your available tools")
		fmt.Println()
	}
	return nil
}
