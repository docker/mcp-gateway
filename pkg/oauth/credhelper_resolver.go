package oauth

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/docker/mcp-gateway/pkg/user"
)

// DockerConfig represents Docker's config.json
type DockerConfig struct {
	CredsStore string `json:"credsStore"`
}

// ParseDockerConfig parses config JSON
func ParseDockerConfig(data []byte) (*DockerConfig, error) {
	var config DockerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return &config, nil
}

// ValidateCredsStore validates credential store name
func ValidateCredsStore(credsStore string) error {
	if credsStore == "" {
		return fmt.Errorf("empty credsStore")
	}
	return nil
}

// HelperBinaryName returns credential helper binary name
func HelperBinaryName(credsStore string) string {
	return "docker-credential-" + credsStore
}

// ConfigReader reads Docker config
type ConfigReader interface {
	ReadConfig() ([]byte, error)
}

// CommandChecker checks command existence
type CommandChecker interface {
	CommandExists(cmd string) bool
}

// ModeDetector detects CE vs Desktop mode
type ModeDetector interface {
	IsCEMode() bool
}

// Resolver resolves credential helper names
type Resolver struct {
	ConfigReader   ConfigReader
	CommandChecker CommandChecker
	ModeDetector   ModeDetector
}

// Resolve determines credential helper to use
func (r *Resolver) Resolve() (string, error) {
	// Desktop mode: prefer docker-credential-desktop
	if !r.ModeDetector.IsCEMode() {
		if r.CommandChecker.CommandExists("docker-credential-desktop") {
			return "desktop", nil
		}
	}

	// Read config
	configData, err := r.ConfigReader.ReadConfig()
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no Docker config found")
		}
		return "", fmt.Errorf("read config: %w", err)
	}

	// Parse config
	config, err := ParseDockerConfig(configData)
	if err != nil {
		return "", fmt.Errorf("parse config: %w", err)
	}

	// Validate
	if err := ValidateCredsStore(config.CredsStore); err != nil {
		return "", fmt.Errorf("invalid config: %w", err)
	}

	// Check binary
	binaryName := HelperBinaryName(config.CredsStore)
	if !r.CommandChecker.CommandExists(binaryName) {
		return "", fmt.Errorf("binary not found: %s", binaryName)
	}

	return config.CredsStore, nil
}

// Production implementations (not tested directly)

type filesystemConfigReader struct{}

func (f filesystemConfigReader) ReadConfig() ([]byte, error) {
	homeDir, err := user.HomeDir()
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(homeDir, ".docker", "config.json"))
}

type execCommandChecker struct{}

func (e execCommandChecker) CommandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

type envModeDetector struct{}

func (e envModeDetector) IsCEMode() bool {
	return IsCEMode()
}

// NewResolver creates resolver with production dependencies
func NewResolver() *Resolver {
	return &Resolver{
		ConfigReader:   filesystemConfigReader{},
		CommandChecker: execCommandChecker{},
		ModeDetector:   envModeDetector{},
	}
}
