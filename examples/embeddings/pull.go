package main

import (
	"context"
	"fmt"
	"os"

	"github.com/docker/mcp-gateway/pkg/gateway/embeddings"
)

func main() {
	fmt.Println("Pulling embeddings from OCI registry...")

	if err := embeddings.Pull(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Successfully pulled embeddings!")
}
