package workingset

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/formatting"
	"github.com/docker/mcp-gateway/pkg/db"
)

type SearchResult struct {
	ID      string   `json:"id" yaml:"id"`
	Name    string   `json:"name" yaml:"name"`
	Servers []Server `json:"servers" yaml:"servers"`
}

func Search(ctx context.Context, dao db.DAO, query string, workingSetID string, format OutputFormat) error {
	dbSets, err := dao.SearchWorkingSets(ctx, query, workingSetID)
	if err != nil {
		return fmt.Errorf("failed to search working sets: %w", err)
	}

	results := filterResults(dbSets, query)
	return outputSearchResults(results, format)
}

func getServerIdentifier(server Server) string {
	if server.Type == ServerTypeImage {
		return server.Image
	}
	return server.Source
}

func filterResults(dbSets []db.WorkingSet, query string) []SearchResult {
	queryLower := strings.ToLower(query)
	results := make([]SearchResult, 0, len(dbSets))

	for _, dbSet := range dbSets {
		workingSet := NewFromDb(&dbSet)
		matchedServers := make([]Server, 0)

		for _, server := range workingSet.Servers {
			if query == "" {
				matchedServers = append(matchedServers, server)
			} else {
				identifier := getServerIdentifier(server)
				if strings.Contains(strings.ToLower(identifier), queryLower) {
					matchedServers = append(matchedServers, server)
				}
			}
		}

		if len(matchedServers) == 0 {
			continue
		}

		sort.Slice(matchedServers, func(i, j int) bool {
			return getServerIdentifier(matchedServers[i]) < getServerIdentifier(matchedServers[j])
		})

		results = append(results, SearchResult{
			ID:      workingSet.ID,
			Name:    workingSet.Name,
			Servers: matchedServers,
		})
	}

	return results
}

func outputSearchResults(results []SearchResult, format OutputFormat) error {
	var data []byte
	var err error

	switch format {
	case OutputFormatHumanReadable:
		printSearchResultsHuman(results)
		return nil
	case OutputFormatJSON:
		data, err = json.MarshalIndent(results, "", "  ")
	case OutputFormatYAML:
		data, err = yaml.Marshal(results)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return fmt.Errorf("failed to format search results: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func printSearchResultsHuman(results []SearchResult) {
	if len(results) == 0 {
		fmt.Println("No working sets found")
		return
	}

	rows := [][]string{{"WORKING SET", "TYPE", "IDENTIFIER"}}
	totalMatches := 0

	for _, result := range results {
		for _, server := range result.Servers {
			rows = append(rows, []string{
				result.ID,
				string(server.Type),
				getServerIdentifier(server),
			})
			totalMatches++
		}
	}

	formatting.PrettyPrintTable(rows, []int{40, 10, 120})
}
