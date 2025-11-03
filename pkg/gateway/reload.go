package gateway

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/prompts"
	// "github.com/docker/mcp-gateway/pkg/prompts"
)

func (g *Gateway) reloadConfiguration(ctx context.Context, configuration Configuration, serverNames []string, clientConfig *clientConfig) error {
	// Which servers are enabled in the registry.yaml?
	if len(serverNames) == 0 {
		serverNames = configuration.ServerNames()
	}
	if len(serverNames) == 0 {
		log.Log("- No server is enabled")
	} else {
		log.Log("- Those servers are enabled:", strings.Join(serverNames, ", "))
	}

	// List all the available tools.
	startList := time.Now()
	log.Log("- Listing MCP tools...")
	capabilities, err := g.listCapabilities(ctx, serverNames, clientConfig)
	if err != nil {
		return fmt.Errorf("listing resources: %w", err)
	}
	log.Log(">", len(capabilities.Tools), "tools listed in", time.Since(startList))

	// Update capabilities
	// Clear existing capabilities per server and register new ones

	// Lock for reading/writing capability tracking
	g.capabilitiesMu.Lock()
	defer g.capabilitiesMu.Unlock()

	// Clear all existing capabilities from tracked servers
	for _, oldCaps := range g.serverCapabilities {
		if len(oldCaps.ToolNames) > 0 {
			g.mcpServer.RemoveTools(oldCaps.ToolNames...)
		}
		if len(oldCaps.PromptNames) > 0 {
			g.mcpServer.RemovePrompts(oldCaps.PromptNames...)
		}
		if len(oldCaps.ResourceURIs) > 0 {
			g.mcpServer.RemoveResources(oldCaps.ResourceURIs...)
		}
		if len(oldCaps.ResourceTemplateURIs) > 0 {
			g.mcpServer.RemoveResourceTemplates(oldCaps.ResourceTemplateURIs...)
		}
	}

	// Clear the tracking maps - we'll rebuild them
	g.serverCapabilities = make(map[string]*ServerCapabilities)
	g.toolRegistrations = make(map[string]ToolRegistration)

	// Add new capabilities and track them per server
	for _, tool := range capabilities.Tools {
		g.mcpServer.AddTool(tool.Tool, tool.Handler)

		// Track by server
		if g.serverCapabilities[tool.ServerName] == nil {
			g.serverCapabilities[tool.ServerName] = &ServerCapabilities{}
		}
		g.serverCapabilities[tool.ServerName].ToolNames = append(
			g.serverCapabilities[tool.ServerName].ToolNames,
			tool.Tool.Name,
		)

		// Track tool registration for mcp-exec
		g.toolRegistrations[tool.Tool.Name] = tool
	}

	// Add internal tools when dynamic-tools feature is enabled
	if g.DynamicTools {
		log.Log("- Adding internal tools (dynamic-tools feature enabled)")

		// Add mcp-find tool
		mcpFindTool := g.createMcpFindTool(configuration)
		g.mcpServer.AddTool(mcpFindTool.Tool, mcpFindTool.Handler)
		g.toolRegistrations[mcpFindTool.Tool.Name] = *mcpFindTool

		// Add mcp-add tool
		mcpAddTool := g.createMcpAddTool(clientConfig)
		g.mcpServer.AddTool(mcpAddTool.Tool, mcpAddTool.Handler)
		g.toolRegistrations[mcpAddTool.Tool.Name] = *mcpAddTool

		// Add mcp-remove tool
		mcpRemoveTool := g.createMcpRemoveTool()
		g.mcpServer.AddTool(mcpRemoveTool.Tool, mcpRemoveTool.Handler)
		g.toolRegistrations[mcpRemoveTool.Tool.Name] = *mcpRemoveTool

		// Add codemode
		codeModeTool := g.createCodeModeTool(clientConfig)
		g.mcpServer.AddTool(codeModeTool.Tool, codeModeTool.Handler)
		g.toolRegistrations[codeModeTool.Tool.Name] = *codeModeTool

		// Add mcp-exec tool
		mcpExecTool := g.createMcpExecTool()
		g.mcpServer.AddTool(mcpExecTool.Tool, mcpExecTool.Handler)
		g.toolRegistrations[mcpExecTool.Tool.Name] = *mcpExecTool

		// Add mcp-config-set tool
		mcpConfigSetTool := g.createMcpConfigSetTool(clientConfig)
		g.mcpServer.AddTool(mcpConfigSetTool.Tool, mcpConfigSetTool.Handler)
		g.toolRegistrations[mcpConfigSetTool.Tool.Name] = *mcpConfigSetTool

		log.Log("  > mcp-find: tool for finding MCP servers in the catalog")
		log.Log("  > mcp-add: tool for adding MCP servers to the registry")
		log.Log("  > mcp-remove: tool for removing MCP servers from the registry")
		log.Log("  > mcp-config-set: tool for setting configuration values for MCP servers")
		log.Log("  > code-mode: write code that calls other MCPs directly")
		log.Log("  > mcp-exec: execute tools that exist in the current session")

		// Add mcp-registry-import tool
		// mcpRegistryImportTool := g.createMcpRegistryImportTool(configuration, clientConfig)
		// g.mcpServer.AddTool(mcpRegistryImportTool.Tool, mcpRegistryImportTool.Handler)
		// g.toolRegistrations[mcpRegistryImportTool.Tool.Name] = *mcpRegistryImportTool

		// Add mcp-session-name tool
		// mcpSessionNameTool := g.createMcpSessionNameTool()
		// g.mcpServer.AddTool(mcpSessionNameTool.Tool, mcpSessionNameTool.Handler)
		// g.toolRegistrations[mcpSessionNameTool.Tool.Name] = *mcpSessionNameTool
		// log.Log("  > mcp-registry-import: tool for importing servers from MCP registry URLs")
		// log.Log("  > mcp-session-name: tool for setting session name to persist configuration")

		// Add prompt
		prompts.AddDiscoverPrompt(g.mcpServer)
		log.Log("  > mcp-discover: prompt for learning about dynamic server management")
	}

	for _, prompt := range capabilities.Prompts {
		g.mcpServer.AddPrompt(prompt.Prompt, prompt.Handler)

		// Track by server
		if g.serverCapabilities[prompt.ServerName] == nil {
			g.serverCapabilities[prompt.ServerName] = &ServerCapabilities{}
		}
		g.serverCapabilities[prompt.ServerName].PromptNames = append(
			g.serverCapabilities[prompt.ServerName].PromptNames,
			prompt.Prompt.Name,
		)
	}

	for _, resource := range capabilities.Resources {
		g.mcpServer.AddResource(resource.Resource, resource.Handler)

		// Track by server
		if g.serverCapabilities[resource.ServerName] == nil {
			g.serverCapabilities[resource.ServerName] = &ServerCapabilities{}
		}
		g.serverCapabilities[resource.ServerName].ResourceURIs = append(
			g.serverCapabilities[resource.ServerName].ResourceURIs,
			resource.Resource.URI,
		)
	}

	// Resource templates are handled as regular resources in the new SDK
	for _, template := range capabilities.ResourceTemplates {
		// Convert ResourceTemplate to Resource
		resource := &mcp.ResourceTemplate{
			URITemplate: template.ResourceTemplate.URITemplate,
			Name:        template.ResourceTemplate.Name,
			Description: template.ResourceTemplate.Description,
			MIMEType:    template.ResourceTemplate.MIMEType,
		}
		g.mcpServer.AddResourceTemplate(resource, template.Handler)

		// Track by server
		if g.serverCapabilities[template.ServerName] == nil {
			g.serverCapabilities[template.ServerName] = &ServerCapabilities{}
		}
		g.serverCapabilities[template.ServerName].ResourceTemplateURIs = append(
			g.serverCapabilities[template.ServerName].ResourceTemplateURIs,
			resource.URITemplate,
		)
	}

	g.health.SetHealthy()

	return nil
}

