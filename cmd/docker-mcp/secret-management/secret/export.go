package secret

import (
	"context"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/docker"
)

func ExportCommand(docker docker.Client) *cobra.Command {
	return &cobra.Command{
		Use:    "export [server1] [server2] ...",
		Short:  "Export secrets for the specified servers",
		Hidden: true,
		Args:   cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			secrets, err := exportSecrets(cmd.Context(), docker, args)
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

func exportSecrets(ctx context.Context, docker docker.Client, serverNames []string) (map[string]string, error) {
	catalog, err := catalog.Get(ctx)
	if err != nil {
		return nil, err
	}

	var secretNames []string
	for _, serverName := range serverNames {
		serverSpec, ok := catalog.Servers[serverName]
		if !ok {
			return nil, fmt.Errorf("server %s not found in catalog", serverName)
		}

		for _, s := range serverSpec.Secrets {
			secretNames = append(secretNames, s.Name)
		}
	}

	if len(secretNames) == 0 {
		return map[string]string{}, nil
	}

	sort.Strings(secretNames)

	return docker.ReadSecrets(ctx, secretNames)
}
