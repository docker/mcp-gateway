package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

//go:embed ui
var uiFiles embed.FS

type StandaloneMCPGateway struct {
	httpServer *http.Server
	port       int
}

type ServerInfo struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Image       string            `json:"image"`
	Active      bool              `json:"active"`
	LongLived   bool              `json:"longLived"`
	Secrets     []string          `json:"secrets"`
	Tools       []string          `json:"tools"`
	Config      map[string]any    `json:"config"`
}

type GatewayConfig struct {
	Port              int      `json:"port"`
	Transport         string   `json:"transport"`
	CatalogURL        string   `json:"catalogUrl"`
	EnableDynamicTools bool    `json:"enableDynamicTools"`
	EnableLogging     bool    `json:"enableLogging"`
	EnableTelemetry   bool    `json:"enableTelemetry"`
	ActiveServers     []string `json:"activeServers"`
}

func NewStandaloneMCPGateway(port int) *StandaloneMCPGateway {
	return &StandaloneMCPGateway{
		port: port,
	}
}

func (s *StandaloneMCPGateway) Start(ctx context.Context) error {
	// Setup HTTP server for UI
	s.setupHTTPServer()

	// Start HTTP server
	log.Printf("Starting Standalone MCP Gateway UI on port %d", s.port)
	log.Printf("Gateway management interface available at: http://localhost:%d", s.port)
	log.Printf("Note: This is the management UI. Use 'docker mcp gateway run' to start the actual MCP gateway.")

	return s.httpServer.ListenAndServe()
}

func (s *StandaloneMCPGateway) setupHTTPServer() {
	mux := http.NewServeMux()

	// Serve static UI files
	uiFS, _ := fs.Sub(uiFiles, "ui")
	mux.Handle("/", http.FileServer(http.FS(uiFS)))

	// API endpoints
	mux.HandleFunc("/api/servers", s.handleServers)
	mux.HandleFunc("/api/servers/add", s.handleAddServer)
	mux.HandleFunc("/api/servers/remove", s.handleRemoveServer)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/export/claude", s.handleExportClaude)
	mux.HandleFunc("/api/export/llmstudio", s.handleExportLLMStudio)
	mux.HandleFunc("/api/export/docker-compose", s.handleExportDockerCompose)
	mux.HandleFunc("/api/catalog/search", s.handleCatalogSearch)
	mux.HandleFunc("/api/registry/import", s.handleRegistryImport)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.corsMiddleware(mux),
	}
}

func (s *StandaloneMCPGateway) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *StandaloneMCPGateway) handleServers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getServers(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *StandaloneMCPGateway) getServers(w http.ResponseWriter, r *http.Request) {
	// Get sample servers data
	servers := s.getSampleServers()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(servers)
}

func (s *StandaloneMCPGateway) getSampleServers() []ServerInfo {
	return []ServerInfo{
		{
			Name:        "filesystem",
			Description: "File system operations for reading, writing, and managing files and directories",
			Image:       "docker/filesystem-mcp",
			Active:      true,
			LongLived:   false,
			Secrets:     []string{},
			Tools:       []string{"read_file", "write_file", "list_directory", "create_directory"},
			Config:      map[string]any{"basePath": "/tmp", "allowedExtensions": []string{".txt", ".md", ".json"}},
		},
		{
			Name:        "duckduckgo",
			Description: "Web search using DuckDuckGo search engine",
			Image:       "docker/duckduckgo-mcp",
			Active:      true,
			LongLived:   false,
			Secrets:     []string{},
			Tools:       []string{"search_web"},
			Config:      map[string]any{},
		},
		{
			Name:        "github",
			Description: "GitHub repository management and operations",
			Image:       "docker/github-mcp",
			Active:      false,
			LongLived:   true,
			Secrets:     []string{"GITHUB_TOKEN"},
			Tools:       []string{"list_repos", "create_issue", "get_file_content"},
			Config:      map[string]any{"defaultOrg": "docker"},
		},
		{
			Name:        "postgres",
			Description: "PostgreSQL database operations and queries",
			Image:       "docker/postgres-mcp",
			Active:      false,
			LongLived:   true,
			Secrets:     []string{"POSTGRES_CONNECTION_STRING"},
			Tools:       []string{"execute_query", "list_tables", "describe_table"},
			Config:      map[string]any{"maxConnections": 10, "queryTimeout": 30000},
		},
		{
			Name:        "slack",
			Description: "Slack messaging and workspace management",
			Image:       "docker/slack-mcp",
			Active:      false,
			LongLived:   false,
			Secrets:     []string{"SLACK_BOT_TOKEN"},
			Tools:       []string{"send_message", "list_channels", "get_user_info"},
			Config:      map[string]any{"defaultChannel": "#general"},
		},
	}
}

func (s *StandaloneMCPGateway) handleAddServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// In a real implementation, this would call the MCP gateway's dynamic management tools
	log.Printf("Adding server: %s", req.Name)

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Successfully added server '%s'", req.Name),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *StandaloneMCPGateway) handleRemoveServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// In a real implementation, this would call the MCP gateway's dynamic management tools
	log.Printf("Removing server: %s", req.Name)

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Successfully removed server '%s'", req.Name),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *StandaloneMCPGateway) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		config := GatewayConfig{
			Port:              8811,
			Transport:         "sse",
			CatalogURL:        "https://desktop.docker.com/mcp/catalog/v2/catalog.yaml",
			EnableDynamicTools: true,
			EnableLogging:     true,
			EnableTelemetry:   false,
			ActiveServers:     []string{"filesystem", "duckduckgo"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(config)

	case http.MethodPost:
		var config GatewayConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		log.Printf("Updating gateway configuration: %+v", config)

		response := map[string]string{
			"status":  "success",
			"message": "Configuration updated successfully",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *StandaloneMCPGateway) handleExportClaude(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	config := map[string]any{
		"mcpServers": map[string]any{
			"MCP_GATEWAY": map[string]any{
				"command": "docker",
				"args":    []string{"mcp", "gateway", "run", "--servers=filesystem,duckduckgo"},
				"env":     map[string]string{},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

func (s *StandaloneMCPGateway) handleExportLLMStudio(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	config := []map[string]any{
		{
			"name": "mcp-gateway",
			"type": "sse",
			"url":  "http://localhost:8811/sse",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

func (s *StandaloneMCPGateway) handleExportDockerCompose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	compose := `services:
  gateway:
    image: docker/mcp-gateway
    command:
      - --servers=filesystem,duckduckgo
      - --transport=sse
      - --port=8811
    ports:
      - "8811:8811"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock`

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(compose))
}

func (s *StandaloneMCPGateway) handleCatalogSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("query")
	if query == "" {
		http.Error(w, "Query parameter required", http.StatusBadRequest)
		return
	}

	// Return filtered servers based on query
	servers := s.getSampleServers()
	var results []ServerInfo

	for _, server := range servers {
		if strings.Contains(strings.ToLower(server.Name), strings.ToLower(query)) ||
			strings.Contains(strings.ToLower(server.Description), strings.ToLower(query)) {
			results = append(results, server)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (s *StandaloneMCPGateway) handleRegistryImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("Importing registry from: %s", req.URL)

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Successfully imported servers from %s", req.URL),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	port := 3000
	if p := os.Getenv("PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	gateway := NewStandaloneMCPGateway(port)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := gateway.Start(ctx); err != nil {
		log.Fatalf("Failed to start gateway: %v", err)
	}
}