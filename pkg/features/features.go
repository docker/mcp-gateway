package features

import (
	"context"
	"os"
	"runtime"

	"github.com/docker/cli/cli/command"

	"github.com/docker/mcp-gateway/pkg/desktop"
)

type Features interface {
	InitError() error
	IsProfilesFeatureEnabled() bool
	IsRunningInDockerDesktop() bool
}

type featuresImpl struct {
	initErr              error
	runningDockerDesktop bool
	profilesEnabled      bool
}

var _ Features = &featuresImpl{}

func New(ctx context.Context, dockerCli command.Cli) (result Features) {
	features := &featuresImpl{}
	result = features

	features.runningDockerDesktop = isRunningInDockerDesktop(ctx)

	features.profilesEnabled, features.initErr = readProfilesFeature(ctx, dockerCli, features.runningDockerDesktop)
	return
}

func AllEnabled() Features {
	return &featuresImpl{
		runningDockerDesktop: true,
		profilesEnabled:      true,
	}
}

func AllDisabled() Features {
	return &featuresImpl{
		runningDockerDesktop: false,
		profilesEnabled:      false,
	}
}

func (f *featuresImpl) InitError() error {
	return f.initErr
}

func (f *featuresImpl) IsProfilesFeatureEnabled() bool {
	return f.profilesEnabled
}

func (f *featuresImpl) IsRunningInDockerDesktop() bool {
	return f.runningDockerDesktop
}

func isRunningInDockerDesktop(ctx context.Context) bool {
	// Not running Docker Desktop in a container
	if os.Getenv("DOCKER_MCP_IN_CONTAINER") == "1" {
		return false
	}

	// Always running in Docker Desktop on Windows and macOS
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return true
	}

	// Otherwise, on Linux check if Docker Desktop is running
	// Hacky, but it's the only way to check before PersistentPreRunE is called with the plugin
	if err := desktop.CheckDesktopIsRunning(ctx); err != nil {
		// If we can't check, assume we're not running in Docker Desktop
		return false
	}
	return true
}

func readProfilesFeature(ctx context.Context, dockerCli command.Cli, runningDockerDesktop bool) (bool, error) {
	if runningDockerDesktop {
		// Check DD feature flag
		return desktop.CheckProfilesFeatureIsEnabled(ctx)
	}

	// Otherwise, check the profiles feature in Docker CE or in a container
	return isProfilesCLIFeatureEnabled(dockerCli), nil
}

func isProfilesCLIFeatureEnabled(dockerCli command.Cli) bool {
	configFile := dockerCli.ConfigFile()
	if configFile == nil || configFile.Features == nil {
		return false
	}

	value, exists := configFile.Features["profiles"]
	if !exists {
		return false
	}
	return value == "enabled"
}
