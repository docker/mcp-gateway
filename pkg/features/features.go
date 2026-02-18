package features

import (
	"context"

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

	features.runningDockerDesktop = desktop.IsRunningInDockerDesktop(ctx)

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

// WithEnabled returns a Features with only the named features enabled.
// Recognised names: "profiles".
func WithEnabled(names []string) Features {
	f := &featuresImpl{}
	for _, name := range names {
		switch name {
		case "profiles":
			f.profilesEnabled = true
		}
	}
	return f
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

func readProfilesFeature(ctx context.Context, dockerCli command.Cli, runningDockerDesktop bool) (bool, error) {
	if runningDockerDesktop {
		// Check DD feature flag
		return desktop.CheckFeatureFlagIsEnabled(ctx, "MCPWorkingSets")
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
