package config

import (
	"context"
	"os"
	"path/filepath"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/docker"
	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/user"
)

func ReadConfig(ctx context.Context, docker docker.Client) ([]byte, error) {
	return ReadConfigFile(ctx, docker, "config.yaml")
}

func ReadRegistry(ctx context.Context, docker docker.Client) ([]byte, error) {
	return ReadConfigFile(ctx, docker, "registry.yaml")
}

func WriteConfig(content []byte) error {
	return writeConfigFile("config.yaml", content)
}

func WriteRegistry(content []byte) error {
	return writeConfigFile("registry.yaml", content)
}

func ReadConfigFile(ctx context.Context, docker docker.Client, name string) ([]byte, error) {
	path, err := FilePath(name)
	if err != nil {
		return nil, err
	}

	buf, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}

		// File does not exist, import from legacy docker volume
		content, err := readFromDockerVolume(ctx, docker, name)
		if err != nil {
			return nil, err
		}

		// Write to a file and forget about the legacy volume
		if err := writeConfigFile(name, content); err != nil {
			return nil, err
		}

		return content, nil
	}

	return buf, nil
}

func writeConfigFile(name string, content []byte) error {
	path, err := FilePath(name)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func FilePath(name string) (string, error) {
	if filepath.IsAbs(name) {
		return name, nil
	}

	homeDir, err := user.HomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".docker", "mcp", name), nil
}
