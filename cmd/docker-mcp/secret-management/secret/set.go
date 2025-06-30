package secret

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/tui"
)

const setExample = `
# Using secrets for postgres password with default policy:
docker mcp secret set POSTGRES_PASSWORD=my-secret-password
docker run -d -l x-secret:POSTGRES_PASSWORD=/pwd.txt -e POSTGRES_PASSWORD_FILE=/pwd.txt -p 5432 postgres

# Or pass the secret via STDIN:
echo my-secret-password > pwd.txt
cat pwd.txt | docker mcp secret set POSTGRES_PASSWORD
`

const (
	Credstore = "credstore"
)

type setOpts struct {
	Provider string
}

func SetCommand() *cobra.Command {
	opts := &setOpts{}
	cmd := &cobra.Command{
		Use:     "set key[=value]",
		Short:   "Set a secret in Docker Desktop's secret store",
		Example: strings.Trim(setExample, "\n"),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !isValidProvider(opts.Provider) {
				return fmt.Errorf("invalid provider: %s", opts.Provider)
			}
			var s secret
			if isNotImplicitReadFromStdinSyntax(args, *opts) {
				va, err := parseArg(args[0], *opts)
				if err != nil {
					return err
				}
				s = *va
			} else {
				val, err := secretMappingFromSTDIN(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				s = *val
			}
			return runSet(cmd.Context(), s, *opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.Provider, "provider", "", "Supported: credstore, oauth/<provider>")
	return cmd
}

func isNotImplicitReadFromStdinSyntax(args []string, opts setOpts) bool {
	return strings.Contains(args[0], "=") || len(args) > 1 || opts.Provider != ""
}

func secretMappingFromSTDIN(ctx context.Context, key string) (*secret, error) {
	data, err := tui.ReadAllWithContext(ctx, os.Stdin)
	if err != nil {
		return nil, err
	}

	return &secret{
		key: key,
		val: string(data),
	}, nil
}

type secret struct {
	key string
	val string
}

func parseArg(arg string, opts setOpts) (*secret, error) {
	if !isDirectValueProvider(opts.Provider) && strings.Contains(arg, "=") {
		return nil, fmt.Errorf("provider cannot be used with key=value pairs: %s", arg)
	}
	if !isDirectValueProvider(opts.Provider) {
		return &secret{key: arg, val: ""}, nil
	}
	parts := strings.Split(arg, "=")
	if len(parts) != 2 {
		return nil, fmt.Errorf("no key=value pair: %s", arg)
	}
	return &secret{key: parts[0], val: parts[1]}, nil
}

func isDirectValueProvider(provider string) bool {
	return provider == "" || provider == Credstore
}

func runSet(ctx context.Context, s secret, opts setOpts) error {
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

func isValidProvider(provider string) bool {
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
