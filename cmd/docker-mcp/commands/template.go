package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/docker/mcp-gateway/pkg/client"
	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/registryapi"
	"github.com/docker/mcp-gateway/pkg/telemetry"
	"github.com/docker/mcp-gateway/pkg/template"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func templateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Manage starter profile templates",
	}

	cmd.AddCommand(listTemplatesCommand())
	cmd.AddCommand(useTemplateCommand())

	return cmd
}

func listTemplatesCommand() *cobra.Command {
	format := string(workingset.OutputFormatHumanReadable)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List available starter templates",
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return template.List(workingset.OutputFormat(format))
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&format, "format", string(workingset.OutputFormatHumanReadable),
		fmt.Sprintf("Supported: %s.", strings.Join(workingset.SupportedFormats(), ", ")))

	return cmd
}

func useTemplateCommand() *cobra.Command {
	cfg := client.ReadConfig()

	var opts struct {
		Name    string
		Connect []string
	}

	cmd := &cobra.Command{
		Use:   "use <template-id>",
		Short: "Create a profile from a starter template",
		Long: `Create a new profile from a starter template.

This is equivalent to: docker mcp profile create --from-template <template-id>

Use 'docker mcp template list' to see available templates.`,
		Example: `  # Create a profile from the ai-coding template
  docker mcp template use ai-coding

  # Override the profile name
  docker mcp template use ai-coding --name "My AI Tools"

  # Create and connect to a client in one shot
  docker mcp template use ai-coding --connect cursor`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateID := args[0]
			tmpl := template.FindByID(templateID)
			if tmpl == nil {
				return fmt.Errorf("unknown template: %s. Use `docker mcp template list` to see available templates", templateID)
			}

			name := opts.Name
			if name == "" {
				name = tmpl.Title
			}

			dao, err := db.New()
			if err != nil {
				return err
			}

			ociService := oci.NewService()

			if err := template.EnsureCatalogExists(cmd.Context(), dao, ociService); err != nil {
				return err
			}

			registryClient := registryapi.NewClient()
			servers := []string{tmpl.CatalogServerRef()}

			if err := workingset.Create(cmd.Context(), dao, registryClient, ociService, "", name, servers, opts.Connect); err != nil {
				return err
			}

			telemetry.Init()
			telemetry.RecordTemplateUsage(cmd.Context(), templateID, "template-use")
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&opts.Name, "name", "", "Override the profile name (defaults to template title)")
	flags.StringArrayVar(&opts.Connect, "connect", []string{},
		fmt.Sprintf("Clients to connect to (can be specified multiple times). Supported clients: %s",
			client.GetSupportedMCPClients(*cfg)))

	return cmd
}
