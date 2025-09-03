package oci

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
)

const MCPServerArtifactType = "application/vnd.docker.mcp.server"

// Catalog represents an OCI catalog structure with a top-level Registry field
type Catalog struct {
	Registry []Server `json:"registry"`
}

// Server represents a server definition in the OCI catalog
type Server struct {
	Server   ServerDetail    `json:"server"`
	Registry json.RawMessage `json:"x-io.modelcontextprotocol.registry"`
}

func CreateArtifactWithSubjectAndPush(ref name.Reference, catalog Catalog, subjectDigest v1.Hash, subjectSize int64, subjectMediaType types.MediaType, push bool) (string, error) {
	// Marshal the catalog to bytes
	content, err := json.Marshal(catalog)
	if err != nil {
		return "", fmt.Errorf("failed to marshal catalog: %w", err)
	}

	// Create empty config blob
	emptyConfig := []byte("{}")
	configDigest := digest.FromBytes(emptyConfig)
	contentDigest := digest.FromBytes(content)

	// Create OCI manifest with subject descriptor using OCI spec types
	manifest := oci.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType:    "application/vnd.oci.image.manifest.v1+json",
		ArtifactType: MCPServerArtifactType,
		Config: oci.Descriptor{
			MediaType: "application/vnd.oci.empty.v1+json",
			Digest:    configDigest,
			Size:      int64(len(emptyConfig)),
		},
		Layers: []oci.Descriptor{
			{
				MediaType: "application/json",
				Digest:    contentDigest,
				Size:      int64(len(content)),
			},
		},
		Subject: &oci.Descriptor{
			MediaType: string(subjectMediaType),
			Digest:    digest.Digest(subjectDigest.String()),
			Size:      subjectSize,
		},
	}

	if push {
		// Upload empty config blob
		err := uploadBlob(ref, emptyConfig, configDigest)
		if err != nil {
			return "", fmt.Errorf("failed to upload config blob: %w", err)
		}

		// Upload content blob
		err = uploadBlob(ref, content, contentDigest)
		if err != nil {
			return "", fmt.Errorf("failed to upload content blob: %w", err)
		}

		// Upload manifest
		manifestBytes, err := json.Marshal(manifest)
		if err != nil {
			return "", fmt.Errorf("failed to marshal manifest: %w", err)
		}

		err = uploadManifest(ref, manifestBytes)
		if err != nil {
			return "", fmt.Errorf("failed to upload manifest: %w", err)
		}

		// Calculate and output the manifest digest
		manifestDigest := sha256.Sum256(manifestBytes)
		return fmt.Sprintf("%x", manifestDigest), nil
	}

	// Store locally using the store package
	// Note: For local storage, we only need the manifest since the store
	// handles the artifact metadata, not the actual blob storage
	fmt.Printf("Storing artifact locally (not pushing to registry)\n")

	return "", nil
}

func uploadBlob(ref name.Reference, data []byte, _ digest.Digest) error {
	// Use go-containerregistry's blob upload mechanism
	repo := ref.Context()

	// Create a layer with the data
	layer := static.NewLayer(data, types.MediaType("application/octet-stream"))

	// Write the layer (blob) to the repository with authentication
	return remote.WriteLayer(repo, layer, remote.WithAuthFromKeychain(authn.DefaultKeychain))
}

func uploadManifest(ref name.Reference, manifestBytes []byte) error {
	// Create a custom image with the manifest
	return remote.Put(ref, &customManifest{data: manifestBytes}, remote.WithAuthFromKeychain(authn.DefaultKeychain))
}

// customManifest implements v1.Image interface for custom manifest
type customManifest struct {
	data []byte
}

func (c *customManifest) Layers() ([]v1.Layer, error) {
	return nil, nil
}

func (c *customManifest) MediaType() (types.MediaType, error) {
	return "application/vnd.oci.image.manifest.v1+json", nil
}

func (c *customManifest) Size() (int64, error) {
	return int64(len(c.data)), nil
}

func (c *customManifest) ConfigName() (v1.Hash, error) {
	return v1.Hash{}, nil
}

func (c *customManifest) ConfigFile() (*v1.ConfigFile, error) {
	return &v1.ConfigFile{}, nil
}

func (c *customManifest) RawConfigFile() ([]byte, error) {
	return []byte("{}"), nil
}

