package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
)

const jcatImage = "docker/jcat@sha256:76719466e8b99a65dd1d37d9ab94108851f009f0f687dce7ff8a6fc90575c4d4"

func (c *dockerClient) ReadSecrets(ctx context.Context, names []string, lenient bool) (map[string]string, error) {
	if len(names) == 0 {
		return map[string]string{}, nil // No secrets to read
	}

	if err := c.PullImage(ctx, jcatImage); err != nil {
		return nil, err
	}

	if lenient && len(names) == 1 {
		// If there's only one secret, read it directly and fall back to one-by-one reading if needed
		return c.readSecretsOneByOneOptional(ctx, names)
	}

	secrets, err := c.readSecrets(ctx, names)
	if err != nil {
		if lenient && strings.Contains(err.Error(), "no such secret") {
			return c.readSecretsOneByOneOptional(ctx, names)
		}

		return nil, fmt.Errorf("reading secrets %w", err)
	}

	return secrets, nil
}

func (c *dockerClient) readSecrets(ctx context.Context, names []string) (map[string]string, error) {
	flags := []string{"--network=none", "--pull=never"}
	var command []string

	for i, name := range names {
		file := fmt.Sprintf("/.s%d", i)
		flags = append(flags, "-l", "x-secret:"+name+"="+file)
		command = append(command, file)
	}

	var args []string

	// When running in cloud mode but not in a container, we might be able to use Docker Desktop's special socket
	// to rad the secrets.
	if os.Getenv("DOCKER_MCP_IN_CONTAINER") != "1" {
		var path string
		switch runtime.GOOS {
		case "windows":
			path = "npipe://" + strings.ReplaceAll(desktop.Paths().RawDockerSocket, `\`, `/`)
		default:
			// On Darwin/Linux, we do it only if the socket actually exists.
			if _, err := os.Stat(desktop.Paths().RawDockerSocket); err == nil {
				path = "unix://" + desktop.Paths().RawDockerSocket
			}
		}
		if path != "" {
			args = append(args, "-H", path)
		}
	}
	args = append(args, "run", "--rm")
	args = append(args, flags...)
	args = append(args, jcatImage)
	args = append(args, command...)
	buf, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("reading secrets %w: %s", err, string(buf))
	}

	var list []string
	if err := json.Unmarshal(buf, &list); err != nil {
		return nil, err
	}

	values := map[string]string{}
	for i := range names {
		values[names[i]] = list[i]
	}

	return values, nil
}

// readSecretsOneByOne reads secrets one by one, which is useful for lenient mode.
// It's slower but can handle cases where some secrets might not exist.
func (c *dockerClient) readSecretsOneByOneOptional(ctx context.Context, names []string) (map[string]string, error) {
	secrets := map[string]string{}

	for _, name := range names {
		values, err := c.readSecrets(ctx, []string{name})
		if err != nil {
			logf("couldn't read secret %s: %v", name, err)
			continue
		}

		maps.Copy(secrets, values)
	}

	return secrets, nil
}
