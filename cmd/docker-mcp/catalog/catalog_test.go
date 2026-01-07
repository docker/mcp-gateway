package catalog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDockerCatalogURL(t *testing.T) {
	t.Run("returns v2 URL when DCR is disabled", func(t *testing.T) {
		url := GetDockerCatalogURL(false)
		expected := "https://desktop.docker.com/mcp/catalog/v2/catalog.yaml"
		assert.Equal(t, expected, url, "should return v2 URL when DCR is disabled")
	})

	t.Run("returns v3 URL when DCR is enabled", func(t *testing.T) {
		url := GetDockerCatalogURL(true)
		expected := "https://desktop.docker.com/mcp/catalog/v3/catalog.yaml"
		assert.Equal(t, expected, url, "should return v3 URL when DCR is enabled")
	})

	t.Run("matches DockerCatalogURL constant when DCR is disabled", func(t *testing.T) {
		url := GetDockerCatalogURL(false)
		assert.Equal(t, DockerCatalogURL, url, "should match DockerCatalogURL constant when DCR is disabled")
	})

	t.Run("matches DockerCatalogURLV3 constant when DCR is enabled", func(t *testing.T) {
		url := GetDockerCatalogURL(true)
		assert.Equal(t, DockerCatalogURLV3, url, "should match DockerCatalogURLV3 constant when DCR is enabled")
	})
}