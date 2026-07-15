package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	seclient "github.com/docker/secrets-engine/client"
	"github.com/docker/secrets-engine/client/realms"
	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/formatting"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/secret"
)

const setSecretExample = `
### Pass the secret via STDIN

> echo my-secret-password > pwd.txt
> cat pwd.txt | docker mcp secret set POSTGRES_PASSWORD
`

func secretCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "secret",
		Short:   "Manage secrets in the local OS Keychain",
		Example: strings.Trim(setSecretExample, "\n"),
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

			var errs []error
			if all {
				listed, err := secret.List(cmd.Context())
				if err != nil {
					return err
				}
				for _, id := range filterAllDockerMCP(listed) {
					errs = append(errs, secret.DeleteByID(cmd.Context(), id))
				}
				return errors.Join(errs...)
			}

			for _, s := range args {
				id, err := seclient.ParseID(s)
				if err != nil {
					errs = append(errs, fmt.Errorf("invalid secret name %q: %w", s, err))
					continue
				}
				errs = append(errs, secret.DeleteDefaultSecret(cmd.Context(), id))
			}
			return errors.Join(errs...)
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&all, "all", false, "Remove all secrets")
	return cmd
}

// filterAllDockerMCP returns only the IDs within the default MCP realm (docker/mcp/**)
func filterAllDockerMCP(ids []seclient.ID) []seclient.ID {
	var out []seclient.ID
	for _, id := range ids {
		if id.Match(realms.DockerMCPDefault) {
			out = append(out, id)
		}
	}
	return out
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
						Name:     secret.StripNamespace(env.ID.String()),
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
				rows = append(rows, []string{v.ID.String(), v.Provider})
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
			if shouldReadValueFromStdin(args) {
				val, err := secret.MappingFromSTDIN(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				s = *val
			} else {
				va, err := secret.ParseArg(args[0], *opts)
				if err != nil {
					return err
				}
				s = *va
			}
			return secret.Set(cmd.Context(), s, *opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.Provider, "provider", "", "Supported: credstore, oauth/<provider>") //nolint:staticcheck // Intentionally using deprecated field for backwards compatibility
	_ = flags.MarkDeprecated("provider", "all secrets now stored via docker pass in OS Keychain")
	return cmd
}

// shouldReadValueFromStdin returns true if the user provided only the key name,
// meaning the value should be read from stdin (for piping or interactive input).
// Returns false if the user used "key=value" syntax with the value inline.
func shouldReadValueFromStdin(args []string) bool {
	return !strings.Contains(args[0], "=")
}
