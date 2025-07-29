package commands

import (
	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/tools"
)

func toolsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "List/count/call MCP tools",
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
		Use:     "list",
		Aliases: []string{"ls"},
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

	return cmd
}