// stringSliceToSet converts a slice to a map for efficient lookup
func stringSliceToSet(slice []string) map[string]bool {
	set := make(map[string]bool, len(slice))
	for _, s := range slice {
		set[s] = true
	}
	return set
}

// diffStringSlices returns items that are in 'newer' but not in 'older' (additions),
// and items that are in 'older' but not in 'newer' (removals)
func diffStringSlices(older, newer []string) (additions, removals []string) {
	oldSet := stringSliceToSet(older)
	newSet := stringSliceToSet(newer)

	for s := range newSet {
		if !oldSet[s] {
			additions = append(additions, s)
		}
	}

	for s := range oldSet {
		if !newSet[s] {
			removals = append(removals, s)
		}
	}

	return additions, removals
}

// allCapabilities builds a ServerCapabilities struct from the available capabilities for a server.
// This function expects g.capabilitiesMu to be locked by the caller.
func (g *Gateway) allCapabilities(serverName string) *ServerCapabilities {
	availableCaps := g.serverAvailableCapabilities[serverName]
	if availableCaps == nil {
		return &ServerCapabilities{}
	}

	newCaps := &ServerCapabilities{
		ToolNames:            make([]string, 0, len(availableCaps.Tools)),
		PromptNames:          make([]string, 0, len(availableCaps.Prompts)),
		ResourceURIs:         make([]string, 0, len(availableCaps.Resources)),
		ResourceTemplateURIs: make([]string, 0, len(availableCaps.ResourceTemplates)),
	}

	for _, tool := range availableCaps.Tools {
		newCaps.ToolNames = append(newCaps.ToolNames, tool.Tool.Name)
	}
	for _, prompt := range availableCaps.Prompts {
		newCaps.PromptNames = append(newCaps.PromptNames, prompt.Prompt.Name)
	}
	for _, resource := range availableCaps.Resources {
		newCaps.ResourceURIs = append(newCaps.ResourceURIs, resource.Resource.URI)
	}
	for _, template := range availableCaps.ResourceTemplates {
		newCaps.ResourceTemplateURIs = append(newCaps.ResourceTemplateURIs, template.ResourceTemplate.URITemplate)
	}

	return newCaps
}

