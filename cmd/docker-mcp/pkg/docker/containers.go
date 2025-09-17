package docker

import (
	"bufio"
	"context"
	"fmt"
	"io"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
)

func (c *dockerClient) ContainerExists(ctx context.Context, container string) (bool, container.InspectResponse, error) {
	response, err := c.apiClient().ContainerInspect(ctx, container)
	if cerrdefs.IsNotFound(err) {
		return false, response, nil
	}

	return err == nil, response, err
}

func (c *dockerClient) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	return c.apiClient().ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: force,
	})
}

func (c *dockerClient) StartContainer(ctx context.Context, containerID string, containerConfig container.Config, hostConfig container.HostConfig, networkingConfig network.NetworkingConfig) error {
	resp, err := c.apiClient().ContainerCreate(ctx, &containerConfig, &hostConfig, &networkingConfig, nil, containerID)
	if err != nil {
		return fmt.Errorf("creating container: %w", err)
	}

	if err := c.apiClient().ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("starting container: %w", err)
	}

	return nil
}

func (c *dockerClient) StopContainer(ctx context.Context, containerID string, timeout int) error {
	return c.apiClient().ContainerStop(ctx, containerID, container.StopOptions{
		Timeout: &timeout,
	})
}

func (c *dockerClient) InspectContainer(ctx context.Context, containerID string) (container.InspectResponse, error) {
	return c.apiClient().ContainerInspect(ctx, containerID)
}

func (c *dockerClient) FindContainerByLabel(ctx context.Context, label string) (string, error) {
	containers, err := c.apiClient().ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.Arg("label", label)),
	})
	if err != nil {
		return "", err
	}

	if len(containers) == 0 {
		return "", nil
	}

	return containers[0].ID, nil
}

func (c *dockerClient) FindAllContainersByLabel(ctx context.Context, label string) ([]string, error) {
	containers, err := c.apiClient().ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.Arg("label", label)),
	})
	if err != nil {
		return nil, err
	}

	ids := make([]string, len(containers))
	for i, container := range containers {
		ids[i] = container.ID
	}

	return ids, nil
}

// Logs will fetch both STDOUT and STDERR from the current container. Returns a
// ReadCloser and leaves it up to the caller to extract what it wants.
//
// This function comes from https://github.com/docker/go-sdk and is subject to
// the Apache 2.0 license. See:
//
// - https://github.com/docker/go-sdk/blob/b076369e03613f2d033373f2201021737a29bdd2/container/container.logs.go#L19-L68
// - https://github.com/docker/go-sdk/blob/b076369e03613f2d033373f2201021737a29bdd2/LICENSE
func (c *dockerClient) ReadLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	const streamHeaderSize = 8

	rc, err := c.apiClient().ContainerLogs(ctx, containerID, options)
	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	r := bufio.NewReader(rc)

	go func() {
		lineStarted := true
		for err == nil {
			line, isPrefix, err := r.ReadLine()

			if lineStarted && len(line) >= streamHeaderSize {
				line = line[streamHeaderSize:] // trim stream header
				lineStarted = false
			}
			if !isPrefix {
				lineStarted = true
			}

			_, errW := pw.Write(line)
			if errW != nil {
				return
			}

			if !isPrefix {
				_, errW := pw.Write([]byte("\n"))
				if errW != nil {
					return
				}
			}

			if err != nil {
				_ = pw.CloseWithError(err)
				return
			}
		}
	}()

	return pr, nil
}
