package hints

import (
	"github.com/docker/cli/cli/command"
	"github.com/fatih/color"
)

func Enabled(dockerCli command.Cli) bool {
	configFile := dockerCli.ConfigFile()
	if configFile == nil || configFile.Plugins == nil {
		return true
	}

	pluginConfig, ok := configFile.Plugins["-x-cli-hints"]
	if !ok {
		return true
	}

	enabledValue, exists := pluginConfig["enabled"]
	if !exists {
		return true
	}

	return enabledValue == "true"
}

var (
	TipCyan           = color.New(color.FgCyan)
	TipCyanBoldItalic = color.New(color.FgCyan, color.Bold, color.Italic)
	TipGreen          = color.New(color.FgGreen)
	WarningColor      = color.New(color.FgYellow)
)