func (g *Gateway) reloadServerCapabilities(ctx context.Context, serverName string, clientConfig *clientConfig) (*ServerCapabilities, error) {
	// Find the server configuration in current config
	serverConfig, _, found := g.configuration.Find(serverName)
	if !found || serverConfig == nil {
		return nil, fmt.Errorf("server %s not found in configuration", serverName)
	}

	// Get current newServerCaps from the server (this reflects the server's current state after it notified us of changes)
	newServerCaps, err := g.listCapabilities(ctx, []string{serverName}, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to list capabilities for %s: %w", serverName, err)
	}

	// Lock for reading/writing capability tracking
	g.capabilitiesMu.Lock()
	defer g.capabilitiesMu.Unlock()

	// Save old capabilities before updating
	oldCaps := g.serverCapabilities[serverName]
	if oldCaps == nil {
		oldCaps = &ServerCapabilities{}
	}

	// Store the full capabilities
	g.serverAvailableCapabilities[serverName] = newServerCaps

	// Update tool registrations for this server
	// This happens regardless of activation so tools can be called via mcp-exec
	if oldCaps != nil {
		// Remove old tool registrations for this server
		for _, toolName := range oldCaps.ToolNames {
			delete(g.toolRegistrations, toolName)
		}
	}
	// Add new tool registrations from the server
	for _, tool := range newServerCaps.Tools {
		g.toolRegistrations[tool.Tool.Name] = tool
	}

	// Return old capabilities for the caller to use with updateServerCapabilities
	// The caller should use g.allCapabilities(serverName) to get newCaps
	// The full capabilities (newServerCaps) are now in g.serverAvailableCapabilities[serverName]
	// g.serverCapabilities will be set by updateServerCapabilities after all updates succeed
	return oldCaps, nil
}

