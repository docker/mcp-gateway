package project

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func TestSaveAndLoadProfile(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "profile-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() { _ = os.Chdir(originalWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create a test profile
	testProfile := workingset.WorkingSet{
		Version: 1,
		ID:      "test-profile",
		Name:    "test-profile",
		Servers: []workingset.Server{
			{
				Type:  workingset.ServerTypeImage,
				Image: "test/image:latest",
				Config: map[string]any{
					"key": "value",
				},
				Tools: []string{"tool1", "tool2"},
				Snapshot: &workingset.ServerSnapshot{
					Server: catalog.Server{
						Name:        "test-server",
						Description: "Test server",
					},
				},
			},
		},
		Secrets: map[string]workingset.Secret{
			"default": {
				Provider: workingset.SecretProviderDockerDesktop,
			},
		},
	}

	// Save the profile
	err = SaveProfile(testProfile)
	if err != nil {
		t.Fatalf("Failed to save profile: %v", err)
	}

	// Verify the file was created
	profilesPath := filepath.Join(tmpDir, "profiles.json")
	if _, err := os.Stat(profilesPath); os.IsNotExist(err) {
		t.Fatal("profiles.json was not created")
	}

	// Load the profiles
	ctx := context.Background()
	profiles, err := LoadProfiles(ctx)
	if err != nil {
		t.Fatalf("Failed to load profiles: %v", err)
	}

	// Verify the profile was loaded
	if len(profiles) != 1 {
		t.Fatalf("Expected 1 profile, got %d", len(profiles))
	}

	loadedProfile, exists := profiles["test-profile"]
	if !exists {
		t.Fatal("Profile 'test-profile' not found in loaded profiles")
	}

	// Verify profile contents
	if loadedProfile.Name != testProfile.Name {
		t.Errorf("Expected name %s, got %s", testProfile.Name, loadedProfile.Name)
	}

	if loadedProfile.ID != testProfile.ID {
		t.Errorf("Expected ID %s, got %s", testProfile.ID, loadedProfile.ID)
	}

	if len(loadedProfile.Servers) != len(testProfile.Servers) {
		t.Errorf("Expected %d servers, got %d", len(testProfile.Servers), len(loadedProfile.Servers))
	}

	if len(loadedProfile.Secrets) != len(testProfile.Secrets) {
		t.Errorf("Expected %d secrets, got %d", len(testProfile.Secrets), len(loadedProfile.Secrets))
	}

	// Verify the JSON structure
	data, err := os.ReadFile(profilesPath)
	if err != nil {
		t.Fatalf("Failed to read profiles.json: %v", err)
	}

	var rawProfiles map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawProfiles); err != nil {
		t.Fatalf("Failed to unmarshal profiles.json: %v", err)
	}

	if len(rawProfiles) != 1 {
		t.Errorf("Expected 1 profile in JSON, got %d", len(rawProfiles))
	}

	if _, exists := rawProfiles["test-profile"]; !exists {
		t.Error("Profile 'test-profile' not found in JSON")
	}
}

func TestSaveMultipleProfiles(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "profile-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() { _ = os.Chdir(originalWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create first profile
	profile1 := workingset.WorkingSet{
		Version: 1,
		ID:      "profile1",
		Name:    "profile1",
		Servers: []workingset.Server{},
		Secrets: map[string]workingset.Secret{},
	}

	// Create second profile
	profile2 := workingset.WorkingSet{
		Version: 1,
		ID:      "profile2",
		Name:    "profile2",
		Servers: []workingset.Server{},
		Secrets: map[string]workingset.Secret{},
	}

	// Save both profiles
	if err := SaveProfile(profile1); err != nil {
		t.Fatalf("Failed to save profile1: %v", err)
	}

	if err := SaveProfile(profile2); err != nil {
		t.Fatalf("Failed to save profile2: %v", err)
	}

	// Load the profiles
	ctx := context.Background()
	profiles, err := LoadProfiles(ctx)
	if err != nil {
		t.Fatalf("Failed to load profiles: %v", err)
	}

	// Verify both profiles were loaded
	if len(profiles) != 2 {
		t.Fatalf("Expected 2 profiles, got %d", len(profiles))
	}

	if _, exists := profiles["profile1"]; !exists {
		t.Error("Profile 'profile1' not found")
	}

	if _, exists := profiles["profile2"]; !exists {
		t.Error("Profile 'profile2' not found")
	}
}

func TestLoadProfilesNoFile(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "profile-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() { _ = os.Chdir(originalWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Load profiles when no file exists
	ctx := context.Background()
	profiles, err := LoadProfiles(ctx)
	if err != nil {
		t.Fatalf("Failed to load profiles: %v", err)
	}

	// Should return empty map, not error
	if len(profiles) != 0 {
		t.Errorf("Expected 0 profiles, got %d", len(profiles))
	}
}

func TestSaveProfileAtomicWrite(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "profile-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() { _ = os.Chdir(originalWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	profilesPath := filepath.Join(tmpDir, "profiles.json")

	// Create initial profile
	profile1 := workingset.WorkingSet{
		Version: 1,
		ID:      "profile1",
		Name:    "profile1",
		Servers: []workingset.Server{},
		Secrets: map[string]workingset.Secret{},
	}

	if err := SaveProfile(profile1); err != nil {
		t.Fatalf("Failed to save initial profile: %v", err)
	}

	// Verify no temp file left behind
	tempPath := profilesPath + ".tmp"
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("Temp file was not cleaned up after successful save")
	}

	// Update the profile
	profile1Updated := workingset.WorkingSet{
		Version: 1,
		ID:      "profile1",
		Name:    "profile1",
		Servers: []workingset.Server{
			{
				Type:  workingset.ServerTypeImage,
				Image: "updated/image:latest",
			},
		},
		Secrets: map[string]workingset.Secret{},
	}

	if err := SaveProfile(profile1Updated); err != nil {
		t.Fatalf("Failed to update profile: %v", err)
	}

	// Verify no temp file left behind
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("Temp file was not cleaned up after update")
	}

	// Verify the update was written
	ctx := context.Background()
	profiles, err := LoadProfiles(ctx)
	if err != nil {
		t.Fatalf("Failed to load profiles: %v", err)
	}

	if len(profiles) != 1 {
		t.Fatalf("Expected 1 profile, got %d", len(profiles))
	}

	loadedProfile := profiles["profile1"]
	if len(loadedProfile.Servers) != 1 {
		t.Errorf("Expected 1 server in updated profile, got %d", len(loadedProfile.Servers))
	}
}
