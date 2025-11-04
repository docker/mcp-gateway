package mocks

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/opencontainers/go-digest"

	"github.com/docker/mcp-gateway/pkg/oci"
)

type MockOCIServiceOption func(*MockOCIServiceOptions)

type MockOCIServiceOptions struct {
	localImages  []MockImage
	remoteImages []MockImage
}

func WithLocalImages(localImages []MockImage) MockOCIServiceOption {
	return func(o *MockOCIServiceOptions) {
		o.localImages = localImages
	}
}

// WithRemoteImages marks specific images as remote-only (GetLocalImage will return "no such image" error)
func WithRemoteImages(remoteImages []MockImage) MockOCIServiceOption {
	return func(o *MockOCIServiceOptions) {
		o.remoteImages = remoteImages
	}
}

type mockOCIService struct {
	options MockOCIServiceOptions
}

func NewMockOCIService(opts ...MockOCIServiceOption) oci.Service {
	options := &MockOCIServiceOptions{
		localImages:  make([]MockImage, 0),
		remoteImages: make([]MockImage, 0),
	}
	for _, opt := range opts {
		opt(options)
	}

	return &mockOCIService{
		options: *options,
	}
}

func (s *mockOCIService) GetImageDigest(_ context.Context, img v1.Image) (string, error) {
	mockImg, ok := img.(*MockImage)
	if !ok {
		return "", fmt.Errorf("expected mockImage, got %T", img)
	}
	return mockImg.DigestString, nil
}

func (s *mockOCIService) GetImageLabels(_ context.Context, img v1.Image) (map[string]string, error) {
	mockImg, ok := img.(*MockImage)
	if !ok {
		return nil, fmt.Errorf("expected mockImage, got %T", img)
	}
	return mockImg.Labels, nil
}

func (s *mockOCIService) GetLocalImage(_ context.Context, ref name.Reference) (v1.Image, error) {
	refStr := ref.String()

	for _, img := range s.options.localImages {
		if img.Ref == refStr {
			return &img, nil
		}
	}

	return nil, fmt.Errorf("no such image: %s", refStr)
}

func (s *mockOCIService) GetRemoteImage(_ context.Context, ref name.Reference) (v1.Image, error) {
	refStr := ref.String()

	for _, img := range s.options.remoteImages {
		if img.Ref == refStr || img.Ref+"@"+img.DigestString == refStr {
			return &img, nil
		}
	}

	return nil, fmt.Errorf("no such image: %s", refStr)
}

// MockImage is a minimal implementation of v1.Image for testing
type MockImage struct {
	Ref          string
	Labels       map[string]string
	DigestString string
}

var _ v1.Image = &MockImage{}

func (m *MockImage) Layers() ([]v1.Layer, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockImage) MediaType() (types.MediaType, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *MockImage) Size() (int64, error) {
	return 0, fmt.Errorf("not implemented")
}

func (m *MockImage) ConfigName() (v1.Hash, error) {
	return v1.Hash{}, fmt.Errorf("not implemented")
}

func (m *MockImage) ConfigFile() (*v1.ConfigFile, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockImage) RawConfigFile() ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockImage) Digest() (v1.Hash, error) {
	d, err := digest.Parse(m.DigestString)
	if err != nil {
		return v1.Hash{}, err
	}
	return v1.Hash{
		Algorithm: string(d.Algorithm()),
		Hex:       d.Encoded(),
	}, nil
}

func (m *MockImage) Manifest() (*v1.Manifest, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockImage) RawManifest() ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockImage) LayerByDigest(v1.Hash) (v1.Layer, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockImage) LayerByDiffID(v1.Hash) (v1.Layer, error) {
	return nil, fmt.Errorf("not implemented")
}