// updateServerCapabilities updates g.mcpServer with capabilities from the server.
// If toolFilter is non-nil, only tools in the filter will be added.
// This function expects g.capabilitiesMu to be locked by the caller.
func (g *Gateway) updateServerCapabilities(serverName string, oldCaps, newCaps *ServerCapabilities, toolFilter []string) error {
	// Get the full capabilities from serverAvailableCapabilities
	newServerCaps := g.serverAvailableCapabilities[serverName]
	if newServerCaps == nil {
		return fmt.Errorf("no available capabilities found for server %s", serverName)
	}

	// Determine what changed
	addedTools, removedTools := diffStringSlices(oldCaps.ToolNames, newCaps.ToolNames)
	addedPrompts, removedPrompts := diffStringSlices(oldCaps.PromptNames, newCaps.PromptNames)
	addedResources, removedResources := diffStringSlices(oldCaps.ResourceURIs, newCaps.ResourceURIs)
	addedTemplates, removedTemplates := diffStringSlices(oldCaps.ResourceTemplateURIs, newCaps.ResourceTemplateURIs)

	// Remove old capabilities that are no longer present
	if len(removedTools) > 0 {
		g.mcpServer.RemoveTools(removedTools...)
		// Remove from tool registrations tracking
		for _, toolName := range removedTools {
			delete(g.toolRegistrations, toolName)
		}
		log.Log("  - Removed", len(removedTools), "tools for", serverName)
	}

	if len(removedPrompts) > 0 {
		g.mcpServer.RemovePrompts(removedPrompts...)
		log.Log("  - Removed", len(removedPrompts), "prompts for", serverName)
	}

	if len(removedResources) > 0 {
		g.mcpServer.RemoveResources(removedResources...)
		log.Log("  - Removed", len(removedResources), "resources for", serverName)
	}

	if len(removedTemplates) > 0 {
		g.mcpServer.RemoveResourceTemplates(removedTemplates...)
		log.Log("  - Removed", len(removedTemplates), "resource templates for", serverName)
	}

	// Build tool filter set if provided
	var toolFilterSet map[string]bool
	if toolFilter != nil {
		toolFilterSet = stringSliceToSet(toolFilter)
	}

	// Add/update all capabilities from this server
	toolsAdded := 0
	for _, tool := range addedTools {
		// If tool filter is provided, only add tools in the filter
		if toolFilterSet != nil && !toolFilterSet[tool] {
			continue
		}
		if registration, err := newServerCaps.getToolByName(tool); err == nil {
			g.mcpServer.AddTool(registration.Tool, registration.Handler)
			toolsAdded++
		}
	}
	if toolsAdded > 0 {
		log.Log("  - Added/updated", toolsAdded, "tools for", serverName)
	}

	for _, prompt := range addedPrompts {
		if registration, err := newServerCaps.getPromptByName(prompt); err == nil {
			g.mcpServer.AddPrompt(registration.Prompt, registration.Handler)
		}
	}
	if len(addedPrompts) > 0 {
		log.Log("  - Added/updated", len(addedPrompts), "prompts for", serverName)
	}

	for _, resource := range addedResources {
		if registration, err := newServerCaps.getResourceByURI(resource); err == nil {
			g.mcpServer.AddResource(registration.Resource, registration.Handler)
		}
	}
	if len(addedResources) > 0 {
		log.Log("  - Added/updated", len(addedResources), "resources for", serverName)
	}

	for _, template := range addedTemplates {
		if registration, err := newServerCaps.getResourceTemplateByURITemplate(template); err == nil {
			g.mcpServer.AddResourceTemplate(&registration.ResourceTemplate, registration.Handler)
		}
	}
	if len(addedTemplates) > 0 {
		log.Log("  - Added/updated", len(addedTemplates), "resource templates for", serverName)
	}

	// Update the stored capabilities now that all updates succeeded
	g.serverCapabilities[serverName] = newCaps

	return nil
}

func (g *Gateway) removeServerConfiguration(_ context.Context, serverName string) error {
	// Find the server configuration in current config
	serverConfig, _, found := g.configuration.Find(serverName)
	if !found || serverConfig == nil {
		return fmt.Errorf("server %s not found in configuration", serverName)
	}

	// Lock for reading/writing capability tracking
	g.capabilitiesMu.Lock()
	defer g.capabilitiesMu.Unlock()

	// Get old capabilities for this server
	oldCaps := g.serverCapabilities[serverName]
	if oldCaps == nil {
		oldCaps = &ServerCapabilities{}
	}

	// Remove old capabilities that are no longer present
	if len(oldCaps.ToolNames) > 0 {
		g.mcpServer.RemoveTools(oldCaps.ToolNames...)
		log.Log("  - Removed", len(oldCaps.ToolNames), "tools for", serverName)
	}

	if len(oldCaps.PromptNames) > 0 {
		g.mcpServer.RemovePrompts(oldCaps.PromptNames...)
		log.Log("  - Removed", len(oldCaps.PromptNames), "prompts for", serverName)
	}

	if len(oldCaps.ResourceURIs) > 0 {
		g.mcpServer.RemoveResources(oldCaps.ResourceURIs...)
		log.Log("  - Removed", len(oldCaps.ResourceURIs), "resources for", serverName)
	}

	if len(oldCaps.ResourceTemplateURIs) > 0 {
		g.mcpServer.RemoveResourceTemplates(oldCaps.ResourceTemplateURIs...)
		log.Log("  - Removed", len(oldCaps.ResourceTemplateURIs), "resource templates for", serverName)
	}

	// Update tracking with new capabilities
	delete(g.serverCapabilities, serverName)

	return nil
}
