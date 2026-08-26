package gcf

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

// PackRoot computes the canonical pack root hash for a graph snapshot
// using the gcf-pack-root-v1 algorithm (SPEC Section 10.2).
//
// Two implementations given the same logical graph MUST produce the same result.
func PackRoot(symbols []Symbol, edges []Edge) string {
	// Build canonical symbol records.
	symRecords := make([]string, len(symbols))
	for i, s := range symbols {
		kind := KindAbbrev[s.Kind]
		if kind == "" {
			kind = s.Kind
		}
		score := formatNumber(s.Score)
		symRecords[i] = fmt.Sprintf("S\t%s\t%s\t%s\t%s\t%d\n", kind, s.QualifiedName, score, s.Provenance, s.Distance)
	}

	// Build canonical edge records.
	// Include source/target kind to disambiguate symbols with same qname but different kinds.
	// Edge endpoints are resolved to their symbol identity (kind, qname).
	symKindMap := make(map[string]string, len(symbols))
	for _, s := range symbols {
		kind := KindAbbrev[s.Kind]
		if kind == "" {
			kind = s.Kind
		}
		symKindMap[s.QualifiedName] = kind
	}
	edgeRecords := make([]string, len(edges))
	for i, e := range edges {
		srcKind := symKindMap[e.Source]
		tgtKind := symKindMap[e.Target]
		edgeRecords[i] = fmt.Sprintf("E\t%s\t%s\t%s\t%s\t%s\n", srcKind, e.Source, tgtKind, e.Target, e.EdgeType)
	}

	// Sort independently by unsigned UTF-8 byte order.
	sort.Strings(symRecords)
	sort.Strings(edgeRecords)

	// Concatenate: all symbols then all edges.
	var b strings.Builder
	for _, r := range symRecords {
		b.WriteString(r)
	}
	for _, r := range edgeRecords {
		b.WriteString(r)
	}

	// SHA-256.
	h := sha256.Sum256([]byte(b.String()))
	return fmt.Sprintf("sha256:%x", h)
}

// VerifyDelta verifies that applying a delta to a base snapshot produces the expected new_root.
// Returns the resulting symbols and edges if verification succeeds, or an error if it fails.
func VerifyDelta(
	baseSymbols []Symbol, baseEdges []Edge,
	removedSymbols []Symbol, addedSymbols []Symbol,
	removedEdges []Edge, addedEdges []Edge,
	expectedNewRoot string,
) ([]Symbol, []Edge, error) {
	// Build index of base symbols by identity (kind, qname).
	type symKey struct{ kind, qname string }
	symMap := make(map[symKey]Symbol, len(baseSymbols))
	for _, s := range baseSymbols {
		symMap[symKey{s.Kind, s.QualifiedName}] = s
	}

	// Apply removals.
	for _, s := range removedSymbols {
		key := symKey{s.Kind, s.QualifiedName}
		if _, exists := symMap[key]; !exists {
			return nil, nil, fmt.Errorf("delta_invalid: removing symbol %s %s that does not exist in base", s.Kind, s.QualifiedName)
		}
		delete(symMap, key)
	}

	// Apply additions.
	for _, s := range addedSymbols {
		key := symKey{s.Kind, s.QualifiedName}
		if _, exists := symMap[key]; exists {
			return nil, nil, fmt.Errorf("delta_invalid: adding symbol %s %s that already exists", s.Kind, s.QualifiedName)
		}
		symMap[key] = s
	}

	// Build result symbols.
	resultSymbols := make([]Symbol, 0, len(symMap))
	for _, s := range symMap {
		resultSymbols = append(resultSymbols, s)
	}

	// Build index of base edges.
	type edgeKey struct{ source, target, edgeType string }
	edgeMap := make(map[edgeKey]Edge, len(baseEdges))
	for _, e := range baseEdges {
		edgeMap[edgeKey{e.Source, e.Target, e.EdgeType}] = e
	}

	// Apply edge removals.
	for _, e := range removedEdges {
		key := edgeKey{e.Source, e.Target, e.EdgeType}
		if _, exists := edgeMap[key]; !exists {
			return nil, nil, fmt.Errorf("delta_invalid: removing edge %s -> %s %s that does not exist", e.Source, e.Target, e.EdgeType)
		}
		delete(edgeMap, key)
	}

	// Apply edge additions.
	for _, e := range addedEdges {
		key := edgeKey{e.Source, e.Target, e.EdgeType}
		if _, exists := edgeMap[key]; exists {
			return nil, nil, fmt.Errorf("delta_invalid: adding edge %s -> %s %s that already exists", e.Source, e.Target, e.EdgeType)
		}
		edgeMap[key] = e
	}

	// Build result edges.
	resultEdges := make([]Edge, 0, len(edgeMap))
	for _, e := range edgeMap {
		resultEdges = append(resultEdges, e)
	}

	// Verify pack root.
	computedRoot := PackRoot(resultSymbols, resultEdges)
	if computedRoot != expectedNewRoot {
		return nil, nil, fmt.Errorf("root_mismatch: computed %s, expected %s", computedRoot, expectedNewRoot)
	}

	return resultSymbols, resultEdges, nil
}
