package workingset

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/registryapi"
)

func Create(ctx context.Context, dao db.DAO, registryClient registryapi.Client, ociService oci.Service, id string, name string, servers []string) error {
	var err error
	if id != "" {
		_, err := dao.GetWorkingSet(ctx, id)
		if err == nil {
			return fmt.Errorf("working set with id %s already exists", id)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to look for existing working set: %w", err)
		}
	} else {
		id, err = createWorkingSetID(ctx, name, dao)
		if err != nil {
			return fmt.Errorf("failed to create working set id: %w", err)
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
		Servers: make([]Server, len(servers)),
		Secrets: secrets,
	}

	for i, server := range servers {
		s, err := resolveServerFromString(ctx, registryClient, ociService, server)
		if err != nil {
			return err
		}
		workingSet.Servers[i] = s
	}

	if err := workingSet.Validate(); err != nil {
		return fmt.Errorf("invalid working set: %w", err)
	}

	err = dao.CreateWorkingSet(ctx, workingSet.ToDb())
	if err != nil {
		return fmt.Errorf("failed to create working set: %w", err)
	}

	fmt.Printf("Created working set %s with %d servers\n", id, len(workingSet.Servers))

	return nil
}
