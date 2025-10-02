package catalog

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/pkg/yq"
)

const (
	DockerCatalogName       = "docker-mcp"
	DockerCatalogURLV2      = "https://desktop.docker.com/mcp/catalog/v2/catalog.yaml"
	DockerCatalogURLV3      = "https://desktop.docker.com/mcp/catalog/v3/catalog.yaml"
	DockerCatalogURL        = DockerCatalogURLV2 // Default to v2
	DockerCatalogFilename   = "docker-mcp.yaml"

	// Docker server names for bootstrap command
	DockerHubServerName = "dockerhub"
	DockerCLIServerName = "docker"
)

var aliasToURL = map[string]string{
	DockerCatalogName: DockerCatalogURL,
}

// GetDockerCatalogURL returns the appropriate Docker catalog URL based on the mcp-oauth-dcr flag
func GetDockerCatalogURL(mcpOAuthDcrEnabled bool) string {
	if mcpOAuthDcrEnabled {
		return DockerCatalogURLV3
	}
	return DockerCatalogURLV2
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
