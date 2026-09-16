package docker

import (
	"context"
	"strings"
	"testing"

	"github.com/docker/docker/client"
)

func TestClientSafeError(t *testing.T) {
	// Direct initialization with a nil client factory
	c := &dockerClient{
		apiClient: func() client.APIClient { return nil },
	}

	// This should NOT panic anymore
	err := c.PullImage(context.Background(), "hello-world")

	if err == nil {
		t.Fatal("Expected an error, got nil")
	}

	expectedErr := "docker client is not available"
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("Expected error to contain %q, got %q", expectedErr, err.Error())
	}
}
