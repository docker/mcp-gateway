package catalog

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/config"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/telemetry"
)

func Rm(name string) error {
	// Initialize telemetry
	telemetry.Init()
	ctx := context.Background()

	start := time.Now()
	var success bool
	defer func() {
		duration := time.Since(start)
		telemetry.RecordCatalogOperation(ctx, "rm", name, float64(duration.Milliseconds()), success)
	}()

	// Prevent users from removing the Docker catalog
	if name == DockerCatalogName {
		return fmt.Errorf("cannot remove catalog '%s' as it is managed by Docker", name)
	}

	cfg, err := ReadConfig()
	if err != nil {
		return err
	}
	if _, ok := cfg.Catalogs[name]; !ok {
		return fmt.Errorf("catalog %q not found", name)
	}
	delete(cfg.Catalogs, name)
	if err := WriteConfig(cfg); err != nil {
		return err
	}
	if err := config.RemoveCatalogFile(name); err != nil {
		return err
	}

	fmt.Printf("removed catalog %q\n", name)
	success = true
	return nil
}
