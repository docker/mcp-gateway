package gateway

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"

	mcpclient "github.com/docker/mcp-cli/cmd/docker-mcp/internal/mcp"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"golang.org/x/sync/errgroup"
)

type Capabilities struct {
	Tools             []server.ServerTool
	Prompts           []server.ServerPrompt
	Resources         []server.ServerResource
	ResourceTemplates []ServerResourceTemplate
}

type ServerResourceTemplate struct {
	ResourceTemplate mcp.ResourceTemplate
	Handler          server.ResourceTemplateHandlerFunc
}

//nolint:gocyclo
func (g *Gateway) listCapabilities(ctx context.Context, configuration Configuration, serverNames []string) (*Capabilities, error) {
	var (
		lock            sync.Mutex
		allCapabilities []Capabilities
	)

	errs, ctx := errgroup.WithContext(ctx)
	errs.SetLimit(runtime.NumCPU())
	for _, serverName := range serverNames {
		serverConfig, toolGroup, found := configuration.Find(serverName)

		switch {
		case !found:
			log("  - MCP server not found:", serverName)

		// It's an MCP Server
		case serverConfig != nil:
			errs.Go(func() error {
				var client mcpclient.MCPClient
				var err error
				if serverConfig.Spec.SSEEndpoint != "" {
					client, err = g.startSSEMCPClient(ctx, serverConfig.Name, serverConfig.Spec.SSEEndpoint, &readOnly)
					if err != nil {
						logf("  > Can't connect to %s: %s", serverConfig.Name, err)
						return nil
					}
				} else {
					client, err = g.startStdioMCPClient(ctx, *serverConfig, &readOnly)
					if err != nil {
						logf("  > Can't start %s: %s", serverConfig.Name, err)
						return nil
					}
				}
				defer client.Close()

				var capabilities Capabilities

				tools, err := client.ListTools(ctx, mcp.ListToolsRequest{})
				if err != nil {
					logf("  > Can't list tools %s: %s", serverConfig.Name, err)
				} else {
					for _, tool := range tools.Tools {
						if !isToolEnabled(serverConfig.Name, serverConfig.Spec.Image, tool.Name, g.ToolNames) {
							continue
						}
						capabilities.Tools = append(capabilities.Tools, server.ServerTool{
							Tool:    tool,
							Handler: g.mcpServerToolHandler(*serverConfig, tool.Annotations),
						})
					}
				}

				prompts, err := client.ListPrompts(ctx, mcp.ListPromptsRequest{})
				if err == nil {
					for _, prompt := range prompts.Prompts {
						capabilities.Prompts = append(capabilities.Prompts, server.ServerPrompt{
							Prompt:  prompt,
							Handler: g.mcpServerPromptHandler(*serverConfig),
						})
					}
				}

				resources, err := client.ListResources(ctx, mcp.ListResourcesRequest{})
				if err == nil {
					for _, resource := range resources.Resources {
						capabilities.Resources = append(capabilities.Resources, server.ServerResource{
							Resource: resource,
							Handler:  g.mcpServerResourceHandler(*serverConfig),
						})
					}
				}

				resourceTemplates, err := client.ListResourceTemplates(ctx, mcp.ListResourceTemplatesRequest{})
				if err == nil {
					for _, resourceTemplate := range resourceTemplates.ResourceTemplates {
						capabilities.ResourceTemplates = append(capabilities.ResourceTemplates, ServerResourceTemplate{
							ResourceTemplate: resourceTemplate,
							Handler:          g.mcpServerResourceTemplateHandler(*serverConfig),
						})
					}
				}

				var log string
				if len(capabilities.Tools) > 0 {
					log += fmt.Sprintf(" (%d tools)", len(capabilities.Tools))
				}
				if len(capabilities.Prompts) > 0 {
					log += fmt.Sprintf(" (%d prompts)", len(capabilities.Prompts))
				}
				if len(capabilities.Resources) > 0 {
					log += fmt.Sprintf(" (%d resources)", len(capabilities.Resources))
				}
				if len(capabilities.ResourceTemplates) > 0 {
					log += fmt.Sprintf(" (%d resourceTemplates)", len(capabilities.ResourceTemplates))
				}
				if log != "" {
					logf("  > %s:%s", serverConfig.Name, log)
				}

				lock.Lock()
				allCapabilities = append(allCapabilities, capabilities)
				lock.Unlock()

				return nil
			})

		// It's a POCI
		case toolGroup != nil:
			var capabilities Capabilities

			for _, tool := range *toolGroup {
				if !isToolEnabled(serverName, "", tool.Name, g.ToolNames) {
					continue
				}

				mcpTool := mcp.Tool{
					Name:        tool.Name,
					Description: tool.Description,
				}
				if len(tool.Parameters.Properties) == 0 {
					mcpTool.InputSchema.Type = "object"
				} else {
					mcpTool.InputSchema.Type = tool.Parameters.Type
					mcpTool.InputSchema.Properties = tool.Parameters.Properties.ToMap()
					mcpTool.InputSchema.Required = tool.Parameters.Required
				}

				capabilities.Tools = append(capabilities.Tools, server.ServerTool{
					Tool:    mcpTool,
					Handler: g.mcpToolHandler(tool),
				})
			}

			lock.Lock()
			allCapabilities = append(allCapabilities, capabilities)
			lock.Unlock()
		}
	}

	if err := errs.Wait(); err != nil {
		return nil, err
	}

	// Merge all capabilities
	var serverTools []server.ServerTool
	var serverPrompts []server.ServerPrompt
	var serverResources []server.ServerResource
	var serverResourceTemplates []ServerResourceTemplate
	for _, capabilities := range allCapabilities {
		serverTools = append(serverTools, capabilities.Tools...)
		serverPrompts = append(serverPrompts, capabilities.Prompts...)
		serverResources = append(serverResources, capabilities.Resources...)
		serverResourceTemplates = append(serverResourceTemplates, capabilities.ResourceTemplates...)
	}

	return &Capabilities{
		Tools:             serverTools,
		Prompts:           serverPrompts,
		Resources:         serverResources,
		ResourceTemplates: serverResourceTemplates,
	}, nil
}

func isToolEnabled(serverName, serverImage, toolName string, enabledTools []string) bool {
	if len(enabledTools) == 0 {
		return true
	}

	for _, enabled := range enabledTools {
		if enabled == "*" ||
			strings.EqualFold(enabled, toolName) ||
			strings.EqualFold(enabled, serverName+":"+toolName) ||
			strings.EqualFold(enabled, serverName+":*") ||
			strings.EqualFold(enabled, "*") {
			return true
		}
	}

	if serverImage != "" {
		for _, enabled := range enabledTools {
			if strings.EqualFold(enabled, serverImage+":"+toolName) ||
				strings.EqualFold(enabled, serverImage+":*") {
				return true
			}
		}
	}

	return false
}
