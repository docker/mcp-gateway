package workingset

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

func WriteManualInstructions(profileID string, format OutputFormat, output io.Writer) error {
	if profileID == "" {
		return fmt.Errorf("profile ID is required")
	}

	command := []string{"docker", "mcp", "gateway", "run", "--profile", profileID}

	switch format {
	case OutputFormatHumanReadable:
		fmt.Fprint(output, strings.Join(command, " "))
	case OutputFormatJSON:
		buf, err := json.Marshal(command)
		if err != nil {
			return err
		}
		_, _ = output.Write(buf)
	case OutputFormatYAML:
		buf, err := yaml.Marshal(command)
		if err != nil {
			return err
		}
		_, _ = output.Write(buf)
	}
	return nil
}
