package hints

import (
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
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

func getDockerConfigFile() *configfile.ConfigFile {
	configFile, err := config.Load("")
	if err != nil {
		return nil
	}
	return configFile
}
