package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/workingset"
	"github.com/spf13/cobra"
)

func workingSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workingset",
		Short: "Manage working sets",
	}

	cmd.AddCommand(exportWorkingSetCommand())
	cmd.AddCommand(importWorkingSetCommand())
	cmd.AddCommand(showWorkingSetCommand())
	cmd.AddCommand(listWorkingSetsCommand())
	cmd.AddCommand(pushWorkingSetCommand())
	cmd.AddCommand(pullWorkingSetCommand())

	return cmd
}

func listWorkingSetsCommand() *cobra.Command {
	format := string(workingset.OutputFormatHumanReadable)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List working sets",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			supported := slices.Contains(workingset.SupportedFormats(), format)
			if !supported {
				return fmt.Errorf("unsupported format: %s", format)
			}
			dao, err := db.New()
			if err != nil {
				return err
			}
			return workingset.List(cmd.Context(), dao, workingset.OutputFormat(format))
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&format, "format", string(workingset.OutputFormatHumanReadable), fmt.Sprintf("Supported: %s.", strings.Join(workingset.SupportedFormats(), ", ")))

	return cmd
}

func showWorkingSetCommand() *cobra.Command {
	format := string(workingset.OutputFormatHumanReadable)

	cmd := &cobra.Command{
		Use:   "show <working-set-id>",
		Short: "Show working set",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			supported := slices.Contains(workingset.SupportedFormats(), format)
			if !supported {
				return fmt.Errorf("unsupported format: %s", format)
			}
			dao, err := db.New()
			if err != nil {
				return err
			}
			return workingset.Show(cmd.Context(), dao, args[0], workingset.OutputFormat(format))
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&format, "format", string(workingset.OutputFormatHumanReadable), fmt.Sprintf("Supported: %s.", strings.Join(workingset.SupportedFormats(), ", ")))

	return cmd
}

func pullWorkingSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "pull <oci-reference>",
		Short: "Pull working set from OCI registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			return workingset.Pull(cmd.Context(), dao, args[0])
		},
	}
}

func pushWorkingSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "push <working-set-id> <oci-reference>",
		Short: "Push working set to OCI registry",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			return workingset.Push(cmd.Context(), dao, args[0], args[1])
		},
	}
}

func exportWorkingSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "export <working-set-id> <output-file>",
		Short: "Export working set to file",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			return workingset.Export(cmd.Context(), dao, args[0], args[1])
		},
	}
}

func importWorkingSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "import <input-file>",
		Short: "Import working set from file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dao, err := db.New()
			if err != nil {
				return err
			}
			return workingset.Import(cmd.Context(), dao, args[0])
		},
	}
}
