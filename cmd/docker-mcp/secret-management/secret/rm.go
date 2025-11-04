package secret

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

type RmOpts struct {
	All bool
}

func Remove(ctx context.Context, names []string, opts RmOpts) error {
	secrets := slices.Clone(names)
	p := NewCredStoreProvider()
	if opts.All && len(secrets) == 0 {
		var err error
		secrets, err = p.List(ctx)
		if err != nil {
			return err
		}
	}

	var errs []error
	for _, secret := range secrets {
		if err := p.DeleteSecret(ctx, secret); err != nil {
			errs = append(errs, err)
			fmt.Printf("failed removing secret %s\n", secret)
			continue
		}
		fmt.Printf("removed secret %s\n", secret)
	}

	return errors.Join(errs...)
}
