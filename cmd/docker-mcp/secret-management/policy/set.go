package policy

import (
	"context"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/desktop"
	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/tui"
)

const setExample = `
# Backup the current policy to a file
docker mcp policy dump > policy.conf

# Set a new policy
docker mcp policy set "my-secret allows postgres"

# Restore the previous policy
cat policy.conf | docker mcp policy set
`

func SetCommand() *cobra.Command {
	cmd := &cobra.Command{
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
			return runSet(cmd.Context(), args[0])
		},
		Example: strings.Trim(setExample, "\n"),
	}
	return cmd
}

func runSet(ctx context.Context, data string) error {
	return desktop.NewSecretsClient().SetJfsPolicy(ctx, data)
}
