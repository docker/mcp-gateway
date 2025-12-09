package embeddings_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/docker/mcp-gateway/pkg/gateway/embeddings"
)

// Example demonstrates how to use the vector DB client
func Example() {
	ctx := context.Background()

	// Create a client which starts the vector DB container
	client, err := embeddings.NewVectorDBClient(ctx, "./data", 1536, func(msg string) {
		fmt.Println(msg)
	})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Check if container is alive
	if !client.IsAlive() {
		log.Fatal("Container is not running")
	}

	// List available tools (connection is already initialized)
	toolsResult, err := client.ListTools(ctx)
	if err != nil {
		log.Fatalf("Failed to list tools: %v", err)
	}
	fmt.Printf("Available tools: %d\n", len(toolsResult.Tools))

	// Create a collection
	_, err = client.CreateCollection(ctx, "my-collection")
	if err != nil {
		log.Fatalf("Failed to create collection: %v", err)
	}

	// List collections
	collections, err := client.ListCollections(ctx)
	if err != nil {
		log.Fatalf("Failed to list collections: %v", err)
	}
	fmt.Printf("Collections: %v\n", collections)

	// Add a vector (1536 dimensions)
	sampleVector := make([]float64, 1536)
	for i := range sampleVector {
		sampleVector[i] = 0.1
	}
	metadata := map[string]any{
		"name": "test-doc",
	}
	_, err = client.AddVector(ctx, "my-collection", sampleVector, metadata)
	if err != nil {
		log.Fatalf("Failed to add vector: %v", err)
	}

	// Search for similar vectors
	results, err := client.SearchVectors(ctx, sampleVector, &embeddings.SearchOptions{
		CollectionName: "my-collection",
		Limit:          5,
	})
	if err != nil {
		log.Fatalf("Failed to search vectors: %v", err)
	}
	fmt.Printf("Search results: %d\n", len(results))
	for _, result := range results {
		fmt.Printf("  ID: %d, Distance: %f, Collection: %s\n",
			result.ID, result.Distance, result.Collection)
	}

	// Delete a vector by ID
	if len(results) > 0 {
		_, err = client.DeleteVector(ctx, results[0].ID)
		if err != nil {
			log.Fatalf("Failed to delete vector: %v", err)
		}
	}

	// Delete a collection
	_, err = client.DeleteCollection(ctx, "my-collection")
	if err != nil {
		log.Fatalf("Failed to delete collection: %v", err)
	}
}

// Example_withTimeout demonstrates usage with context timeouts
func Example_withTimeout() {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create client with the timeout context
	client, err := embeddings.NewVectorDBClient(ctx, "./data", 1536, nil)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Perform operations (connection is already initialized)
	collections, err := client.ListCollections(ctx)
	if err != nil {
		log.Fatalf("Failed to list collections: %v", err)
	}
	fmt.Printf("Collections: %v\n", collections)
}

// Example_longRunning demonstrates waiting for container completion
func Example_longRunning() {
	ctx := context.Background()

	client, err := embeddings.NewVectorDBClient(ctx, "./data", 1536, nil)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// In a separate goroutine, wait for container to exit
	go func() {
		if err := client.Wait(); err != nil {
			log.Printf("Container exited with error: %v", err)
		} else {
			log.Println("Container exited successfully")
		}
	}()

	// Do work with the client (already initialized)...
	// For example: client.ListCollections(ctx), client.SearchVectors(ctx, ...), etc.

	// When done, close the client (which stops the container)
	if err := client.Close(); err != nil {
		log.Printf("Failed to close client: %v", err)
	}
}
