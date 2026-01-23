package workingset

import (
	"context"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/policy"
	policycli "github.com/docker/mcp-gateway/pkg/policy/cli"
	policycontext "github.com/docker/mcp-gateway/pkg/policy/context"
	"github.com/docker/mcp-gateway/pkg/policy/policyutil"
)

// policyContextForWorkingSet returns policy context for a working set.
func policyContextForWorkingSet(workingSetID string) policycontext.Context {
	return policycontext.Context{
		Catalog:    "",
		WorkingSet: workingSetID,
	}
}

// attachWorkingSetPolicy decorates a working set and its servers with policy
// decisions. It uses batch evaluation to minimize HTTP overhead when
// evaluating many servers and tools.
func attachWorkingSetPolicy(
	ctx context.Context,
	client policy.Client,
	ws *WorkingSet,
	includeTools bool,
) {
	if client == nil || ws == nil {
		return
	}

	ctxData := policyContextForWorkingSet(ws.ID)

	// Metadata for mapping batch results back to working set/servers/tools.
	type serverMeta struct {
		index int // Index in batch request.
	}
	type toolMeta struct {
		serverIndex int // Index into ws.Servers.
		toolIndex   int // Index into server.Snapshot.Server.Tools.
		loadIndex   int // Index in batch request for load action.
		invokeIndex int // Index in batch request for invoke action.
	}

	var requests []policy.Request
	var wsIndex int
	var serverMetas []serverMeta
	var toolMetas []toolMeta

	// Add working set load policy request.
	wsIndex = len(requests)
	requests = append(requests, policycontext.BuildWorkingSetRequest(
		ctxData, ws.ID, policy.ActionLoad,
	))

	// Build batch request with all server and tool policy evaluations.
	for i := range ws.Servers {
		server := &ws.Servers[i]
		serverCtx := ctxData
		serverCtx.ServerSourceTypeOverride = string(server.Type)

		spec, serverName := policyWorkingSetServerSpec(*server)
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

	// Apply working set decision.
	ws.Policy = decisionToPtr(decisions[wsIndex])

	// Apply server decisions.
	for i, sm := range serverMetas {
		if sm.index < 0 {
			continue
		}
		ws.Servers[i].Policy = decisionToPtr(decisions[sm.index])
	}

	// Apply tool decisions. Use load result if blocked, otherwise invoke.
	for _, tm := range toolMetas {
		server := &ws.Servers[tm.serverIndex]
		tool := &server.Snapshot.Server.Tools[tm.toolIndex]
		loadDecision := decisionToPtr(decisions[tm.loadIndex])
		if loadDecision != nil {
			tool.Policy = loadDecision
		} else {
			tool.Policy = decisionToPtr(decisions[tm.invokeIndex])
		}
	}
}

// decisionToPtr converts a policy decision to a pointer. Returns nil for
// allowed decisions (matching DecisionForRequest behavior).
func decisionToPtr(dec policy.Decision) *policy.Decision {
	if dec.Allowed {
		return nil
	}
	return &dec
}

// policyWorkingSetServerSpec returns a catalog server spec and name for policy evaluation.
func policyWorkingSetServerSpec(server Server) (catalog.Server, string) {
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

//nolint:unused // kept for future use
func allowedToolCount(tools []catalog.Tool) int {
	return policyutil.AllowedToolCount(tools)
}
