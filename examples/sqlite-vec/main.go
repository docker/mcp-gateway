package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/mattn/go-sqlite3"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// Default vector dimension (e.g., OpenAI ada-002 = 1536)
	defaultDimension = 1536
)

func init() {
	// Register sqlite3 driver with extension loading enabled
	sql.Register("sqlite3_with_extensions",
		&sqlite3.SQLiteDriver{
			ConnectHook: func(conn *sqlite3.SQLiteConn) error {
				extPath := os.Getenv("VEC_EXT_PATH")
				if extPath == "" {
					extPath = "/usr/local/lib/sqlite/vec0"
				}
				return conn.LoadExtension(extPath, "sqlite3_vec_init")
			},
		})
}

type VectorServer struct {
	db  *sql.DB
	dim int
}

// API Models
type Collection struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type Vector struct {
	ID           int             `json:"id"`
	CollectionID int             `json:"collection_id"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	CreatedAt    string          `json:"created_at"`
}

type AddVectorRequest struct {
	CollectionName string          `json:"collection_name"`
	Vector         []float32       `json:"vector"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
}

type SearchRequest struct {
	Vector             []float32 `json:"vector"`
	Limit              int       `json:"limit,omitempty"`
	CollectionName     string    `json:"collection_name,omitempty"`     // Deprecated: use collection_names instead
	CollectionNames    []string  `json:"collection_names,omitempty"`    // Search in specific collections (empty = search all)
	ExcludeCollections []string  `json:"exclude_collections,omitempty"` // Collections to exclude from search
}

type SearchResult struct {
	VectorID       int             `json:"vector_id"`
	CollectionName string          `json:"collection_name"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	Distance       float64         `json:"distance"`
}

func main() {
	dimension := defaultDimension
	if dim := os.Getenv("VECTOR_DIMENSION"); dim != "" {
		if d, err := strconv.Atoi(dim); err == nil {
			dimension = d
		}
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "/data/vectors.db"
	}

	// Setup signal handling
	ctx, done := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer done()

	// Direct logs to stderr (stdout is used for MCP protocol)
	log.SetOutput(os.Stderr)

	// Open database with custom driver that has vec extension loaded
	db, err := sql.Open("sqlite3_with_extensions", dbPath)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		log.Fatal("Failed to enable foreign keys:", err)
	}

	vs := &VectorServer{db: db, dim: dimension}

	// Initialize schema if needed
	if err := vs.initSchema(); err != nil {
		log.Fatal("Failed to initialize schema:", err)
	}

	// Create MCP server
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "sqlite-vec",
			Version: "1.0.0",
		},
		&mcp.ServerOptions{
			HasTools: true,
		},
	)

	// Register all tools
	vs.registerTools(server)

	// Create transport with logging
	transport := &mcp.LoggingTransport{
		Transport: &mcp.StdioTransport{},
		Writer:    os.Stderr,
	}

	// Run server
	errCh := make(chan error, 1)
	go func() {
		log.Printf("[INFO] MCP sqlite-vec server starting (dimension=%d)", dimension)
		defer log.Print("[INFO] MCP sqlite-vec server stopped")

		if err := server.Run(ctx, transport); err != nil && !errors.Is(err, mcp.ErrConnectionClosed) {
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	// Wait for error or context cancellation
	select {
	case err := <-errCh:
		log.Printf("[ERROR] Server failed: %s", err)
		os.Exit(1)
	case <-ctx.Done():
		log.Print("[INFO] Shutdown signal received")
	}
}

func (vs *VectorServer) registerTools(server *mcp.Server) {
	// Tool 1: list_collections
	server.AddTool(
		&mcp.Tool{
			Name:        "list_collections",
			Description: "List all vector collections in the database",
			InputSchema: &jsonschema.Schema{
				Type:       "object",
				Properties: map[string]*jsonschema.Schema{},
			},
			OutputSchema: &jsonschema.Schema{
				Type: "array",
				Items: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"id": {
							Type:        "integer",
							Description: "Unique identifier for the collection",
						},
						"name": {
							Type:        "string",
							Description: "Name of the collection",
						},
						"created_at": {
							Type:        "string",
							Description: "Timestamp when the collection was created",
						},
					},
					Required: []string{"id", "name", "created_at"},
				},
			},
		},
		vs.handleListCollections,
	)

	// Tool 2: create_collection
	server.AddTool(
		&mcp.Tool{
			Name:        "create_collection",
			Description: "Create a new vector collection",
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"name": {
						Type:        "string",
						Description: "Name of the collection to create",
					},
				},
				Required: []string{"name"},
			},
		},
		vs.handleCreateCollection,
	)

	// Tool 3: delete_collection
	server.AddTool(
		&mcp.Tool{
			Name:        "delete_collection",
			Description: "Delete a collection and all its vectors (cascade delete)",
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"name": {
						Type:        "string",
						Description: "Name of the collection to delete",
					},
				},
				Required: []string{"name"},
			},
		},
		vs.handleDeleteCollection,
	)

	// Tool 4: add_vector
	server.AddTool(
		&mcp.Tool{
			Name:        "add_vector",
			Description: "Add a vector to a collection (creates collection if it doesn't exist)",
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"collection_name": {
						Type:        "string",
						Description: "Name of the collection",
					},
					"vector": {
						Type:        "array",
						Description: fmt.Sprintf("Vector embedding (must be %d dimensions)", vs.dim),
						Items: &jsonschema.Schema{
							Type: "number",
						},
					},
					"metadata": {
						Type:        "object",
						Description: "Optional metadata as JSON object",
					},
				},
				Required: []string{"collection_name", "vector"},
			},
		},
		vs.handleAddVector,
	)

	// Tool 5: delete_vector
	server.AddTool(
		&mcp.Tool{
			Name:        "delete_vector",
			Description: "Delete a vector by its ID",
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"id": {
						Type:        "integer",
						Description: "ID of the vector to delete",
					},
				},
				Required: []string{"id"},
			},
		},
		vs.handleDeleteVector,
	)

	// Tool 6: search
	server.AddTool(
		&mcp.Tool{
			Name:        "search",
			Description: "Search for similar vectors using cosine distance",
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"vector": {
						Type:        "array",
						Description: fmt.Sprintf("Query vector (must be %d dimensions)", vs.dim),
						Items: &jsonschema.Schema{
							Type: "number",
						},
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of results to return (default: 10)",
					},
					"collection_name": {
						Type:        "string",
						Description: "Optional: search only within this single collection (deprecated, use collection_names instead)",
					},
					"collection_names": {
						Type:        "array",
						Description: "Optional: search only within these collections. If empty, searches all collections.",
						Items: &jsonschema.Schema{
							Type: "string",
						},
					},
					"exclude_collections": {
						Type:        "array",
						Description: "Optional: search all collections except these",
						Items: &jsonschema.Schema{
							Type: "string",
						},
					},
				},
				Required: []string{"vector"},
			},
		},
		vs.handleSearch,
	)
}

func (vs *VectorServer) initSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS collections (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS vectors (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			collection_id INTEGER NOT NULL,
			vector_blob BLOB NOT NULL,
			metadata TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (collection_id) REFERENCES collections(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_vectors_collection ON vectors(collection_id);
		CREATE INDEX IF NOT EXISTS idx_collections_name ON collections(name);
	`
	_, err := vs.db.Exec(schema)
	return err
}

