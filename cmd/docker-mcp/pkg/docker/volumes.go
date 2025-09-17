package docker

import (
	"context"

	"github.com/docker/docker/api/types/volume"
)

func (c *dockerClient) InspectVolume(ctx context.Context, name string) (volume.Volume, error) {
	return c.apiClient().VolumeInspect(ctx, name)
}
