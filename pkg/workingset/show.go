package workingset

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/pkg/client"
	"github.com/docker/mcp-gateway/pkg/db"
	policycli "github.com/docker/mcp-gateway/pkg/policy/cli"
)

type WithOptions struct {
	WorkingSet `yaml:",inline"`
	Clients    map[string]any `json:"clients" yaml:"clients"`
}

func Show(ctx context.Context, dao db.DAO, id string, format OutputFormat, showClients bool, yqExpr string) error {
	dbSet, err := dao.GetWorkingSet(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("profile %s not found", id)
		}
		return fmt.Errorf("failed to get profile: %w", err)
	}

	workingSet := NewFromDb(dbSet)
	policyClient := policycli.ClientForCLI(ctx)
	attachWorkingSetPolicy(ctx, policyClient, &workingSet, true)

	var data []byte
	switch format {
	case OutputFormatJSON:
		if showClients {
			outputData := WithOptions{
				WorkingSet: workingSet,
				Clients:    client.FindClientsByProfile(ctx, id),
			}
			data, err = json.MarshalIndent(outputData, "", "  ")
		} else {
			data, err = json.MarshalIndent(workingSet, "", "  ")
		}
	case OutputFormatYAML, OutputFormatHumanReadable:
		if showClients {
			outputData := WithOptions{
				WorkingSet: workingSet,
				Clients:    client.FindClientsByProfile(ctx, id),
			}
			data, err = yaml.Marshal(outputData)
		} else {
			data, err = yaml.Marshal(workingSet)
		}
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}

	if yqExpr != "" {
		data, err = ApplyYqExpression(data, format, yqExpr)
		if err != nil {
			return err // wrapping error here would be redundant
		}
	}

	fmt.Println(string(data))

	return nil
}
