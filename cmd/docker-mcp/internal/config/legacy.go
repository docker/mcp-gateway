package config

import (
	"context"
	"errors"
	"os/exec"

	cerrdefs "github.com/containerd/errdefs"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/docker"
)

const busybox = "busybox@sha256:37f7b378a29ceb4c551b1b5582e27747b855bbfaa73fa11914fe0df028dc581f"

type ExitCodeErr interface {
	ExitCode() int
}

// CmdOutput is used for testing purposes to allow mocking the output of the command execution.
var CmdOutput = func(cmd *exec.Cmd) ([]byte, error) {
	return cmd.Output()
}

func readFromDockerVolume(ctx context.Context, docker docker.Client, filename string) ([]byte, error) {
	volumeName := "docker-prompts"

	// Check that the volume exists. If it doesn't, it's ok, assume empty configuration.
	found, err := findVolume(ctx, docker, volumeName)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, nil
	}

	cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "-v", volumeName+":/data", busybox, "/bin/sh", "-c", "cat /data/"+filename+" || exit 42")
	out, err := CmdOutput(cmd)
	if err != nil {
		// The config file doesn't exist, return empty string
		var exitError ExitCodeErr
		if errors.As(err, &exitError) && exitError.ExitCode() == 42 {
			return nil, nil
		}
		return nil, err
	}

	return out, nil
}

func findVolume(ctx context.Context, docker docker.Client, name string) (bool, error) {
	if _, err := docker.InspectVolume(ctx, name); err != nil {
		if cerrdefs.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
