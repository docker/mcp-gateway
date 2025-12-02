package client

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/mcp-gateway/pkg/user"
)

type FileConfig struct {
	Profile string `json:"profile"`
}

// Currently only used for Gordon.
type ProfilesFile = map[string]FileConfig

func writeGordonProfile(workingSet string) error {
	profilePath, err := getProfilePath()
	if err != nil {
		return err
	}

	profiles, err := readProfile(profilePath)
	if err != nil {
		return err
	}
	profiles[VendorGordon] = FileConfig{Profile: workingSet}
	return writeProfile(profilePath, profiles)
}

func ReadGordonProfile() (string, error) {
	profilePath, err := getProfilePath()
	if err != nil {
		return "", err
	}
	profiles, err := readProfile(profilePath)
	if err != nil {
		return "", err
	}
	if _, ok := profiles[VendorGordon]; ok {
		return profiles[VendorGordon].Profile, nil
	}
	return "", nil
}

func getProfilePath() (string, error) {
	homeDir, err := user.HomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".docker", "mcp", "profiles.json"), nil
}

func readProfile(path string) (ProfilesFile, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return make(ProfilesFile), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read profile file: %w", err)
	}
	var profiles ProfilesFile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, fmt.Errorf("failed to unmarshal profile file: %w", err)
	}
	return profiles, nil
}

func writeProfile(path string, profiles ProfilesFile) error {
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal profiles: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create profile directory: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}
