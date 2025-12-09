# SQLite-Vec MCP Server

A Model Context Protocol (MCP) server for storing and searching vector embeddings using SQLite with the sqlite-vec extension. This server exposes vector database operations as MCP tools that can be used by AI assistants and other MCP clients.

## Features

- **MCP stdio Protocol**: Communicates via stdin/stdout following the MCP specification
- **Collection Management**: Organize vectors into named collections
- **Vector Storage**: Store embeddings with custom metadata
- **Similarity Search**: Search within specific collections, across all collections, or exclude specific collections
- **Docker-based**: Run as a containerized MCP server
- **6 MCP Tools**: Complete vector database operations exposed as tools

## Quick Start

### Build and Run with Docker

```bash
# Build the Docker image
docker build -t jimclark106/vector-db:latest .

# Or use the Makefile
make build

# Run the MCP server (stdio mode)
docker run --rm -i \
  -v $(pwd)/data:/data \
  -e DB_PATH=/data/vectors.db \
  -e VECTOR_DIMENSION=1536 \
  jimclark106/vector-db:latest
```

### Configuration

Set the following environment variables:

- `VECTOR_DIMENSION`: Dimension of your embeddings (default: 1536 for OpenAI ada-002)
- `DB_PATH`: SQLite database file path (default: /data/vectors.db)
- `VEC_EXT_PATH`: Path to sqlite-vec extension (default: /usr/local/lib/sqlite/vec0)

## MCP Tools

The server exposes the following MCP tools:

### 1. list_collections

List all vector collections in the database.

**Parameters:** None

**Example:**
```json
{
  "name": "list_collections",
  "arguments": {}
}
```

**Response:**
```json
[
  {
    "id": 1,
    "name": "code_embeddings",
    "created_at": "2025-01-08 10:30:00"
  }
]
```

### 2. create_collection

Create a new vector collection.

**Parameters:**
- `name` (string, required): Name of the collection to create

**Example:**
```json
{
  "name": "create_collection",
  "arguments": {
    "name": "my_collection"
  }
}
```

### 3. delete_collection

Delete a collection and all its vectors (cascade delete).

**Parameters:**
- `name` (string, required): Name of the collection to delete

**Example:**
```json
{
  "name": "delete_collection",
  "arguments": {
    "name": "old_collection"
  }
}
```

### 4. add_vector

Add a vector to a collection (creates collection if it doesn't exist).

**Parameters:**
- `collection_name` (string, required): Name of the collection
- `vector` (array of numbers, required): Vector embedding (must match configured dimension)
- `metadata` (object, optional): Optional metadata as JSON object

**Example:**
```json
{
  "name": "add_vector",
  "arguments": {
    "collection_name": "code_embeddings",
    "vector": [0.1, 0.2, 0.3, ..., 0.5],
    "metadata": {
      "file": "main.go",
      "line": 42,
      "function": "main"
    }
  }
}
```

**Response:**
```json
{
  "id": 123,
  "collection_id": 1
}
```

### 5. delete_vector

Delete a vector by its ID.

**Parameters:**
- `id` (integer, required): ID of the vector to delete

**Example:**
```json
{
  "name": "delete_vector",
  "arguments": {
    "id": 123
  }
}
```

### 6. search

Search for similar vectors using cosine distance.

**Parameters:**
- `vector` (array of numbers, required): Query vector (must match configured dimension)
- `limit` (integer, optional): Maximum number of results to return (default: 10)
- `collection_name` (string, optional): Search only within this collection
- `exclude_collections` (array of strings, optional): Search all collections except these

**Example - Search specific collection:**
```json
{
  "name": "search",
  "arguments": {
    "vector": [0.1, 0.2, 0.3, ..., 0.5],
    "collection_name": "code_embeddings",
    "limit": 10
  }
}
```

**Example - Search all collections:**
```json
{
  "name": "search",
  "arguments": {
    "vector": [0.1, 0.2, 0.3, ..., 0.5],
    "limit": 10
  }
}
```

**Example - Search with exclusions:**
```json
{
  "name": "search",
  "arguments": {
    "vector": [0.1, 0.2, 0.3, ..., 0.5],
    "exclude_collections": ["test_data", "archived"],
    "limit": 10
  }
}
```

**Response:**
```json
[
  {
    "vector_id": 123,
    "collection_name": "code_embeddings",
    "metadata": {
      "file": "main.go",
      "line": 42
    },
    "distance": 0.234
  },
  {
    "vector_id": 456,
    "collection_name": "documentation",
    "metadata": {
      "doc": "api.md",
      "section": "authentication"
    },
    "distance": 0.456
  }
]
```

Distance is cosine distance (lower = more similar).

## Using with MCP Clients

### Claude Desktop

Add to your Claude Desktop configuration (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "sqlite-vec": {
      "command": "docker",
      "args": [
        "run",
        "--rm",
        "-i",
        "-v", "/path/to/data:/data",
        "-e", "DB_PATH=/data/vectors.db",
        "-e", "VECTOR_DIMENSION=1536",
        "jimclark106/vector-db:latest"
      ]
    }
  }
}
```

### Docker MCP Gateway

Add to your MCP Gateway catalog:

```yaml
- name: sqlite-vec
  description: Vector database for semantic search
  image: jimclark106/vector-db:latest
  env:
    VECTOR_DIMENSION: "1536"
    DB_PATH: "/data/vectors.db"
  volumes:
    - ./data:/data
