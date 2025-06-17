package secret

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/desktop"
)

type rmOpts struct {
	All bool
}

func RmCommand() *cobra.Command {
	opts := rmOpts{}
	cmd := &cobra.Command{
		Use:   "rm name1 name2 ...",
		Short: "Remove secrets from Docker Desktop's secret store",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateArgs(args, opts); err != nil {
				return err
			}
			return runRm(cmd.Context(), args, opts)
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&opts.All, "all", false, "Remove all secrets")
	return cmd
}

func validateArgs(args []string, opts rmOpts) error {
	if len(args) == 0 && !opts.All {
		return errors.New("either provide a secret name or use --all to remove all secrets")
	}
	return nil
}

func runRm(ctx context.Context, names []string, opts rmOpts) error {
	c := desktop.NewSecretsClient()
	if opts.All && len(names) == 0 {
		l, err := c.ListJfsSecrets(ctx)
		if err != nil {
			return err
		}
		for _, secret := range l {
			names = append(names, secret.Name)
		}
	}
	var errs []error
	for _, name := range names {
		if err := c.DeleteJfsSecret(ctx, name); err != nil {
			errs = append(errs, err)
			fmt.Printf("failed removing secret %s\n", name)
			continue
		}
		fmt.Printf("removed secret %s\n", name)
	}
	return errors.Join(errs...)
}
