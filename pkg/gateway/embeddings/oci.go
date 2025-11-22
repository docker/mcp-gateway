package embeddings

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/user"
)

const (
	embeddingsImageRef = "jimclark106/embeddings:latest"
	vectorDBFileName   = "vectors.db"
)

// Pull downloads the embeddings OCI artifact, extracts it to a temp directory,
// and copies the vector.db directory to ~/.docker/mcp if it doesn't already exist.
//
// Example usage:
//
//	go run ./examples/embeddings/pull.go
func Pull(ctx context.Context) error {
	// Get the home directory to determine the target path
	homeDir, err := user.HomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	mcpDir := filepath.Join(homeDir, ".docker", "mcp")
	targetPath := filepath.Join(mcpDir, vectorDBFileName)

	// Check if vector.db already exists
	if _, err := os.Stat(targetPath); err == nil {
		log.Logf("Vector database already exists at %s, skipping download", targetPath)
		return nil
	}

	log.Logf("Downloading embeddings from %s", embeddingsImageRef)

	// Parse the image reference
	ref, err := name.ParseReference(embeddingsImageRef)
	if err != nil {
		return fmt.Errorf("failed to parse image reference: %w", err)
	}

	// Pull the image
	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	// Create a temporary directory for extraction
	tmpDir, err := os.MkdirTemp("", "embeddings-extract-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	log.Logf("Extracting image to temporary directory: %s", tmpDir)

	// Get all layers
	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("failed to get image layers: %w", err)
	}

	// Extract each layer
	for i, layer := range layers {
		if err := extractLayer(layer, tmpDir); err != nil {
			return fmt.Errorf("failed to extract layer %d: %w", i, err)
		}
	}

	// Verify that vector.db directory exists in the extracted content
	extractedVectorDB := filepath.Join(tmpDir, vectorDBFileName)
	if _, err := os.Stat(extractedVectorDB); os.IsNotExist(err) {
		return fmt.Errorf("vectors.db directory not found in extracted image")
	}

	// Ensure the target directory exists
	if err := os.MkdirAll(mcpDir, 0o755); err != nil {
		return fmt.Errorf("failed to create mcp directory: %w", err)
	}

	// Copy vector.db directory to ~/.docker/mcp
	log.Logf("Copying vector.db to %s", targetPath)
	if err := copyDir(extractedVectorDB, targetPath); err != nil {
		return fmt.Errorf("failed to copy vector.db directory: %w", err)
	}

	log.Logf("Successfully installed vector database at %s", targetPath)
	return nil
}

// extractLayer extracts a single layer (tar archive) to the destination directory
func extractLayer(layer interface{ Uncompressed() (io.ReadCloser, error) }, destDir string) error {
	rc, err := layer.Uncompressed()
	if err != nil {
		return fmt.Errorf("failed to get layer content: %w", err)
	}
	defer rc.Close()

	tarReader := tar.NewReader(rc)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		target := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}

		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			if _, err := io.Copy(file, tarReader); err != nil {
				file.Close()
				return fmt.Errorf("failed to write file: %w", err)
			}
			file.Close()

		case tar.TypeSymlink:
			// Handle symlinks
			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("failed to create symlink: %w", err)
			}

		default:
			// Skip other types (block devices, etc.)
			log.Logf("Skipping unsupported tar entry type %d: %s", header.Typeflag, header.Name)
		}
	}

	return nil
}

// copyDir recursively copies a directory from src to dst
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate the relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			// Create directory
			return os.MkdirAll(targetPath, info.Mode())
		}

		// Copy file
		return copyFile(path, targetPath, info.Mode())
	})
}

// copyFile copies a single file from src to dst with the specified mode
func copyFile(src, dst string, mode os.FileMode) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	destFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	return nil
}

// Push creates an OCI artifact containing the vector database directory and pushes it to the specified OCI reference.
// The directory will always be named "vectors.db" in the OCI artifact regardless of the source directory name.
//
// Example usage:
//
//	go run ./examples/embeddings/push.go ~/.docker/mcp/vectors.db jimclark106/embeddings:v1.0
func Push(ctx context.Context, vectorDBPath string, ociRef string) error {
	log.Logf("Pushing vector database from %s to %s", vectorDBPath, ociRef)

	// Verify that the source directory exists
	if _, err := os.Stat(vectorDBPath); os.IsNotExist(err) {
		return fmt.Errorf("vectors.db directory not found at %s", vectorDBPath)
	}

	// Parse the OCI reference
	ref, err := name.ParseReference(ociRef)
	if err != nil {
		return fmt.Errorf("failed to parse OCI reference: %w", err)
	}

	// Create a tar archive from the vector.db directory
	log.Logf("Creating tar archive from %s", vectorDBPath)
	tarBuffer, err := createTarFromDirectory(vectorDBPath)
	if err != nil {
		return fmt.Errorf("failed to create tar archive: %w", err)
	}

	// Create a layer from the tar archive
	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(tarBuffer.Bytes())), nil
	})
	if err != nil {
		return fmt.Errorf("failed to create layer from tar: %w", err)
	}

	// Start with an empty image
	img := empty.Image

	// Add the layer to the image
	img, err = mutate.AppendLayers(img, layer)
	if err != nil {
		return fmt.Errorf("failed to append layer to image: %w", err)
	}

	// Push the image to the registry
	log.Logf("Pushing image to %s", ociRef)
	if err := remote.Write(ref, img, remote.WithAuthFromKeychain(authn.DefaultKeychain), remote.WithContext(ctx)); err != nil {
		return fmt.Errorf("failed to push image: %w", err)
	}

	log.Logf("Successfully pushed vector database to %s", ociRef)
	return nil
}

// createTarFromDirectory creates a tar archive from the specified directory
func createTarFromDirectory(srcDir string) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	defer tw.Close()

	// Walk the directory tree and add files to the tar archive
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get the relative path from the source directory
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("failed to create tar header: %w", err)
		}

		// Always use vectorDBFileName as the root directory name in the archive
		// This ensures consistency regardless of the source directory name
		if relPath == "." {
			header.Name = vectorDBFileName
		} else {
			header.Name = filepath.Join(vectorDBFileName, relPath)
		}

		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("failed to read symlink: %w", err)
			}
			header.Linkname = linkTarget
		}

		// Write the header
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}

		// If it's a regular file, write the content
		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				return fmt.Errorf("failed to write file to tar: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return &buf, nil
}
