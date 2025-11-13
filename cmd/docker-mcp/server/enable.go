package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/docker/cli/cli/command"
	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/hints"
	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/config"
	"github.com/docker/mcp-gateway/pkg/desktop"
	"github.com/docker/mcp-gateway/pkg/docker"
	pkgoauth "github.com/docker/mcp-gateway/pkg/oauth"
)

func Disable(ctx context.Context, docker docker.Client, dockerCli command.Cli, serverNames []string, mcpOAuthDcrEnabled bool) error {
	return update(ctx, docker, dockerCli, nil, serverNames, mcpOAuthDcrEnabled, false)
}

func Enable(ctx context.Context, docker docker.Client, dockerCli command.Cli, serverNames []string, mcpOAuthDcrEnabled bool, skipConfig bool) error {
	return update(ctx, docker, dockerCli, serverNames, nil, mcpOAuthDcrEnabled, skipConfig)
}

func update(ctx context.Context, docker docker.Client, dockerCli command.Cli, add []string, remove []string, mcpOAuthDcrEnabled bool, skipConfig bool) error {
	registry, userConfig, err := loadRegistryWithConfig(ctx, docker)
	if err != nil {
		return err
	}

	catalog, err := catalog.GetWithOptions(ctx, true, nil)
	if err != nil {
		return err
	}

	updatedRegistry := config.Registry{
		Servers: map[string]config.Tile{},
	}

	// Keep only servers that are still in the catalog.
	for serverName := range registry.Servers {
		if _, found := catalog.Servers[serverName]; found {
			existingTile := registry.Servers[serverName]
			// Copy the existing tile, preserving Config
			var configCopy map[string]any
			if existingTile.Config != nil {
				configCopy = deepCopyMap(existingTile.Config)
			} else {
				configCopy = make(map[string]any)
			}
			updatedRegistry.Servers[serverName] = config.Tile{
				Ref:    existingTile.Ref,
				Config: configCopy,
			}
		}
	}

	// Enable servers.
	for _, serverName := range add {
		if server, found := catalog.Servers[serverName]; found {
			tile := config.Tile{
				Ref:    "",
				Config: make(map[string]any),
			}

			// Check if server has existing config in registry or updatedRegistry or userConfig
			if existingTile, hasExisting := updatedRegistry.Servers[serverName]; hasExisting && existingTile.Config != nil && len(existingTile.Config) > 0 {
				// Use config from updatedRegistry (which was copied from registry)
				tile.Config = deepCopyMap(existingTile.Config)
			} else if existingTile, hasExisting := registry.Servers[serverName]; hasExisting && existingTile.Config != nil && len(existingTile.Config) > 0 {
				// Fallback to original registry (which should now have userConfig populated)
				tile.Config = deepCopyMap(existingTile.Config)
			} else if userServerConfig, hasUserConfig := userConfig[serverName]; hasUserConfig && len(userServerConfig) > 0 {
				// Fallback to userConfig directly
				tile.Config = deepCopyMap(userServerConfig)
			}

			// DCR flag enabled AND type="remote" AND oauth present
			if mcpOAuthDcrEnabled && server.IsRemoteOAuthServer() {
				// In CE mode, skip lazy setup - DCR happens during oauth authorize
				if pkgoauth.IsCEMode() {
					fmt.Printf("OAuth server %s enabled. Run 'docker mcp oauth authorize %s' to authenticate\n", serverName, serverName)
				} else {
					// Desktop mode - register provider for lazy setup
					if err := pkgoauth.RegisterProviderForLazySetup(ctx, serverName); err != nil {
						fmt.Printf("Warning: Failed to register OAuth provider for %s: %v\n", serverName, err)
						fmt.Printf("   You can run 'docker mcp oauth authorize %s' later to set up authentication.\n", serverName)
					} else {
						fmt.Printf("OAuth provider configured for %s - use 'docker mcp oauth authorize %s' to authenticate\n", serverName, serverName)
					}
				}
			} else if !mcpOAuthDcrEnabled && server.IsRemoteOAuthServer() {
				// Provide guidance when DCR is needed but disabled
				fmt.Printf("Server %s requires OAuth authentication but DCR is disabled.\n", serverName)
				fmt.Printf("   To enable automatic OAuth setup, run: docker mcp feature enable mcp-oauth-dcr\n")
				fmt.Printf("   Or set up OAuth manually using: docker mcp oauth authorize %s\n", serverName)
			}

			// Check if server has secrets requirements and prompt for them
			if !skipConfig && len(server.Secrets) > 0 {
				if err := handleSecretsConfiguration(ctx, dockerCli, serverName, server.Secrets); err != nil {
					return err
				}
			}

			// Check if server has config requirements and prompt for them
			if !skipConfig && len(server.Config) > 0 {
				if err := handleConfigsConfiguration(ctx, dockerCli, serverName, server.Config, tile.Config); err != nil {
					return err
				}
			}

			updatedRegistry.Servers[serverName] = tile
		} else {
			return fmt.Errorf("server %s not found in catalog", serverName)
		}
	}

	// Disable servers.
	for _, serverName := range remove {
		delete(updatedRegistry.Servers, serverName)
	}

	// Save it.
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(updatedRegistry); err != nil {
		return fmt.Errorf("encoding registry config: %w", err)
	}

	if err := config.WriteRegistry(buf.Bytes()); err != nil {
		return fmt.Errorf("writing registry config: %w", err)
	}

	if len(add) > 0 && hints.Enabled(dockerCli) {
		hints.TipCyan.Print("Tip: ")
		hints.TipGreen.Print("✓")
		hints.TipCyan.Print(" Server enabled. To view all enabled servers, use ")
		hints.TipCyanBoldItalic.Println("docker mcp server ls")
		fmt.Println()
	}

	if len(remove) > 0 && hints.Enabled(dockerCli) {
		hints.TipCyan.Print("Tip: ")
		hints.TipGreen.Print("✓")
		hints.TipCyan.Print(" Server disabled. To see remaining enabled servers, use ")
		hints.TipCyanBoldItalic.Println("docker mcp server ls")
		fmt.Println()
	}

	return nil
}

