package proxies

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/docker"
)

type fakeDocker struct {
	networks []string
	removed  []string
}

func (f *fakeDocker) ContainerExists(context.Context, string) (bool, container.InspectResponse, error) {
	return true, container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{State: &container.State{Running: true}},
	}, nil
}
func (f *fakeDocker) RemoveContainer(_ context.Context, id string, _ bool) error {
	f.removed = append(f.removed, id)
	return nil
}
func (f *fakeDocker) StartContainer(context.Context, string, container.Config, container.HostConfig, network.NetworkingConfig) error {
	return nil
}
func (f *fakeDocker) StopContainer(context.Context, string, int) error { return nil }
func (f *fakeDocker) FindContainerByLabel(context.Context, string) (string, error) {
	return "", nil
}
func (f *fakeDocker) FindAllContainersByLabel(context.Context, string) ([]string, error) {
	return nil, nil
}
func (f *fakeDocker) InspectContainer(context.Context, string) (container.InspectResponse, error) {
	return container.InspectResponse{}, nil
}
func (f *fakeDocker) ReadLogs(context.Context, string, container.LogsOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (f *fakeDocker) ImageExists(context.Context, string) (bool, error) { return false, nil }
func (f *fakeDocker) InspectImage(context.Context, string) (image.InspectResponse, error) {
	return image.InspectResponse{}, nil
}
func (f *fakeDocker) PullImage(context.Context, string) error { return nil }
func (f *fakeDocker) PullImages(context.Context, ...string) error {
	return nil
}
func (f *fakeDocker) CreateNetwork(_ context.Context, name string, _ bool, _ map[string]string) error {
	f.networks = append(f.networks, name)
	return nil
}
func (f *fakeDocker) RemoveNetwork(_ context.Context, name string) error {
	f.removed = append(f.removed, name)
	return nil
}
func (f *fakeDocker) ConnectNetwork(context.Context, string, string, string) error { return nil }
func (f *fakeDocker) InspectVolume(context.Context, string) (volume.Volume, error) {
	return volume.Volume{}, nil
}

func TestRunNetworkProxiesUsesDistinctNetworks(t *testing.T) {
	spec, err := ParseProxySpec("example.com:443/https")
	require.NoError(t, err)

	cli := &fakeDocker{}
	first, cleanup1, err := RunNetworkProxies(context.Background(), cli, []Proxy{spec}, false, false)
	require.NoError(t, err)
	second, cleanup2, err := RunNetworkProxies(context.Background(), cli, []Proxy{spec}, false, false)
	require.NoError(t, err)

	require.NotEqual(t, first.NetworkName, second.NetworkName)
	require.True(t, strings.HasPrefix(first.NetworkName, "docker-mcp-proxies-int-"))
	require.True(t, strings.HasPrefix(second.NetworkName, "docker-mcp-proxies-int-"))

	require.NoError(t, cleanup1(context.Background()))
	require.NoError(t, cleanup2(context.Background()))

	var sawFirst, sawSecond bool
	for _, name := range cli.removed {
		if name == first.NetworkName {
			sawFirst = true
		}
		if name == second.NetworkName {
			sawSecond = true
		}
	}
	require.True(t, sawFirst)
	require.True(t, sawSecond)
}

var _ docker.Client = (*fakeDocker)(nil)
