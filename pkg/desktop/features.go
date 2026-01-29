package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"time"

	"github.com/PaesslerAG/jsonpath"
)

type Feature struct {
	Enabled bool `json:"enabled"`
}

func CheckFeatureFlagIsEnabled(ctx context.Context, featureName string) (bool, error) {
	// Copied from https://github.com/docker/ai/commit/ae5c7d328f8aa42bc63d9398157a0673de9ffcf5
	// Save and restore working directory because pinata code might change it.
	wd, err := os.Getwd()
	if err != nil {
		return false, err
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	features, err := getFeatures(ctx)
	if err != nil {
		//nolint:staticcheck
		return false, errors.New("Docker Desktop is not running")
	}

	return isFeatureEnabled(featureName, features), nil
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

func CheckDesktopIsRunning(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	if err := ClientBackend.Get(ctx, "/ping", nil); err != nil {
		//nolint:staticcheck
		return errors.New("Docker Desktop is not running")
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

func getFeatures(ctx context.Context) (map[string]Feature, error) {
	var result map[string]Feature
	if err := ClientBackend.Get(ctx, "/features", &result); err != nil {
		return nil, err
	}
	return result, nil
}

func isFeatureEnabled(featureName string, features map[string]Feature) bool {
	for name, feature := range features {
		if name == featureName && feature.Enabled {
			return true
		}
	}
	return false
}

// noDockerDesktopContextKey is the context key for skipping Desktop detection.
type noDockerDesktopContextKey struct{}

// WithNoDockerDesktop marks the context so Desktop detection returns false.
func WithNoDockerDesktop(ctx context.Context) context.Context {
	return context.WithValue(ctx, noDockerDesktopContextKey{}, true)
}

// IsRunningInDockerDesktop checks if the CLI is running with Docker Desktop.
func IsRunningInDockerDesktop(ctx context.Context) bool {
	// Allow callers to force CE mode by disabling Desktop detection via
	// context. This is useful for tests or standalone gateway operation.
	if ctx != nil {
		if v, ok := ctx.Value(noDockerDesktopContextKey{}).(bool); ok && v {
			return false
		}
	}

	// When running inside the gateway container (DOCKER_MCP_IN_CONTAINER=1), we
	// must not touch the Docker API before the CLI is fully initialized. The
	// plugin lifecycle initializes the Docker CLI later, so probing here would
	// fail with "no context store initialized". In this mode we skip probing.
	if os.Getenv("DOCKER_MCP_IN_CONTAINER") == "1" {
		return false
	}

	// Always running in Docker Desktop on Windows and macOS
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return true
	}

	// Otherwise, on Linux check if Docker Desktop is running
	// Hacky, but it's the only way to check before PersistentPreRunE is called with the plugin
	if err := CheckDesktopIsRunning(ctx); err != nil {
		// If we can't check, assume we're not running in Docker Desktop
		return false
	}
	return true
}
