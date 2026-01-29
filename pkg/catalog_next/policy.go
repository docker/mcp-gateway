package catalognext

import (
	"context"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/policy"
	policycli "github.com/docker/mcp-gateway/pkg/policy/cli"
	policycontext "github.com/docker/mcp-gateway/pkg/policy/context"
	"github.com/docker/mcp-gateway/pkg/policy/policyutil"
)

// policyContextForCatalog returns policy context for a catalog reference.
func policyContextForCatalog(catalogRef string) policycontext.Context {
	return policycontext.Context{
		Catalog:    catalogRef,
		WorkingSet: "",
	}
}

// attachCatalogPolicy decorates a catalog and its servers with policy
// decisions. It uses batch evaluation to minimize HTTP overhead when
// evaluating many servers and tools.
func attachCatalogPolicy(
	ctx context.Context,
	client policy.Client,
	catalogRef string,
	cat *CatalogWithDigest,
	includeTools bool,
) {
	if client == nil || cat == nil {
		return
	}

	ctxData := policyContextForCatalog(catalogRef)

	// Metadata for mapping batch results back to catalog/servers/tools.
	type serverMeta struct {
		index int // Index in batch request.
	}
	type toolMeta struct {
		serverIndex int // Index into catalog.Servers.
		toolIndex   int // Index into server.Snapshot.Server.Tools.
		loadIndex   int // Index in batch request for load action.
		invokeIndex int // Index in batch request for invoke action.
	}

	var requests []policy.Request
	var catalogIndex int
	var serverMetas []serverMeta
	var toolMetas []toolMeta

	// Add catalog load policy request.
	catalogIndex = len(requests)
	requests = append(requests, policycontext.BuildCatalogRequest(
		ctxData, catalogRef, policy.ActionLoad,
	))

	// Build batch request with all server and tool policy evaluations.
	for i := range cat.Servers {
		server := &cat.Servers[i]
		serverCtx := ctxData
		serverCtx.ServerSourceTypeOverride = string(server.Type)

		spec, serverName := policyServerSpec(*server)
		if serverName == "" && spec.Type == "" && spec.Image == "" &&
			spec.Remote.URL == "" {
			serverMetas = append(serverMetas, serverMeta{index: -1})
			continue
		}

		// Add server load policy request.
		serverMetas = append(serverMetas, serverMeta{index: len(requests)})
		requests = append(requests, policycontext.BuildRequest(
			serverCtx, serverName, spec, "", policy.ActionLoad,
		))

		// Add tool policy requests (both load and invoke) if requested.
		if !includeTools || server.Snapshot == nil {
			continue
		}
		for j, tool := range server.Snapshot.Server.Tools {
			loadIndex := len(requests)
			requests = append(requests, policycontext.BuildRequest(
				serverCtx, serverName, spec, tool.Name, policy.ActionLoad,
			))
			invokeIndex := len(requests)
			requests = append(requests, policycontext.BuildRequest(
				serverCtx, serverName, spec, tool.Name, policy.ActionInvoke,
			))
			toolMetas = append(toolMetas, toolMeta{
				serverIndex: i,
				toolIndex:   j,
				loadIndex:   loadIndex,
				invokeIndex: invokeIndex,
			})
		}
	}

	if len(requests) == 0 {
		return
	}

	// Evaluate all requests in a single batch call.
	decisions, err := client.EvaluateBatch(ctx, requests)
	decisions, _ = policycli.NormalizeBatchDecisions(requests, decisions, err)

	// Apply catalog decision.
	cat.Policy = policy.DecisionForOutput(decisions[catalogIndex])

	// Apply server decisions.
	for i, sm := range serverMetas {
		if sm.index < 0 {
			continue
		}
		cat.Servers[i].Policy = policy.DecisionForOutput(decisions[sm.index])
	}

	// Apply tool decisions. Use load result if blocked, otherwise invoke.
	for _, tm := range toolMetas {
		server := &cat.Servers[tm.serverIndex]
		tool := &server.Snapshot.Server.Tools[tm.toolIndex]
		loadDecision := policy.DecisionForOutput(decisions[tm.loadIndex])
		if loadDecision != nil {
			tool.Policy = loadDecision
		} else {
			tool.Policy = policy.DecisionForOutput(decisions[tm.invokeIndex])
		}
	}
}

// policyServerSpec returns a catalog server spec and name for policy evaluation.
func policyServerSpec(server Server) (catalog.Server, string) {
	return policyutil.ServerSpecFromSnapshot(
		server.snapshotServer(),
		string(server.Type),
		server.Source,
		server.Image,
		server.Endpoint,
	)
}

// snapshotServer returns the catalog server snapshot if present.
func (server Server) snapshotServer() *catalog.Server {
	if server.Snapshot == nil {
		return nil
	}
	return &server.Snapshot.Server
}

// allowedToolCount returns the number of tools not blocked by policy.
func allowedToolCount(tools []catalog.Tool) int {
	return policyutil.AllowedToolCount(tools)
}
