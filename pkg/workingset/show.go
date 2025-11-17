package workingset

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/pkg/db"
)

func Show(ctx context.Context, dao db.DAO, id string, format OutputFormat) error {
	dbSet, err := dao.GetWorkingSet(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("profile %s not found", id)
		}
		return fmt.Errorf("failed to get profile: %w", err)
	}

	workingSet := NewFromDb(dbSet)

	var data []byte
	switch format {
	case OutputFormatHumanReadable:
		data = []byte(printHumanReadable(workingSet))
	case OutputFormatJSON:
		data, err = json.MarshalIndent(workingSet, "", "  ")
	case OutputFormatYAML:
		data, err = yaml.Marshal(workingSet)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}

	fmt.Println(string(data))

	return nil
}

func printHumanReadable(workingSet WorkingSet) string {
	servers := ""
	for _, server := range workingSet.Servers {
		servers += fmt.Sprintf("  - Type: %s\n", server.Type)
		switch server.Type {
		case ServerTypeRegistry:
			servers += fmt.Sprintf("    Source: %s\n", server.Source)
		case ServerTypeImage:
			servers += fmt.Sprintf("    Image: %s\n", server.Image)
		}
		servers += fmt.Sprintf("    Config: %v\n", server.Config)
		servers += fmt.Sprintf("    Secrets: %s\n", server.Secrets)
		servers += fmt.Sprintf("    Tools: %v\n", server.Tools)
	}
	servers = strings.TrimSuffix(servers, "\n")
	secrets := ""
	for name, secret := range workingSet.Secrets {
		secrets += fmt.Sprintf("  - Name: %s\n", name)
		secrets += fmt.Sprintf("    Provider: %s\n", secret.Provider)
	}
	secrets = strings.TrimSuffix(secrets, "\n")
	return fmt.Sprintf("ID: %s\nName: %s\nServers:\n%s\nSecrets:\n%s", workingSet.ID, workingSet.Name, servers, secrets)
}
