package catalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

func Update(ctx context.Context, args []string, mcpOAuthDcrEnabled bool) error {
	cfg, err := ReadConfig()
	if err != nil {
		return err
	}
	var names []string
	if len(args) == 0 {
		names = getAllCatalogNames(*cfg)
	}
	for _, arg := range args {
		if _, ok := cfg.Catalogs[arg]; ok {
			names = append(names, arg)
		} else {
			return fmt.Errorf("unknown catalog %q", arg)
		}
	}
	var errs []error
	for _, name := range names {
		catalog, ok := cfg.Catalogs[name]
		if !ok {
			continue
		}
		if err := updateCatalog(ctx, name, catalog, mcpOAuthDcrEnabled); err != nil {
			errs = append(errs, err)
		}
		fmt.Println("updated:", name)

	}
	return errors.Join(errs...)
}

func getAllCatalogNames(cfg Config) []string {
	var names []string
	for name := range cfg.Catalogs {
		names = append(names, name)
	}
	return names
}

func updateCatalog(ctx context.Context, name string, catalog Catalog, mcpOAuthDcrEnabled bool) error {
	url := catalog.URL

	var (
		catalogContent []byte
		err            error
	)
	// For the docker catalog, override URL to match the feature flag state if:
	// 1. No URL is set or invalid, OR
	// 2. The URL is an official v2/v3 catalog URL (prod or staging) that doesn't match the feature flag
	// This preserves truly custom URLs while ensuring official catalogs switch between v2/v3.
	if name == DockerCatalogName {
		if url == "" || !isValidURL(url) {
			url = GetDockerCatalogURL(mcpOAuthDcrEnabled)
		} else {
			// If it's an official v2/v3 catalog URL, always use the URL that matches the feature flag
			isV2URL := strings.Contains(url, "/catalog/v2/catalog.yaml")
			isV3URL := strings.Contains(url, "/catalog/v3/catalog.yaml")

			if isV2URL || isV3URL {
				url = GetDockerCatalogURL(mcpOAuthDcrEnabled)
			}
		}
	}

	if isValidURL(url) {
		catalogContent, err = DownloadFile(ctx, url)
	} else {
		catalogContent, err = os.ReadFile(url)
	}
	if err != nil {
		return err
	}

	cfg, err := ReadConfig()
	if err != nil {
		return err
	}
	cfg.Catalogs[name] = Catalog{
		DisplayName: catalog.DisplayName,
		URL:         catalog.URL,
		LastUpdate:  time.Now().Format(time.RFC3339),
	}
	if err := WriteConfig(cfg); err != nil {
		return err
	}

	if err := WriteCatalogFile(name, catalogContent); err != nil {
		return fmt.Errorf("failed to write catalog %q: %w", name, err)
	}
	return nil
}