// handleSecretsConfiguration prompts for and saves missing secrets for a server
func handleSecretsConfiguration(ctx context.Context, dockerCli command.Cli, serverName string, requiredSecrets []catalog.Secret) error {
	missingSecrets := getMissingSecrets(ctx, requiredSecrets)

	if len(missingSecrets) == 0 {
		return nil
	}

	fmt.Printf("\nServer %s requires secrets. Please provide the following:\n\n", serverName)
	fmt.Println("Press Ctrl+C to cancel configuration.")

	// Prompt for each missing secret
	for _, secret := range missingSecrets {
		value, err := promptForSecret(ctx, dockerCli, secret)
		if err != nil {
			if errors.Is(err, command.ErrPromptTerminated) {
				fmt.Println("\nConfiguration cancelled.")
				return fmt.Errorf("configuration cancelled by user")
			}
			// For validation errors, show warning but continue
			fmt.Fprintf(dockerCli.Out(), "\n")
			if strings.Contains(err.Error(), "too long") {
				hints.WarningColor.Fprintf(dockerCli.Err(), "Warning: %v\n", err)
				hints.WarningColor.Fprintf(dockerCli.Err(), "Skipping secret %s. The server may not work without it.\n", secret.Name)
				hints.WarningColor.Fprintf(dockerCli.Err(), "You can set it later with: ")
				hints.TipCyanBoldItalic.Fprintf(dockerCli.Err(), "docker mcp secret set %s=<value>\n\n", secret.Name)
				continue
			}
			// For other errors, also continue with warning
			hints.WarningColor.Fprintf(dockerCli.Err(), "Warning: %v\n", err)
			hints.WarningColor.Fprintf(dockerCli.Err(), "Skipping secret %s. The server may not work without it.\n", secret.Name)
			hints.WarningColor.Fprintf(dockerCli.Err(), "You can set it later with: ")
			hints.TipCyanBoldItalic.Fprintf(dockerCli.Err(), "docker mcp secret set %s=<value>\n\n", secret.Name)
			continue
		}

		// If secret is empty, warn but continue
		if value == "" {
			fmt.Fprintf(dockerCli.Out(), "\n")
			hints.WarningColor.Fprintf(dockerCli.Err(), "Warning: Secret %s is required but was left empty.\n", secret.Name)
			hints.WarningColor.Fprintf(dockerCli.Err(), "The server may not work without it. You can set it later with: ")
			hints.TipCyanBoldItalic.Fprintf(dockerCli.Err(), "docker mcp secret set %s=<value>\n\n", secret.Name)
			continue
		}

		// Save the secret
		secretsClient := desktop.NewSecretsClient()
		if err := secretsClient.SetJfsSecret(ctx, desktop.Secret{
			Name:  secret.Name,
			Value: value,
		}); err != nil {
			fmt.Fprintf(dockerCli.Out(), "\n")
			hints.WarningColor.Fprintf(dockerCli.Err(), "Warning: Failed to save secret %s: %v\n", secret.Name, err)
			hints.WarningColor.Fprintf(dockerCli.Err(), "You can set it later with: ")
			hints.TipCyanBoldItalic.Fprintf(dockerCli.Err(), "docker mcp secret set %s=<value>\n\n", secret.Name)
			continue
		}
	}

	fmt.Println()
	return nil
}

