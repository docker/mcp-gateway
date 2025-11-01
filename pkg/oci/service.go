package oci

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type Service interface {
	GetImageDigest(ctx context.Context, ref name.Reference) (string, error)
	GetImageLabels(ctx context.Context, ref name.Reference) (map[string]string, error)
}

type service struct{}

// TODO (cody): migrate everything in the other files over to the service
func NewService() Service {
	return &service{}
}

func (s *service) GetImageDigest(ctx context.Context, ref name.Reference) (string, error) {
	img, err := resolveImage(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("failed to get image: %w", err)
	}

	digest, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("failed to get digest: %w", err)
	}

	return digest.String(), nil
}

func (s *service) GetImageLabels(ctx context.Context, ref name.Reference) (map[string]string, error) {
	img, err := resolveImage(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to get image: %w", err)
	}

	labels, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("failed to get config file: %w", err)
	}

	return labels.Config.Labels, nil
}

func resolveImage(ctx context.Context, ref name.Reference) (v1.Image, error) {
	img, err := daemon.Image(ref, daemon.WithContext(ctx))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such image") {
			img, err = remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithContext(ctx))
			if err != nil {
				return nil, fmt.Errorf("failed to get image from remote: %w", err)
			}
			return img, nil
		}
		return nil, fmt.Errorf("failed to get image from daemon: %w", err)
	}

	return img, nil
}
