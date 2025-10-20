package workingset

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

func Show(name string, format Format) error {
	cfg, err := ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to read working-sets config: %w", err)
	}

	ws, exists := cfg.WorkingSets[name]
	if !exists {
		return fmt.Errorf("working-set %q not found", name)
	}

	switch format {
	case JSON:
		data, err := json.MarshalIndent(ws, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal to JSON: %w", err)
		}
		fmt.Println(string(data))
	case YAML:
		data, err := yaml.Marshal(ws)
		if err != nil {
			return fmt.Errorf("failed to marshal to YAML: %w", err)
		}
		fmt.Print(string(data))
	default:
		humanPrintWorkingSet(name, ws)
	}

	return nil
}

func humanPrintWorkingSet(name string, ws WorkingSet) {
	fmt.Println()
	fmt.Printf("  \033[1m%s\033[0m\n", name)
	if ws.Description != "" {
		fmt.Printf("  Description: %s\n", ws.Description)
	}
	fmt.Println()
	fmt.Println("  Servers:")
	for _, server := range ws.Servers {
		fmt.Printf("    - %s\n", server)
	}
	fmt.Println()
}