// handleConfigsConfiguration prompts for and saves missing config fields for a server
func handleConfigsConfiguration(ctx context.Context, dockerCli command.Cli, serverName string, configSchema []any, userConfig map[string]any) error {
	// Get required fields that are not yet configured
	missingConfigs := getMissingConfigs(configSchema, userConfig)

	if len(missingConfigs) == 0 {
		return nil
	}

	fmt.Printf("\nServer %s requires configuration. Please provide the following:\n\n", serverName)
	fmt.Println("Press Ctrl+C to cancel configuration.")

	// Prompt for each missing config
	for _, field := range missingConfigs {
		var value any
		var err error

		// Retry loop for validation errors
		for {
			value, err = promptForConfigField(ctx, dockerCli, field)
			if err != nil {
				if errors.Is(err, command.ErrPromptTerminated) {
					fmt.Println("\nConfiguration cancelled.")
					return fmt.Errorf("configuration cancelled by user")
				}
				// Show validation error and retry
				fmt.Fprintf(dockerCli.Err(), "Error: %v\n", err)
				fmt.Fprintf(dockerCli.Err(), "Please try again.\n\n")
				continue
			}
			break
		}

		// Only save non-empty values (skip if user pressed Enter with no default)
		if !isEmptyValue(value) {
			// Store the value in the config map (handling nested keys)
			setNestedConfig(userConfig, field.Key, value)
		}
	}

	fmt.Println()
	return nil
}

