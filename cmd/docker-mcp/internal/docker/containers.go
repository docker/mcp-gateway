package docker

import (
	"context"
	"fmt"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
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
