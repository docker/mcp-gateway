package desktop

import (
	"context"
	"fmt"
)

type McpGatewayStateClientsInner struct {
	Name    string `json:"name,omitempty"`
	Status  string `json:"status,omitempty"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version,omitempty"`
}

type McpGatewayStateServersInner struct {
	Name  string   `json:"name"`
	Tools []string `json:"tools"`
}

type McpGatewayStateRootsInner struct {
	Uri string `json:"uri"`
}

type McpGatewayState struct {
	Args      []string                      `json:"args,omitempty"`
	Clients   []McpGatewayStateClientsInner `json:"clients,omitempty"`
	Cwd       string                        `json:"cwd,omitempty"`
	Profile   string                        `json:"profile,omitempty"`
	Roots     []McpGatewayStateRootsInner   `json:"roots,omitempty"`
	Servers   []McpGatewayStateServersInner `json:"servers,omitempty"`
	SessionId string                        `json:"sessionId"`
	Status    string                        `json:"status,omitempty"`
}

type McpGatewayEventsInner struct {
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp string                 `json:"timestamp,omitempty"`
	Type      string                 `json:"type,omitempty"`
}

type PostMcpGatewayEventsRequest struct {
	Events    []McpGatewayEventsInner `json:"events,omitempty"`
	SessionId string                  `json:"sessionId,omitempty"`
}

func PostMcpGatewayEvents(ctx context.Context, sessionId string, events []McpGatewayEventsInner) error {
	return ClientBackend.Post(ctx, "/mcp/gateway/events", &PostMcpGatewayEventsRequest{
		Events:    events,
		SessionId: sessionId,
	}, nil)
}

func GetMcpGatewayEvents(ctx context.Context, sessionId string) ([]McpGatewayEventsInner, error) {
	var result []McpGatewayEventsInner
	if err := ClientBackend.Get(ctx, fmt.Sprintf("/mcp/gateway/events/%s", sessionId), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func ListMcpGatewayStates(ctx context.Context) ([]McpGatewayState, error) {
	var result []McpGatewayState
	if err := ClientBackend.Get(ctx, "/mcp/gateways", &result); err != nil {
		return nil, err
	}
	return result, nil
}

func GetMcpGatewayState(ctx context.Context, sessionId string) (*McpGatewayState, error) {
	var result McpGatewayState
	if err := ClientBackend.Get(ctx, fmt.Sprintf("/mcp/gateway/state/%s", sessionId), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func UpdateMcpGatewayState(ctx context.Context, sessionId string, state McpGatewayState) error {
	return ClientBackend.Post(ctx, "/mcp/gateway/state", &state, nil)
}
