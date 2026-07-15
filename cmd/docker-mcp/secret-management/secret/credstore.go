// This package stores secrets in the local OS Keychain.
package secret

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/docker/secrets-engine/client"
	"github.com/docker/secrets-engine/client/realms"
)

type CredStoreProvider struct{}

func cmd(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "docker", append([]string{"pass"}, args...)...)
}

// realmPrefix returns the fixed prefix of a realm pattern by dropping its
// trailing `**` wildcard (e.g. "docker/mcp/oauth/**" => "docker/mcp/oauth/").
func realmPrefix(realm client.Pattern) string {
	return strings.TrimSuffix(realm.String(), "**")
}

// GetDefaultSecretKey returns the full ID for an MCP secret in the default realm
// (docker/mcp/**), e.g. "postgres_password" => "docker/mcp/postgres_password".
func GetDefaultSecretKey(name client.ID) (client.ID, error) {
	return realms.DockerMCPDefault.ExpandID(name)
}

// GetOAuthKey returns the full ID for an OAuth token (docker/mcp/oauth/**).
func GetOAuthKey(provider client.ID) (client.ID, error) {
	return realms.DockerMCPOAuth.ExpandID(provider)
}

// GetDCRKey returns the full ID for a DCR client config (docker/mcp/oauth-dcr/**).
func GetDCRKey(serverName client.ID) (client.ID, error) {
	return realms.DockerMCPOAuthDCR.ExpandID(serverName)
}

// StripNamespace removes the realm prefix from a secret ID to get the simple name.
// OAuth and DCR realms must be stripped first (more specific), then default.
func StripNamespace(secretID string) string {
	name := strings.TrimPrefix(secretID, realmPrefix(realms.DockerMCPOAuth))
	name = strings.TrimPrefix(name, realmPrefix(realms.DockerMCPOAuthDCR))
	name = strings.TrimPrefix(name, realmPrefix(realms.DockerMCPDefault))
	return name
}

// List returns the IDs of all secrets stored in the local OS Keychain.
// Stored keys that are not valid secret IDs are skipped.
func List(ctx context.Context) ([]client.ID, error) {
	c := cmd(ctx, "ls")
	out, err := c.Output()
	if err != nil {
		return nil, fmt.Errorf("could not list secrets: %s\n%s", bytes.TrimSpace(out), err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	var secrets []client.ID
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}
		id, err := client.ParseID(line)
		if err != nil {
			continue
		}
		secrets = append(secrets, id)
	}
	return secrets, nil
}

// setDefaultSecret stores a secret in the default realm (docker/mcp/**).
func setDefaultSecret(ctx context.Context, id client.ID, value string) error {
	key, err := GetDefaultSecretKey(id)
	if err != nil {
		return err
	}
	c := cmd(ctx, "set", key.String())
	c.Stdin = strings.NewReader(value)
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not store secret: %s\n%s", bytes.TrimSpace(out), err)
	}
	return nil
}

// DeleteDefaultSecret removes a secret from the default realm (docker/mcp/**).
func DeleteDefaultSecret(ctx context.Context, id client.ID) error {
	key, err := GetDefaultSecretKey(id)
	if err != nil {
		return err
	}
	out, err := cmd(ctx, "rm", key.String()).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not delete secret: %s\n%s\n%s", id.String(), bytes.TrimSpace(out), err)
	}
	return nil
}

// SetOAuthToken stores an OAuth token via docker pass at docker/mcp/oauth/{serverID}.
// The value should be base64-encoded JSON of the full oauth2.Token.
func SetOAuthToken(ctx context.Context, serverID client.ID, value string) error {
	keyID, err := GetOAuthKey(serverID)
	if err != nil {
		return err
	}
	key := keyID.String()

	keys, err := List(ctx)
	if err != nil {
		return fmt.Errorf("could not check existing OAuth token for %s: %w", serverID.String(), err)
	}

	// docker pass set is insert-only, so if the key already exists we need to remove it first.
	for _, k := range keys {
		if k == keyID {
			if out, err := cmd(ctx, "rm", key).CombinedOutput(); err != nil {
				return fmt.Errorf("could not remove existing OAuth token for %s: %s\n%s", serverID.String(), bytes.TrimSpace(out), err)
			}
			break
		}
	}

	c := cmd(ctx, "set", key)
	c.Stdin = strings.NewReader(value)
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not store OAuth token for %s: %s\n%s", serverID.String(), bytes.TrimSpace(out), err)
	}
	return nil
}

// DeleteOAuthToken removes an OAuth token from docker pass.
func DeleteOAuthToken(ctx context.Context, serverID client.ID) error {
	keyID, err := GetOAuthKey(serverID)
	if err != nil {
		return err
	}
	out, err := cmd(ctx, "rm", keyID.String()).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not delete OAuth token for %s: %s\n%s", serverID.String(), bytes.TrimSpace(out), err)
	}
	return nil
}

// SetDCRClient stores a DCR client config via docker pass at docker/mcp/oauth-dcr/{serverID}.
// The value should be base64-encoded JSON of the DCR client.
func SetDCRClient(ctx context.Context, serverID client.ID, value string) error {
	keyID, err := GetDCRKey(serverID)
	if err != nil {
		return err
	}
	key := keyID.String()

	keys, err := List(ctx)
	if err != nil {
		return fmt.Errorf("could not check existing DCR client for %s: %w", serverID.String(), err)
	}

	// docker pass set is insert-only, so if the key already exists we need to remove it first.
	for _, k := range keys {
		if k == keyID {
			if out, err := cmd(ctx, "rm", key).CombinedOutput(); err != nil {
				return fmt.Errorf("could not remove existing DCR client for %s: %s\n%s", serverID.String(), bytes.TrimSpace(out), err)
			}
			break
		}
	}

	c := cmd(ctx, "set", key)
	c.Stdin = strings.NewReader(value)
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not store DCR client for %s: %s\n%s", serverID.String(), bytes.TrimSpace(out), err)
	}
	return nil
}

// DeleteDCRClient removes a DCR client config from docker pass.
func DeleteDCRClient(ctx context.Context, serverID client.ID) error {
	keyID, err := GetDCRKey(serverID)
	if err != nil {
		return err
	}
	out, err := cmd(ctx, "rm", keyID.String()).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not delete DCR client for %s: %s\n%s", serverID.String(), bytes.TrimSpace(out), err)
	}
	return nil
}
