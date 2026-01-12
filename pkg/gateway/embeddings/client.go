package embeddings

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/log"
)

// VectorDBClient wraps the MCP client connection to the vector DB server
type VectorDBClient struct {
	cmd           *exec.Cmd
	client        *mcp.Client
	session       *mcp.ClientSession
	containerName string
	logFunc       func(string)
	mu            sync.Mutex
}

// NewVectorDBClient creates a new MCP client and starts the vector DB container.
// The dataDir parameter specifies where the vector database will store its data.
// The dimension parameter specifies the vector dimension (default 1536 for OpenAI embeddings).
// The logFunc parameter is optional and can be used to log MCP messages.
func NewVectorDBClient(ctx context.Context, dataDir string, dimension int, logFunc func(string)) (*VectorDBClient, error) {
	// Use default dimension if not specified
	if dimension <= 0 {
		dimension = 1536
	}

	// Generate a unique container name
	containerName := fmt.Sprintf("vector-db-%d", time.Now().UnixNano())

	// Create the docker command to run the vector-db container
	cmd := exec.CommandContext(ctx,
		"docker", "run", "-i", "--rm",
		"--name", containerName,
		"--platform", "linux/amd64",
		"-v", fmt.Sprintf("%s:/data", dataDir),
		"-e", "DB_PATH=/data/vectors.db",
		"-e", fmt.Sprintf("VECTOR_DIMENSION=%d", dimension),
		"jimclark106/vector-db:latest",
	)

	client := &VectorDBClient{
		cmd:           cmd,
		containerName: containerName,
		logFunc:       logFunc,
	}

	// Create MCP client with notification handlers
	mcpClient := mcp.NewClient(
		&mcp.Implementation{
			Name:    "vector-db-client",
			Version: "1.0.0",
		},
		&mcp.ClientOptions{
			LoggingMessageHandler: func(_ context.Context, req *mcp.LoggingMessageRequest) {
				if client.logFunc != nil {
					msg := fmt.Sprintf("LOG: %s - %s", req.Params.Level, req.Params.Data)
					client.logFunc(msg)
				}
			},
		},
	)

	// Use CommandTransport which handles all the stdio plumbing
	transport := &mcp.CommandTransport{Command: cmd}

	// Connect to the MCP server (this starts the command)
	session, err := mcpClient.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	client.client = mcpClient
	client.session = session

	return client, nil
}

// IsAlive checks if the container process is still running
func (c *VectorDBClient) IsAlive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd == nil || c.cmd.Process == nil {
		return false
	}

	// On Unix, sending signal 0 checks if process exists
	err := c.cmd.Process.Signal(nil)
	return err == nil
}

// Wait waits for the container to exit and returns any error
func (c *VectorDBClient) Wait() error {
	if c.cmd == nil {
		return nil
	}
	return c.cmd.Wait()
}

// Session returns the MCP client session
func (c *VectorDBClient) Session() *mcp.ClientSession {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.session
}

// ListTools lists available tools from the MCP server
func (c *VectorDBClient) ListTools(ctx context.Context) (*mcp.ListToolsResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session == nil {
		return nil, fmt.Errorf("session not initialized")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := c.session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, fmt.Errorf("list tools request failed: %w", err)
	}

	return result, nil
}

// CallTool calls a tool on the MCP server with the given name and arguments.
// The arguments parameter accepts any type - the MCP SDK handles JSON marshaling.
func (c *VectorDBClient) CallTool(ctx context.Context, toolName string, arguments any) (*mcp.CallToolResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session == nil {
		return nil, fmt.Errorf("session not initialized")
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := c.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: arguments,
	})
	if err != nil {
		return nil, fmt.Errorf("tool call '%s' failed: %w", toolName, err)
	}

	return result, nil
}

// Close closes the MCP client session and stops the Docker container
func (c *VectorDBClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var sessionErr error
	if c.session != nil {
		sessionErr = c.session.Close()
	}

	log.Log("close the DBClient")
	// Stop the Docker container using docker stop
	// This properly signals the container to shut down
	if c.containerName != "" {
		log.Logf("Stopping container: %s", c.containerName)
		ctx := context.Background()
		stopCmd := exec.CommandContext(ctx, "docker", "stop", "-t", "2", c.containerName)
		if err := stopCmd.Run(); err != nil {
			// Container might already be stopped or removed - that's fine
			log.Logf("Container %s stop result: %v (this is expected if already stopped)", c.containerName, err)
		}
		// Clear the container name so we don't try to stop it again
		c.containerName = ""
	}

	// Wait for the docker run process to exit if it hasn't already
	// The --rm flag will automatically remove the container after it stops
	if c.cmd != nil {
		log.Log("Waiting for docker run process to exit")
		// Wait will reap the process and clean up resources
		// Ignore "wait was already called" or "no child processes" errors
		waitErr := c.cmd.Wait()
		if waitErr != nil && waitErr.Error() != "exec: Wait was already called" {
			log.Logf("Docker run process exited with: %v", waitErr)
		}
		c.cmd = nil
	}

	log.Log("DBClient closed")
	return sessionErr
}

