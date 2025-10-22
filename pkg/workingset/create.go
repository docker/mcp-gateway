package workingset

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/mcp-gateway/pkg/db"
)

func Create(ctx context.Context, dao db.DAO, id string, name string, servers []string) error {
	var err error
	if id != "" {
		existingSet, err := dao.GetWorkingSet(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to look for existing working set: %w", err)
		}
		if existingSet != nil {
			return fmt.Errorf("working set with id %s already exists", id)
		}
	} else {
		id, err = createWorkingSetId(ctx, name, dao)
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
		// TODO finalize image schema
		if strings.HasPrefix(server, "docker://") {
			workingSet.Servers[i] = Server{
				Type:  ServerTypeImage,
				Image: strings.TrimPrefix(server, "docker://"),
			}
		} else {
			workingSet.Servers[i] = Server{
				Type:   ServerTypeRegistry,
				Source: server,
			}
		}
	}

	err = dao.CreateWorkingSet(ctx, workingSet.ToDb())
	if err != nil {
		return fmt.Errorf("failed to create working set: %w", err)
	}

	fmt.Printf("Created working set %s with %d servers\n", id, len(workingSet.Servers))

	return nil
}
