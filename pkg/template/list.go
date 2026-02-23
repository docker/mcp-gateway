package template

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/pkg/workingset"
)

// List prints the available templates in the specified output format.
func List(format workingset.OutputFormat) error {
	var data []byte
	var err error

	switch format {
	case workingset.OutputFormatHumanReadable:
		fmt.Println(printHumanReadable())
		return nil
	case workingset.OutputFormatJSON:
		data, err = json.MarshalIndent(Templates, "", "  ")
	case workingset.OutputFormatYAML:
		data, err = yaml.Marshal(Templates)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal templates: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func printHumanReadable() string {
	var sb strings.Builder
	sb.WriteString("ID\tTitle\tServers\tDescription\n")
	sb.WriteString("----\t----\t----\t----\n")
	for _, t := range Templates {
		sb.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\n",
			t.ID, t.Title, strings.Join(t.ServerNames, ", "), t.Description))
	}
	return strings.TrimSuffix(sb.String(), "\n")
}
