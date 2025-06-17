package catalog

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/user"
	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/yq"
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

func toCatalogFilePath(name string) (string, error) {
	homeDir, err := user.HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".docker", configDir, catalogsDir, SanitizeFilename(name)+".yaml"), nil
}

func SanitizeFilename(input string) string {
	s := strings.TrimSpace(input)
	s = strings.ToLower(s)
	illegalChars := regexp.MustCompile(`[<>:"/\\|?*\x00]`)
	s = illegalChars.ReplaceAllString(s, "_")
	if len(s) > 250 {
		s = s[:250]
	}
	return s
}
