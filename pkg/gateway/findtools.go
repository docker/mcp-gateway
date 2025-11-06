package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/gateway/embeddings"
	"github.com/docker/mcp-gateway/pkg/log"
)

// generateEmbedding generates an embedding vector from text using OpenAI's API
func generateEmbedding(ctx context.Context, text string) ([]float64, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable not set")
	}

	type embeddingRequest struct {
		Input string `json:"input"`
		Model string `json:"model"`
	}

	type embeddingResponse struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}

	reqBody, err := json.Marshal(embeddingRequest{
		Input: text,
		Model: "text-embedding-3-small",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/embeddings", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var embResp embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return embResp.Data[0].Embedding, nil
}

// findToolsByEmbedding finds relevant tools using vector similarity search
func (g *Gateway) findToolsByEmbedding(ctx context.Context, prompt string) ([]map[string]any, error) {
	if g.embeddingsClient == nil {
		return nil, fmt.Errorf("embeddings client not initialized")
	}

	// Generate embedding for the prompt
	queryVector, err := generateEmbedding(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Search for similar tools, excluding the mcp-server-collection
	results, err := g.embeddingsClient.SearchVectors(ctx, queryVector, &embeddings.SearchOptions{
		ExcludeCollections: []string{"mcp-server-collection"},
		Limit:              5,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search vectors: %w", err)
	}

	// Map results to tools in tools/list format
	var tools []map[string]any
	for _, result := range results {
		// Extract tool name from metadata
		toolNameInterface, ok := result.Metadata["tool"]
		if !ok {
			log.Logf("Warning: search result %d missing 'tool' in metadata", result.ID)
			continue
		}

		// Handle nested structure: metadata.tool.name
		var toolName string
		switch v := toolNameInterface.(type) {
		case map[string]any:
			if nameInterface, ok := v["name"]; ok {
				toolName, _ = nameInterface.(string)
			}
		case string:
			toolName = v
		}

		if toolName == "" {
			log.Logf("Warning: could not extract tool name from metadata: %v", result.Metadata)
			continue
		}

		// Look up the tool registration
		toolReg, ok := g.toolRegistrations[toolName]
		if !ok {
			log.Logf("Warning: tool %s not found in registrations", toolName)
			continue
		}

		// Build tool map in tools/list format
		toolMap := map[string]any{
			"name":        toolReg.Tool.Name,
			"description": toolReg.Tool.Description,
		}
		if toolReg.Tool.InputSchema != nil {
			toolMap["inputSchema"] = toolReg.Tool.InputSchema
		}

		tools = append(tools, toolMap)
	}

	return tools, nil
}

// createFindToolsTool implements a tool for finding relevant tools based on a user's task description
func (g *Gateway) createFindToolsTool(_ *clientConfig) *ToolRegistration {
	tool := &mcp.Tool{
		Name:        "find-tools",
		Description: "Analyze a task description and recommend relevant MCP tools that could help accomplish it. Uses AI to intelligently match your needs to available tools.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"prompt": {
					Type:        "string",
					Description: "Description of the task or goal you want to accomplish. An AI will analyze this and recommend relevant tools from the available inventory.",
				},
			},
			Required: []string{"prompt"},
		},
	}

	handler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse parameters
		var params struct {
			Prompt string `json:"prompt"`
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

		if params.Prompt == "" {
			return nil, fmt.Errorf("prompt parameter is required")
		}

		// Use vector similarity search to find relevant tools
		tools, err := g.findToolsByEmbedding(ctx, params.Prompt)
		if err != nil {
			return nil, fmt.Errorf("failed to find tools: %w", err)
		}

		// Format response in tools/list format
		response := map[string]any{
			"tools": tools,
		}

		responseJSON, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Text: string(responseJSON),
			}},
		}, nil
	}

	return &ToolRegistration{
		ServerName: "", // Internal tool
		Tool:       tool,
		Handler:    handler,
	}
}
