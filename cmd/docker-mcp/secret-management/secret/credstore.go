package secret

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
)

type CredStoreProvider struct{}

func cmd(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "docker", append([]string{"pass"}, args...)...)
}

func NewCredStoreProvider() *CredStoreProvider {
	return &CredStoreProvider{}
}

func getSecretKey(secretName string) string {
	return "docker/mcp/generic/" + secretName
}

func (store *CredStoreProvider) List(ctx context.Context) ([]string, error) {
	c := cmd(ctx, "ls")
	out, err := c.Output()
	if err != nil {
		return nil, err
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

func (store *CredStoreProvider) SetSecret(ctx context.Context, id string, value string) error {
	c := cmd(ctx, "set", getSecretKey(id))
	in, err := c.StdinPipe()
	if err != nil {
		return err
	}
	if err := c.Start(); err != nil {
		return err
	}
	_, err = in.Write([]byte(value))
	if err != nil {
		return err
	}
	return c.Wait()
}

func (store *CredStoreProvider) DeleteSecret(ctx context.Context, id string) error {
	c := cmd(ctx, "rm", getSecretKey(id))
	return c.Run()
}
