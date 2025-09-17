package commands

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/pkg/tui"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/policy"
)

const setPolicyExample = `
### Backup the current policy to a file
docker mcp policy dump > policy.conf

### Set a new policy
docker mcp policy set "my-secret allows postgres"

### Restore the previous policy
cat policy.conf | docker mcp policy set
`

func policyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "policy",
		Aliases: []string{"policies"},
		Short:   "Manage secret policies",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "set <content>",
		Short: "Set a policy for secret management in Docker Desktop",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				bytes, err := tui.ReadAllWithContext(cmd.Context(), os.Stdin)
				if err != nil {
					return err
				}
				args = append(args, string(bytes))
			}
			return policy.Set(cmd.Context(), args[0])
		},
		Example: strings.Trim(setPolicyExample, "\n"),
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "dump",
		Short: "Dump the policy content",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return policy.Dump(cmd.Context())
		},
	})

	return cmd
}
