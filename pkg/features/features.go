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

func (f *featuresImpl) InitError() error {
	return f.initErr
}

func (f *featuresImpl) IsProfilesFeatureEnabled() bool {
	return f.profilesEnabled
}

func (f *featuresImpl) IsRunningInDockerDesktop() bool {
	return f.runningDockerDesktop
}

func readProfilesFeature(_ context.Context, _ command.Cli, _ bool) (bool, error) {
	return true, nil
}
