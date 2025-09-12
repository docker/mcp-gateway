package secret

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/provider"
)

type RmOpts struct {
	All bool
}

func Remove(ctx context.Context, names []string, opts RmOpts) error {
	secretProvider := provider.GetDefaultProvider()
	
	if opts.All && len(names) == 0 {
		l, err := secretProvider.ListSecrets(ctx)
		if err != nil {
			return err
		}
		for _, secret := range l {
			names = append(names, secret.Name)
		}
	}
	var errs []error
	for _, name := range names {
		if err := secretProvider.DeleteSecret(ctx, name); err != nil {
			errs = append(errs, err)
			fmt.Printf("failed removing secret %s\n", name)
			continue
		}
		fmt.Printf("removed secret %s\n", name)
	}
	return errors.Join(errs...)
}
