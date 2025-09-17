package secret

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/pkg/desktop"
)

type RmOpts struct {
	All bool
}

func Remove(ctx context.Context, names []string, opts RmOpts) error {
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
