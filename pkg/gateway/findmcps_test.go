package gateway

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/catalog"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "hyphenated words",
			input:    "github-api",
			expected: []string{"github", "api"},
		},
		{
			name:     "mixed case and underscores",
			input:    "File_System",
			expected: []string{"file", "system"},
		},
		{
			name:     "dots and spaces",
			input:    "access.github repositories",
			expected: []string{"access", "github", "repositories"},
		},
		{
			name:     "numbers",
			input:    "python3 test123",
			expected: []string{"python3", "test123"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "special characters only",
			input:    "!@#$%",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokenize(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("tokenize(%q) = %v, want %v", tt.input, result, tt.expected)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("tokenize(%q)[%d] = %q, want %q", tt.input, i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestBM25Strategy(t *testing.T) {
	// Create test configuration with sample servers
	config := Configuration{
		serverNames: []string{"github", "filesystem", "slack"},
		servers: map[string]catalog.Server{
			"github": {
				Name:        "github",
				Title:       "GitHub API",
				Description: "Access GitHub repositories and issues",
				Tools: []catalog.Tool{
					{Name: "create_issue", Description: "Create a new issue"},
					{Name: "list_repos", Description: "List all repositories"},
				},
			},
			"filesystem": {
				Name:        "filesystem",
				Title:       "File System",
				Description: "Read and write files on the local filesystem",
				Tools: []catalog.Tool{
					{Name: "read_file", Description: "Read a file"},
					{Name: "write_file", Description: "Write to a file"},
				},
			},
			"slack": {
				Name:        "slack",
				Title:       "Slack Integration",
				Description: "Send messages to Slack channels",
				Tools: []catalog.Tool{
					{Name: "send_message", Description: "Send a message to a channel"},
				},
			},
		},
	}

	// Create a mock gateway with the test configuration
	mockGateway := &Gateway{
		configuration: config,
	}

	handler := bm25Strategy(mockGateway)

	tests := []struct {
		name           string
		query          string
		limit          int
		expectedCount  int
		expectedFirst  string // name of expected first result
		shouldContain  []string
		shouldNotMatch bool
	}{
		{
			name:          "single term matching server name",
			query:         "github",
			limit:         10,
			expectedCount: 1,
			expectedFirst: "github",
		},
		{
			name:          "multi-word query matching description",
			query:         "access repositories",
			limit:         10,
			expectedCount: 1,
			expectedFirst: "github",
		},
		{
			name:          "query with no matches",
			query:         "nonexistent database postgresql",
			limit:         10,
			expectedCount: 0,
		},
		{
			name:          "limit parameter respected",
			query:         "file",
			limit:         1,
			expectedCount: 1,
		},
		{
			name:          "rare term IDF boost",
			query:         "slack",
			limit:         10,
			expectedCount: 1,
			expectedFirst: "slack",
		},
		{
			name:          "partial word match",
			query:         "repo",
			limit:         10,
			expectedCount: 0, // "repo" doesn't match "repositories" (no substring matching)
		},
		{
			name:          "tool name match",
			query:         "read file",
			limit:         10,
			expectedCount: 1,
			expectedFirst: "filesystem",
		},
		{
			name:          "common term in server name and tools",
			query:         "file",
			limit:         10,
			expectedCount: 1, // Only filesystem matches (name + tools)
			shouldContain: []string{"filesystem"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			args := map[string]any{
				"query": tt.query,
				"limit": tt.limit,
			}

			argsBytes, err := json.Marshal(args)
			if err != nil {
				t.Fatalf("failed to marshal args: %v", err)
			}

			params := &mcp.CallToolParamsRaw{
				Arguments: argsBytes,
			}

			req := &mcp.CallToolRequest{
				Params: params,
			}

			// Call handler
			result, err := handler(context.Background(), req)
			if err != nil {
				t.Fatalf("handler returned error: %v", err)
			}

			// Parse response
			if len(result.Content) != 1 {
				t.Fatalf("expected 1 content item, got %d", len(result.Content))
			}

			textContent, ok := result.Content[0].(*mcp.TextContent)
			if !ok {
				t.Fatalf("expected TextContent, got %T", result.Content[0])
			}

			var response map[string]any
			if err := json.Unmarshal([]byte(textContent.Text), &response); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			// Check total matches
			totalMatches, ok := response["total_matches"].(float64)
			if !ok {
				t.Fatalf("total_matches not found or wrong type")
			}

			if int(totalMatches) != tt.expectedCount {
				t.Errorf("expected %d matches, got %d", tt.expectedCount, int(totalMatches))
			}

			// Check servers
			serversRaw, ok := response["servers"]
			if !ok {
				t.Fatalf("servers field not found in response")
			}

			servers, ok := serversRaw.([]any)
			if !ok {
				t.Fatalf("servers not found or wrong type: got %T", serversRaw)
			}

			if len(servers) != tt.expectedCount {
				t.Errorf("expected %d servers, got %d", tt.expectedCount, len(servers))
			}

			// Check first result if expected
			if tt.expectedFirst != "" && len(servers) > 0 {
				firstServer, ok := servers[0].(map[string]any)
				if !ok {
					t.Fatalf("first server is not a map")
				}

				firstName, ok := firstServer["name"].(string)
				if !ok {
					t.Fatalf("first server name not found or wrong type")
				}

				if firstName != tt.expectedFirst {
					t.Errorf("expected first result to be %q, got %q", tt.expectedFirst, firstName)
				}
			}

			// Check that certain servers are in results
			if len(tt.shouldContain) > 0 {
				serverNames := make(map[string]bool)
				for _, s := range servers {
					server, ok := s.(map[string]any)
					if !ok {
						continue
					}
					name, ok := server["name"].(string)
					if !ok {
						continue
					}
					serverNames[name] = true
				}

				for _, expected := range tt.shouldContain {
					if !serverNames[expected] {
						t.Errorf("expected results to contain %q, but it was not found", expected)
					}
				}
			}
		})
	}
}

func TestBM25Index(t *testing.T) {
	// Test index building
	config := Configuration{
		servers: map[string]catalog.Server{
			"test-server": {
				Name:        "test-server",
				Title:       "Test Server",
				Description: "A test server for BM25",
				Tools: []catalog.Tool{
					{Name: "test_tool", Description: "A test tool"},
				},
			},
		},
	}

	index := buildBM25Index(config)

	// Verify index structure
	if index.N != 1 {
		t.Errorf("expected N=1, got N=%d", index.N)
	}

	if len(index.docs) != 1 {
		t.Errorf("expected 1 document, got %d", len(index.docs))
	}

	// Verify document contains expected terms
	doc := index.docs[0]
	if doc.serverName != "test-server" {
		t.Errorf("expected serverName='test-server', got %q", doc.serverName)
	}

	// Server name should be weighted 4x
	if doc.termFreq["test"] < 4 {
		t.Errorf("expected 'test' to appear at least 4 times (from server name weight), got %d", doc.termFreq["test"])
	}

	// Verify document frequency
	if index.docFreq["test"] != 1 {
		t.Errorf("expected docFreq['test']=1, got %d", index.docFreq["test"])
	}

	// Test scoring
	queryTokens := []string{"test", "bm25"}
	score := index.score(&doc, queryTokens)

	// Score should be > 0 since "test" appears in the document
	if score <= 0 {
		t.Errorf("expected score > 0, got %f", score)
	}

	// Query with no matching terms should score 0
	nonMatchingTokens := []string{"nonexistent", "words"}
	nonMatchingScore := index.score(&doc, nonMatchingTokens)
	if nonMatchingScore != 0 {
		t.Errorf("expected score=0 for non-matching query, got %f", nonMatchingScore)
	}
}

func TestBM25FieldWeighting(t *testing.T) {
	// Test that field weighting is applied by checking term frequency
	config := Configuration{
		serverNames: []string{"widget"},
		servers: map[string]catalog.Server{
			"widget": {
				Name:        "widget",
				Title:       "Tool",
				Description: "Server",
			},
		},
	}

	index := buildBM25Index(config)

	if len(index.docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(index.docs))
	}

	doc := index.docs[0]

	// Check that "widget" appears 4 times (server name weight)
	if doc.termFreq["widget"] != 4 {
		t.Errorf("expected 'widget' term frequency of 4 (name weight), got %d", doc.termFreq["widget"])
	}

	// Check that "tool" appears 3 times (title weight)
	if doc.termFreq["tool"] != 3 {
		t.Errorf("expected 'tool' term frequency of 3 (title weight), got %d", doc.termFreq["tool"])
	}

	// Check that "server" appears 1 time (description weight)
	if doc.termFreq["server"] != 1 {
		t.Errorf("expected 'server' term frequency of 1 (description weight), got %d", doc.termFreq["server"])
	}

	// Verify that higher term frequency leads to higher scores (all else being equal)
	// Since all terms appear in only this document, they have the same IDF
	// So score should be proportional to TF (with length normalization)
	widgetScore := index.score(&doc, []string{"widget"})
	toolScore := index.score(&doc, []string{"tool"})
	serverScore := index.score(&doc, []string{"server"})

	if widgetScore <= toolScore {
		t.Errorf("expected widget score (%f) > tool score (%f) due to higher TF", widgetScore, toolScore)
	}

	if toolScore <= serverScore {
		t.Errorf("expected tool score (%f) > server score (%f) due to higher TF", toolScore, serverScore)
	}
}
