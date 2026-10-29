package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportSanitizedCatalogName(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	const name = "My:Catalog"
	content := []byte("name: My:Catalog\nregistry: {}\n")

	cfg := &Config{Catalogs: map[string]Catalog{name: {DisplayName: name}}}
	require.NoError(t, WriteConfig(cfg))
	require.NoError(t, WriteCatalogFile(name, content))

	out := filepath.Join(tempHome, "out", "export.yaml")
	require.NoError(t, Export(t.Context(), name, out))

	got, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}
