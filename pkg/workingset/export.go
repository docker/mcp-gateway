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

func Export(ctx context.Context, dao *db.Dao, id string, filename string) error {
	dbSet, err := dao.GetWorkingSet(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get working set: %w", err)
	}
	if dbSet == nil {
		return fmt.Errorf("working set %s not found", id)
	}

	servers := make([]Server, len(dbSet.Servers))
	for i, server := range dbSet.Servers {
		servers[i] = Server{
			Type:    ServerType(server.Type),
			Config:  server.Config,
			Secrets: server.Secrets,
			Tools:   server.Tools,
		}
		if server.Type == "registry" {
			servers[i].Source = server.Source
		}
		if server.Type == "image" {
			servers[i].Image = server.Image
		}
	}

	secrets := make(map[string]Secret)
	for name, secret := range dbSet.Secrets {
		secrets[name] = Secret{
			Provider: SecretProvider(secret.Provider),
		}
	}

	workingSet := WorkingSet{
		Version: 1,
		ID:      dbSet.ID,
		Name:    dbSet.Name,
		Servers: servers,
		Secrets: secrets,
	}

	var data []byte
	if strings.HasSuffix(strings.ToLower(filename), ".yaml") {
		data, err = yaml.Marshal(workingSet)
	} else if strings.HasSuffix(strings.ToLower(filename), ".json") {
		data, err = json.MarshalIndent(workingSet, "", "  ")
	} else {
		return fmt.Errorf("unsupported file extension: %s, must be .yaml or .json", filename)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal working set: %w", err)
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write working set: %w", err)
	}

	return nil
}
