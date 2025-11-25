package client

import (
	"context"
	"fmt"

	"github.com/docker/cli/cli/command"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/hints"
	"github.com/docker/mcp-gateway/pkg/client"
	"github.com/docker/mcp-gateway/pkg/db"
)

func Connect(ctx context.Context, dockerCli command.Cli, cwd string, config client.Config, vendor string, global, quiet bool, workingSet string) error {
	dao, err := db.New()
	if err != nil {
		return err
	}
	defer dao.Close()

	if err := client.Connect(ctx, dao, cwd, config, vendor, global, workingSet); err != nil {
		return err
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
