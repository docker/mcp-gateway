package policy

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
)

func DumpCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "dump",
		Short: "Dump the policy content",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDump(cmd.Context())
		},
	}
}

func runDump(ctx context.Context) error {
	l, err := desktop.NewSecretsClient().GetJfsPolicy(ctx)
	if err != nil {
		return err
	}

	fmt.Println(l)

	return nil
}
