package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/oci"
)

func registryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "registry",
		Short:  "Registry operations",
		Hidden: true, // Hidden command
	}

	cmd.AddCommand(registryConvertCommand())

	return cmd
}

func registryConvertCommand() *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:   "convert",
		Short: "Convert OCI registry server definition to catalog server format",
		RunE: func(_ *cobra.Command, _ []string) error {
			if filePath == "" {
				return fmt.Errorf("--file flag is required")
			}

			// Read the file contents
			fileContents, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", filePath, err)
			}

			// Parse into ServerDetail
			var serverDetail oci.ServerDetail
			err = json.Unmarshal(fileContents, &serverDetail)
			if err != nil {
				return fmt.Errorf("failed to parse JSON from file %s: %w", filePath, err)
			}

			// Convert to catalog server
			catalogServer := serverDetail.ToCatalogServer()

			// Marshal to YAML and print to stdout
			outputYAML, err := yaml.Marshal(catalogServer)
			if err != nil {
				return fmt.Errorf("failed to marshal catalog server to YAML: %w", err)
			}

			fmt.Print(string(outputYAML))
			return nil
		},
	}

	cmd.Flags().StringVar(&filePath, "file", "", "Path to the OCI registry server definition JSON file")
	if err := cmd.MarkFlagRequired("file"); err != nil {
		// This should not happen in practice, but we need to handle the error for linting
		panic(fmt.Sprintf("failed to mark flag as required: %v", err))
	}

	return cmd
}
