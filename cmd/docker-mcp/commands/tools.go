package commands

import (
	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/pkg/docker"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/tools"
)

func toolsCommand(docker docker.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "Manage tools",
	}

	var (
		version     string
		verbose     bool
		format      string
		gatewayArgs []string
	)
	cmd.PersistentFlags().StringVar(&version, "version", "2", "Version of the gateway")
	cmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Verbose output")
	cmd.PersistentFlags().StringVar(&format, "format", "list", "Output format (json|list)")
	cmd.PersistentFlags().StringSliceVar(&gatewayArgs, "gateway-arg", nil, "Additional arguments passed to the gateway")

	cmd.AddCommand(&cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List tools",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return tools.List(cmd.Context(), version, gatewayArgs, verbose, "list", "", format)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "count",
		Short: "Count tools",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return tools.List(cmd.Context(), version, gatewayArgs, verbose, "count", "", format)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "inspect",
		Short: "Inspect a tool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return tools.List(cmd.Context(), version, gatewayArgs, verbose, "inspect", args[0], format)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "call",
		Short: "Call a tool",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tools.Call(cmd.Context(), version, gatewayArgs, verbose, args)
		},
	})

	var enableServerName string
	enableCmd := &cobra.Command{
		Use:   "enable [tool1] [tool2] ...",
		Short: "enable one or more tools",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return tools.Enable(cmd.Context(), docker, args, enableServerName)
		},
	}
	enableCmd.Flags().StringVar(&enableServerName, "server", "", "Specify which server provides the tools (optional, will auto-discover if not provided)")
	cmd.AddCommand(enableCmd)

	var disableServerName string
	disableCmd := &cobra.Command{
		Use:   "disable [tool1] [tool2] ...",
		Short: "disable one or more tools",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return tools.Disable(cmd.Context(), docker, args, disableServerName)
		},
	}
	disableCmd.Flags().StringVar(&disableServerName, "server", "", "Specify which server provides the tools (optional, will auto-discover if not provided)")
	cmd.AddCommand(disableCmd)

	return cmd
}
