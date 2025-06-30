package catalog

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/config"
)

func newRmCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a catalog",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runRm(args[0])
		},
		Hidden: true,
	}
	return cmd
}

func runRm(name string) error {
	cfg, err := ReadConfig()
	if err != nil {
		return err
	}
	if _, ok := cfg.Catalogs[name]; !ok {
		return fmt.Errorf("catalog %q not found", name)
	}
	delete(cfg.Catalogs, name)
	if err := WriteConfig(cfg); err != nil {
		return err
	}
	if err := config.RemoveCatalogFile(name); err != nil {
		return err
	}

	fmt.Printf("removed catalog %q\n", name)
	return nil
}
