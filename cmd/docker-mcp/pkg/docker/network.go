package docker

import (
	"context"

	"github.com/docker/docker/api/types/network"
)

func (c *dockerClient) CreateNetwork(ctx context.Context, name string, internal bool, labels map[string]string) error {
	_, err := c.apiClient().NetworkCreate(ctx, name, network.CreateOptions{
		Internal: internal,
		Labels:   labels,
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *dockerClient) RemoveNetwork(ctx context.Context, name string) error {
	return c.apiClient().NetworkRemove(ctx, name)
}

func (c *dockerClient) ConnectNetwork(ctx context.Context, networkName, container, hostname string) error {
	return c.apiClient().NetworkConnect(ctx, networkName, container, &network.EndpointSettings{
		Aliases: []string{hostname},
	})
}