// promptForConfigField prompts the user for a config field value
func promptForConfigField(ctx context.Context, dockerCli command.Cli, field configField) (any, error) {
	// Build the prompt message
	var prompt strings.Builder
	prompt.WriteString(fmt.Sprintf("  %s", field.Key))

	if field.Description != "" {
		prompt.WriteString(fmt.Sprintf(" (%s)", field.Description))
	}

	if field.Default != nil {
		prompt.WriteString(fmt.Sprintf(" [default: %v]", field.Default))
	}

	if len(field.Enum) > 0 {
		prompt.WriteString(fmt.Sprintf(" (options: %v)", field.Enum))
	}

	prompt.WriteString(": ")

	// Read user input using Docker CLI's prompt which handles Ctrl+C properly
	input, err := command.PromptForInput(ctx, dockerCli.In(), dockerCli.Out(), prompt.String())
	if err != nil {
		return nil, err
	}

	// Sanitize input: trim whitespace (already done by PromptForInput, but be explicit)
	input = strings.TrimSpace(input)

	// If input is empty and there's a default, use the default
	if input == "" && field.Default != nil {
		return field.Default, nil
	}

	// If input is empty and no default, return sentinel value to indicate skip
	if input == "" {
		return skipConfigValue, nil
	}

	// Validate input length to prevent extremely long values (DoS protection)
	const maxInputLength = 10000 // Reasonable limit for config values
	if len(input) > maxInputLength {
		return nil, fmt.Errorf("input too long (max %d characters)", maxInputLength)
	}

	// Validate enum if present
	if len(field.Enum) > 0 {
		valid := false
		for _, enumVal := range field.Enum {
			if fmt.Sprintf("%v", enumVal) == input {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("invalid value %q. Expected one of: %v", input, field.Enum)
		}
	}

	// Convert to appropriate type if needed
	if field.Type == "number" || field.Type == "integer" {
		// Validate and parse as number
		var num float64
		if _, err := fmt.Sscanf(input, "%f", &num); err != nil {
			return nil, fmt.Errorf("invalid number: %q", input)
		}
		// Check for integer overflow
		if field.Type == "integer" {
			if num > math.MaxInt || num < math.MinInt {
				return nil, fmt.Errorf("number out of range for integer: %v", num)
			}
			return int(num), nil
		}
		return num, nil
	}

	if field.Type == "boolean" {
		lower := strings.ToLower(input)
		if lower == "true" || lower == "yes" || lower == "y" || lower == "1" {
			return true, nil
		}
		if lower == "false" || lower == "no" || lower == "n" || lower == "0" {
			return false, nil
		}
	}

	// Default to string
	return input, nil
}

// getMissingSecrets returns a list of secrets that are required but not yet configured
func getMissingSecrets(ctx context.Context, requiredSecrets []catalog.Secret) []catalog.Secret {
	configuredSecretNames, err := getConfiguredSecretNames(ctx)
	if err != nil {
		// If we can't get secrets, assume none are configured
		return requiredSecrets
	}

	var missing []catalog.Secret
	for _, secret := range requiredSecrets {
		if _, ok := configuredSecretNames[secret.Name]; !ok {
			missing = append(missing, secret)
		}
	}

	return missing
}

// promptForSecret prompts the user for a secret value with password masking
func promptForSecret(ctx context.Context, dockerCli command.Cli, secret catalog.Secret) (string, error) {
	// Build the prompt message
	prompt := fmt.Sprintf("  %s", secret.Name)
	if secret.Env != "" {
		prompt += fmt.Sprintf(" (env: %s)", secret.Env)
	}
	prompt += ": "

	// Disable input echo for password masking
	restore, err := command.DisableInputEcho(dockerCli.In())
	if err == nil {
		defer func() {
			if restoreErr := restore(); restoreErr != nil {
				// Log but don't fail if restore fails
				_ = restoreErr
			}
		}()
	}
	// If we can't disable echo, continue anyway (non-terminal or unsupported)

	// Read user input
	input, err := command.PromptForInput(ctx, dockerCli.In(), dockerCli.Out(), prompt)
	if err != nil {
		return "", err
	}

	// Sanitize input: trim whitespace
	input = strings.TrimSpace(input)

	// Validate input length to prevent extremely long values (DoS protection)
	const maxInputLength = 10000 // Reasonable limit for secret values
	if len(input) > maxInputLength {
		return "", fmt.Errorf("secret too long (max %d characters)", maxInputLength)
	}

	// Allow empty secrets (user can skip, but will be warned)
	// Return empty string - caller will handle the warning
	return input, nil
}
