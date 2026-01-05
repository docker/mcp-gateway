# Embeddings OCI Examples

This directory contains examples for pulling and pushing vector database embeddings to/from OCI registries.

## Pull Example

Downloads the embeddings OCI artifact and installs the vector.db directory to `~/.docker/mcp/`.

### Usage

```bash
# From repository root
go run ./examples/embeddings/pull/main.go
```

The Pull function will:
1. Download the image from `jimclark106/embeddings:latest`
2. Extract all layers to a temporary directory
3. Verify that `vectors.db` file exists
4. Copy `vectors.db` to `~/.docker/mcp/` (skips if already exists)
5. Clean up temporary files

## Push Example

Creates an OCI artifact from a local vector.db directory and pushes it to a registry.

### Usage

```bash
# From repository root
go run ./examples/embeddings/push/main.go <vector-db-path> <oci-ref>
```

### Example

```bash
# Push the local vectors.db to your own registry
go run ./examples/embeddings/push/main.go ~/.docker/mcp/vectors.db jimclark106/embeddings:v1.0
```

The Push function will:
1. Verify the source directory exists
2. Create a tar archive from the entire directory tree (always naming the root as `vectors.db` in the archive)
3. Create an OCI image layer from the tar
4. Push the image to the specified OCI reference

Note: Regardless of your local directory name, the OCI artifact will always contain `vectors.db` at the root for consistency.

## Authentication

Both examples use the Docker credential helper for authentication. Make sure you're logged in to the registry:

```bash
docker login
```

## Notes

- Pull is idempotent - it won't overwrite existing `vectors.db` files
- Push requires write access to the specified OCI registry
- Push always stores the directory as `vectors.db` in the OCI artifact for consistency
- File permissions and symlinks are preserved during push/pull operations
