package workingset

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/docker/mcp-gateway/pkg/client"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/registryapi"
)

func Create(ctx context.Context, dao db.DAO, registryClient registryapi.Client, ociService oci.Service, id string, name string, servers []string, connectClients []string) error {
	var cfg client.Config
	if len(connectClients) > 0 {
		cfg = *client.ReadConfig()
		if err := verifySupportedClients(cfg, connectClients); err != nil {
			return err
		}
	}

	var err error
	if id != "" {
		_, err := dao.GetWorkingSet(ctx, id)
		if err == nil {
			return fmt.Errorf("profile with id %s already exists", id)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to look for existing profile: %w", err)
		}
	} else {
		id, err = createWorkingSetID(ctx, name, dao)
		if err != nil {
			return fmt.Errorf("failed to create profile id: %w", err)
		}
	}

	// Add default secrets
	secrets := make(map[string]Secret)
	secrets["default"] = Secret{
		Provider: SecretProviderDockerDesktop,
	}

	workingSet := WorkingSet{
		ID:      id,
		Name:    name,
		Version: CurrentWorkingSetVersion,
		Servers: make([]Server, 0),
		Secrets: secrets,
	}

	for _, server := range servers {
		ss, err := ResolveServersFromString(ctx, registryClient, ociService, dao, server)
		if err != nil {
			return err
		}
		workingSet.Servers = append(workingSet.Servers, ss...)
	}

	RegisterOAuthProvidersForServers(ctx, workingSet.Servers)

	if err := workingSet.Validate(); err != nil {
		return fmt.Errorf("invalid profile: %w", err)
	}

	err = dao.CreateWorkingSet(ctx, workingSet.ToDb())
	if err != nil {
		return fmt.Errorf("failed to create profile: %w", err)
	}

	if len(connectClients) > 0 {
		for _, c := range connectClients {
			if err := client.Connect(ctx, dao, "", cfg, c, true, workingSet.ID); err != nil {
				return fmt.Errorf("profile created, but failed to connect to client %s: %w", c, err)
			}
		}
	}

	fmt.Printf("Created profile %s with %d servers\n", id, len(workingSet.Servers))
	if len(connectClients) > 0 {
		fmt.Printf("Connected to clients: %s\n", strings.Join(connectClients, ", "))
	}

	return nil
}

func verifySupportedClients(cfg client.Config, clients []string) error {
	for _, c := range clients {
		if !client.IsSupportedMCPClient(cfg, c, true) {
			return fmt.Errorf("client %s is not supported. Supported clients: %s", c, strings.Join(client.GetSupportedMCPClients(cfg), ", "))
		}
	}
	return nil
}
