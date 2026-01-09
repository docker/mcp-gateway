package secret

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/docker/mcp-gateway/pkg/desktop"
	"github.com/docker/mcp-gateway/pkg/tui"
)

const (
	Credstore = "credstore"
)

type SetOpts struct {
	Provider string
}

func MappingFromSTDIN(ctx context.Context, key string) (*Secret, error) {
	data, err := tui.ReadAllWithContext(ctx, os.Stdin)
	if err != nil {
		return nil, err
	}

	return &Secret{
		key: key,
		val: string(data),
	}, nil
}

type Secret struct {
	key string
	val string
}

func ParseArg(arg string, opts SetOpts) (*Secret, error) {
	if !isDirectValueProvider(opts.Provider) && strings.Contains(arg, "=") {
		return nil, fmt.Errorf("provider cannot be used with key=value pairs: %s", arg)
	}
	if !isDirectValueProvider(opts.Provider) {
		return &Secret{key: arg, val: ""}, nil
	}
	parts := strings.SplitN(arg, "=", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("no key=value pair: %s", arg)
	}
	return &Secret{key: parts[0], val: parts[1]}, nil
}

func isDirectValueProvider(provider string) bool {
	return provider == "" || provider == Credstore
}

func Set(ctx context.Context, s Secret, opts SetOpts) error {
	if opts.Provider == Credstore {
		p := NewCredStoreProvider()
		if err := p.SetSecret(s.key, s.val); err != nil {
			return err
		}
	}
	return desktop.NewSecretsClient().SetJfsSecret(ctx, desktop.Secret{
		Name:     s.key,
		Value:    s.val,
		Provider: opts.Provider,
	})
}

func IsValidProvider(provider string) bool {
	if provider == "" {
		return true
	}
	if strings.HasPrefix(provider, "oauth/") {
		return true
	}
	if provider == Credstore {
		return true
	}
	return false
}
