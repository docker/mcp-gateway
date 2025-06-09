package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"sync/atomic"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/r3labs/sse/v2"
)

type sseMCPClient struct {
	name               string
	initiationEndpoint string
	sessionEndpoint    *string

	requestID   atomic.Int64
	responses   sync.Map
	close       func() error
	initialized atomic.Bool
}

func NewSSEClient(name string, endpoint string) Client {
	return &sseMCPClient{
		name:               name,
		initiationEndpoint: endpoint,
	}
}

func (c *sseMCPClient) Initialize(ctx context.Context, request mcp.InitializeRequest, debug bool) (*mcp.InitializeResult, error) {
	if c.initialized.Load() {
		return nil, fmt.Errorf("client already initialized")
	}

	parsedURL, err := url.Parse(c.initiationEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse initiation endpoint: %w", err)
	}

	ctxResponses, cancelResponses := context.WithCancel(context.WithoutCancel(ctx))

	// 5 MB max buffer size allows things like large screenshots to be sent
	sseClient := sse.NewClient(c.initiationEndpoint, sse.ClientMaxBufferSize((1<<20)*5))
	c.close = func() error {
		cancelResponses()
		return nil
	}

	sessionPathCh := make(chan string)

	go func() {
		_ = c.readResponses(ctxResponses, sseClient, sessionPathCh)
		if debug {
			ep := "[uninitialized]"

			if sessionEndpoint := c.sessionEndpoint; sessionEndpoint != nil { // Reassign to avoid the potential for a nil pointer dereference in a race condition
				ep = *sessionEndpoint
			}
			fmt.Fprintf(os.Stderr, "  - disconnected from session endpoint for %s: %s\n", c.name, ep)
		}
	}()

	// Wait for the session endpoint after subscribing
	select {
	case <-ctx.Done():
		cancelResponses()
		return nil, ctx.Err()
	case sessionPath := <-sessionPathCh:
		sessionEndpoint := fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, sessionPath)
		if debug {
			fmt.Fprintf(os.Stderr, "  - connected to session endpoint for %s: %s\n", c.name, sessionEndpoint)
		}
		c.sessionEndpoint = &sessionEndpoint
	}

	var result mcp.InitializeResult

	errCh := make(chan error)
	go func() {
		errCh <- func() error {
			params := struct {
				ProtocolVersion string                 `json:"protocolVersion"`
				ClientInfo      mcp.Implementation     `json:"clientInfo"`
				Capabilities    mcp.ClientCapabilities `json:"capabilities"`
			}{
				ProtocolVersion: request.Params.ProtocolVersion,
				ClientInfo:      request.Params.ClientInfo,
				Capabilities:    request.Params.Capabilities,
			}

			if err := c.request(ctx, "initialize", params, &result); err != nil {
				return err
			}

			data, err := json.Marshal(mcp.JSONRPCNotification{
				JSONRPC: mcp.JSONRPC_VERSION,
				Notification: mcp.Notification{
					Method: "notifications/initialized",
				},
			})
			if err != nil {
				return fmt.Errorf("failed to marshal initialized notification: %w", err)
			}

			err = c.doHTTPRequest(ctx, data)
			if err != nil {
				return err
			}

			c.initialized.Store(true)
			return nil
		}()
	}()

	err = <-errCh
	if err != nil {
		cancelResponses() // cancel the response reader
		return nil, err
	}

	return &result, nil
}

func (c *sseMCPClient) Close() error {
	return c.close()
}

func (c *sseMCPClient) readResponses(ctx context.Context, sseClient *sse.Client, sessionPathCh chan string) error {
	return sseClient.SubscribeRawWithContext(ctx, func(msg *sse.Event) {
		event := string(msg.Event)
		if event == "endpoint" {
			sessionPath := string(msg.Data)
			sessionPathCh <- sessionPath
			return
		}

		buf := msg.Data
		var baseMessage BaseMessage
		if err := json.Unmarshal(buf, &baseMessage); err != nil {
			return
		}

		if baseMessage.ID == nil {
			return
		}

		if ch, ok := c.responses.LoadAndDelete(*baseMessage.ID); ok {
			responseChan := ch.(chan RPCResponse)

			if baseMessage.Error != nil {
				responseChan <- RPCResponse{
					Error: &baseMessage.Error.Message,
				}
			} else {
				responseChan <- RPCResponse{
					Response: &baseMessage.Result,
				}
			}
		}
	})
}

func (c *sseMCPClient) doHTTPRequest(ctx context.Context, jsonData []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, *c.sessionEndpoint, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	// Response comes from SSE
	resp.Body.Close()
	return nil
}

func (c *sseMCPClient) sendRequest(ctx context.Context, method string, params any) (*json.RawMessage, error) {
	id := c.requestID.Add(1)
	responseChan := make(chan RPCResponse, 1)
	c.responses.Store(id, responseChan)

	data, err := json.Marshal(mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(id),
		Request: mcp.Request{
			Method: method,
		},
		Params: params,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	err = c.doHTTPRequest(ctx, data)
	if err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case response := <-responseChan:
		if response.Error != nil {
			return nil, errors.New(*response.Error)
		}
		return response.Response, nil
	}
}

func (c *sseMCPClient) request(ctx context.Context, method string, params any, v any) error {
	response, err := c.sendRequest(ctx, method, params)
	if err != nil {
		return err
	}

	return json.Unmarshal(*response, &v)
}

func (c *sseMCPClient) ListTools(ctx context.Context, request mcp.ListToolsRequest) (*mcp.ListToolsResult, error) {
	var result mcp.ListToolsResult
	if err := c.request(ctx, "tools/list", request, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *sseMCPClient) ListPrompts(ctx context.Context, request mcp.ListPromptsRequest) (*mcp.ListPromptsResult, error) {
	var result mcp.ListPromptsResult
	if err := c.request(ctx, "prompts/list", request, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *sseMCPClient) ListResources(ctx context.Context, request mcp.ListResourcesRequest) (*mcp.ListResourcesResult, error) {
	var result mcp.ListResourcesResult
	if err := c.request(ctx, "resources/list", request, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *sseMCPClient) ListResourceTemplates(ctx context.Context, request mcp.ListResourceTemplatesRequest) (*mcp.ListResourceTemplatesResult, error) {
	var result mcp.ListResourceTemplatesResult
	if err := c.request(ctx, "resources/templates/list", request, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *sseMCPClient) CallTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	response, err := c.sendRequest(ctx, "tools/call", request.Params)
	if err != nil {
		return nil, err
	}

	return mcp.ParseCallToolResult(response)
}

func (c *sseMCPClient) GetPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	response, err := c.sendRequest(ctx, "prompts/get", request.Params)
	if err != nil {
		return nil, err
	}

	return mcp.ParseGetPromptResult(response)
}

func (c *sseMCPClient) ReadResource(ctx context.Context, request mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	response, err := c.sendRequest(ctx, "resources/read", request.Params)
	if err != nil {
		return nil, err
	}

	return mcp.ParseReadResourceResult(response)
}
