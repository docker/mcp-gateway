package gcf

import (
	"fmt"
	"strconv"
	"strings"
)

// DeltaPayload represents the diff between a prior context pack and the
// current result. Used for incremental context delivery.
type DeltaPayload struct {
	Tool         string
	BaseRoot     string // pack_root the consumer has
	NewRoot      string // pack_root of the current result
	Removed      []Symbol
	Added        []Symbol
	RemovedEdges []Edge
	AddedEdges   []Edge
	DeltaTokens  int
	FullTokens   int
}

// EncodeDelta serializes a DeltaPayload into GCF delta format.
func EncodeDelta(d *DeltaPayload) string {
	var b strings.Builder

	// Header.
	savings := 0.0
	if d.FullTokens > 0 {
		savings = 100.0 * (1.0 - float64(d.DeltaTokens)/float64(d.FullTokens))
	}
	b.WriteString(fmt.Sprintf("GCF profile=graph tool=%s delta=true base_root=%s new_root=%s tokens=%d savings=%.0f%%\n",
		d.Tool, d.BaseRoot, d.NewRoot, d.DeltaTokens, savings))

	// Removed symbols: short references (consumer already has the full declaration).
	if len(d.Removed) > 0 {
		b.WriteString("## removed\n")
		for _, s := range d.Removed {
			kind := KindAbbrev[s.Kind]
			if kind == "" {
				kind = s.Kind
			}
			b.WriteString(fmt.Sprintf("%s %s\n", kind, s.QualifiedName))
		}
	}

	// Added symbols: full declarations (consumer doesn't have these).
	if len(d.Added) > 0 {
		b.WriteString("## added\n")
		for i, s := range d.Added {
			kind := KindAbbrev[s.Kind]
			if kind == "" {
				kind = s.Kind
			}
			b.WriteString(fmt.Sprintf("@%d %s %s %.2f %s %d\n",
				i, kind, s.QualifiedName, s.Score, s.Provenance, s.Distance))
		}
	}

	// Removed edges.
	if len(d.RemovedEdges) > 0 {
		b.WriteString("## edges_removed\n")
		for _, e := range d.RemovedEdges {
			b.WriteString(fmt.Sprintf("%s -> %s %s\n", e.Source, e.Target, e.EdgeType))
		}
	}

	// Added edges.
	if len(d.AddedEdges) > 0 {
		b.WriteString("## edges_added\n")
		for _, e := range d.AddedEdges {
			b.WriteString(fmt.Sprintf("%s -> %s %s\n", e.Source, e.Target, e.EdgeType))
		}
	}

	return b.String()
}

// expandKind reverses a kind abbreviation to its full form (identity if unknown).
func expandKind(k string) string {
	if full, ok := KindExpand[k]; ok {
		return full
	}
	return k
}

// parseDeltaEdge parses a `source -> target type` delta edge line.
func parseDeltaEdge(line string) (Edge, error) {
	idx := strings.Index(line, " -> ")
	if idx < 0 {
		return Edge{}, fmt.Errorf("malformed_delta: edge line missing ' -> ': %q", line)
	}
	source := line[:idx]
	rest := strings.Fields(line[idx+4:])
	if len(rest) != 2 {
		return Edge{}, fmt.Errorf("malformed_delta: edge line %q must be 'source -> target type'", line)
	}
	return Edge{Source: source, Target: rest[0], EdgeType: rest[1]}, nil
}

// DecodeDelta parses a GCF graph delta wire payload (as produced by EncodeDelta)
// back into a DeltaPayload. Kind abbreviations on removed/added lines are expanded
// to their full form so the result matches a base snapshot's symbol identities.
func DecodeDelta(input string) (*DeltaPayload, error) {
	lines := strings.Split(strings.TrimRight(input, "\n"), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return nil, fmt.Errorf("missing_header: empty delta payload")
	}
	header := strings.TrimRight(lines[0], "\r")
	if !strings.HasPrefix(header, "GCF profile=graph") {
		return nil, fmt.Errorf("missing_profile: delta header must begin with 'GCF profile=graph'")
	}

	d := &DeltaPayload{}
	for _, field := range strings.Fields(header) {
		kv := strings.SplitN(field, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "tool":
			d.Tool = kv[1]
		case "base_root":
			d.BaseRoot = kv[1]
		case "new_root":
			d.NewRoot = kv[1]
		}
	}

	section := ""
	for _, raw := range lines[1:] {
		line := strings.TrimRight(raw, "\r")
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			section = strings.TrimSpace(line[3:])
			switch section {
			case "removed", "added", "edges_removed", "edges_added":
			default:
				return nil, fmt.Errorf("malformed_delta: unknown section %q", section)
			}
			continue
		}
		switch section {
		case "removed":
			parts := strings.Fields(line)
			if len(parts) != 2 {
				return nil, fmt.Errorf("malformed_delta: removed line %q must be 'kind qname'", line)
			}
			d.Removed = append(d.Removed, Symbol{Kind: expandKind(parts[0]), QualifiedName: parts[1]})
		case "added":
			parts := strings.Fields(line)
			if len(parts) != 6 {
				return nil, fmt.Errorf("malformed_delta: added line %q must be '@id kind qname score provenance distance'", line)
			}
			score, err := strconv.ParseFloat(parts[3], 64)
			if err != nil {
				return nil, fmt.Errorf("malformed_delta: invalid added score %q", parts[3])
			}
			dist, err := strconv.Atoi(parts[5])
			if err != nil {
				return nil, fmt.Errorf("malformed_delta: invalid added distance %q", parts[5])
			}
			d.Added = append(d.Added, Symbol{
				Kind:          expandKind(parts[1]),
				QualifiedName: parts[2],
				Score:         score,
				Provenance:    parts[4],
				Distance:      dist,
			})
		case "edges_removed", "edges_added":
			e, err := parseDeltaEdge(line)
			if err != nil {
				return nil, err
			}
			if section == "edges_removed" {
				d.RemovedEdges = append(d.RemovedEdges, e)
			} else {
				d.AddedEdges = append(d.AddedEdges, e)
			}
		default:
			return nil, fmt.Errorf("malformed_delta: data line %q before any section header", line)
		}
	}
	return d, nil
}
