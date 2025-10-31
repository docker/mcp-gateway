package mocks

import (
	"context"

	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/google/go-containerregistry/pkg/name"
)

type mockOCIServiceOption func(*mockOCIServiceOptions)

type mockOCIServiceOptions struct {
	digests map[string]string
	labels  map[string]map[string]string
}

func WithDigests(digests map[string]string) mockOCIServiceOption {
	return func(o *mockOCIServiceOptions) {
		o.digests = digests
	}
}

func WithLabels(labels map[string]map[string]string) mockOCIServiceOption {
	return func(o *mockOCIServiceOptions) {
		o.labels = labels
	}
}

type mockOCIService struct {
	options mockOCIServiceOptions
}

func NewMockOCIService(opts ...mockOCIServiceOption) oci.Service {
	options := &mockOCIServiceOptions{
		digests: make(map[string]string),
		labels:  make(map[string]map[string]string),
	}
	for _, opt := range opts {
		opt(options)
	}
	return &mockOCIService{
		options: *options,
	}
}

func (s *mockOCIService) GetImageDigest(ctx context.Context, ref name.Reference) (string, error) {
	return s.options.digests[ref.String()], nil
}

func (s *mockOCIService) GetImageLabels(ctx context.Context, ref name.Reference) (map[string]string, error) {
	return s.options.labels[ref.String()], nil
}