// Tool handlers

func (vs *VectorServer) handleListCollections(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	rows, err := vs.db.Query("SELECT id, name, created_at FROM collections ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("failed to query collections: %w", err)
	}
	defer rows.Close()

	var collections []Collection
	for rows.Next() {
		var c Collection
		if err := rows.Scan(&c.ID, &c.Name, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan collection: %w", err)
		}
		collections = append(collections, c)
	}

	resultJSON, err := json.MarshalIndent(collections, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal collections: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(resultJSON)},
		},
	}, nil
}

func (vs *VectorServer) handleCreateCollection(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var params struct {
		Name string `json:"name"`
	}

	if err := parseArguments(req, &params); err != nil {
		return nil, err
	}

	if params.Name == "" {
		return nil, fmt.Errorf("collection name is required")
	}

	result, err := vs.db.Exec("INSERT INTO collections (name) VALUES (?)", params.Name)
	if err != nil {
		return nil, fmt.Errorf("collection already exists or database error: %w", err)
	}

	id, _ := result.LastInsertId()
	response := map[string]any{"id": id, "name": params.Name}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(resultJSON)},
		},
	}, nil
}

func (vs *VectorServer) handleDeleteCollection(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var params struct {
		Name string `json:"name"`
	}

	if err := parseArguments(req, &params); err != nil {
		return nil, err
	}

	result, err := vs.db.Exec("DELETE FROM collections WHERE name = ?", params.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to delete collection: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, fmt.Errorf("collection not found: %s", params.Name)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Collection '%s' deleted successfully", params.Name)},
		},
	}, nil
}

