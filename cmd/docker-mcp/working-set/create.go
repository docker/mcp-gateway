package workingset

import (
	"fmt"
)

func Create(name, description string, servers []string) error {
	// Validate input
	if name == "" {
		return fmt.Errorf("working-set name cannot be empty")
	}

	if len(servers) == 0 {
		return fmt.Errorf("at least one server must be specified")
	}

	// Read existing config
	cfg, err := ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to read working-sets config: %w", err)
	}

	// Check if working-set already exists
	if _, exists := cfg.WorkingSets[name]; exists {
		return fmt.Errorf("working-set %q already exists", name)
	}

	// Create new working-set
	cfg.WorkingSets[name] = WorkingSet{
		Name:        name,
		Description: description,
		Servers:     servers,
	}

	// Write config
	if err := WriteConfig(cfg); err != nil {
		return fmt.Errorf("failed to write working-sets config: %w", err)
	}

	fmt.Printf("Created working-set %q with %d server(s)\n", name, len(servers))
	return nil
}
