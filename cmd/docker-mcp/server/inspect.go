package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"

	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/catalog"
	"github.com/docker/mcp-gateway/pkg/config"
	"github.com/docker/mcp-gateway/pkg/docker"
)

type Info struct {
	Tools  []Tool `json:"tools"`
	Readme string `json:"readme"`
}

func (s Info) ToJSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

type ToolArgument struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Desc string `json:"desc"`
}

type Tool struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Arguments   []ToolArgument             `json:"arguments,omitempty"`
	Annotations map[string]json.RawMessage `json:"annotations,omitempty"`
	Enabled     bool                       `json:"enabled"`
}

func Inspect(ctx context.Context, dockerClient docker.Client, serverName string) (Info, error) {
	catalogYAML, err := catalog.ReadCatalogFile(catalog.DockerCatalogName)
	if err != nil {
		return Info{}, err
	}

	var registry catalog.Registry
	if err := yaml.Unmarshal(catalogYAML, &registry); err != nil {
		return Info{}, err
	}

	server, found := registry.Registry[serverName]
	if !found {
		return Info{}, fmt.Errorf("server %q not found in catalog", serverName)
	}

	var (
		tools     []Tool
		readmeRaw []byte
		errs      errgroup.Group
	)
	errs.Go(func() error {
		// Do not fetch tools if config states tools will be dynamic
		if server.Dynamic != nil && server.Dynamic.Tools {
			tools = []Tool{}
			return nil
		}

		toolsRaw, err := fetch(ctx, server.ToolsURL)
		if err != nil {
			return err
		}

		if err := json.Unmarshal(toolsRaw, &tools); err != nil {
			return err
		}

		toolsYAML, err := config.ReadTools(ctx, dockerClient)
		if err != nil {
			return err
		}

		toolsConfig, err := config.ParseToolsConfig(toolsYAML)
		if err != nil {
			return err
		}

		serverTools, exists := toolsConfig.ServerTools[serverName]
		for i := range tools {
			// If server is not present => all tools are enabled
			if !exists {
				tools[i].Enabled = true
				continue
			}
			// If server is present => only listed tools are enabled
			tools[i].Enabled = slices.Contains(serverTools, tools[i].Name)
		}

		return nil
	})
	errs.Go(func() error {
		var err error
		readmeRaw, err = fetch(ctx, server.ReadmeURL)
		if err != nil {
			return err
		}

		return nil
	})
	if err := errs.Wait(); err != nil {
		return Info{}, err
	}

	return Info{
		Tools:  tools,
		Readme: string(readmeRaw),
	}, nil
}

// TODO: Should we get all those directly with the catalog?
func fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: http.DefaultTransport,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch %s: %s", url, resp.Status)
	}

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return buf, nil
}
