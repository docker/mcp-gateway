package workingset

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/docker/mcp-gateway/pkg/db"
	"gopkg.in/yaml.v3"
)

func Import(ctx context.Context, dao *db.Dao, filename string) error {
	workingSetBuf, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read working set file: %w", err)
	}

	// TODO add validation
	var workingSet WorkingSet
	if strings.HasSuffix(strings.ToLower(filename), ".yaml") {
		if err := yaml.Unmarshal(workingSetBuf, &workingSet); err != nil {
			return fmt.Errorf("failed to unmarshal working set: %w", err)
		}
	} else if strings.HasSuffix(strings.ToLower(filename), ".json") {
		if err := json.Unmarshal(workingSetBuf, &workingSet); err != nil {
			return fmt.Errorf("failed to unmarshal working set: %w", err)
		}
	} else {
		return fmt.Errorf("unsupported file extension: %s, must be .yaml or .json", filename)
	}

	if workingSet.Version != 1 {
		return fmt.Errorf("unsupported working set version: %d", workingSet.Version)
	}

	dbServers := make(db.ServerList, len(workingSet.Servers))
	for i, server := range workingSet.Servers {
		dbServers[i] = db.Server{
			Type:    string(server.Type),
			Config:  server.Config,
			Secrets: server.Secrets,
			Tools:   server.Tools,
		}
		if server.Type == ServerTypeRegistry {
			dbServers[i].Source = server.Source
		}
		if server.Type == ServerTypeImage {
			dbServers[i].Image = server.Image
		}
	}

	dbSecrets := make(db.SecretMap, len(workingSet.Secrets))
	for name, secret := range workingSet.Secrets {
		dbSecrets[name] = db.Secret{
			Provider: string(secret.Provider),
		}
	}

	dbSet := db.WorkingSet{
		ID:      workingSet.ID,
		Name:    workingSet.Name,
		Servers: dbServers,
		Secrets: dbSecrets,
	}

	existingSet, err := dao.GetWorkingSet(ctx, workingSet.ID)
	if err != nil {
		return fmt.Errorf("failed to get working set: %w", err)
	}

	if existingSet == nil {
		err = dao.CreateWorkingSet(ctx, dbSet)
		if err != nil {
			return fmt.Errorf("failed to create working set: %w", err)
		}
	} else {
		err = dao.UpdateWorkingSet(ctx, dbSet)
		if err != nil {
			return fmt.Errorf("failed to update working set: %w", err)
		}
	}

	return nil
}
