package gateway

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

	"github.com/docker/mcp-gateway/pkg/catalog"
)

func TestPullAndVerifyVerifiesMCPImagesBeforePull(t *testing.T) {
	oldVerify := verifyDockerImageSignatures
	defer func() {
		verifyDockerImageSignatures = oldVerify
	}()

	var events []string
	verifyDockerImageSignatures = func(_ context.Context, images []string) error {
		events = append(events, "verify:"+strings.Join(images, ","))
		return nil
	}

	docker := &recordingDockerClient{
		pullImages: func(_ context.Context, names ...string) error {
			for _, name := range names {
				events = append(events, "pull:"+name)
			}
			return nil
		},
	}
	g := &Gateway{
		Options: Options{VerifySignatures: true},
		docker:  docker,
	}

	err := g.pullAndVerify(context.Background(), Configuration{
		serverNames: []string{"custom", "signed", "legacy"},
		servers: map[string]catalog.Server{
			"custom": {Image: "ghcr.io/acme/server:latest"},
			"signed": {Image: "mcp/time@sha256:9c46a918633fb474bf8035e3ee90ebac6bcf2b18ccb00679ac4c179cba0ebfcf"},
			"legacy": {Image: "index.docker.io/mcp/github@sha256:756e73c2bd3777032dff922f4a8768135b5446b42619fe9239a190e20eb61757"},
		},
	})

	require.NoError(t, err)
	require.Equal(t, []string{
		"verify:index.docker.io/mcp/github@sha256:756e73c2bd3777032dff922f4a8768135b5446b42619fe9239a190e20eb61757,mcp/time@sha256:9c46a918633fb474bf8035e3ee90ebac6bcf2b18ccb00679ac4c179cba0ebfcf",
		"pull:ghcr.io/acme/server:latest",
		"pull:index.docker.io/mcp/github@sha256:756e73c2bd3777032dff922f4a8768135b5446b42619fe9239a190e20eb61757",
		"pull:mcp/time@sha256:9c46a918633fb474bf8035e3ee90ebac6bcf2b18ccb00679ac4c179cba0ebfcf",
	}, events)
}

func TestPullAndVerifyImageRejectsMutableMCPReference(t *testing.T) {
	oldVerify := verifyDockerImageSignatures
	defer func() {
		verifyDockerImageSignatures = oldVerify
	}()

	var verifierCalled bool
	verifyDockerImageSignatures = func(_ context.Context, _ []string) error {
		verifierCalled = true
		return nil
	}

	docker := &recordingDockerClient{}
	g := &Gateway{
		Options: Options{VerifySignatures: true},
		docker:  docker,
	}

	err := g.pullAndVerifyImage(context.Background(), "mcp/time:latest")

	require.ErrorContains(t, err, "must be referenced by digest")
	require.False(t, verifierCalled)
	require.Empty(t, docker.pulledImages)
}

func TestPullAndVerifyImageSkipsNonMCPImages(t *testing.T) {
	oldVerify := verifyDockerImageSignatures
	defer func() {
		verifyDockerImageSignatures = oldVerify
	}()

	var verifierCalled bool
	verifyDockerImageSignatures = func(_ context.Context, _ []string) error {
		verifierCalled = true
		return nil
	}

	docker := &recordingDockerClient{}
	g := &Gateway{
		Options: Options{VerifySignatures: true},
		docker:  docker,
	}

	err := g.pullAndVerifyImage(context.Background(), "ghcr.io/acme/server:latest")

	require.NoError(t, err)
	require.False(t, verifierCalled)
	require.Equal(t, []string{"ghcr.io/acme/server:latest"}, docker.pulledImages)
}

func TestPullAndVerifyImageCanSkipVerificationWhenDisabled(t *testing.T) {
	oldVerify := verifyDockerImageSignatures
	defer func() {
		verifyDockerImageSignatures = oldVerify
	}()

	var verifierCalled bool
	verifyDockerImageSignatures = func(_ context.Context, _ []string) error {
		verifierCalled = true
		return nil
	}

	docker := &recordingDockerClient{}
	g := &Gateway{
		Options: Options{VerifySignatures: false},
		docker:  docker,
	}

	err := g.pullAndVerifyImage(context.Background(), "mcp/time:latest")

	require.NoError(t, err)
	require.False(t, verifierCalled)
	require.Equal(t, []string{"mcp/time:latest"}, docker.pulledImages)
}

type recordingDockerClient struct {
	pulledImages []string
	pullImages   func(context.Context, ...string) error
}

func (c *recordingDockerClient) ContainerExists(context.Context, string) (bool, container.InspectResponse, error) {
	return false, container.InspectResponse{}, nil
}

func (c *recordingDockerClient) RemoveContainer(context.Context, string, bool) error {
	return nil
}

func (c *recordingDockerClient) StartContainer(context.Context, string, container.Config, container.HostConfig, network.NetworkingConfig) error {
	return nil
}

func (c *recordingDockerClient) StopContainer(context.Context, string, int) error {
	return nil
}

func (c *recordingDockerClient) FindContainerByLabel(context.Context, string) (string, error) {
	return "", nil
}

func (c *recordingDockerClient) FindAllContainersByLabel(context.Context, string) ([]string, error) {
	return nil, nil
}

func (c *recordingDockerClient) InspectContainer(context.Context, string) (container.InspectResponse, error) {
	return container.InspectResponse{}, nil
}

func (c *recordingDockerClient) ReadLogs(context.Context, string, container.LogsOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (c *recordingDockerClient) ImageExists(context.Context, string) (bool, error) {
	return false, nil
}

func (c *recordingDockerClient) InspectImage(context.Context, string) (image.InspectResponse, error) {
	return image.InspectResponse{}, nil
}

func (c *recordingDockerClient) PullImage(_ context.Context, name string) error {
	c.pulledImages = append(c.pulledImages, name)
	return nil
}

func (c *recordingDockerClient) PullImages(ctx context.Context, names ...string) error {
	c.pulledImages = append(c.pulledImages, names...)
	if c.pullImages != nil {
		return c.pullImages(ctx, names...)
	}
	return nil
}

func (c *recordingDockerClient) CreateNetwork(context.Context, string, bool, map[string]string) error {
	return nil
}

func (c *recordingDockerClient) RemoveNetwork(context.Context, string) error {
	return nil
}

func (c *recordingDockerClient) ConnectNetwork(context.Context, string, string, string) error {
	return nil
}

func (c *recordingDockerClient) InspectVolume(context.Context, string) (volume.Volume, error) {
	return volume.Volume{}, nil
}
