package hints

import (
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/fatih/color"
)

func Enabled() bool {
	configFile := getDockerConfigFile()
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

func getDockerConfigFile() *configfile.ConfigFile {
	configFile, err := config.Load("")
	if err != nil {
		return nil
	}
	return configFile
}
