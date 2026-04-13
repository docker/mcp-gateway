package commands

import (
	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/oauth"
)

func oauthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "oauth",
		Short: "Manage OAuth app authorizations",
	}
	cmd.AddCommand(lsOauthCommand())
	cmd.AddCommand(authorizeOauthCommand())
	cmd.AddCommand(revokeOauthCommand())
	return cmd
}

func lsOauthCommand() *cobra.Command {
	var opts struct {
		JSON bool
	}
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List available OAuth apps.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return oauth.Ls(cmd.Context(), opts.JSON)
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&opts.JSON, "json", false, "Print as JSON.")
	return cmd
}

func authorizeOauthCommand() *cobra.Command {
	var opts struct {
		Scopes      string
		OpenBrowser bool
	}
	cmd := &cobra.Command{
		Use:   "authorize <app>",
		Short: "Authorize the specified OAuth app.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return oauth.Authorize(cmd.Context(), args[0], opts.Scopes, opts.OpenBrowser)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.Scopes, "scopes", "", "OAuth scopes to request (space-separated)")
	flags.BoolVar(&opts.OpenBrowser, "open-browser", false, "Open the authorization URL in the default browser automatically")
	return cmd
}

func revokeOauthCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <app>",
		Args:  cobra.ExactArgs(1),
		Short: "Revoke the specified OAuth app.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return oauth.Revoke(cmd.Context(), args[0])
		},
	}
}
