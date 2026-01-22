package project

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/log"
)

// ProfilesConfig represents the structure of profiles.json
type ProfilesConfig struct {
	Profiles []string `json:"profiles"`
}

// ProfileActivator is an interface for activating profiles
type ProfileActivator interface {
	ActivateProfile(ctx context.Context, profileName string) error
}

// LoadProfiles loads and activates profiles from profiles.json in the current working directory
func LoadProfiles(ctx context.Context, activator ProfileActivator) error {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Check for profiles.json in current directory
	profilesPath := filepath.Join(cwd, "profiles.json")

	// Check if file exists
	if _, err := os.Stat(profilesPath); os.IsNotExist(err) {
		// File doesn't exist, nothing to do
		log.Log("- No profiles.json found in current directory")
		return nil
	}

	// Read the file
	data, err := os.ReadFile(profilesPath)
	if err != nil {
		return fmt.Errorf("failed to read profiles.json: %w", err)
	}

	// Parse JSON
	var config ProfilesConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse profiles.json: %w", err)
	}

	// Check if profiles array is empty
	if len(config.Profiles) == 0 {
		log.Log("- profiles.json contains no profiles to activate")
		return nil
	}

	// Activate each profile
	log.Log(fmt.Sprintf("- Found profiles.json with %d profile(s) to activate", len(config.Profiles)))
	for _, profileName := range config.Profiles {
		log.Log(fmt.Sprintf("- Activating profile: %s", profileName))
		if err := activator.ActivateProfile(ctx, profileName); err != nil {
			log.Log(fmt.Sprintf("! Failed to activate profile %s: %v", profileName, err))
			// Continue with other profiles even if one fails
			continue
		}
		log.Log(fmt.Sprintf("- Successfully activated profile: %s", profileName))
	}

	return nil
}

// SaveProfile adds or updates a profile in profiles.json in the current working directory
func SaveProfile(profileName string) error {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Check for profiles.json in current directory
	profilesPath := filepath.Join(cwd, "profiles.json")

	// Read existing profiles if the file exists
	var config ProfilesConfig
	if data, err := os.ReadFile(profilesPath); err == nil {
		// File exists, parse it
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse existing profiles.json: %w", err)
		}
	} else if !os.IsNotExist(err) {
		// Some other error occurred
		return fmt.Errorf("failed to read profiles.json: %w", err)
	}
	// If file doesn't exist, config.Profiles will be empty slice

	// Add profile to the list if it's not already there
	if !slices.Contains(config.Profiles, profileName) {
		config.Profiles = append(config.Profiles, profileName)
		log.Log(fmt.Sprintf("- Added profile '%s' to profiles.json", profileName))
	} else {
		log.Log(fmt.Sprintf("- Profile '%s' already exists in profiles.json", profileName))
	}

	// Marshal config to JSON with indentation for readability
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal profiles config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(profilesPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write profiles.json: %w", err)
	}

	log.Log(fmt.Sprintf("- Updated profiles.json at: %s", profilesPath))
	return nil
}

// IsClaudeCodeClient checks if the client is Claude Code
func IsClaudeCodeClient(clientInfo *mcp.Implementation) bool {
	return clientInfo != nil && clientInfo.Name == "claude-code"
}

// SaveProfileForClient saves a profile for the given client if it's Claude Code
// This encapsulates Claude Code-specific logic for profile saving
func SaveProfileForClient(clientInfo *mcp.Implementation, profileName string) error {
	if !IsClaudeCodeClient(clientInfo) {
		// Not Claude Code, nothing to do
		return nil
	}

	log.Log("- Claude Code detected, updating profiles.json")
	if err := SaveProfile(profileName); err != nil {
		log.Log(fmt.Sprintf("! Failed to update profiles.json: %v", err))
		// Don't fail the entire operation, just log the error
		return err
	}
	return nil
}

// LoadProfilesForClient loads profiles for the given client if it's Claude Code
// This encapsulates Claude Code-specific logic for profile loading
func LoadProfilesForClient(ctx context.Context, clientInfo *mcp.Implementation, activator ProfileActivator) error {
	if !IsClaudeCodeClient(clientInfo) {
		// Not Claude Code, nothing to do
		return nil
	}

	log.Log("- Claude Code detected, checking for profiles.json")
	if err := LoadProfiles(ctx, activator); err != nil {
		log.Log(fmt.Sprintf("! Failed to load profiles: %v", err))
		return err
	}
	return nil
}
