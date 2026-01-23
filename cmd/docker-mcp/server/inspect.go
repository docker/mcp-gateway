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
	catalogpkg "github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/config"
	"github.com/docker/mcp-gateway/pkg/desktop"
	"github.com/docker/mcp-gateway/pkg/docker"
	"github.com/docker/mcp-gateway/pkg/policy"
	policycli "github.com/docker/mcp-gateway/pkg/policy/cli"
	policycontext "github.com/docker/mcp-gateway/pkg/policy/context"
)

type Info struct {
	Tools  []Tool `json:"tools"`
	Readme string `json:"readme"`
	// Policy describes the policy decision for this server.
	Policy *policy.Decision `json:"policy,omitempty"`
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
	// Policy describes the policy decision for this tool.
	Policy *policy.Decision `json:"policy,omitempty"`
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

	catalogID := catalogpkg.DockerCatalogFilename
	if _, name, _, err := catalogpkg.ReadOne(ctx, catalogpkg.DockerCatalogFilename); err == nil && name != "" {
		catalogID = name
	}
	policyClient := policycli.ClientForCLI(ctx)
	policyCtx := policycontext.Context{
		Catalog:                  catalogID,
		WorkingSet:               "",
		ServerSourceTypeOverride: "registry",
	}
	var serverSpec catalogpkg.Server
	catalogData, err := catalogpkg.Get(ctx)
	if err == nil {
		if catalogServer, ok := catalogData.Servers[serverName]; ok {
			serverSpec = catalogServer
		}
	}
	var serverPolicy *policy.Decision
	if serverSpec.Name != "" || serverSpec.Image != "" || serverSpec.Type != "" {
		serverPolicy = policycli.DecisionForRequest(
			ctx,
			policyClient,
			policycontext.BuildRequest(
				policyCtx,
				serverName,
				serverSpec,
				"",
				policy.ActionLoad,
			),
		)
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

	// Attach policy decisions for tools using batch evaluation.
	if len(tools) > 0 && policyClient != nil &&
		(serverSpec.Name != "" || serverSpec.Image != "" || serverSpec.Type != "") {
		// Metadata for mapping batch results back to tools.
		type toolMeta struct {
			toolIndex   int // Index into tools slice.
			loadIndex   int // Index in batch request for load action.
			invokeIndex int // Index in batch request for invoke action.
		}

		var requests []policy.Request
		var toolMetas []toolMeta

		// Build batch request with all tool policy evaluations.
		for i, tool := range tools {
			loadIndex := len(requests)
			requests = append(requests, policycontext.BuildRequest(
				policyCtx,
				serverName,
				serverSpec,
				tool.Name,
				policy.ActionLoad,
			))
			invokeIndex := len(requests)
			requests = append(requests, policycontext.BuildRequest(
				policyCtx,
				serverName,
				serverSpec,
				tool.Name,
				policy.ActionInvoke,
			))
			toolMetas = append(toolMetas, toolMeta{
				toolIndex:   i,
				loadIndex:   loadIndex,
				invokeIndex: invokeIndex,
			})
		}

		// Evaluate all requests in a single batch call.
		decisions, err := policyClient.EvaluateBatch(ctx, requests)
		decisions, _ = policycli.NormalizeBatchDecisions(requests, decisions, err)

		// Apply tool decisions. Use load result if blocked, otherwise invoke.
		for _, tm := range toolMetas {
			loadDecision := decisionToPtr(decisions[tm.loadIndex])
			if loadDecision != nil {
				tools[tm.toolIndex].Policy = loadDecision
			} else {
				tools[tm.toolIndex].Policy = decisionToPtr(decisions[tm.invokeIndex])
			}
		}
	}

	return Info{
		Tools:  tools,
		Readme: string(readmeRaw),
		Policy: serverPolicy,
	}, nil
}

// TODO: Should we get all those directly with the catalog?
func fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: desktop.ProxyTransport(),
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

// decisionToPtr converts a policy decision to a pointer. Returns nil for
// allowed decisions (matching DecisionForRequest behavior).
func decisionToPtr(dec policy.Decision) *policy.Decision {
	if dec.Allowed {
		return nil
	}
	return &dec
}
