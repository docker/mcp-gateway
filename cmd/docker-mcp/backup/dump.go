package backup

import (
	"context"
	"encoding/json"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/config"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/docker"
)

func Dump(ctx context.Context, docker docker.Client) ([]byte, error) {
	configContent, err := config.ReadConfig(ctx, docker)
	if err != nil {
		return nil, err
	}

	registryContent, err := config.ReadRegistry(ctx, docker)
	if err != nil {
		return nil, err
	}

	catalogContent, err := config.ReadCatalog()
	if err != nil {
		return nil, err
	}

	catalogConfig, err := catalog.ReadConfig()
	if err != nil {
		return nil, err
	}

	catalogFiles := make(map[string]string)
	for name := range catalogConfig.Catalogs {
		catalogFileContent, err := config.ReadCatalogFile(name)
		if err != nil {
			return nil, err
		}
		catalogFiles[name] = string(catalogFileContent)
	}

	secretsClient := desktop.NewSecretsClient()
	storedSecrets, err := secretsClient.ListJfsSecrets(ctx)
	if err != nil {
		return nil, err
	}

	var secretNames []string
	for _, secret := range storedSecrets {
		secretNames = append(secretNames, secret.Name)
	}
	secretValues, err := docker.ReadSecrets(ctx, secretNames)
	if err != nil {
		return nil, err
	}

	var secrets []desktop.Secret
	for _, secret := range storedSecrets {
		secrets = append(secrets, desktop.Secret{
			Name:     secret.Name,
			Provider: secret.Provider,
			Value:    secretValues[secret.Name],
		})
	}

	policy, err := secretsClient.GetJfsPolicy(ctx)
	if err != nil {
		return nil, err
	}

	backup := Backup{
		Config:       string(configContent),
		Registry:     string(registryContent),
		Catalog:      string(catalogContent),
		CatalogFiles: catalogFiles,
		Secrets:      secrets,
		Policy:       policy,
	}

	return json.Marshal(backup)
}
