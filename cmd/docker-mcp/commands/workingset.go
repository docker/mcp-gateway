package commands

import (
	"fmt"

	"github.com/docker/mcp-gateway/pkg/workingset"
	"github.com/spf13/cobra"
)

func workingSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workingset",
		Short: "Manage working sets",
	}

	// TODO clean these up
	cmd.AddCommand(testWriteWorkingSetCommand())
	cmd.AddCommand(testReadWorkingSetCommand())

	return cmd
}

func testWriteWorkingSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "test-write",
		Short: "Test write working set",
		RunE: func(cmd *cobra.Command, args []string) error {
			return workingset.Write(workingset.WorkingSet{
				ID:   "test",
				Name: "My working set",
				Servers: []workingset.Server{
					{
						Type:   "registry",
						Source: "https://example-registry.com/v0/servers/312e45a4-2216-4b21-b9a8-0f1a51425073",
						Config: map[string]interface{}{
							"username": "bobbarker",
						},
						Secrets: "default",
					},
					{
						Type:  "image",
						Image: "mcp/notion:v0.1.0",
						Tools: []string{"do_something"},
					},
				},
				Secrets: map[string]workingset.Secret{
					"default": {
						Provider: "docker-desktop-store",
					},
				},
			})
		},
	}
}

func testReadWorkingSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "test-read",
		Short: "Test read working set",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := workingset.Read("test")
			if err != nil {
				return err
			}
			fmt.Println(ws)
			return nil
		},
	}
}
