package secret

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/formatting"
)

type ListOptions struct {
	JSON bool
}

// TODO: List needs to query the secrets engine
func List(ctx context.Context, opts ListOptions) error {
	var secrets []struct {
		Name     string
		Provider string
	}
	// fetch secrets from secrets engine

	if opts.JSON {
		jsonData, err := json.MarshalIndent(secrets, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(jsonData))
		return nil
	}
	var rows [][]string
	for _, v := range secrets {
		rows = append(rows, []string{v.Name, v.Provider})
	}
	formatting.PrettyPrintTable(rows, []int{40, 120})
	return nil
}
