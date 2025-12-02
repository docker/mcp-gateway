package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/formatting"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/secret"
	"github.com/docker/mcp-gateway/pkg/desktop"
)

const setSecretExample = `
### Use secrets for postgres password with default policy

> docker mcp secret set postgres_password=my-secret-password

Inject the secret by querying by ID
> docker run -d -e POSTGRES_PASSWORD=se://docker/mcp/generic/postgres_password -p 5432 postgres

Another way to inject secrets would be to use a pattern.
> docker run -d -e POSTGRES_PASSWORD=se://**/postgres_password -p 5432 postgres

### Pass the secret via STDIN

> echo my-secret-password > pwd.txt
> cat pwd.txt | docker mcp secret set POSTGRES_PASSWORD
`

func secretCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "secret",
		Short:   "Manage secrets in the local OS Keychain",
		Example: strings.Trim(setSecretExample, "\n"),
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			err := desktop.CheckHasDockerPass(cmd.Context())
			if err != nil {
				return err
			}
			return nil
		},
	}
	cmd.AddCommand(rmSecretCommand())
	cmd.AddCommand(listSecretCommand())
	cmd.AddCommand(setSecretCommand())
	return cmd
}

func rmSecretCommand() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "rm name1 name2 ...",
		Short: "Remove secrets from the local OS Keychain",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateRmArgs(args, all); err != nil {
				return err
			}

			ids := slices.Clone(args)
			if all {
				var err error
				ids, err = secret.List(cmd.Context())
				if err != nil {
					return err
				}
			}

			var errs []error
			for _, s := range ids {
				errs = append(errs, secret.DeleteSecret(cmd.Context(), s))
			}
			return errors.Join(errs...)
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&all, "all", false, "Remove all secrets")
	return cmd
}

func validateRmArgs(args []string, all bool) error {
	if len(args) == 0 && !all {
		return errors.New("either provide a secret name or use --all to remove all secrets")
	}
	return nil
}

func listSecretCommand() *cobra.Command {
	var outJSON bool
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all secrets from the local OS Keychain as well as any active Secrets Engine provider",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// query the Secrets Engine instead to get all the secrets from
			// all active providers.
			l, err := secret.GetSecrets(cmd.Context())
			if err != nil {
				return err
			}
			if outJSON {
				type secretListItem struct {
					Name     string `json:"name"`
					Provider string `json:"provider,omitempty"`
				}
				output := make([]secretListItem, 0, len(l))
				for _, env := range l {
					output = append(output, secretListItem{
						Name:     secret.StripNamespace(env.ID),
						Provider: env.Provider,
					})
				}
				if len(output) == 0 {
					output = []secretListItem{} // Guarantee empty list (instead of displaying null)
				}
				jsonData, err := json.MarshalIndent(output, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(jsonData))
				return nil
			}
			var rows [][]string
			for _, v := range l {
				rows = append(rows, []string{v.ID, v.Provider})
			}
			formatting.PrettyPrintTable(rows, []int{40, 120})
			return nil
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&outJSON, "json", false, "Print as JSON.")
	return cmd
}

func setSecretCommand() *cobra.Command {
	opts := &secret.SetOpts{}
	cmd := &cobra.Command{
		Use:     "set key[=value]",
		Short:   "Set a secret in the local OS Keychain",
		Example: strings.Trim(setSecretExample, "\n"),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
	_ = flags.MarkDeprecated("provider", "all secrets now stored via docker pass in OS Keychain")
	return cmd
}

func isNotImplicitReadFromStdinSyntax(args []string, _ secret.SetOpts) bool {
	return strings.Contains(args[0], "=") || len(args) > 1
}
