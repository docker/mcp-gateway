package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/policy"
)

func addMcpExecHandler(g *Gateway) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse parameters
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
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

		if params.Name == "" {
			return nil, fmt.Errorf("name parameter is required")
		}

		toolName := strings.TrimSpace(params.Name)

		// Look up the tool in current tool registrations
		g.capabilitiesMu.RLock()
		toolReg, found := g.toolRegistrations[toolName]
		g.capabilitiesMu.RUnlock()

		if !found {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf("Error: Tool '%s' not found in current session. Make sure the server providing this tool is added to the session.", toolName),
				}},
			}, nil
		}

		// Handle the case where arguments might be a JSON-encoded string
		// This happens when the schema previously specified Type: "string"
		var toolArguments json.RawMessage
		if len(params.Arguments) > 0 {
			// Try to unmarshal as a string first (for backward compatibility)
			var argString string
			if err := json.Unmarshal(params.Arguments, &argString); err == nil {
				// It was a JSON string, use the unescaped content
				toolArguments = json.RawMessage(argString)
			} else {
				// It's already a proper JSON object/value
				toolArguments = params.Arguments
			}
		}

		// Create a new CallToolRequest with the provided arguments
		log.Logf("calling tool %s with %s", toolName, toolArguments)
		toolCallRequest := &mcp.CallToolRequest{
			Session: req.Session,
			Params: &mcp.CallToolParamsRaw{
				Meta:      req.Params.Meta,
				Name:      toolName,
				Arguments: toolArguments,
			},
			Extra: req.Extra,
		}

		// Policy check before executing the tool
		if g.policyClient != nil {
			policyReq := g.configuration.policyRequest(toolReg.ServerName, toolName, policy.ActionInvoke)
			decision, err := g.policyClient.Evaluate(ctx, policyReq)
			event := buildAuditEvent(policyReq, decision, err, auditClientInfoFromSession(req.Session))
			submitAuditEvent(g.policyClient, event)
			if err != nil {
				log.Logf("policy check failed for mcp-exec %s: %v (denying)", toolName, err)
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{
						Text: fmt.Sprintf("Error: Tool '%s' blocked due to policy check error: %v", toolName, err),
					}},
				}, nil
			}
			if !decision.Allowed {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{
						Text: fmt.Sprintf("Error: Tool '%s' blocked by policy: %s", toolName, decision.Reason),
					}},
				}, nil
			}
		}

		// Execute the tool using its registered handler
		result, err := toolReg.Handler(ctx, toolCallRequest)
		if err != nil {
			return nil, fmt.Errorf("tool execution failed: %w", err)
		}

		return result, nil
	}
}