```

### Direct Usage (for testing)

```bash
# Start the server
docker run --rm -i \
  -v $(pwd)/data:/data \
  -e VECTOR_DIMENSION=1536 \
  jimclark106/vector-db:latest

# The server will communicate via stdio using the MCP protocol
# Send MCP requests as JSON-RPC messages
```

## Data Persistence

The SQLite database is stored in the `./data` directory (mounted as a volume). This ensures your vectors persist across container restarts.

To backup your data:
```bash
# Copy the database file
cp ./data/vectors.db ./backup/vectors-$(date +%Y%m%d).db
```

To reset/clear all data:
```bash
# Stop the container and remove the database
docker stop sqlite-vec-mcp
rm -f ./data/vectors.db
```

## Architecture

- **Protocol**: Model Context Protocol (MCP) over stdio
- **Database**: SQLite with sqlite-vec extension
- **SDK**: Official golang MCP SDK (`github.com/modelcontextprotocol/go-sdk`)
- **Vector Storage**: Vectors stored as BLOBs using `vec_f32()`
- **Search**: Cosine distance similarity using `vec_distance_cosine()`
- **Metadata**: Flexible JSON storage per vector

## Development

### Using Make

The project includes a Makefile for common tasks:

```bash
# Show all available commands
make help

# Build Docker image (multi-platform)
make build

# Build and push to Docker registry
make build-push

# Run the server locally in Docker
make run

# Build binary locally
make build-local

# Run tests
make test

# Run linters
make lint

# Format code
make fmt

# Download dependencies
make deps

# Clean up artifacts
make clean
```

### Manual Build

```bash
# Install dependencies
go mod download

# Build the binary
CGO_ENABLED=1 go build -tags "sqlite_extensions" -o sqlite-vec-mcp main.go

# Run locally (for testing)
./sqlite-vec-mcp
```

### Running Tests

```bash
make test
# or
go test ./...
```

### Linting

```bash
make lint
# or
golangci-lint run ./...
```

### Building for Docker Registry

```bash
# Build for multiple platforms and push to jimclark106/vector-db
make build-push

# Or with a specific tag
TAG=v1.0.0 make build-push
```

## Performance Notes

- SQLite is single-writer, so concurrent writes are serialized
- Suitable for moderate workloads (thousands to hundreds of thousands of vectors)
- For larger scale (millions of vectors), consider Qdrant, Weaviate, or Pinecone
- Search performance is linear O(n) - no index structure yet in sqlite-vec
- MCP stdio protocol is efficient for single-client scenarios

## Platform Support

**Supported Platform**: `linux/amd64` only

The Docker image is currently built for `linux/amd64` (x86_64) only. The sqlite-vec prebuilt binaries for ARM64 are 32-bit and incompatible with 64-bit ARM systems.

For development on Apple Silicon (M1/M2/M3) Macs, you can:
- Deploy to a linux/amd64 environment (cloud, CI/CD, production servers)
- Build sqlite-vec from source for native ARM64 (advanced, not covered here)
- Use x86_64 emulation (may have compatibility issues)

## Troubleshooting

### Server won't start

Check logs:
```bash
docker logs sqlite-vec-mcp
```

Verify sqlite-vec extension is loaded:
```bash
docker run --rm -it --platform linux/amd64 jimclark106/vector-db:latest sqlite3 /tmp/test.db "SELECT vec_version();"
```

### "unsupported relocation type" or "Exec format error"

This error indicates an architecture mismatch. Ensure you're running on a linux/amd64 system or using proper platform emulation:
```bash
docker run --platform linux/amd64 ...
```

### Dimension mismatch errors

Ensure all vectors have the same dimension as specified in `VECTOR_DIMENSION` environment variable. The dimension must be consistent across all operations.

### MCP connection issues

- Ensure the server is running in stdio mode (not HTTP)
- Check that stdin/stdout are not buffered or redirected
- Verify the MCP client is sending valid JSON-RPC requests

## License

This example is provided as-is for educational and development purposes.
