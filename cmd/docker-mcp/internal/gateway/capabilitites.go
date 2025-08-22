package gateway

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/sync/errgroup"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/telemetry"
)

type Capabilities struct {
	Tools             []ToolRegistration
	Prompts           []PromptRegistration
	Resources         []ResourceRegistration
	ResourceTemplates []ResourceTemplateRegistration
}

type ToolRegistration struct {
	Tool    *mcp.Tool
	Handler mcp.ToolHandler
}

type PromptRegistration struct {
	Prompt  *mcp.Prompt
	Handler mcp.PromptHandler
}

type ResourceRegistration struct {
	Resource *mcp.Resource
	Handler  mcp.ResourceHandler
}

type ResourceTemplateRegistration struct {
	ResourceTemplate mcp.ResourceTemplate
	Handler          mcp.ResourceHandler
}

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
				client, err := g.clientPool.AcquireClient(ctx, serverConfig, nil)
				if err != nil {
					logf("  > Can't start %s: %s", serverConfig.Name, err)
					return nil
				}
				defer g.clientPool.ReleaseClient(client)

				var capabilities Capabilities

				tools, err := client.Session().ListTools(ctx, &mcp.ListToolsParams{})
				if err != nil {
					logf("  > Can't list tools %s: %s", serverConfig.Name, err)
				} else {
					// Record the number of tools discovered from this server
					telemetry.RecordToolList(ctx, serverConfig.Name, len(tools.Tools))

					for _, tool := range tools.Tools {
						if !isToolEnabled(configuration, serverConfig.Name, serverConfig.Spec.Image, tool.Name, g.ToolNames) {
							continue
						}
						capabilities.Tools = append(capabilities.Tools, ToolRegistration{
							Tool:    tool,
							Handler: g.mcpServerToolHandler(serverConfig, g.mcpServer, tool.Annotations),
						})
					}
				}

				prompts, err := client.Session().ListPrompts(ctx, &mcp.ListPromptsParams{})
				if err == nil {
					// Record the number of prompts discovered from this server
					telemetry.RecordPromptList(ctx, serverConfig.Name, len(prompts.Prompts))

					for _, prompt := range prompts.Prompts {
						capabilities.Prompts = append(capabilities.Prompts, PromptRegistration{
							Prompt:  prompt,
							Handler: g.mcpServerPromptHandler(serverConfig, g.mcpServer),
						})
					}
				}

				resources, err := client.Session().ListResources(ctx, &mcp.ListResourcesParams{})
				if err == nil {
					// Record the number of resources discovered from this server
					telemetry.RecordResourceList(ctx, serverConfig.Name, len(resources.Resources))

					for _, resource := range resources.Resources {
						capabilities.Resources = append(capabilities.Resources, ResourceRegistration{
							Resource: resource,
							Handler:  g.mcpServerResourceHandler(serverConfig, g.mcpServer),
						})
					}
				}

				resourceTemplates, err := client.Session().ListResourceTemplates(ctx, &mcp.ListResourceTemplatesParams{})
				if err == nil {
					// Record the number of resource templates discovered from this server
					telemetry.RecordResourceTemplateList(ctx, serverConfig.Name, len(resourceTemplates.ResourceTemplates))

					for _, resourceTemplate := range resourceTemplates.ResourceTemplates {
						capabilities.ResourceTemplates = append(capabilities.ResourceTemplates, ResourceTemplateRegistration{
							ResourceTemplate: *resourceTemplate,
							Handler:          g.mcpServerResourceHandler(serverConfig, g.mcpServer),
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
				if !isToolEnabled(configuration, serverName, "", tool.Name, g.ToolNames) {
					continue
				}

				mcpTool := mcp.Tool{
					Name:        tool.Name,
					Description: tool.Description,
					InputSchema: &jsonschema.Schema{},
				}
				// TODO: Properly convert tool.Parameters to jsonschema.Schema
				// For now, we'll create a simple schema structure
				if len(tool.Parameters.Properties) == 0 {
					mcpTool.InputSchema.Type = "object"
				} else {
					mcpTool.InputSchema.Type = tool.Parameters.Type
					// Note: tool.Parameters.Properties.ToMap() returns map[string]any
					// but we need map[string]*jsonschema.Schema
					// This is a complex conversion that needs proper implementation
				}

				capabilities.Tools = append(capabilities.Tools, ToolRegistration{
					Tool:    &mcpTool,
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
	var allTools []ToolRegistration
	var allPrompts []PromptRegistration
	var allResources []ResourceRegistration
	var allResourceTemplates []ResourceTemplateRegistration
	for _, capabilities := range allCapabilities {
		allTools = append(allTools, capabilities.Tools...)
		allPrompts = append(allPrompts, capabilities.Prompts...)
		allResources = append(allResources, capabilities.Resources...)
		allResourceTemplates = append(allResourceTemplates, capabilities.ResourceTemplates...)
	}

	return &Capabilities{
		Tools:             allTools,
		Prompts:           allPrompts,
		Resources:         allResources,
		ResourceTemplates: allResourceTemplates,
	}, nil
}

func (c *Capabilities) ToolNames() []string {
	var names []string
	for _, tool := range c.Tools {
		names = append(names, tool.Tool.Name)
	}
	return names
}

func (c *Capabilities) PromptNames() []string {
	var names []string
	for _, prompt := range c.Prompts {
		names = append(names, prompt.Prompt.Name)
	}
	return names
}

func isToolEnabled(configuration Configuration, serverName, serverImage, toolName string, enabledTools []string) bool {
	if len(enabledTools) == 0 {
		tools, exists := configuration.tools.ServerTools[serverName]
		if !exists {
			return true
		}

		return slices.Contains(tools, toolName)
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