func (vs *VectorServer) handleAddVector(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var params AddVectorRequest

	if err := parseArguments(req, &params); err != nil {
		return nil, err
	}

	if len(params.Vector) != vs.dim {
		return nil, fmt.Errorf("vector dimension mismatch: expected %d, got %d", vs.dim, len(params.Vector))
	}

	// Get or create collection
	var collectionID int
	err := vs.db.QueryRow("SELECT id FROM collections WHERE name = ?", params.CollectionName).Scan(&collectionID)
	if err == sql.ErrNoRows {
		result, err := vs.db.Exec("INSERT INTO collections (name) VALUES (?)", params.CollectionName)
		if err != nil {
			return nil, fmt.Errorf("failed to create collection: %w", err)
		}
		id, _ := result.LastInsertId()
		collectionID = int(id)
	} else if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	// Convert float32 slice to JSON array for vec_f32
	vectorJSON, _ := json.Marshal(params.Vector)

	metadata := params.Metadata
	if metadata == nil {
		metadata = json.RawMessage("{}")
	}

	result, err := vs.db.Exec(
		"INSERT INTO vectors (collection_id, vector_blob, metadata) VALUES (?, vec_f32(?), ?)",
		collectionID, string(vectorJSON), string(metadata),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert vector: %w", err)
	}

	id, _ := result.LastInsertId()
	response := map[string]any{
		"id":            id,
		"collection_id": collectionID,
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(resultJSON)},
		},
	}, nil
}

func (vs *VectorServer) handleDeleteVector(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var params struct {
		ID int `json:"id"`
	}

	if err := parseArguments(req, &params); err != nil {
		return nil, err
	}

	result, err := vs.db.Exec("DELETE FROM vectors WHERE id = ?", params.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete vector: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, fmt.Errorf("vector not found: %d", params.ID)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Vector %d deleted successfully", params.ID)},
		},
	}, nil
}

func (vs *VectorServer) handleSearch(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var params SearchRequest

	if err := parseArguments(req, &params); err != nil {
		return nil, err
	}

	if len(params.Vector) != vs.dim {
		return nil, fmt.Errorf("vector dimension mismatch: expected %d, got %d", vs.dim, len(params.Vector))
	}

	if params.Limit == 0 {
		params.Limit = 10
	}

	// Support backward compatibility: if collection_name is set, add it to collection_names
	if params.CollectionName != "" && len(params.CollectionNames) == 0 {
		params.CollectionNames = []string{params.CollectionName}
	}

	vectorJSON, _ := json.Marshal(params.Vector)

	var rows *sql.Rows
	var err error

	if len(params.CollectionNames) > 0 {
		// Search within specific collections using IN clause
		placeholders := make([]string, len(params.CollectionNames))
		args := []any{string(vectorJSON)}
		for i, name := range params.CollectionNames {
			placeholders[i] = "?"
			args = append(args, name)
		}
		args = append(args, params.Limit)

		query := fmt.Sprintf(`
			SELECT v.id, c.name, v.metadata, vec_distance_cosine(v.vector_blob, vec_f32(?)) as distance
			FROM vectors v
			JOIN collections c ON v.collection_id = c.id
			WHERE c.name IN (%s)
			ORDER BY distance
			LIMIT ?
		`, strings.Join(placeholders, ","))

		rows, err = vs.db.Query(query, args...)
	} else if len(params.ExcludeCollections) > 0 {
		// Search across all collections EXCEPT the excluded ones
		placeholders := make([]string, len(params.ExcludeCollections))
		args := []any{string(vectorJSON)}
		for i, name := range params.ExcludeCollections {
			placeholders[i] = "?"
			args = append(args, name)
		}
		args = append(args, params.Limit)

		query := fmt.Sprintf(`
			SELECT v.id, c.name, v.metadata, vec_distance_cosine(v.vector_blob, vec_f32(?)) as distance
			FROM vectors v
			JOIN collections c ON v.collection_id = c.id
			WHERE c.name NOT IN (%s)
			ORDER BY distance
			LIMIT ?
		`, strings.Join(placeholders, ","))

		rows, err = vs.db.Query(query, args...)
	} else {
		// Search across all collections
		rows, err = vs.db.Query(`
			SELECT v.id, c.name, v.metadata, vec_distance_cosine(v.vector_blob, vec_f32(?)) as distance
			FROM vectors v
			JOIN collections c ON v.collection_id = c.id
			ORDER BY distance
			LIMIT ?
		`, string(vectorJSON), params.Limit)
	}

	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var metadata sql.NullString
		if err := rows.Scan(&r.VectorID, &r.CollectionName, &metadata, &r.Distance); err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		if metadata.Valid && metadata.String != "" {
			r.Metadata = json.RawMessage(metadata.String)
		}
		results = append(results, r)
	}

	resultJSON, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal results: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(resultJSON)},
		},
	}, nil
}

// Helper function to parse arguments from CallToolRequest
func parseArguments(req *mcp.CallToolRequest, params any) error {
	if req.Params.Arguments == nil {
		return fmt.Errorf("missing arguments")
	}

	paramsBytes, err := json.Marshal(req.Params.Arguments)
	if err != nil {
		return fmt.Errorf("failed to marshal arguments: %w", err)
	}

	if err := json.Unmarshal(paramsBytes, params); err != nil {
		return fmt.Errorf("failed to parse arguments: %w", err)
	}

	return nil
}
