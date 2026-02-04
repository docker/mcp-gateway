package docker

import (
	"context"

	"github.com/docker/docker/api/types/volume"
)

func (c *dockerClient) InspectVolume(ctx context.Context, name string) (volume.Volume, error) {
	cli, err := c.client()
	if err != nil {
		return volume.Volume{}, err
	}
	return cli.VolumeInspect(ctx, name)
}
