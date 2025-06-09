package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"github.com/PaesslerAG/jsonpath"
)

type Feature struct {
	Enabled bool `json:"enabled"`
}

// CheckFeatureIsEnabled verifies if a feature is enabled in either admin-settings.json or Docker Desktop settings.
// settingName is the setting name (e.g. "enableDockerMCPToolkit", "enableDockerAI", etc.)
// label is the human-readable name of the feature for error messages
func CheckFeatureIsEnabled(ctx context.Context, settingName string, label string) error {
	// If there's an admin-settings.json file and the feature is locked=`true` with value=`false`,
	// then the feature is always disabled.
	adminSettings, err := getAdminSettings()
	if err == nil {
		locked, _ := jsonpath.Get("$."+settingName+".locked", adminSettings)
		if locked == true {
			value, _ := jsonpath.Get("$."+settingName+".value", adminSettings)
			if value == false {
				return errors.New("The \"" + label + "\" feature needs to be enabled by your Administrator")
			}
		}
	}

	// Otherwise, check that the feature is enabled in the Docker Desktop settings.
	settings, err := getSettings(ctx)
	if err != nil {
		//nolint:staticcheck
		return errors.New("Docker Desktop is not running")
	}
	value, _ := jsonpath.Get("$.desktop."+settingName+".value", settings)
	if value == false {
		return errors.New("The \"" + label + "\" feature needs to be enabled in Docker Desktop Settings")
	}

	return nil
}

func getAdminSettings() (map[string]any, error) {
	buf, err := os.ReadFile(Paths().AdminSettingPath)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(buf, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func getSettings(ctx context.Context) (any, error) {
	var result any
	if err := ClientBackend.Get(ctx, "/app/settings", &result); err != nil {
		return nil, err
	}
	return result, nil
}
