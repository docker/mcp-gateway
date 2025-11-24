package client

import "context"

func Disconnect(ctx context.Context, cwd string, config Config, vendor string, global bool) error {
	if vendor == VendorCodex {
		if !global {
			return ErrCodexOnlySupportsGlobalConfiguration
		}
		if err := DisconnectCodex(ctx); err != nil {
			return err
		}
	} else if vendor == VendorGordon && global {
		if err := DisconnectGordon(ctx); err != nil {
			return err
		}
	} else {
		updater, err := getUpdater(vendor, global, cwd, config)
		if err != nil {
			return err
		}
		if err := updater(DockerMCPCatalog, nil); err != nil {
			return err
		}
	}
	return nil
}
