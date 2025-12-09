package embeddings

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestExtractLayerPathTraversalPrevention tests that the extractLayer function
// properly prevents path traversal attacks (zip slip vulnerability)
func TestExtractLayerPathTraversalPrevention(t *testing.T) {
	tests := []struct {
		name        string
		tarEntries  []tarEntry
		shouldError bool
		description string
	}{
		{
			name: "legitimate nested path",
			tarEntries: []tarEntry{
				{name: "vectors.db/data.db", content: "legitimate content", isDir: false},
			},
			shouldError: false,
			description: "should allow legitimate nested paths",
		},
		{
			name: "path traversal with ..",
			tarEntries: []tarEntry{
				{name: "../../etc/passwd", content: "malicious content", isDir: false},
			},
			shouldError: true,
			description: "should reject paths with .. that escape destination",
		},
		{
			name: "absolute path",
			tarEntries: []tarEntry{
				{name: "/etc/passwd", content: "malicious content", isDir: false},
			},
			shouldError: true,
			description: "should reject absolute paths that escape destination",
		},
		{
			name: "path traversal in middle",
			tarEntries: []tarEntry{
				{name: "vectors.db/../../etc/passwd", content: "malicious content", isDir: false},
			},
			shouldError: true,
			description: "should reject paths with .. in the middle that escape",
		},
		{
			name: "legitimate .. that stays within destination",
			tarEntries: []tarEntry{
				{name: "vectors.db/subdir", content: "", isDir: true},
				{name: "vectors.db/subdir/../file.db", content: "legitimate content", isDir: false},
			},
			shouldError: false,
			description: "should allow .. if it resolves within destination",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary directory for extraction
			destDir := t.TempDir()

			// Create a tar archive with the test entries
			tarBuffer := createTestTar(t, tt.tarEntries)

			// Create a mock layer
			layer := &mockLayer{data: tarBuffer.Bytes()}

			// Try to extract the layer
			err := extractLayer(layer, destDir)

			if tt.shouldError {
				if err == nil {
					t.Errorf("%s: expected error but got none", tt.description)
				}
			} else {
				if err != nil {
					t.Errorf("%s: unexpected error: %v", tt.description, err)
				}
			}
		})
	}
}

// tarEntry represents a single entry in a tar archive for testing
type tarEntry struct {
	name    string
	content string
	isDir   bool
}

// createTestTar creates a tar archive with the given entries
func createTestTar(t *testing.T, entries []tarEntry) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	defer tw.Close()

	for _, entry := range entries {
		var header *tar.Header
		if entry.isDir {
			header = &tar.Header{
				Name:     entry.name,
				Mode:     0o755,
				Typeflag: tar.TypeDir,
			}
		} else {
			header = &tar.Header{
				Name:     entry.name,
				Mode:     0o644,
				Size:     int64(len(entry.content)),
				Typeflag: tar.TypeReg,
			}
		}

		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}

		if !entry.isDir {
			if _, err := tw.Write([]byte(entry.content)); err != nil {
				t.Fatalf("failed to write tar content: %v", err)
			}
		}
	}

	return &buf
}

// mockLayer implements the interface required by extractLayer
type mockLayer struct {
	data []byte
}

func (m *mockLayer) Uncompressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(m.data)), nil
}

// TestExtractLayerSymlinkSafety tests that symlinks are handled safely
func TestExtractLayerSymlinkSafety(t *testing.T) {
	destDir := t.TempDir()

	// Create a tar with a directory and a symlink
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Add the parent directory first
	dirHeader := &tar.Header{
		Name:     "vectors.db/",
		Mode:     0o755,
		Typeflag: tar.TypeDir,
	}
	if err := tw.WriteHeader(dirHeader); err != nil {
		t.Fatalf("failed to write directory header: %v", err)
	}

	// Add a symlink
	header := &tar.Header{
		Name:     "vectors.db/link",
		Linkname: "/etc/passwd",
		Typeflag: tar.TypeSymlink,
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatalf("failed to write symlink header: %v", err)
	}

	tw.Close()

	layer := &mockLayer{data: buf.Bytes()}

	// Extract should succeed (we extract the symlink but validate the path)
	err := extractLayer(layer, destDir)
	if err != nil {
		t.Errorf("unexpected error extracting symlink: %v", err)
	}

	// Verify the symlink was created in the destination
	linkPath := filepath.Join(destDir, "vectors.db", "link")
	if _, err := os.Lstat(linkPath); err != nil {
		t.Errorf("symlink was not created: %v", err)
	}
}
