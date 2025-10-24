package hints

import (
	"github.com/docker/cli/cli/command"
	"github.com/fatih/color"
)

func Enabled(dockerCli command.Cli) bool {
	configFile := dockerCli.ConfigFile()
	if configFile != nil && configFile.Plugins != nil {
		if pluginConfig, ok := configFile.Plugins["-x-cli-hints"]; ok {
			if enabledValue, exists := pluginConfig["enabled"]; exists {
				return enabledValue == "true"
			}
		}
	}

	return true
}

var (
	TipCyan           = color.New(color.FgCyan)
	TipCyanBoldItalic = color.New(color.FgCyan, color.Bold, color.Italic)
	TipGreen          = color.New(color.FgGreen)
)
