package gateway

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	_ "embed"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/catalog"
)

const (
	dynamicToolsCapabilityKey = "__dynamic_tools__"

	toolManagerView             = "mcp-tool-manager"
	toolManagerResourceURI      = "ui://widget/mcp-tool-manager.html"
	toolManagerResourceName     = "mcp-tool-manager-ui"
	toolManagerResourceMIMEType = "text/html+skybridge"
)

//go:embed ui/embedded/tool-manager.js
var toolManagerBundle string

var toolManagerOutputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"view":          map[string]any{"type": "string"},
		"status":        map[string]any{"type": "string"},
		"message":       map[string]any{"type": "string"},
		"query":         map[string]any{"type": "string"},
		"limit":         map[string]any{"type": "integer"},
		"totalMatches":  map[string]any{"type": "integer"},
		"activeServers": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"results": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":            map[string]any{"type": "string"},
					"description":     map[string]any{"type": "string"},
					"type":            map[string]any{"type": "string"},
					"remoteUrl":       map[string]any{"type": "string"},
					"image":           map[string]any{"type": "string"},
					"longLived":       map[string]any{"type": "boolean"},
					"requiredSecrets": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"isActive":        map[string]any{"type": "boolean"},
				},
			},
		},
		"server": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
		},
		"lastAction": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"type":    map[string]any{"type": "string"},
				"status":  map[string]any{"type": "string"},
				"message": map[string]any{"type": "string"},
				"server":  map[string]any{"type": "string"},
			},
		},
		"timestamp": map[string]any{"type": "string"},
		"error":     map[string]any{"type": "string"},
	},
}

type toolManagerServer struct {
	Name            string                  `json:"name"`
	Description     string                  `json:"description,omitempty"`
	Type            string                  `json:"type,omitempty"`
	RemoteURL       string                  `json:"remoteUrl,omitempty"`
	Image           string                  `json:"image,omitempty"`
	LongLived       bool                    `json:"longLived,omitempty"`
	RequiredSecrets []string                `json:"requiredSecrets,omitempty"`
	ConfigSchema    []any                   `json:"configSchema,omitempty"`
	Tools           []toolManagerServerTool `json:"tools,omitempty"`
	OAuthProviders  []string                `json:"oauthProviders,omitempty"`
	IsActive        bool                    `json:"isActive,omitempty"`
}

type toolManagerServerTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type toolManagerAction struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Server  string `json:"server,omitempty"`
}

type toolManagerPayload struct {
	View          string              `json:"view"`
	SourceTool    string              `json:"sourceTool"`
	Status        string              `json:"status"`
	Message       string              `json:"message,omitempty"`
	Query         string              `json:"query,omitempty"`
	Limit         int                 `json:"limit,omitempty"`
	TotalMatches  int                 `json:"totalMatches,omitempty"`
	ActiveServers []string            `json:"activeServers"`
	Results       []toolManagerServer `json:"results,omitempty"`
	Server        *toolManagerServer  `json:"server,omitempty"`
	LastAction    *toolManagerAction  `json:"lastAction,omitempty"`
	Timestamp     string              `json:"timestamp"`
	Error         string              `json:"error,omitempty"`
}

func ensureToolManagerBundle() string {
	escaped := strings.ReplaceAll(toolManagerBundle, "</script>", "<\\/script>")
	return fmt.Sprintf(`<div id="mcp-tool-manager-root"></div><script type="module">%s</script>`, escaped)
}

func (g *Gateway) ensureDynamicCapabilities() *ServerCapabilities {
	if g.serverCapabilities[dynamicToolsCapabilityKey] == nil {
		g.serverCapabilities[dynamicToolsCapabilityKey] = &ServerCapabilities{}
	}
	return g.serverCapabilities[dynamicToolsCapabilityKey]
}

