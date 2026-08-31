package docker

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/require"
)

func TestPullImageContinuesAfterInspectError(t *testing.T) {
	apiClient := &imageAPIClient{inspectErr: errors.New("engine is starting")}
	dockerClient := &dockerClient{
		apiClient: func() client.APIClient {
			return apiClient
		},
	}

	err := dockerClient.pullImage(t.Context(), "docker.io/library/alpine:latest", func() string {
		return ""
	})

	require.NoError(t, err)
	require.Equal(t, 1, apiClient.pullCalls)
}

type imageAPIClient struct {
	client.APIClient
	inspectErr error
	pullCalls  int
}

func (c *imageAPIClient) ImageInspect(
	context.Context, string, ...client.ImageInspectOption,
) (image.InspectResponse, error) {
	return image.InspectResponse{}, c.inspectErr
}

func (c *imageAPIClient) ImagePull(
	context.Context, string, image.PullOptions,
) (io.ReadCloser, error) {
	c.pullCalls++
	return io.NopCloser(strings.NewReader("")), nil
}
