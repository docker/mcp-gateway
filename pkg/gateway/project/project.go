package project

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

// ProfilesConfig represents the structure of profiles.json
// Maps profile names to their full WorkingSet data
type ProfilesConfig map[string]workingset.WorkingSet

// ProfileActivator is an interface for activating profiles
type ProfileActivator interface {
	ActivateProfile(ctx context.Context, ws workingset.WorkingSet) error
}

// LoadProfiles loads and returns all profiles from profiles.json in the current working directory
func LoadProfiles(_ context.Context) (ProfilesConfig, error) {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Check for profiles.json in current directory
	profilesPath := filepath.Join(cwd, "profiles.json")

	// Check if file exists
	if _, err := os.Stat(profilesPath); os.IsNotExist(err) {
		// File doesn't exist, return empty config
		log.Log("- No profiles.json found in current directory")
		return make(ProfilesConfig), nil
	}

	// Read the file
	data, err := os.ReadFile(profilesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read profiles.json: %w", err)
	}

	// Parse JSON
	var config ProfilesConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse profiles.json: %w", err)
	}

	// Check if profiles map is empty
	if len(config) == 0 {
		log.Log("- profiles.json contains no profiles")
		return config, nil
	}

	log.Log(fmt.Sprintf("- Loaded %d profile(s) from profiles.json", len(config)))
	for profileName := range config {
		log.Log(fmt.Sprintf("  - Profile: %s", profileName))
	}

	return config, nil
}

// SaveProfile adds or updates a profile in profiles.json in the current working directory
// Uses atomic write pattern (write to temp file, then rename) to prevent data loss from concurrent writes
func SaveProfile(profile workingset.WorkingSet) error {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Check for profiles.json in current directory
	profilesPath := filepath.Join(cwd, "profiles.json")

	// Read existing profiles if the file exists
	config := make(ProfilesConfig)
	if data, err := os.ReadFile(profilesPath); err == nil {
		// File exists, parse it
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse existing profiles.json: %w", err)
		}
	} else if !os.IsNotExist(err) {
		// Some other error occurred
		return fmt.Errorf("failed to read profiles.json: %w", err)
	}
	// If file doesn't exist, config will be an empty map

	// Check if profile already exists
	_, exists := config[profile.Name]

	// Add or update profile
	config[profile.Name] = profile

	if exists {
		log.Log(fmt.Sprintf("- Updated profile '%s' in profiles.json", profile.Name))
	} else {
		log.Log(fmt.Sprintf("- Added profile '%s' to profiles.json", profile.Name))
	}

	// Marshal config to JSON with indentation for readability
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal profiles config: %w", err)
	}

	// Use atomic write pattern: write to temp file, then rename
	// This prevents data loss from concurrent writes or partial writes
	tempPath := profilesPath + ".tmp"

	// Write to temp file
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write temp profiles file: %w", err)
	}

	// Atomically rename temp file to final destination
	// On POSIX systems, rename is atomic and will replace the existing file
	if err := os.Rename(tempPath, profilesPath); err != nil {
		// Clean up temp file on error
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename profiles file: %w", err)
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
func SaveProfileForClient(clientInfo *mcp.Implementation, profile workingset.WorkingSet) error {
	if !IsClaudeCodeClient(clientInfo) {
		// Not Claude Code, nothing to do
		return nil
	}

	log.Log("- Claude Code detected, updating profiles.json")
	if err := SaveProfile(profile); err != nil {
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
	profiles, err := LoadProfiles(ctx)
	if err != nil {
		log.Log(fmt.Sprintf("! Failed to load profiles: %v", err))
		return err
	}

	// Activate each profile
	if len(profiles) > 0 {
		log.Log(fmt.Sprintf("- Activating %d profile(s) from profiles.json", len(profiles)))
		activatedCount := 0
		for profileName, ws := range profiles {
			log.Log(fmt.Sprintf("- Activating profile: %s", profileName))
			if err := activator.ActivateProfile(ctx, ws); err != nil {
				log.Log(fmt.Sprintf("! Failed to activate profile %s: %v", profileName, err))
				// Continue with other profiles even if one fails
				continue
			}
			log.Log(fmt.Sprintf("- Successfully activated profile: %s", profileName))
			activatedCount++
		}

		// Return error if all profiles failed to activate
		if activatedCount == 0 {
			return fmt.Errorf("failed to activate any of the %d profiles from profiles.json", len(profiles))
		}
	}

	return nil
}
