package catalog

import (
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/yq"
)

const (
	DockerCatalogName = "docker-mcp"
	DockerCatalogURL  = "https://desktop.docker.com/mcp/catalog/v2/catalog.yaml"
)

var aliasToURL = map[string]string{
	DockerCatalogName: DockerCatalogURL,
}

func NewCatalogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "catalog",
		Aliases: []string{"catalogs"},
		Short:   "Manage the catalog",
	}
	cmd.AddCommand(newImportCmd())
	cmd.AddCommand(newLsCommand())
	cmd.AddCommand(newRmCommand())
	cmd.AddCommand(newUpdateCommand())
	cmd.AddCommand(newShowCommand())
	cmd.AddCommand(newForkCmd())
	cmd.AddCommand(newCreateCmd())
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newAddCommand())
	cmd.AddCommand(newResetCommand())
	return cmd
}

type MetaData struct {
	Name        string `yaml:"name,omitempty"`
	DisplayName string `yaml:"displayName,omitempty"`
}

func readCatalogMetaData(yamlData []byte) (*MetaData, error) {
	var data MetaData
	if err := yaml.Unmarshal(yamlData, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}
	return &data, nil
}

func setCatalogMetaData(yamlData []byte, meta MetaData) ([]byte, error) {
	if len(yamlData) == 0 {
		yamlData = []byte("null")
	}
	query := fmt.Sprintf(`.name = "%s" | .displayName = "%s"`, meta.Name, meta.DisplayName)
	return yq.Evaluate(query, yamlData, yq.NewYamlDecoder(), yq.NewYamlEncoder())
}
