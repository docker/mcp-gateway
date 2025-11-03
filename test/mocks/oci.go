package mocks

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/docker/mcp-gateway/pkg/oci"
)

type MockOCIServiceOption func(*MockOCIServiceOptions)

type MockOCIServiceOptions struct {
	digests map[string]string
	labels  map[string]map[string]string
}

func WithDigests(digests map[string]string) MockOCIServiceOption {
	return func(o *MockOCIServiceOptions) {
		o.digests = digests
	}
}

func WithLabels(labels map[string]map[string]string) MockOCIServiceOption {
	return func(o *MockOCIServiceOptions) {
		o.labels = labels
	}
}

type mockOCIService struct {
	options MockOCIServiceOptions
}

func NewMockOCIService(opts ...MockOCIServiceOption) oci.Service {
	options := &MockOCIServiceOptions{
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

func (s *mockOCIService) GetImageDigest(_ context.Context, ref name.Reference) (string, error) {
	return s.options.digests[ref.String()], nil
}

func (s *mockOCIService) GetImageLabels(_ context.Context, ref name.Reference) (map[string]string, error) {
	return s.options.labels[ref.String()], nil
}
