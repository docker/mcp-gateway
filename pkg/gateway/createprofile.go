package gateway

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func createProfileHandler(g *Gateway) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Parse parameters
		var params struct {
			Name string `json:"name"`
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

		profileName := params.Name

		// Create DAO and OCI service
		dao, err := db.New()
		if err != nil {
			return nil, fmt.Errorf("failed to create database client: %w", err)
		}

		ociService := oci.NewService()

		// Build the working set from current gateway state
		servers := make([]workingset.Server, 0, len(g.configuration.serverNames))
		for _, serverName := range g.configuration.serverNames {
			catalogServer, found := g.configuration.servers[serverName]
			if !found {
				log.Logf("Warning: server %s not found in catalog, skipping", serverName)
				continue
			}

			// Determine server type based on whether it has an image
			serverType := workingset.ServerTypeImage
			if catalogServer.Image == "" {
				// Skip servers without images for now (registry servers)
				log.Logf("Warning: server %s has no image, skipping", serverName)
				continue
			}

			// Get config for this server
			serverConfig := g.configuration.config[serverName]
			if serverConfig == nil {
				serverConfig = make(map[string]any)
			}

			// Get tools for this server
			var serverTools []string
			if g.configuration.tools.ServerTools != nil {
				serverTools = g.configuration.tools.ServerTools[serverName]
			}

			// Create server entry
			server := workingset.Server{
				Type:    serverType,
				Image:   catalogServer.Image,
				Config:  serverConfig,
				Secrets: "default",
				Tools:   serverTools,
				Snapshot: &workingset.ServerSnapshot{
					Server: catalogServer,
				},
			}

			servers = append(servers, server)
		}

		if len(servers) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: "No servers with images found in current gateway state. Cannot create profile.",
				}},
				IsError: true,
			}, nil
		}

		// Add default secrets
		secrets := make(map[string]workingset.Secret)
		secrets["default"] = workingset.Secret{
			Provider: workingset.SecretProviderDockerDesktop,
		}

		// Check if profile already exists
		existingProfile, err := dao.GetWorkingSet(ctx, profileName)
		isUpdate := false
		profileID := profileName

		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("failed to check for existing profile: %w", err)
			}
			// Profile doesn't exist, we'll create it
		} else {
			// Profile exists, we'll update it
			isUpdate = true
			profileID = existingProfile.ID
		}

		// Create working set
		ws := workingset.WorkingSet{
			Version: workingset.CurrentWorkingSetVersion,
			ID:      profileID,
			Name:    profileName,
			Servers: servers,
			Secrets: secrets,
		}

		// Ensure snapshots are resolved
		if err := ws.EnsureSnapshotsResolved(ctx, ociService); err != nil {
			return nil, fmt.Errorf("failed to resolve snapshots: %w", err)
		}

		// Validate the working set
		if err := ws.Validate(); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf("Profile validation failed: %v", err),
				}},
				IsError: true,
			}, nil
		}

		// Create or update the profile
		if isUpdate {
			if err := dao.UpdateWorkingSet(ctx, ws.ToDb()); err != nil {
				return nil, fmt.Errorf("failed to update profile: %w", err)
			}
			log.Logf("Updated profile %s with %d servers", profileID, len(servers))
		} else {
			if err := dao.CreateWorkingSet(ctx, ws.ToDb()); err != nil {
				return nil, fmt.Errorf("failed to create profile: %w", err)
			}
			log.Logf("Created profile %s with %d servers", profileID, len(servers))
		}

		// Build success message
		var resultMessage string
		if isUpdate {
			resultMessage = fmt.Sprintf("Successfully updated profile '%s' (ID: %s) with %d servers:\n", profileName, profileID, len(servers))
		} else {
			resultMessage = fmt.Sprintf("Successfully created profile '%s' (ID: %s) with %d servers:\n", profileName, profileID, len(servers))
		}

		// List the servers in the profile
		for i, server := range servers {
			serverName := server.Snapshot.Server.Name
			resultMessage += fmt.Sprintf("\n%d. %s", i+1, serverName)
			if server.Image != "" {
				resultMessage += fmt.Sprintf(" (image: %s)", server.Image)
			}
			if len(server.Tools) > 0 {
				resultMessage += fmt.Sprintf(" - %d tools", len(server.Tools))
			}
			if len(server.Config) > 0 {
				resultMessage += " - configured"
			}
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Text: resultMessage,
			}},
		}, nil
	}
}
