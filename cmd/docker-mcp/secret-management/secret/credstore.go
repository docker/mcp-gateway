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

type CredStoreProvider struct{}

func cmd(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "docker", append([]string{"pass"}, args...)...)
}

// getSecretKey prefixes the secrets with the docker/mcp/generic namespace.
// Additional namespaces can be added when defining the secretName.
//
// Example:
//
//	secretName = "mysecret/application/id"
//	return "docker/mcp/generic/mysecret/application/id"
//
// This can later then be queried by the Secrets Engine using a pattern or direct
// ID match.
//
// Example:
//
//	# anything under mcp/generic
//	pattern = "docker/mcp/generic/**"
//	# specific to mysecret
//	pattern = "docker/mcp/generic/mysecret/application/**"
func getSecretKey(secretName string) string {
	return path.Join("docker/mcp/generic/", secretName)
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

func setSecret(ctx context.Context, id string, value string) error {
	c := cmd(ctx, "set", getSecretKey(id))
	c.Stdin = strings.NewReader(value)
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not store secret: %s\n%s", bytes.TrimSpace(out), err)
	}
	return nil
}

func DeleteSecret(ctx context.Context, id string) error {
	out, err := cmd(ctx, "rm", getSecretKey(id)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not delete secret: %s\n%s\n%s", id, bytes.TrimSpace(out), err)
	}
	return nil
}
