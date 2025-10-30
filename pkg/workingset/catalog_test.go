package workingset

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCatalogFromWorkingSet(t *testing.T) {
	workingSet := WorkingSet{
		Version: CurrentWorkingSetVersion,
		ID:      "test-id",
		Name:    "Test Working Set",
		Servers: []Server{
			{
				Type:   ServerTypeRegistry,
				Source: "https://example.com/server",
				Config: map[string]any{"key": "value"},
				Tools:  []string{"tool1", "tool2"},
			},
			{
				Type:  ServerTypeImage,
				Image: "docker/test:latest",
				Tools: []string{"tool3"},
			},
		},
		Secrets: map[string]Secret{
			"default": {Provider: SecretProviderDockerDesktop},
		},
	}

	catalog := NewCatalogFromWorkingSet(workingSet)

	assert.Equal(t, "Test Working Set", catalog.Name)
	assert.Len(t, catalog.Servers, 2)

	// Check registry server
	assert.Equal(t, "registry", catalog.Servers[0].Type)
	assert.Equal(t, "https://example.com/server", catalog.Servers[0].Source)
	assert.Equal(t, map[string]any{"key": "value"}, catalog.Servers[0].Config)
	assert.Equal(t, []string{"tool1", "tool2"}, catalog.Servers[0].Tools)

	// Check image server
	assert.Equal(t, "image", catalog.Servers[1].Type)
	assert.Equal(t, "docker/test:latest", catalog.Servers[1].Image)
	assert.Equal(t, []string{"tool3"}, catalog.Servers[1].Tools)
}

func TestCatalogToWorkingSet(t *testing.T) {
	catalog := Catalog{
		Name: "Test Catalog",
		Servers: []CatalogServer{
			{
				Type:   "registry",
				Source: "https://example.com/server",
				Config: map[string]any{"key": "value"},
				Tools:  []string{"tool1", "tool2"},
			},
			{
				Type:  "image",
				Image: "docker/test:latest",
				Tools: []string{"tool3"},
			},
		},
	}

	workingSet := catalog.ToWorkingSet()

	assert.Equal(t, "Test Catalog", workingSet.Name)
	assert.Equal(t, CurrentWorkingSetVersion, workingSet.Version)
	assert.Len(t, workingSet.Servers, 2)

	// Check registry server
	assert.Equal(t, ServerTypeRegistry, workingSet.Servers[0].Type)
	assert.Equal(t, "https://example.com/server", workingSet.Servers[0].Source)
	assert.Equal(t, map[string]any{"key": "value"}, workingSet.Servers[0].Config)
	assert.Equal(t, "default", workingSet.Servers[0].Secrets)
	assert.Equal(t, []string{"tool1", "tool2"}, workingSet.Servers[0].Tools)

	// Check image server
	assert.Equal(t, ServerTypeImage, workingSet.Servers[1].Type)
	assert.Equal(t, "docker/test:latest", workingSet.Servers[1].Image)
	assert.Equal(t, "default", workingSet.Servers[1].Secrets)
	assert.Equal(t, []string{"tool3"}, workingSet.Servers[1].Tools)

	// Check default secrets were added
	assert.Len(t, workingSet.Secrets, 1)
	assert.Equal(t, SecretProviderDockerDesktop, workingSet.Secrets["default"].Provider)
}

func TestCatalogRoundTrip(t *testing.T) {
	original := Catalog{
		Name: "Test Catalog",
		Servers: []CatalogServer{
			{
				Type:   "registry",
				Source: "https://example.com/server",
				Config: map[string]any{"key": "value"},
				Tools:  []string{"tool1", "tool2"},
			},
		},
	}

	// Convert to working set and back
	workingSet := original.ToWorkingSet()
	roundTripped := NewCatalogFromWorkingSet(workingSet)

	// Name should be preserved
	assert.Equal(t, original.Name, roundTripped.Name)

	// Servers should be preserved
	assert.Len(t, roundTripped.Servers, len(original.Servers))
	assert.Equal(t, original.Servers[0].Type, roundTripped.Servers[0].Type)
	assert.Equal(t, original.Servers[0].Source, roundTripped.Servers[0].Source)
	assert.Equal(t, original.Servers[0].Config, roundTripped.Servers[0].Config)
	assert.Equal(t, original.Servers[0].Tools, roundTripped.Servers[0].Tools)
}

func TestCatalogWithEmptyServers(t *testing.T) {
	catalog := Catalog{
		Name:    "Empty Catalog",
		Servers: []CatalogServer{},
	}

	workingSet := catalog.ToWorkingSet()

	assert.Equal(t, "Empty Catalog", workingSet.Name)
	assert.Empty(t, workingSet.Servers)
	assert.Len(t, workingSet.Secrets, 1) // Default secret should still be added
}
