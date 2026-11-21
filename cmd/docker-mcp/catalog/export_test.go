package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExportSanitizedCatalogName(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const name = "My:Catalog"
	content := []byte("name: My:Catalog\nregistry: {}\n")
	require.NoError(t, WriteConfig(&Config{Catalogs: map[string]Catalog{
		name: {DisplayName: name},
	}}))
	require.NoError(t, WriteCatalogFile(name, content))

	output := filepath.Join(t.TempDir(), "catalog.yaml")
	require.NoError(t, Export(t.Context(), name, output))
	actual, err := os.ReadFile(output)
	require.NoError(t, err)
	require.Equal(t, content, actual)
}
