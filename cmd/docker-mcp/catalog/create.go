package catalog

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/telemetry"
)

func Create(name string) error {
	// Initialize telemetry
	telemetry.Init()
	ctx := context.Background()

	start := time.Now()
	var success bool
	defer func() {
		duration := time.Since(start)
		telemetry.RecordCatalogOperation(ctx, "create", name, float64(duration.Milliseconds()), success)
	}()

	// Prevent users from creating the Docker catalog
	if name == DockerCatalogName {
		return fmt.Errorf("cannot create catalog '%s' as it is reserved for Docker's official catalog", name)
	}

	cfg, err := ReadConfig()
	if err != nil {
		return err
	}
	if _, ok := cfg.Catalogs[name]; ok {
		return fmt.Errorf("catalog %q already exists", name)
	}
	cfg.Catalogs[name] = Catalog{DisplayName: name}
	if err := WriteConfig(cfg); err != nil {
		return err
	}
	content, err := setCatalogMetaData([]byte{}, MetaData{Name: name, DisplayName: name})
	if err != nil {
		return err
	}
	if err := WriteCatalogFile(name, content); err != nil {
		return err
	}
	fmt.Printf("created empty catalog %s\n", name)
	success = true
	return nil
}
