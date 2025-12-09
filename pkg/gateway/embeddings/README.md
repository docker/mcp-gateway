# Embeddings Package

This package provides a Go client for the vector-db MCP server running in a Docker container. It's a translation of the Clojure namespace from `test/embeddings/clj/vector_db_process.clj`.

## Overview

The embeddings package provides:

1. **Container Management** - Automatically starts and manages the vector-db Docker container
2. **MCP Client** - Connects to the vector database via the official Go MCP SDK
3. **Vector Operations** - High-level functions for working with vector collections and embeddings

## Features

- Start/stop vector DB container automatically
- MCP protocol communication via stdio
- Collection management (create, delete, list)
- Vector operations (add, delete, search)
- Cosine distance similarity search
- Metadata support for vectors
- Full type safety with Go

## Usage

### Basic Example

```go
package main

import (
    "context"
    "log"

    "github.com/docker/mcp-gateway/pkg/gateway/embeddings"
)

func main() {
    ctx := context.Background()

    // Create client (this starts the container)
    // The dimension parameter specifies the vector dimension (1536 for OpenAI embeddings)
    client, err := embeddings.NewVectorDBClient(ctx, "./data", 1536, nil)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Create a collection
    _, err = client.CreateCollection(ctx, "my-vectors")
    if err != nil {
        log.Fatal(err)
    }

    // Add a vector (1536 dimensions for OpenAI embeddings)
    vector := make([]float64, 1536)
    for i := range vector {
        vector[i] = 0.1 // Your actual embedding values here
    }

    metadata := map[string]interface{}{
        "text": "This is my document",
        "source": "example.txt",
    }

    _, err = client.AddVector(ctx, "my-vectors", vector, metadata)
    if err != nil {
        log.Fatal(err)
    }

    // Search for similar vectors
    results, err := client.SearchVectors(ctx, vector, &embeddings.SearchOptions{
        CollectionName: "my-vectors",
        Limit: 10,
    })
    if err != nil {
        log.Fatal(err)
    }

    for _, result := range results {
        log.Printf("Match: ID=%d, Distance=%f, Metadata=%v\n",
            result.ID, result.Distance, result.Metadata)
    }
}
```

### Collection Operations

```go
// List all collections
collections, err := client.ListCollections(ctx)

// Delete a collection
_, err = client.DeleteCollection(ctx, "my-vectors")
```

### Vector Operations

```go
// Add vector with metadata
metadata := map[string]interface{}{
    "title": "My Document",
    "category": "research",
}
result, err := client.AddVector(ctx, "collection-name", vector, metadata)

// Search with options
results, err := client.SearchVectors(ctx, queryVector, &embeddings.SearchOptions{
    CollectionName: "my-vectors",  // Search in specific collection
    Limit: 20,                       // Return top 20 results
})

// Search across multiple collections (exclude some)
results, err := client.SearchVectors(ctx, queryVector, &embeddings.SearchOptions{
    ExcludeCollections: []string{"test-data"},
    Limit: 10,
})

// Delete a vector by ID
_, err = client.DeleteVector(ctx, vectorID)
```

### Advanced: Direct Tool Access

```go
// List available MCP tools
tools, err := client.ListTools(ctx)

// Call any tool directly
result, err := client.CallTool(ctx, "tool-name", map[string]interface{}{
    "param1": "value1",
    "param2": 123,
})
```

## Key Differences from Clojure Version

1. **Simplified API**: Uses `CommandTransport` instead of manual pipe management
2. **Automatic Initialization**: MCP initialization happens during `Connect()`
3. **Strong Typing**: Uses Go structs instead of dynamic maps
4. **Error Handling**: Explicit error returns instead of Clojure's exception model
5. **Concurrency**: Uses `sync.Mutex` instead of Clojure's core.async channels

## Vector Database Details

- **Image**: `jimclark106/vector-db:latest`
- **Vector Dimension**: Configurable via the `dimension` parameter (default: 1536 for OpenAI embeddings)
  - Pass `0` or negative value to use default (1536)
  - Common dimensions: 1536 (OpenAI), 768 (sentence transformers), 384 (MiniLM)
- **Database**: SQLite with vec extension
- **Transport**: stdio (JSON-RPC over stdin/stdout)

## Requirements

- Docker daemon running
- Go 1.24+
- The official MCP Go SDK (`github.com/modelcontextprotocol/go-sdk/mcp`)

## Architecture

```
┌─────────────────┐
│  Your Go App    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ VectorDBClient  │  (this package)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  MCP Client     │  (go-sdk/mcp)
└────────┬────────┘
         │ stdio
         ▼
┌─────────────────┐
│ Docker Container│  (jimclark106/vector-db)
└─────────────────┘
```

## See Also

- Original Clojure implementation: `test/embeddings/clj/vector_db_process.clj`
- MCP Go SDK: https://github.com/modelcontextprotocol/go-sdk
- Example usage: `example_test.go`