func (g *Gateway) registerToolManagerResource() {
	resource := &mcp.Resource{
		Name:        toolManagerResourceName,
		URI:         toolManagerResourceURI,
		Title:       "MCP Tool Manager",
		Description: "Interactive UI for searching, adding, and removing MCP servers in the gateway.",
		MIMEType:    toolManagerResourceMIMEType,
		Meta: mcp.Meta{
			"openai/widgetDescription": "Interactive UI for searching the MCP catalog, enabling servers, and removing them from the session.",
		},
	}

	g.mcpServer.AddResource(resource, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      toolManagerResourceURI,
					MIMEType: toolManagerResourceMIMEType,
					Text:     ensureToolManagerBundle(),
				},
			},
		}, nil
	})

	caps := g.ensureDynamicCapabilities()
	caps.ResourceURIs = append(caps.ResourceURIs, toolManagerResourceURI)
}

func (g *Gateway) activeServerNames() []string {
	seen := map[string]struct{}{}
	var names []string
	for _, name := range g.configuration.serverNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func summarizeServer(name string, server catalog.Server) toolManagerServer {
	summary := toolManagerServer{
		Name:        name,
		Description: server.Description,
		Type:        server.Type,
		Image:       server.Image,
		LongLived:   server.LongLived,
	}

	if server.Remote.URL != "" {
		summary.RemoteURL = server.Remote.URL
	} else if server.SSEEndpoint != "" {
		summary.RemoteURL = server.SSEEndpoint
	}

	if len(server.Secrets) > 0 {
		secrets := make([]string, 0, len(server.Secrets))
		for _, secret := range server.Secrets {
			if secret.Name != "" {
				secrets = append(secrets, secret.Name)
			}
		}
		summary.RequiredSecrets = secrets
	}

	if len(server.Tools) > 0 {
		tools := make([]toolManagerServerTool, 0, len(server.Tools))
		for _, tool := range server.Tools {
			tools = append(tools, toolManagerServerTool{
				Name:        tool.Name,
				Description: tool.Description,
			})
		}
		summary.Tools = tools
	}

	if len(server.Config) > 0 {
		summary.ConfigSchema = server.Config
	}

	if server.OAuth != nil && len(server.OAuth.Providers) > 0 {
		providers := make([]string, 0, len(server.OAuth.Providers))
		for _, provider := range server.OAuth.Providers {
			if provider.Provider != "" {
				providers = append(providers, provider.Provider)
			}
		}
		summary.OAuthProviders = providers
	}

	return summary
}

func newToolManagerPayload(sourceTool, status, message string) toolManagerPayload {
	if status == "" {
		status = "info"
	}
	payload := toolManagerPayload{
		View:          toolManagerView,
		SourceTool:    sourceTool,
		Status:        status,
		Message:       message,
		ActiveServers: []string{},
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}
	return payload
}

func newToolManagerAction(actionType, status, message, server string) *toolManagerAction {
	if actionType == "" {
		actionType = "find"
	}
	if status != "error" {
		status = "success"
	}
	return &toolManagerAction{
		Type:    actionType,
		Status:  status,
		Message: message,
		Server:  server,
	}
}

func (g *Gateway) buildToolManagerResult(payload toolManagerPayload, contentMessage string) *mcp.CallToolResult {
	payload.ActiveServers = g.activeServerNames()
	if payload.LastAction == nil && payload.SourceTool != "" {
		actionType := "find"
		switch payload.SourceTool {
		case "mcp-add":
			actionType = "add"
		case "mcp-remove":
			actionType = "remove"
		}
		serverName := ""
		if payload.Server != nil {
			serverName = payload.Server.Name
		}
		payload.LastAction = newToolManagerAction(actionType, payload.Status, payload.Message, serverName)
	}

	result := &mcp.CallToolResult{
		Meta: mcp.Meta{
			"openai/outputTemplate": toolManagerResourceURI,
		},
		Content: []mcp.Content{
			&mcp.TextContent{Text: contentMessage},
		},
		StructuredContent: payload,
	}
	return result
}
