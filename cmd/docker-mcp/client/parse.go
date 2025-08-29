package client

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// specs found in the wild:
// - VSCode: https://code.visualstudio.com/docs/copilot/chat/mcp-servers#_configuration-format
// - Cursor: https://docs.cursor.com/context/model-context-protocol#configuring-mcp-servers
// - Claude Desktop: https://modelcontextprotocol.io/examples#configuring-with-claude

type MCPJSONLists struct {
	STDIOServers []MCPServerSTDIO
	SSEServers   []MCPServerSSE
	HTTPServers  []MCPServerHTTP
}

type MCPServerSTDIO struct {
	Name    string            `json:"name,omitempty"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

func (c *MCPServerSTDIO) String() string {
	var result string
	for k, v := range c.Env {
		result += fmt.Sprintf("%s=%s ", k, v)
	}
	result += c.Command
	if len(c.Args) > 0 {
		result += " " + strings.Join(c.Args, " ")
	}
	return result
}

type MCPServerSSE struct {
	Name    string            `json:"name"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type MCPServerHTTP struct {
	Name    string            `json:"name"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

func (c *MCPServerSSE) String() string {
	return c.URL
}

func (c *MCPServerHTTP) String() string {
	return c.URL
}

func UnmarshalMCPJSONList(data []byte) (*MCPJSONLists, error) {
	if len(data) == 0 {
		return &MCPJSONLists{}, nil
	}
	var temp []json.RawMessage
	if err := json.Unmarshal(data, &temp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON list: %w", err)
	}
	cfg := &MCPJSONLists{
		STDIOServers: []MCPServerSTDIO{},
		SSEServers:   []MCPServerSSE{},
		HTTPServers:  []MCPServerHTTP{},
	}
	for _, raw := range temp {
		itemType, _ := getType(raw) // type is an optional field, default to stdio if not explicitly set
		switch itemType {
		case "stdio", "":
			var server MCPServerSTDIO
			if err := json.Unmarshal(raw, &server); err != nil {
				return nil, err
			}
			cfg.STDIOServers = append(cfg.STDIOServers, server)
		case "sse":
			var server MCPServerSSE
			if err := json.Unmarshal(raw, &server); err != nil {
				return nil, err
			}
			cfg.SSEServers = append(cfg.SSEServers, server)
		case "http":
			var server MCPServerHTTP
			if err := json.Unmarshal(raw, &server); err != nil {
				return nil, err
			}
			cfg.HTTPServers = append(cfg.HTTPServers, server)
		default:
			fmt.Fprintf(os.Stderr, "unknown server type for %q\n", itemType)
		}
	}
	return cfg, nil
}

func getType(raw json.RawMessage) (string, error) {
	var typeHolder struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &typeHolder); err != nil {
		return "", fmt.Errorf("failed to unmarshal type: %w", err)
	}
	return typeHolder.Type, nil
}
