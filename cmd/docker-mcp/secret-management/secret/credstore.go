// This package stores secrets in the local OS Keychain.
package secret

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path"
	"strings"
)

const (
	// Namespace prefixes for different secret types
	NamespaceDefault  = "docker/mcp/"
	NamespaceOAuth    = "docker/mcp/oauth/"
	NamespaceOAuthDCR = "docker/mcp/oauth-dcr/"
)

type CredStoreProvider struct{}

func cmd(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "docker", append([]string{"pass"}, args...)...)
}

// GetDefaultSecretKey constructs the full namespaced ID for an MCP secret
// using the default namespace (docker/mcp/).
//
// Example:
//
//	secretName = "postgres_password"
//	return "docker/mcp/postgres_password"
//
// This can later be queried by the Secrets Engine using a pattern or direct ID match.
func GetDefaultSecretKey(secretName string) string {
	return path.Join(NamespaceDefault, secretName)
}

// GetOAuthKey constructs the full namespaced ID for an OAuth token
func GetOAuthKey(provider string) string {
	return path.Join(NamespaceOAuth, provider)
}

// GetDCRKey constructs the full namespaced ID for a DCR client config
func GetDCRKey(serverName string) string {
	return path.Join(NamespaceOAuthDCR, serverName)
}

// StripNamespace removes the namespace prefix from a secret ID to get the simple name.
// OAuth and DCR namespaces must be stripped first (more specific), then default.
func StripNamespace(secretID string) string {
	name := strings.TrimPrefix(secretID, NamespaceOAuth)
	name = strings.TrimPrefix(name, NamespaceOAuthDCR)
	name = strings.TrimPrefix(name, NamespaceDefault)
	return name
}

func List(ctx context.Context) ([]string, error) {
	c := cmd(ctx, "ls")
	out, err := c.Output()
	if err != nil {
		return nil, fmt.Errorf("could not list secrets: %s\n%s", bytes.TrimSpace(out), err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	var secrets []string
	for scanner.Scan() {
		secret := scanner.Text()
		if len(secret) == 0 {
			continue
		}
		secrets = append(secrets, secret)
	}
	return secrets, nil
}

// setDefaultSecret stores a secret in the default namespace (docker/mcp/).
func setDefaultSecret(ctx context.Context, id string, value string) error {
	c := cmd(ctx, "set", GetDefaultSecretKey(id))
	c.Stdin = strings.NewReader(value)
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not store secret: %s\n%s", bytes.TrimSpace(out), err)
	}
	return nil
}

// DeleteDefaultSecret removes a secret from the default namespace (docker/mcp/).
func DeleteDefaultSecret(ctx context.Context, id string) error {
	out, err := cmd(ctx, "rm", GetDefaultSecretKey(id)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not delete secret: %s\n%s\n%s", id, bytes.TrimSpace(out), err)
	}
	return nil
}
