package docker

import (
	"context"
	"io"
	"sync"

	"github.com/docker/cli/cli/command"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

type Client interface {
	ContainerExists(ctx context.Context, container string) (bool, container.InspectResponse, error)
	RemoveContainer(ctx context.Context, containerID string, force bool) error
	StartContainer(ctx context.Context, containerID string, containerConfig container.Config, hostConfig container.HostConfig, networkingConfig network.NetworkingConfig) error
	StopContainer(ctx context.Context, containerID string, timeout int) error
	InspectContainer(ctx context.Context, containerID string) (container.InspectResponse, error)
	ReadLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
	ImageExists(ctx context.Context, name string) (bool, error)
	PullImage(ctx context.Context, name string) error
	PullImages(ctx context.Context, names ...string) error
	CreateNetwork(ctx context.Context, name string, internal bool, labels map[string]string) error
	RemoveNetwork(ctx context.Context, name string) error
	ConnectNetwork(ctx context.Context, networkName, container, hostname string) error
	InspectVolume(ctx context.Context, name string) (volume.Volume, error)
	ReadSecrets(ctx context.Context, names []string, lenient bool) (map[string]string, error)
}

type dockerClient struct {
	apiClient func() client.APIClient
}

func NewClient(cli command.Cli) Client {
	return &dockerClient{
		apiClient: sync.OnceValue(func() client.APIClient {
			return cli.Client()
		}),
	}
}
