package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"

	"github.com/docker/docker-mcp/cmd/docker-mcp/catalog"
)

type Info struct {
	Tools  []any  `json:"tools"`
	Readme string `json:"readme"`
}

func (s Info) ToJSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

func Inspect(ctx context.Context, serverName string) (Info, error) {
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
		tools     []any
		readmeRaw []byte
		errs      errgroup.Group
	)
	errs.Go(func() error {
		toolsRaw, err := fetch(ctx, server.ToolsURL)
		if err != nil {
			return err
		}

		if err := json.Unmarshal(toolsRaw, &tools); err != nil {
			return err
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

	resp, err := http.DefaultClient.Do(req)
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
