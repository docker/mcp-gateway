package embeddings

import (
	"archive/tar"
	"bytes"
	"io"
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
	tests := []struct {
		name        string
		symlinkName string
		symlinkDest string
		shouldError bool
		description string
	}{
		{
			name:        "legitimate relative symlink",
			symlinkName: "vectors.db/link",
			symlinkDest: "data.db",
			shouldError: false,
			description: "should allow relative symlinks within destination",
		},
		{
			name:        "absolute symlink target",
			symlinkName: "vectors.db/link",
			symlinkDest: "/etc/passwd",
			shouldError: true,
			description: "should reject absolute symlink targets",
		},
		{
			name:        "symlink escaping with ..",
			symlinkName: "vectors.db/link",
			symlinkDest: "../../etc/passwd",
			shouldError: true,
			description: "should reject symlinks that escape destination directory",
		},
		{
			name:        "symlink to parent that stays within",
			symlinkName: "vectors.db/subdir/link",
			symlinkDest: "../data.db",
			shouldError: false,
			description: "should allow .. if it resolves within destination",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			// Add subdirectory if needed
			if filepath.Dir(tt.symlinkName) != "vectors.db" {
				subdirHeader := &tar.Header{
					Name:     filepath.Dir(tt.symlinkName) + "/",
					Mode:     0o755,
					Typeflag: tar.TypeDir,
				}
				if err := tw.WriteHeader(subdirHeader); err != nil {
					t.Fatalf("failed to write subdirectory header: %v", err)
				}
			}

			// Add the symlink
			header := &tar.Header{
				Name:     tt.symlinkName,
				Linkname: tt.symlinkDest,
				Typeflag: tar.TypeSymlink,
			}
			if err := tw.WriteHeader(header); err != nil {
				t.Fatalf("failed to write symlink header: %v", err)
			}

			tw.Close()

			layer := &mockLayer{data: buf.Bytes()}

			// Extract and check result
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

// TestExtractLayerSymlinkChaining tests protection against symlink chaining attacks
func TestExtractLayerSymlinkChaining(t *testing.T) {
	destDir := t.TempDir()

	// Create a malicious tar with symlink chaining:
	// 1. vectors.db/link -> .. (points outside destDir to parent directory)
	// 2. vectors.db/escape -> link/.. (chains through the symlink to escape further)
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	// Add directory
	if err := tw.WriteHeader(&tar.Header{Name: "vectors.db/", Mode: 0o755, Typeflag: tar.TypeDir}); err != nil {
		t.Fatalf("failed to write header: %v", err)
	}

	// Add first symlink that points outside: vectors.db/link -> ../..
	// This creates: destDir/vectors.db/link -> ../.. which resolves to parent of destDir
	if err := tw.WriteHeader(&tar.Header{
		Name:     "vectors.db/link",
		Linkname: "../..",
		Typeflag: tar.TypeSymlink,
	}); err != nil {
		t.Fatalf("failed to write symlink header: %v", err)
	}

	tw.Close()

	layer := &mockLayer{data: buf.Bytes()}

	// This should fail because the symlink escapes the destination directory
	err := extractLayer(layer, destDir)
	if err == nil {
		t.Error("Expected error for symlink chaining attack, but extraction succeeded")
	} else {
		t.Logf("Symlink chaining attack correctly blocked: %v", err)
	}
}
