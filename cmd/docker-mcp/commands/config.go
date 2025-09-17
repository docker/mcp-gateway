package commands

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/backup"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/pkg/config"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/pkg/docker"
)

func configCommand(docker docker.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage the configuration",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "read",
		Short: "Read the configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			content, err := config.ReadConfig(cmd.Context(), docker)
			if err != nil {
				return err
			}
			_, _ = cmd.OutOrStdout().Write(content)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "write",
		Short: "Write the configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return config.WriteConfig([]byte(args[0]))
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "reset",
		Short: "Reset the configuration",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return config.WriteConfig(nil)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:    "dump",
		Short:  "Dump the whole configuration",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out, err := backup.Dump(cmd.Context(), docker)
			if err != nil {
				return err
			}
			_, _ = cmd.OutOrStdout().Write(out)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:    "restore",
		Short:  "Restore the whole configuration",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]

			var backupData []byte
			if path == "-" {
				var err error
				backupData, err = io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading from stdin: %w", err)
				}
			} else {
				var err error
				backupData, err = os.ReadFile(path)
				if err != nil {
					return err
				}
			}

			return backup.Restore(cmd.Context(), backupData)
		},
	})

	return cmd
}
