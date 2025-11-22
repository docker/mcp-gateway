package main

import (
	"context"
	"fmt"
	"os"

	"github.com/docker/mcp-gateway/pkg/gateway/embeddings"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <vector-db-path> <oci-ref>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s ~/.docker/mcp/vectors.db jimclark106/embeddings:v1.0\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nNote: The directory will be stored as 'vectors.db' in the OCI artifact.\n")
		os.Exit(1)
	}

	vectorDBPath := os.Args[1]
	ociRef := os.Args[2]

	fmt.Printf("Pushing vector database from %s to %s...\n", vectorDBPath, ociRef)

	if err := embeddings.Push(context.Background(), vectorDBPath, ociRef); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully pushed to %s!\n", ociRef)
}
