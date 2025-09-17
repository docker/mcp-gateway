package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/pkg/telemetry"
)

func Ls(ctx context.Context, format Format) error {
	// Initialize telemetry
	telemetry.Init()

	start := time.Now()
	cfg, err := ReadConfigWithDefaultCatalog(ctx)
	duration := time.Since(start)

	if err != nil {
		telemetry.RecordCatalogOperation(ctx, "ls", "", float64(duration.Milliseconds()), false)
		return err
	}

	// Record successful operation
	telemetry.RecordCatalogOperation(ctx, "ls", "all", float64(duration.Milliseconds()), true)

	if format == JSON {
		data, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	} else {
		humanPrintCatalog(*cfg)
	}

	return nil
}

func humanPrintCatalog(cfg Config) {
	if len(cfg.Catalogs) == 0 {
		fmt.Println("No catalogs configured.")
		return
	}

	for name, catalog := range cfg.Catalogs {
		fmt.Printf("%s: %s\n", name, catalog.DisplayName)
	}
}
