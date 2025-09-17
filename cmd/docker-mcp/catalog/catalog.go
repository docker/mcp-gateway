package catalog

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/pkg/yq"
)

const (
	DockerCatalogName     = "docker-mcp"
	DockerCatalogURL      = "https://desktop.docker.com/mcp/catalog/v2/catalog.yaml"
	DockerCatalogFilename = "docker-mcp.yaml"

	// Docker server names for bootstrap command
	DockerHubServerName = "dockerhub"
	DockerCLIServerName = "docker"
)

var aliasToURL = map[string]string{
	DockerCatalogName: DockerCatalogURL,
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
