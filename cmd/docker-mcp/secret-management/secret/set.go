package secret

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/docker/mcp-gateway/pkg/tui"
)

const (
	// Specify to use the credential store provider.
	//
	// Deprecated: Not used.
	Credstore = "credstore"
)

type SetOpts struct {
	// Provider sets the store provider
	// Deprecated: this field will be removed in the next release
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
	if !isDirectValueProvider(opts.Provider) {
		return &Secret{key: arg, val: ""}, nil
	}
	parts := strings.Split(arg, "=")
	if len(parts) != 2 {
		return nil, fmt.Errorf("no key=value pair: %s", arg)
	}
	return &Secret{key: parts[0], val: parts[1]}, nil
}

func isDirectValueProvider(provider string) bool {
	return provider == "" || provider == Credstore
}

func Set(ctx context.Context, s Secret, opts SetOpts) error {
	return setSecret(ctx, s.key, s.val)
}
