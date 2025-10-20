package workingset

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Format string

const (
	JSON Format = "json"
	YAML Format = "yaml"
)

var supportedFormats = []Format{JSON, YAML}

func (e *Format) String() string {
	return string(*e)
}

func (e *Format) Set(v string) error {
	actual := Format(v)
	for _, allowed := range supportedFormats {
		if allowed == actual {
			*e = actual
			return nil
		}
	}
	return fmt.Errorf("must be one of %s", SupportedFormats())
}

// Type is only used in help text
func (e *Format) Type() string {
	return "format"
}

func SupportedFormats() string {
	var quoted []string
	for _, v := range supportedFormats {
		quoted = append(quoted, "\""+string(v)+"\"")
	}
	return strings.Join(quoted, ", ")
}

// WorkingSetMetadata contains summary info about a working-set (excluding full server list)
type WorkingSetMetadata struct {
	Description  string `json:"description,omitempty" yaml:"description,omitempty"`
	ServerCount  int    `json:"serverCount" yaml:"serverCount"`
}

type ListOutput struct {
	WorkingSets map[string]WorkingSetMetadata `json:"workingSets" yaml:"workingSets"`
}

func List(format Format) error {
	cfg, err := ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to read working-sets config: %w", err)
	}

	switch format {
	case JSON:
		output := buildListOutput(cfg.WorkingSets)
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal to JSON: %w", err)
		}
		fmt.Println(string(data))
	case YAML:
		output := buildListOutput(cfg.WorkingSets)
		data, err := yaml.Marshal(output)
		if err != nil {
			return fmt.Errorf("failed to marshal to YAML: %w", err)
		}
		fmt.Print(string(data))
	default:
		humanPrintWorkingSetsList(cfg.WorkingSets)
	}

	return nil
}

func buildListOutput(workingSets map[string]WorkingSet) ListOutput {
	output := ListOutput{
		WorkingSets: make(map[string]WorkingSetMetadata),
	}

	for name, ws := range workingSets {
		output.WorkingSets[name] = WorkingSetMetadata{
			Description: ws.Description,
			ServerCount: len(ws.Servers),
		}
	}

	return output
}

func humanPrintWorkingSetsList(workingSets map[string]WorkingSet) {
	if len(workingSets) == 0 {
		fmt.Println("No working-sets configured.")
		return
	}

	// Sort by name for consistent output
	names := make([]string, 0, len(workingSets))
	for name := range workingSets {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		ws := workingSets[name]
		if ws.Description != "" {
			fmt.Printf("%s: %s\n", name, ws.Description)
		} else {
			fmt.Println(name)
		}
	}
}
