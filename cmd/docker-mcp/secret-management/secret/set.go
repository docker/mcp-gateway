package secret

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/docker/cli/cli/command"
	"github.com/docker/mcp-gateway/pkg/desktop"
	dockerenv "github.com/docker/mcp-gateway/pkg/docker"
	"github.com/docker/mcp-gateway/pkg/tui"
)

const (
	Credstore = "credstore"
)

type SetOpts struct {
	Provider string
}

func MappingFromSTDIN(ctx context.Context, key string) (*Secret, error) {
	// Read the entire secret value from STDIN.
	// This allows piping values securely without exposing them
	// via command-line arguments or shell history.
	data, err := tui.ReadAllWithContext(ctx, os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret value from STDIN: %w", err)
	}

	return &Secret{
		key: key,
		val: string(data),
	}, nil
}

// Secret represents a key/value pair used by the secret management commands.
// The fields are intentionally unexported to avoid accidental exposure
// outside of the secret management package.
type Secret struct {
	key string
	val string
}

func ParseArg(arg string, opts SetOpts) (*Secret, error) {
	// Direct-value providers expect secrets in the form key=value.
	// Non-direct providers only accept the key, with the value
	// being resolved by the provider itself.
	directProvider := isDirectValueProvider(opts.Provider)

	// Reject key=value syntax when the provider does not accept direct values.
	if !directProvider && strings.Contains(arg, "=") {
		return nil, fmt.Errorf(
			"provider %q does not support key=value syntax: %s",
			opts.Provider, arg,
		)
	}

	// For non-direct providers, only the key is required.
	if !directProvider {
		return &Secret{key: arg, val: ""}, nil
	}

	// Split key=value input for direct providers.
	parts := strings.SplitN(arg, "=", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("expected key=value pair, got: %s", arg)
	}

	return &Secret{
		key: parts[0],
		val: parts[1],
	}, nil
}

func isDirectValueProvider(provider string) bool {
	// Direct-value providers receive the secret value directly
	// from the CLI input (key=value).
	// Currently supported direct providers:
	//   - empty provider (default)
	//   - credstore
	return provider == "" || provider == Credstore
}

func Set(ctx context.Context, s Secret, opts SetOpts) error {
	// Handle the credstore provider first.
	// This provider does not depend on Docker Desktop or JFS.
	if opts.Provider == Credstore {
		p := NewCredStoreProvider()
		if err := p.SetSecret(s.key, s.val); err != nil {
			return err
		}
	}

	// Initialize Docker CLI to detect the runtime environment.
	// This is required to determine whether Docker Desktop is available.
	dockerCli, err := command.NewDockerCli()
	if err != nil {
		return fmt.Errorf("failed to create Docker CLI: %w", err)
	}

	if err := dockerCli.Initialize(nil); err != nil {
		return fmt.Errorf("failed to initialize Docker CLI: %w", err)
	}

	// Detect if we are running on Docker Engine (non-Desktop).
	// On headless Docker Engine setups, Docker Desktop services
	// (including the JFS secrets backend) are not available.
	isCE, err := dockerenv.RunningInDockerCE(ctx, dockerCli)
	if err != nil {
		return err
	}

	if isCE {
		return fmt.Errorf(
			"Docker Desktop is not available " +
				"`docker mcp secret set` requires Docker Desktop to manage secrets " +
				"If you are running Docker Engine in a headless environment, " +
				"use --secrets with a .env file instead",
		)
	}

	// Docker Desktop is available: proceed with the JFS-backed secrets client.
	return desktop.NewSecretsClient().SetJfsSecret(ctx, desktop.Secret{
		Name:     s.key,
		Value:    s.val,
		Provider: opts.Provider,
	})
}

func IsValidProvider(provider string) bool {
	// An empty provider is valid and represents the default behavior.
	if provider == "" {
		return true
	}

	// OAuth-based providers are identified by the "oauth/" prefix.
	// The concrete provider implementation is resolved at runtime.
	if strings.HasPrefix(provider, "oauth/") {
		return true
	}

	// Credstore is a built-in provider that stores secrets locally.
	if provider == Credstore {
		return true
	}

	// Any other provider value is considered invalid.
	return false
}