func (c *customManifest) Digest() (v1.Hash, error) {
	h := sha256.Sum256(c.data)
	return v1.Hash{Algorithm: "sha256", Hex: fmt.Sprintf("%x", h)}, nil
}

func (c *customManifest) Manifest() (*v1.Manifest, error) {
	// Convert OCI manifest to go-containerregistry manifest
	var ociManifest oci.Manifest
	err := json.Unmarshal(c.data, &ociManifest)
	if err != nil {
		return nil, err
	}

	// Create basic manifest structure that go-containerregistry expects
	// Note: This is a simplified conversion - the subject field won't be preserved
	// in the go-containerregistry v1.Manifest type, but the raw manifest will contain it
	manifest := &v1.Manifest{
		SchemaVersion: int64(ociManifest.SchemaVersion),
		MediaType:     types.MediaType(ociManifest.MediaType),
	}

	// Convert config descriptor
	configHash, _ := v1.NewHash(ociManifest.Config.Digest.String())
	manifest.Config = v1.Descriptor{
		MediaType: types.MediaType(ociManifest.Config.MediaType),
		Size:      ociManifest.Config.Size,
		Digest:    configHash,
	}

	// Convert layer descriptors
	for _, layer := range ociManifest.Layers {
		layerHash, _ := v1.NewHash(layer.Digest.String())
		manifest.Layers = append(manifest.Layers, v1.Descriptor{
			MediaType: types.MediaType(layer.MediaType),
			Size:      layer.Size,
			Digest:    layerHash,
		})
	}

	return manifest, nil
}

func (c *customManifest) RawManifest() ([]byte, error) {
	return c.data, nil
}

// ReadArtifact reads an OCI artifact by reference and returns parsed Catalog from the first layer
// if the artifact type is application/vnd.docker.mcp.server, otherwise returns an error
func ReadArtifact(ociRef string) (Catalog, error) {
	if ociRef == "" {
		return Catalog{}, fmt.Errorf("OCI reference is required")
	}

	// Parse the OCI reference
	ref, err := name.ParseReference(ociRef)
	if err != nil {
		return Catalog{}, fmt.Errorf("failed to parse OCI reference %s: %w", ociRef, err)
	}

	// Get the image/artifact from the registry
	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return Catalog{}, fmt.Errorf("failed to fetch image/artifact %s: %w", ociRef, err)
	}

	// Get the raw manifest to check if it's an OCI artifact
	rawManifest, err := img.RawManifest()
	if err != nil {
		return Catalog{}, fmt.Errorf("failed to get raw manifest: %w", err)
	}

	// Parse as OCI manifest to check artifact type
	var ociManifest oci.Manifest
	if err := json.Unmarshal(rawManifest, &ociManifest); err != nil {
		return Catalog{}, fmt.Errorf("failed to parse OCI manifest: %w", err)
	}

	// Check if this is an MCP server artifact
	if ociManifest.ArtifactType != MCPServerArtifactType {
		return Catalog{}, fmt.Errorf("artifact type %s is not %s", ociManifest.ArtifactType, MCPServerArtifactType)
	}

	// Get the layers
	layers, err := img.Layers()
	if err != nil {
		return Catalog{}, fmt.Errorf("failed to get layers: %w", err)
	}

	if len(layers) == 0 {
		return Catalog{}, fmt.Errorf("no layers found in artifact")
	}

	// Get content from the first layer
	firstLayer := layers[0]
	rc, err := firstLayer.Uncompressed()
	if err != nil {
		return Catalog{}, fmt.Errorf("failed to get first layer content: %w", err)
	}
	defer rc.Close()

	content, err := io.ReadAll(rc)
	if err != nil {
		return Catalog{}, fmt.Errorf("failed to read first layer content: %w", err)
	}

	// Parse JSON from first layer as Catalog
	var catalog Catalog
	if err := json.Unmarshal(content, &catalog); err != nil {
		return Catalog{}, fmt.Errorf("failed to parse Catalog from first layer: %w", err)
	}

	return catalog, nil
}

// InspectArtifact reads an OCI artifact and outputs formatted JSON content
func InspectArtifact(ociRef string) error {
	// Use ReadArtifact to get the parsed Catalog data
	catalog, err := ReadArtifact(ociRef)
	if err != nil {
		return fmt.Errorf("failed to read artifact: %w", err)
	}

	// Format and output the JSON data
	prettyJSON, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format JSON: %w", err)
	}

	fmt.Printf("%s\n", string(prettyJSON))
	return nil
}
