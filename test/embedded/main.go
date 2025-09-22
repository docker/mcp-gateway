package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/docker/cli/cli/command"
	"github.com/docker/mcp-gateway/pkg/docker"
	"github.com/docker/mcp-gateway/pkg/gateway"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a mock Docker CLI for testing (we won't actually run containers)
	mockCli := &command.DockerCli{}
	dockerClient := docker.NewClient(mockCli)

	// Create Gateway configuration
	config := gateway.Config{
		Options: gateway.Options{
			Transport: "stdio", // Use stdio transport mode
			Verbose:   true,
			DryRun:    false, // We need the server to actually run
			Static:    true,  // Don't pull images
		},
		ServerNames: []string{}, // Empty for now, no actual MCP servers
		CatalogPath: []string{},
		ConfigPath:  []string{},
		RegistryPath: []string{},
		ToolsPath:   []string{},
		SecretsPath: "",
	}

	// Create Gateway instance
	gw := gateway.NewGateway(config, dockerClient)

	// Create in-memory transports for Gateway<->Client communication
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	fmt.Println("Setting up Gateway with in-memory transport...")

	// Now use the actual Gateway with our custom transport method
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Gateway panic: %v", err)
			}
		}()

		// Use the Gateway's new RunWithTransport method
		if err := gw.RunWithTransport(ctx, serverTransport); err != nil {
			log.Printf("Gateway error: %v", err)
		}
	}()

	// Create MCP client
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "Test Client",
		Version: "1.0.0",
	}, nil)

	// Connect client to the test server
	fmt.Println("Connecting client to server...")
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		log.Fatalf("Failed to connect client to server: %v", err)
	}
	defer session.Close()

	fmt.Println("✅ Successfully connected to MCP Server!")

	// Test: List tools
	fmt.Println("\n📝 Listing tools...")
	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to list tools: %v", err)
	}

	fmt.Printf("Found %d tools:\n", len(toolsResult.Tools))
	for i, tool := range toolsResult.Tools {
		fmt.Printf("  %d. %s - %s\n", i+1, tool.Name, tool.Description)
	}

	// Test: List prompts
	fmt.Println("\n💬 Listing prompts...")
	promptsResult, err := session.ListPrompts(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to list prompts: %v", err)
	}

	fmt.Printf("Found %d prompts:\n", len(promptsResult.Prompts))
	for i, prompt := range promptsResult.Prompts {
		fmt.Printf("  %d. %s - %s\n", i+1, prompt.Name, prompt.Description)
	}

	// Test: List resources
	fmt.Println("\n📚 Listing resources...")
	resourcesResult, err := session.ListResources(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to list resources: %v", err)
	}

	fmt.Printf("Found %d resources:\n", len(resourcesResult.Resources))
	for i, resource := range resourcesResult.Resources {
		fmt.Printf("  %d. %s (%s) - %s\n", i+1, resource.Name, resource.URI, resource.Description)
	}

	// Test: Call tool only if tools exist
	if len(toolsResult.Tools) > 0 {
		fmt.Println("\n🔧 Calling first available tool...")
		toolResult, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: toolsResult.Tools[0].Name,
			Arguments: map[string]interface{}{},
		})
		if err != nil {
			log.Printf("Failed to call tool: %v", err)
		} else if len(toolResult.Content) > 0 {
			if textContent, ok := toolResult.Content[0].(*mcp.TextContent); ok {
				fmt.Printf("Tool result: %s\n", textContent.Text)
			} else {
				fmt.Printf("Tool result: %v\n", toolResult.Content[0])
			}
		}
	} else {
		fmt.Println("\n🔧 No tools available to call (Gateway has no configured MCP servers)")
	}

	// Wait a bit for cleanup
	time.Sleep(100 * time.Millisecond)
}
