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

// SetOAuthToken stores an OAuth token via docker pass at docker/mcp/oauth/{serverName}.
// The value should be base64-encoded JSON of the full oauth2.Token.
func SetOAuthToken(ctx context.Context, serverName string, value string) error {
	c := cmd(ctx, "set", GetOAuthKey(serverName))
	c.Stdin = strings.NewReader(value)
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not store OAuth token for %s: %s\n%s", serverName, bytes.TrimSpace(out), err)
	}
	return nil
}

// DeleteOAuthToken removes an OAuth token from docker pass.
func DeleteOAuthToken(ctx context.Context, serverName string) error {
	out, err := cmd(ctx, "rm", GetOAuthKey(serverName)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not delete OAuth token for %s: %s\n%s", serverName, bytes.TrimSpace(out), err)
	}
	return nil
}

// SetDCRClient stores a DCR client config via docker pass at docker/mcp/oauth-dcr/{serverName}.
// The value should be base64-encoded JSON of the DCR client.
func SetDCRClient(ctx context.Context, serverName string, value string) error {
	c := cmd(ctx, "set", GetDCRKey(serverName))
	c.Stdin = strings.NewReader(value)
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not store DCR client for %s: %s\n%s", serverName, bytes.TrimSpace(out), err)
	}
	return nil
}

// DeleteDCRClient removes a DCR client config from docker pass.
func DeleteDCRClient(ctx context.Context, serverName string) error {
	out, err := cmd(ctx, "rm", GetDCRKey(serverName)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not delete DCR client for %s: %s\n%s", serverName, bytes.TrimSpace(out), err)
	}
	return nil
}
