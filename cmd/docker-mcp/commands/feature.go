package commands

import (
	"fmt"
	"strconv"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/spf13/cobra"
)

// featureCommand creates the `feature` command and its subcommands
func featureCommand(dockerCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feature",
		Short: "Manage experimental features",
		Long: `Manage experimental features for Docker MCP Gateway.

Features are stored in your Docker configuration file (~/.docker/config.json)
and control optional functionality that may change in future versions.`,
	}

	cmd.AddCommand(
		featureEnableCommand(dockerCli),
		featureDisableCommand(dockerCli),
		featureListCommand(dockerCli),
	)

	return cmd
}

// featureEnableCommand creates the `feature enable` command
func featureEnableCommand(dockerCli command.Cli) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <feature-name>",
		Short: "Enable an experimental feature",
		Long: `Enable an experimental feature.

Available features:
  oauth-interceptor      Enable GitHub OAuth flow interception for automatic authentication
  mcp-oauth-dcr          Enable Dynamic Client Registration (DCR) for automatic OAuth client setup
  dynamic-tools          Enable internal MCP management tools (mcp-find, mcp-add, mcp-remove)`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			featureName := args[0]

			// Validate feature name
			if !isKnownFeature(featureName) {
				return fmt.Errorf("unknown feature: %s\n\nAvailable features:\n  oauth-interceptor      Enable GitHub OAuth flow interception\n  mcp-oauth-dcr          Enable Dynamic Client Registration for automatic OAuth setup\n  dynamic-tools          Enable internal MCP management tools", featureName)
			}

			// Enable the feature
			configFile := dockerCli.ConfigFile()
			if configFile.Features == nil {
				configFile.Features = make(map[string]string)
			}
			configFile.Features[featureName] = "enabled"

			// Save the configuration
			if err := configFile.Save(); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}

			fmt.Printf("Feature '%s' enabled successfully.\n", featureName)

			// Provide usage hints for features
			switch featureName {
			case "oauth-interceptor":
				fmt.Println("\nThis feature enables automatic GitHub OAuth interception when 401 errors occur.")
				fmt.Println("When enabled, the gateway will automatically provide OAuth URLs for authentication.")
				fmt.Println("\nNo additional flags are needed - this applies to all gateway runs.")
			case "mcp-oauth-dcr":
				fmt.Println("\nThis feature enables Dynamic Client Registration (DCR) for MCP servers.")
				fmt.Println("When enabled, remote servers with OAuth configuration will automatically:")
				fmt.Println("  - Discover OAuth authorization servers")
				fmt.Println("  - Register public OAuth clients using PKCE")
				fmt.Println("  - Provide seamless OAuth authentication flows")
				fmt.Println("\nOnly affects remote servers with OAuth configuration - traditional OAuth flows are unchanged.")
			case "dynamic-tools":
				fmt.Println("\nThis feature enables dynamic tool discovery and execution capabilities.")
				fmt.Println("When enabled, the gateway provides internal tools for managing MCP servers:")
				fmt.Println("  - mcp-find: search for available MCP servers in the catalog")
				fmt.Println("  - mcp-add: add MCP servers to the registry and reload configuration")
				fmt.Println("  - mcp-remove: remove MCP servers from the registry and reload configuration")
				fmt.Println("\nNo additional flags are needed - this applies to all gateway runs.")
			}

			return nil
		},
	}
}

// featureDisableCommand creates the `feature disable` command
func featureDisableCommand(dockerCli command.Cli) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <feature-name>",
		Short: "Disable an experimental feature",
		Long:  "Disable an experimental feature that was previously enabled.",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			featureName := args[0]

			// Validate feature name
			if !isKnownFeature(featureName) {
				return fmt.Errorf("unknown feature: %s", featureName)
			}

			// Disable the feature
			configFile := dockerCli.ConfigFile()
			if configFile.Features == nil {
				configFile.Features = make(map[string]string)
			}
			configFile.Features[featureName] = "disabled"

			// Save the configuration
			if err := configFile.Save(); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}

			fmt.Printf("Feature '%s' disabled successfully.\n", featureName)
			return nil
		},
	}
}

// featureListCommand creates the `feature list` command
func featureListCommand(dockerCli command.Cli) *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List all available features and their status",
		Long:    "List all available experimental features and show whether they are enabled or disabled.",
		RunE: func(_ *cobra.Command, _ []string) error {
			configFile := dockerCli.ConfigFile()

			fmt.Println("Available experimental features:")
			fmt.Println()

			// Show all known features
			knownFeatures := []string{"oauth-interceptor", "mcp-oauth-dcr", "dynamic-tools"}
			for _, feature := range knownFeatures {
				status := "disabled"
				if isFeatureEnabledFromCli(dockerCli, feature) {
					status = "enabled"
				}

				fmt.Printf("  %-20s %s\n", feature, status)

				// Add description for each feature
				switch feature {
				case "oauth-interceptor":
					fmt.Printf("  %-20s %s\n", "", "Enable GitHub OAuth flow interception for automatic authentication")
				case "mcp-oauth-dcr":
					fmt.Printf("  %-20s %s\n", "", "Enable Dynamic Client Registration (DCR) for automatic OAuth client setup")
				case "dynamic-tools":
					fmt.Printf("  %-20s %s\n", "", "Enable internal MCP management tools (mcp-find, mcp-add, mcp-remove)")
				}
				fmt.Println()
			}

			// Show any other features in config that we don't know about
			if configFile.Features != nil {
				unknownFeatures := make([]string, 0)
				for feature := range configFile.Features {
					if !isKnownFeature(feature) {
						unknownFeatures = append(unknownFeatures, feature)
					}
				}

				if len(unknownFeatures) > 0 {
					fmt.Println("Unknown features in configuration:")
					for _, feature := range unknownFeatures {
						status := configFile.Features[feature]
						fmt.Printf("  %-20s %s (unknown)\n", feature, status)
					}
				}
			}

			return nil
		},
	}
}

// isFeatureEnabledFromCli checks if a feature is enabled using the CLI interface
func isFeatureEnabledFromCli(dockerCli command.Cli, feature string) bool {
	configFile := dockerCli.ConfigFile()
	return isFeatureEnabledFromConfig(configFile, feature)
}

// isFeatureEnabledFromConfig checks if a feature is enabled from a config file
func isFeatureEnabledFromConfig(configFile *configfile.ConfigFile, feature string) bool {
	if configFile.Features == nil {
		return false
	}

	value, exists := configFile.Features[feature]
	if !exists {
		return false
	}

	// Handle both boolean string values and "enabled"/"disabled" strings
	if value == "enabled" {
		return true
	}
	if value == "disabled" {
		return false
	}

	// Fallback to parsing as boolean
	enabled, err := strconv.ParseBool(value)
	return err == nil && enabled
}

// isKnownFeature checks if the feature name is valid
func isKnownFeature(feature string) bool {
	knownFeatures := []string{
		"oauth-interceptor",
		"mcp-oauth-dcr",
		"dynamic-tools",
	}

	for _, known := range knownFeatures {
		if feature == known {
			return true
		}
	}
	return false
}