// ==================================================
// Vector DB Tool Operations
// ==================================================

// Collection represents a vector collection
type Collection struct {
	Name string `json:"name"`
}

// SearchResult represents a single search result
type SearchResult struct {
	ID           int64          `json:"id"`
	Collection   string         `json:"collection"`
	Distance     float64        `json:"distance"`
	Metadata     map[string]any `json:"metadata"`
	VectorLength int            `json:"vector_length"`
}

// CreateCollection creates a new vector collection
func (c *VectorDBClient) CreateCollection(ctx context.Context, collectionName string) (*mcp.CallToolResult, error) {
	return c.CallTool(ctx, "create_collection", map[string]any{
		"name": collectionName,
	})
}

// DeleteCollection deletes a collection and all its vectors
func (c *VectorDBClient) DeleteCollection(ctx context.Context, collectionName string) (*mcp.CallToolResult, error) {
	return c.CallTool(ctx, "delete_collection", map[string]any{
		"name": collectionName,
	})
}

// ListCollections lists all vector collections in the database.
// Returns a slice of collection names.
func (c *VectorDBClient) ListCollections(ctx context.Context) ([]string, error) {
	result, err := c.CallTool(ctx, "list_collections", map[string]any{})
	if err != nil {
		return nil, err
	}

	if result.IsError {
		return nil, fmt.Errorf("tool returned error: %s", result.Content)
	}

	// Parse the result content
	if len(result.Content) == 0 {
		return nil, fmt.Errorf("empty response from list_collections")
	}

	// Extract text from content
	var textContent string
	for _, content := range result.Content {
		if tc, ok := content.(*mcp.TextContent); ok {
			textContent = tc.Text
			break
		}
	}

	if textContent == "" {
		return nil, fmt.Errorf("no text content in response")
	}

	// Parse the JSON response
	var collections []string
	if err := json.Unmarshal([]byte(textContent), &collections); err != nil {
		return nil, fmt.Errorf("failed to parse collections response: %w", err)
	}

	return collections, nil
}

// AddVector adds a vector to a collection (creates collection if it doesn't exist).
// The vector must be a slice of 1536 float64 numbers.
// Metadata is optional.
func (c *VectorDBClient) AddVector(ctx context.Context, collectionName string, vector []float64, metadata map[string]any) (*mcp.CallToolResult, error) {
	args := map[string]any{
		"collection_name": collectionName,
		"vector":          vector,
	}

	if metadata != nil {
		args["metadata"] = metadata
	}

	return c.CallTool(ctx, "add_vector", args)
}

// DeleteVector deletes a vector by its ID
func (c *VectorDBClient) DeleteVector(ctx context.Context, vectorID int64) (*mcp.CallToolResult, error) {
	return c.CallTool(ctx, "delete_vector", map[string]any{
		"id": vectorID,
	})
}

// SearchOptions contains options for vector search
type SearchOptions struct {
	CollectionName     string   // Search only within this collection
	ExcludeCollections []string // Collections to exclude from search
	Limit              int      // Maximum number of results (default 10)
}

// SearchArgs combines search options with the vector for the search tool call
type SearchArgs struct {
	Vector             []float64 `json:"vector"`
	CollectionName     string    `json:"collection_name,omitempty"`
	ExcludeCollections []string  `json:"exclude_collections,omitempty"`
	Limit              int       `json:"limit,omitempty"`
}

// SearchVectors searches for similar vectors using cosine distance.
// The vector must be a slice of 1536 float64 numbers.
// Returns a slice of search results.
func (c *VectorDBClient) SearchVectors(ctx context.Context, vector []float64, options *SearchOptions) ([]SearchResult, error) {
	args := SearchArgs{
		Vector: vector,
	}

	if options != nil {
		args.CollectionName = options.CollectionName
		args.ExcludeCollections = options.ExcludeCollections
		args.Limit = options.Limit
	}

	result, err := c.CallTool(ctx, "search", args)
	if err != nil {
		return nil, err
	}

	if result.IsError {
		return nil, fmt.Errorf("tool returned error: %s", result.Content)
	}

	// Parse the result content
	if len(result.Content) == 0 {
		return nil, fmt.Errorf("empty response from search")
	}

	// Extract text from content
	var textContent string
	for _, content := range result.Content {
		if tc, ok := content.(*mcp.TextContent); ok {
			textContent = tc.Text
			break
		}
	}

	if textContent == "" {
		return nil, fmt.Errorf("no text content in response")
	}

	// Parse the JSON response
	var results []SearchResult
	if err := json.Unmarshal([]byte(textContent), &results); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	return results, nil
}
