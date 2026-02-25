package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/gateway/embeddings"
	"github.com/docker/mcp-gateway/pkg/log"
)

// ServerMatch represents a search result
type ServerMatch struct {
	Name   string
	Server catalog.Server
	Score  int
}

// tokenize splits text into tokens (lowercase alphanumeric words)
// Example: "github-api" → ["github", "api"]
func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	start := -1

	for i, r := range text {
		isAlnum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlnum {
			if start == -1 {
				start = i
			}
		} else {
			if start != -1 {
				tokens = append(tokens, text[start:i])
				start = -1
			}
		}
	}

	if start != -1 {
		tokens = append(tokens, text[start:])
	}

	return tokens
}

// bm25Doc represents a document (server) in the BM25 index
type bm25Doc struct {
	serverName string
	server     catalog.Server
	termFreq   map[string]int // tf[term] = count in this doc
	docLen     int            // total number of tokens
}

// bm25Index contains the BM25 index for all servers
type bm25Index struct {
	docs      []bm25Doc      // one per server
	docFreq   map[string]int // df[term] = # docs containing that term
	avgDocLen float64        // average document length
	N         int            // total number of documents
}

// buildBM25Index constructs a BM25 index from the configuration
// Field weighting (BM25F approximation via token repetition):
// - Server name: × 4
// - Title: × 3
// - Tags, Category: × 3
// - Tool names: × 2
// - Description, Tool descriptions: × 1
func buildBM25Index(configuration Configuration) *bm25Index {
	startTime := time.Now()
	log.Logf("[BM25] Building index from configuration with %d servers", len(configuration.servers))

	index := &bm25Index{
		docs:    make([]bm25Doc, 0, len(configuration.servers)),
		docFreq: make(map[string]int),
	}

	totalLen := 0

	for serverName, server := range configuration.servers {
		log.Logf("[BM25]   Indexing server: %s (title: %q, description length: %d, tools: %d)",
			serverName, server.Title, len(server.Description), len(server.Tools))

		doc := bm25Doc{
			serverName: serverName,
			server:     server,
			termFreq:   make(map[string]int),
		}

		// Helper to add tokens with repetition for field weighting
		addTokens := func(text string, weight int) {
			tokens := tokenize(text)
			for _, token := range tokens {
				for range weight {
					doc.termFreq[token]++
					doc.docLen++
				}
			}
		}

		// Server name (weight: 4)
		addTokens(serverName, 4)

		// Title (weight: 3)
		if server.Title != "" {
			addTokens(server.Title, 3)
		}

		// Description (weight: 1)
		if server.Description != "" {
			addTokens(server.Description, 1)
		}

		// Tools
		for _, tool := range server.Tools {
			// Tool name (weight: 2)
			addTokens(tool.Name, 2)
			// Tool description (weight: 1)
			if tool.Description != "" {
				addTokens(tool.Description, 1)
			}
		}

		// Update document frequency for unique terms
		seenTerms := make(map[string]bool)
		for term := range doc.termFreq {
			if !seenTerms[term] {
				index.docFreq[term]++
				seenTerms[term] = true
			}
		}

		totalLen += doc.docLen
		index.docs = append(index.docs, doc)
	}

	index.N = len(index.docs)
	if index.N > 0 {
		index.avgDocLen = float64(totalLen) / float64(index.N)
	}

	buildDuration := time.Since(startTime)
	log.Logf("[BM25] Index built: %d documents, %.1f avg doc length, %d unique terms (took %v)",
		index.N, index.avgDocLen, len(index.docFreq), buildDuration)

	return index
}

