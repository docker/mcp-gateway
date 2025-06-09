package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/docker/cli/cli-plugins/manager"
	"github.com/docker/cli/cli-plugins/plugin"
	"github.com/docker/cli/cli/command"
	"github.com/spf13/cobra"

	"github.com/docker/mcp-cli/cmd/docker-mcp/version"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// We need to preserve CWD as paths.Init will change it.
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	if plugin.RunningStandalone() {
		os.Args = append([]string{os.Args[0], "mcp"}, os.Args[1:]...)
	}

	plugin.Run(func(dockerCli command.Cli) *cobra.Command {
		return rootCommand(ctx, cwd, dockerCli)
	},
		manager.Metadata{
			SchemaVersion:    "0.1.0",
			Vendor:           "Docker Inc.",
			Version:          version.Version,
			ShortDescription: "Docker MCP Plugin",
		},
	)
}
