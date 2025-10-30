package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/secret"
	"github.com/docker/mcp-gateway/pkg/docker"
)

const setSecretExample = `
### Use secrets for postgres password with default policy

> docker mcp secret set POSTGRES_PASSWORD=my-secret-password
> docker run -d -l x-secret:POSTGRES_PASSWORD=/pwd.txt -e POSTGRES_PASSWORD_FILE=/pwd.txt -p 5432 postgres

### Pass the secret via STDIN

> echo my-secret-password > pwd.txt
> cat pwd.txt | docker mcp secret set POSTGRES_PASSWORD
`

func secretCommand(docker docker.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "secret",
		Short:   "Manage secrets",
		Example: strings.Trim(setSecretExample, "\n"),
	}
	cmd.AddCommand(rmSecretCommand())
	cmd.AddCommand(listSecretCommand())
	cmd.AddCommand(setSecretCommand())
	cmd.AddCommand(exportSecretCommand(docker))
	return cmd
}

func rmSecretCommand() *cobra.Command {
	var opts secret.RmOpts
	cmd := &cobra.Command{
		Use:   "rm name1 name2 ...",
		Short: "Remove secrets from Docker Desktop's secret store",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateRmArgs(args, opts); err != nil {
				return err
			}
			return secret.Remove(cmd.Context(), args, opts)
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&opts.All, "all", false, "Remove all secrets")
	return cmd
}

func validateRmArgs(args []string, opts secret.RmOpts) error {
	if len(args) == 0 && !opts.All {
		return errors.New("either provide a secret name or use --all to remove all secrets")
	}
	return nil
}

func listSecretCommand() *cobra.Command {
	var opts secret.ListOptions
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all secret names in Docker Desktop's secret store",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return secret.List(cmd.Context(), opts)
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&opts.JSON, "json", false, "Print as JSON.")
	return cmd
}

func setSecretCommand() *cobra.Command {
	opts := &secret.SetOpts{}
	cmd := &cobra.Command{
		Use:     "set key[=value]",
		Short:   "Set a secret in Docker Desktop's secret store",
		Example: strings.Trim(setSecretExample, "\n"),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !secret.IsValidProvider(opts.Provider) {
				return fmt.Errorf("invalid provider: %s", opts.Provider)
			}
			var s secret.Secret
			if isNotImplicitReadFromStdinSyntax(args, *opts) {
				va, err := secret.ParseArg(args[0], *opts)
				if err != nil {
					return err
				}
				s = *va
			} else {
				val, err := secret.MappingFromSTDIN(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				s = *val
			}
			return secret.Set(cmd.Context(), s, *opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.Provider, "provider", "", "Supported: credstore, oauth/<provider>")
	return cmd
}

func isNotImplicitReadFromStdinSyntax(args []string, _ secret.SetOpts) bool {
	return strings.Contains(args[0], "=") || len(args) > 1
}

func exportSecretCommand(docker docker.Client) *cobra.Command {
	return &cobra.Command{
		Use:    "export [server1] [server2] ...",
		Short:  "Export secrets for the specified servers",
		Hidden: true,
		Args:   cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			secrets, err := secret.Export(cmd.Context(), docker, args)
			if err != nil {
				return err
			}

			for name, secret := range secrets {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", name, secret)
			}

			return nil
		},
	}
}