// score computes the BM25 score for a query against a document
// Uses Okapi BM25 with k1=1.5, b=0.75 (standard defaults)
// Formula:
//
//	IDF(t) = ln((N - df(t) + 0.5) / (df(t) + 0.5) + 1)
//	TF_norm = tf * (k1+1) / (tf + k1*(1 - b + b*docLen/avgDocLen))
//	score = Σ IDF(t) * TF_norm(t)
func (idx *bm25Index) score(doc *bm25Doc, queryTokens []string) float64 {
	const k1 = 1.5
	const b = 0.75

	score := 0.0
	seen := make(map[string]bool)

	for _, token := range queryTokens {
		// Skip already-processed tokens (avoid double-counting)
		if seen[token] {
			continue
		}
		seen[token] = true

		// Get document frequency for this term
		df := idx.docFreq[token]
		if df == 0 {
			continue // term not in any document
		}

		// Compute IDF
		idf := math.Log((float64(idx.N-df)+0.5)/(float64(df)+0.5) + 1.0)

		// Get term frequency in this document
		tf := float64(doc.termFreq[token])

		// Compute normalized TF
		docLenNorm := 1.0 - b + b*float64(doc.docLen)/idx.avgDocLen
		tfNorm := tf * (k1 + 1.0) / (tf + k1*docLenNorm)

		score += idf * tfNorm
	}

	return score
}

// bm25Strategy creates a BM25-based search handler for mcp-find
// Builds the index on every query to include dynamically activated servers
func bm25Strategy(g *Gateway) mcp.ToolHandler {
	return func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		queryStartTime := time.Now()
		log.Log("[BM25] Handler invoked - starting search")

		// Build index fresh on each query to include any newly activated servers
		// This is necessary because profiles can be activated after initial gateway setup
		// Acquire read lock to safely access configuration
		g.configurationMu.Lock()
		currentConfig := g.configuration
		g.configurationMu.Unlock()

		log.Logf("[BM25] Configuration has %d servers total", len(currentConfig.servers))

		index := buildBM25Index(currentConfig)

		// Parse parameters
		var params struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}

		if req.Params.Arguments == nil {
			return nil, fmt.Errorf("missing arguments")
		}

		paramsBytes, err := json.Marshal(req.Params.Arguments)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal arguments: %w", err)
		}

		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return nil, fmt.Errorf("failed to parse arguments: %w", err)
		}

		if params.Query == "" {
			return nil, fmt.Errorf("query parameter is required")
		}

		if params.Limit <= 0 {
			params.Limit = 10
		}

		log.Logf("[BM25] Query received: %q (limit: %d)", params.Query, params.Limit)

		// Tokenize query
		queryTokens := tokenize(params.Query)
		log.Logf("[BM25] Query tokens: %v", queryTokens)
		if len(queryTokens) == 0 {
			// Empty query after tokenization
			response := map[string]any{
				"prompt":        params.Query,
				"total_matches": 0,
				"servers":       []map[string]any{},
			}
			responseBytes, _ := json.Marshal(response)
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: string(responseBytes)}},
			}, nil
		}

		// Score all documents
		scoringStartTime := time.Now()
		type scoredDoc struct {
			doc   *bm25Doc
			score float64
		}
		var scored []scoredDoc

		for i := range index.docs {
			doc := &index.docs[i]
			s := index.score(doc, queryTokens)
			if s > 0 {
				scored = append(scored, scoredDoc{doc: doc, score: s})
			}
		}

		scoringDuration := time.Since(scoringStartTime)
		log.Logf("[BM25] Scored %d documents, %d with score > 0 (took %v)", len(index.docs), len(scored), scoringDuration)

		// Sort by score descending
		for i := range len(scored) - 1 {
			for j := i + 1; j < len(scored); j++ {
				if scored[i].score < scored[j].score {
					scored[i], scored[j] = scored[j], scored[i]
				}
			}
		}

		// Limit results
		if len(scored) > params.Limit {
			scored = scored[:params.Limit]
		}

		log.Logf("[BM25] Returning %d results (after limit of %d)", len(scored), params.Limit)

		// Log top results with scores for debugging
		for i, s := range scored {
			if i < 3 { // Log top 3 results
				log.Logf("[BM25]   Result %d: %s (score: %.2f)", i+1, s.doc.serverName, s.score)
			}
		}

		// Format results (same format as keywordStrategy)
		results := make([]map[string]any, 0, len(scored))
		for _, s := range scored {
			serverInfo := map[string]any{
				"name": s.doc.serverName,
			}

			if s.doc.server.Description != "" {
				serverInfo["description"] = s.doc.server.Description
			}

			if len(s.doc.server.Secrets) > 0 {
				var secrets []string
				for _, secret := range s.doc.server.Secrets {
					secrets = append(secrets, secret.Name)
				}
				serverInfo["required_secrets"] = secrets
			}

			if len(s.doc.server.Config) > 0 {
				serverInfo["config_schema"] = s.doc.server.Config
			}

			serverInfo["long_lived"] = s.doc.server.LongLived

			results = append(results, serverInfo)
		}

		response := map[string]any{
			"prompt":        params.Query,
			"total_matches": len(results),
			"servers":       results,
		}

		responseBytes, err := json.Marshal(response)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response: %w", err)
		}

		queryDuration := time.Since(queryStartTime)
		log.Logf("[BM25] Query completed in %v (returned %d results)", queryDuration, len(results))

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(responseBytes)}},
		}, nil
	}
}

