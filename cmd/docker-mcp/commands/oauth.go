package commands

import (
	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/oauth"
)

func oauthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "oauth",
		Hidden: true,
	}
	cmd.AddCommand(lsOauthCommand())
	cmd.AddCommand(authorizeOauthCommand())
	cmd.AddCommand(revokeOauthCommand())
	cmd.AddCommand(registerOauthCommand())
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
		Scopes string
	}
	cmd := &cobra.Command{
		Use:   "authorize <app>",
		Short: "Authorize the specified OAuth app.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return oauth.Authorize(cmd.Context(), args[0], opts.Scopes)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.Scopes, "scopes", "", "OAuth scopes to request (space-separated)")
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

func registerOauthCommand() *cobra.Command {
	var opts oauth.RegisterOptions
	cmd := &cobra.Command{
		Use:   "register <server-name>",
		Short: "Manually register OAuth client credentials for a server.",
		Long: `Manually register OAuth client credentials for servers that don't support Dynamic Client Registration (DCR).

This command allows you to configure pre-registered OAuth client credentials from your OAuth provider.
After registration, you can authorize with: docker mcp oauth authorize <server-name>

Examples:
  # Register with client ID and secret (confidential client)
  docker mcp oauth register my-server \
    --client-id "abc123" \
    --client-secret "secret456" \
    --auth-endpoint "https://provider.com/oauth/authorize" \
    --token-endpoint "https://provider.com/oauth/token" \
    --scopes "read,write"

  # Register public client (no secret)
  docker mcp oauth register my-server \
    --client-id "public-client-id" \
    --auth-endpoint "https://provider.com/oauth/authorize" \
    --token-endpoint "https://provider.com/oauth/token"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return oauth.Register(cmd.Context(), args[0], opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.ClientID, "client-id", "", "OAuth client ID (required)")
	flags.StringVar(&opts.ClientSecret, "client-secret", "", "OAuth client secret (optional, for confidential clients)")
	flags.StringVar(&opts.AuthorizationEndpoint, "auth-endpoint", "", "Authorization endpoint URL (required)")
	flags.StringVar(&opts.TokenEndpoint, "token-endpoint", "", "Token endpoint URL (required)")
	flags.StringVar(&opts.Scopes, "scopes", "", "Comma-separated list of OAuth scopes")
	flags.StringVar(&opts.Provider, "provider", "", "Provider name (defaults to server name)")
	flags.StringVar(&opts.ResourceURL, "resource-url", "", "Resource URL for the OAuth provider (defaults to auth endpoint base)")

	_ = cmd.MarkFlagRequired("client-id")
	_ = cmd.MarkFlagRequired("auth-endpoint")
	_ = cmd.MarkFlagRequired("token-endpoint")

	return cmd
}
