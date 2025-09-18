package docker

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/image"
	"golang.org/x/sync/errgroup"
)

func (c *dockerClient) ImageExists(ctx context.Context, name string) (bool, error) {
	_, err := c.apiClient().ContainerInspect(ctx, name)
	if cerrdefs.IsNotFound(err) {
		return false, nil
	}

	return err == nil, err
}

func (c *dockerClient) PullImages(ctx context.Context, names ...string) error {
	registryAuthFn := sync.OnceValue(func() string {
		return getRegistryAuth(ctx)
	})

	errs, ctx := errgroup.WithContext(ctx)
	errs.SetLimit(runtime.NumCPU())

	for _, name := range names {
		errs.Go(func() error {
			return c.pullImage(ctx, name, registryAuthFn)
		})
	}

	return errs.Wait()
}

func (c *dockerClient) PullImage(ctx context.Context, name string) error {
	return c.pullImage(ctx, name, func() string {
		return getRegistryAuth(ctx)
	})
}

func (c *dockerClient) InspectImage(ctx context.Context, name string) (image.InspectResponse, error) {
	return c.apiClient().ImageInspect(ctx, name)
}

func (c *dockerClient) pullImage(ctx context.Context, imageName string, registryAuthFn func() string) error {
	inspect, err := c.apiClient().ImageInspect(ctx, imageName)
	if err != nil && !cerrdefs.IsNotFound(err) {
		return fmt.Errorf("inspecting docker image %s: %w", imageName, err)
	}

	if len(inspect.RepoDigests) > 0 {
		if inspect.RepoDigests[0] == imageName {
			return nil
		}
	}

	ref, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		return fmt.Errorf("parsing image reference %s: %w", imageName, err)
	}

	// Useful for tests. Assume that the untagged image we have locally is the right one.
	if len(inspect.RepoTags) > 0 {
		if _, digested := ref.(reference.Digested); !digested {
			return nil
		}
	}

	var pullOptions image.PullOptions
	if strings.HasPrefix(ref.Name(), "docker.io/") {
		pullOptions.RegistryAuth = registryAuthFn()
	}

	response, err := c.apiClient().ImagePull(ctx, imageName, pullOptions)
	if err != nil {
		return fmt.Errorf("pulling docker image %s: %w", imageName, err)
	}

	if _, err := io.Copy(io.Discard, response); err != nil {
		return fmt.Errorf("pulling docker image %s: %w", imageName, err)
	}

	return nil
}
