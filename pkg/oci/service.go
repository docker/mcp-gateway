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
	GetImageDigest(ctx context.Context, img v1.Image) (string, error)
	GetImageLabels(ctx context.Context, img v1.Image) (map[string]string, error)
	GetLocalImage(ctx context.Context, ref name.Reference) (v1.Image, error)
	GetRemoteImage(ctx context.Context, ref name.Reference) (v1.Image, error)
}

type service struct{}

// TODO (cody): migrate everything in the other files over to the service
func NewService() Service {
	return &service{}
}

func (s *service) GetImageDigest(ctx context.Context, img v1.Image) (string, error) {
	digest, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("failed to get digest: %w", err)
	}

	return digest.String(), nil
}

func (s *service) GetImageLabels(ctx context.Context, img v1.Image) (map[string]string, error) {
	labels, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("failed to get config file: %w", err)
	}

	return labels.Config.Labels, nil
}

func (s *service) GetLocalImage(ctx context.Context, ref name.Reference) (v1.Image, error) {
	return daemon.Image(ref, daemon.WithContext(ctx))
}

func (s *service) GetRemoteImage(ctx context.Context, ref name.Reference) (v1.Image, error) {
	return remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithContext(ctx))
}

func IsNoSuchImageError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "no such image")
}
