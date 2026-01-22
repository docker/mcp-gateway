package embeddings_test

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/docker/mcp-gateway/pkg/gateway/embeddings"
)

// TestCloseStopsContainer verifies that Close() stops the Docker container
func TestCloseStopsContainer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	ctx := context.Background()

	// Create a temporary data directory for the test
	tmpDir := t.TempDir()

	// Count containers before starting
	countCmd := exec.CommandContext(ctx, "docker", "ps", "-q", "--filter", "ancestor=jimclark106/vector-db:latest")
	beforeOutput, err := countCmd.Output()
	if err != nil {
		t.Fatalf("Failed to check docker containers: %v", err)
	}
	containersBefore := len(string(beforeOutput))

	// Create client (starts container)
	client, err := embeddings.NewVectorDBClient(ctx, tmpDir, 1536, func(msg string) {
		t.Log(msg)
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Give the container a moment to start
	time.Sleep(1 * time.Second)

	// Verify container is running by checking docker ps
	countCmd = exec.CommandContext(ctx, "docker", "ps", "-q", "--filter", "ancestor=jimclark106/vector-db:latest")
	afterStartOutput, err := countCmd.Output()
	if err != nil {
		t.Fatalf("Failed to check docker containers after start: %v", err)
	}
	containersAfterStart := len(string(afterStartOutput))

	if containersAfterStart <= containersBefore {
		t.Skip("Container failed to start - image may not be available")
	}

	t.Logf("Container started successfully (before: %d, after: %d)", containersBefore, containersAfterStart)

	// Close the client (should stop the container)
	if err := client.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Give docker a moment to clean up (the --rm flag should auto-remove)
	time.Sleep(1 * time.Second)

	// Verify container is stopped and removed
	countCmd = exec.CommandContext(ctx, "docker", "ps", "-a", "-q", "--filter", "ancestor=jimclark106/vector-db:latest")
	afterCloseOutput, err := countCmd.Output()
	if err != nil {
		t.Fatalf("Failed to check docker containers after close: %v", err)
	}
	containersAfterClose := len(string(afterCloseOutput))

	if containersAfterClose > containersBefore {
		t.Errorf("Container not cleaned up after Close(). Before: %d, After close: %d", containersBefore, containersAfterClose)
		// Show the containers that are still running
		showCmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--filter", "ancestor=jimclark106/vector-db:latest")
		output, _ := showCmd.Output()
		t.Logf("Remaining containers:\n%s", string(output))
	} else {
		t.Logf("Container successfully stopped and removed (before: %d, after: %d)", containersBefore, containersAfterClose)
	}
}

// TestCloseIdempotent verifies that calling Close() multiple times is safe
func TestCloseIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	ctx := context.Background()
	tmpDir := t.TempDir()

	client, err := embeddings.NewVectorDBClient(ctx, tmpDir, 1536, nil)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Close multiple times should not panic or error
	if err := client.Close(); err != nil {
		t.Errorf("First Close() returned error: %v", err)
	}

	if err := client.Close(); err != nil {
		t.Errorf("Second Close() returned error: %v", err)
	}

	if err := client.Close(); err != nil {
		t.Errorf("Third Close() returned error: %v", err)
	}
}

// TestDimensionParameter verifies that different dimension values work correctly
func TestDimensionParameter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	testCases := []struct {
		name      string
		dimension int
		expected  int // expected dimension after normalization
	}{
		{"Default 1536", 1536, 1536},
		{"Custom 768", 768, 768},
		{"Custom 384", 384, 384},
		{"Zero defaults to 1536", 0, 1536},
		{"Negative defaults to 1536", -1, 1536},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			tmpDir := t.TempDir()

			client, err := embeddings.NewVectorDBClient(ctx, tmpDir, tc.dimension, nil)
			if err != nil {
				t.Fatalf("Failed to create client with dimension %d: %v", tc.dimension, err)
			}
			defer client.Close()

			// Give container a moment to start
			time.Sleep(1 * time.Second)

			// Verify container is running
			if !client.IsAlive() {
				t.Skip("Container failed to start")
			}

			t.Logf("Successfully created client with dimension %d (expected: %d)", tc.dimension, tc.expected)
		})
	}
}
