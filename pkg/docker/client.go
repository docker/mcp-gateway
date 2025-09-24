package docker

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"sync"

	"github.com/docker/cli/cli/command"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/version"
)

type Client interface {
	ContainerExists(ctx context.Context, container string) (bool, container.InspectResponse, error)
	RemoveContainer(ctx context.Context, containerID string, force bool) error
	StartContainer(ctx context.Context, containerID string, containerConfig container.Config, hostConfig container.HostConfig, networkingConfig network.NetworkingConfig) error
	StopContainer(ctx context.Context, containerID string, timeout int) error
	FindContainerByLabel(ctx context.Context, label string) (string, error)
	FindAllContainersByLabel(ctx context.Context, label string) ([]string, error)
	InspectContainer(ctx context.Context, containerID string) (container.InspectResponse, error)
	ReadLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
	ImageExists(ctx context.Context, name string) (bool, error)
	InspectImage(ctx context.Context, name string) (image.InspectResponse, error)
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
			_ = cli.Apply(func(cli *command.DockerCli) error {
				if mobyClient, ok := cli.Client().(*client.Client); ok {
					_ = client.WithUserAgent(version.UserAgent())(mobyClient)
				}
				return nil
			})

			return cli.Client()
		}),
	}
}

func RunningInDockerCE(ctx context.Context, dockerCli command.Cli) (bool, error) {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return false, nil
	}

	info, err := dockerCli.Client().Info(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to ping Docker daemon: %w", err)
	}

	return info.OperatingSystem != "Docker Desktop", nil
}