func embeddingStrategy(g *Gateway) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse parameters
		var params struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}

		if req.Params.Arguments == nil {
			return nil, fmt.Errorf("missing arguments")
		}

		paramsBytes, err := json.Marshal(req.Params.Arguments)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal arguments: %w", err)
		}

		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			return nil, fmt.Errorf("failed to parse arguments: %w", err)
		}

		if params.Query == "" {
			return nil, fmt.Errorf("query parameter is required")
		}

		if params.Limit <= 0 {
			params.Limit = 10
		}

		// Use vector similarity search to find relevant servers
		results, err := g.findServersByEmbedding(ctx, params.Query, params.Limit)
		if err != nil {
			return nil, fmt.Errorf("failed to find servers: %w", err)
		}

		response := map[string]any{
			"prompt":        params.Query,
			"total_matches": len(results),
			"servers":       results,
		}

		responseBytes, err := json.Marshal(response)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(responseBytes)}},
		}, nil
	}
}

// findServersByEmbedding finds relevant MCP servers using vector similarity search
func (g *Gateway) findServersByEmbedding(ctx context.Context, query string, limit int) ([]map[string]any, error) {
	if g.embeddingsClient == nil {
		return nil, fmt.Errorf("embeddings client not initialized")
	}

	// Generate embedding for the query
	queryVector, err := generateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Search for similar servers in mcp-server-collection only
	results, err := g.embeddingsClient.SearchVectors(ctx, queryVector, &embeddings.SearchOptions{
		CollectionName: "mcp-server-collection",
		Limit:          limit,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search vectors: %w", err)
	}

	// Map results to servers from catalog
	var servers []map[string]any
	for _, result := range results {
		// Extract server name from metadata
		serverNameInterface, ok := result.Metadata["name"]
		if !ok {
			log.Logf("Warning: search result %d missing 'name' in metadata", result.ID)
			continue
		}

		serverName, ok := serverNameInterface.(string)
		if !ok {
			log.Logf("Warning: server name is not a string: %v", serverNameInterface)
			continue
		}

		// Look up the server in the catalog
		server, _, found := g.configuration.Find(serverName)
		if !found {
			log.Logf("Warning: server %s not found in catalog", serverName)
			continue
		}

		// Build server info map (same format as mcp-find)
		serverInfo := map[string]any{
			"name": serverName,
		}

		if server.Spec.Description != "" {
			serverInfo["description"] = server.Spec.Description
		}

		if len(server.Spec.Secrets) > 0 {
			var secrets []string
			for _, secret := range server.Spec.Secrets {
				secrets = append(secrets, secret.Name)
			}
			serverInfo["required_secrets"] = secrets
		}

		if len(server.Spec.Config) > 0 {
			serverInfo["config_schema"] = server.Spec.Config
		}

		serverInfo["long_lived"] = server.Spec.LongLived

		servers = append(servers, serverInfo)
	}

	return servers, nil
}
