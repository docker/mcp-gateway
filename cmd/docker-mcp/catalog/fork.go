package catalog

import (
	"fmt"
)

func Fork(src, dst string) error {
	// Prevent users from creating a destination catalog with the Docker name
	if dst == DockerCatalogName {
		return fmt.Errorf("cannot create catalog '%s' as it is reserved for Docker's official catalog", dst)
	}

	cfg, err := ReadConfig()
	if err != nil {
		return err
	}

	// Special handling for Docker catalog - it exists but not in cfg.Catalogs
	if src != DockerCatalogName {
		if _, ok := cfg.Catalogs[src]; !ok {
			return fmt.Errorf("catalog %q not found", src)
		}
	}

	if _, ok := cfg.Catalogs[dst]; ok {
		return fmt.Errorf("catalog %q already exists", dst)
	}
	dstDisplayName := fmt.Sprintf("%s (forked from %s)", dst, src)
	cfg.Catalogs[dst] = Catalog{DisplayName: dstDisplayName}
	if err := WriteConfig(cfg); err != nil {
		return err
	}
	content, err := ReadCatalogFile(src)
	if err != nil {
		return err
	}
	dstContent, err := setCatalogMetaData(content, MetaData{DisplayName: dstDisplayName, Name: dst})
	if err != nil {
		return err
	}
	return WriteCatalogFile(dst, dstContent)
}
