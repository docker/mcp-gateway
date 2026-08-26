package gcf

import (
	"fmt"
	"strconv"
	"strings"
)

// Decode parses GCF text back into a Payload.
func Decode(input string) (*Payload, error) {
	lines := strings.Split(input, "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("gcf: empty input")
	}

	p := &Payload{}

	// Parse header.
	header := lines[0]
	if !strings.HasPrefix(header, "GCF ") {
		return nil, fmt.Errorf("gcf: invalid header, expected 'GCF ...' got %q", header)
	}
	if err := parseHeader(header[4:], p); err != nil {
		return nil, err
	}
	// Graph payloads MUST declare profile=graph as the first header field
	// (SPEC Sections 3.1 and 16.3). The buffered, session, and streaming
	// encoders all emit it; reject a header that omits or misstates it.
	if hf := strings.Fields(header[4:]); len(hf) == 0 || hf[0] != "profile=graph" {
		return nil, fmt.Errorf("gcf: graph header must begin with 'GCF profile=graph', got %q", header)
	}
	// tool is optional since v3.1 (SHOULD for MCP, not required)

	// Detect delta mode.
	isDelta := false
	for _, part := range strings.Fields(header[4:]) {
		if part == "delta=true" {
			isDelta = true
		}
	}

	var validDeltaSections = map[string]bool{
		"removed": true, "added": true, "edges_removed": true, "edges_added": true,
	}

	// Parse body: symbols and edges.
	var symbols []Symbol
	symByID := make(map[int]*Symbol)
	currentDistance := 0
	inEdges := false
	declaredEdges := -1
	edgesDeclared := false

	for _, line := range lines[1:] {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}

		// Skip ##! summary trailer.
		if strings.HasPrefix(line, "##! ") {
			continue
		}

		// Group header.
		if strings.HasPrefix(line, "## ") {
			group := line[3:]
			// Strip bracket suffix: "edges [200]" -> "edges", capturing the
			// declared count so it can be enforced per Section 13.
			declaredCount := -1
			if idx := strings.Index(group, " ["); idx >= 0 {
				bracket := group[idx+2:]
				group = group[:idx]
				if end := strings.Index(bracket, "]"); end >= 0 {
					cntStr := bracket[:end]
					if cntStr != "?" { // "[?]" is a streaming deferred count (Section 8)
						n, err := strconv.Atoi(cntStr)
						if err != nil {
							return nil, fmt.Errorf("count_mismatch: invalid section count %q", cntStr)
						}
						declaredCount = n
					}
				}
			}

			if isDelta && !validDeltaSections[group] {
				return nil, fmt.Errorf("malformed_delta: invalid delta section %q", group)
			}

			inEdges = group == "edges"
			if inEdges && declaredCount >= 0 {
				declaredEdges = declaredCount
				edgesDeclared = true
			}
			if !inEdges {
				switch group {
				case "targets":
					currentDistance = 0
				case "related":
					currentDistance = 1
				case "extended":
					currentDistance = 2
				default:
					if strings.HasPrefix(group, "distance_") {
						d, err := strconv.Atoi(group[9:])
						if err == nil {
							currentDistance = d
						}
					}
				}
			}
			continue
		}

		// Comment.
		if strings.HasPrefix(line, "# ") {
			continue
		}

		if inEdges {
			edge, err := parseEdgeLine(line, symByID)
			if err != nil {
				return nil, err
			}
			p.Edges = append(p.Edges, edge)
		} else {
			sym, id, err := parseSymbolLine(line, currentDistance)
			if err != nil {
				return nil, err
			}
			symbols = append(symbols, sym)
			symByID[id] = &symbols[len(symbols)-1]
		}
	}

	// Section 13: a declared [N] section count MUST match the actual item count.
	// The graph edges section is the graph profile's only [N]-bearing section.
	if edgesDeclared && len(p.Edges) != declaredEdges {
		return nil, fmt.Errorf("count_mismatch: declared %d edges, got %d", declaredEdges, len(p.Edges))
	}

	p.Symbols = symbols
	return p, nil
}

func parseHeader(fields string, p *Payload) error {
	for _, part := range strings.Fields(fields) {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "tool":
			p.Tool = kv[1]
		case "budget":
			v, err := strconv.Atoi(kv[1])
			if err != nil {
				return fmt.Errorf("gcf: invalid budget %q: %w", kv[1], err)
			}
			p.TokenBudget = v
		case "tokens":
			v, err := strconv.Atoi(kv[1])
			if err != nil {
				return fmt.Errorf("gcf: invalid tokens %q: %w", kv[1], err)
			}
			p.TokensUsed = v
		case "pack_root":
			p.PackRoot = kv[1]
		case "symbols":
			// informational, reconstructed from parsed symbols
		}
	}
	return nil
}

func parseSymbolLine(line string, distance int) (Symbol, int, error) {
	if !strings.HasPrefix(line, "@") {
		return Symbol{}, 0, fmt.Errorf("gcf: expected symbol line starting with @, got %q", line)
	}

	parts := strings.Fields(line)
	if len(parts) < 5 {
		return Symbol{}, 0, fmt.Errorf("invalid_node_line: symbol line needs at least 5 fields, got %d in %q", len(parts), line)
	}

	idStr := parts[0][1:] // strip @
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return Symbol{}, 0, fmt.Errorf("invalid_symbol_id: invalid symbol id %q: %w", idStr, err)
	}

	kind := parts[1]
	if expanded, ok := KindExpand[kind]; ok {
		kind = expanded
	}

	qname := parts[2]

	score, err := strconv.ParseFloat(parts[3], 64)
	if err != nil {
		return Symbol{}, 0, fmt.Errorf("invalid_score: invalid score %q: %w", parts[3], err)
	}

	provenance := parts[4]

	return Symbol{
		QualifiedName: qname,
		Kind:          kind,
		Score:         score,
		Provenance:    provenance,
		Distance:      distance,
	}, id, nil
}

func parseEdgeLine(line string, symByID map[int]*Symbol) (Edge, error) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return Edge{}, fmt.Errorf("gcf: edge line needs at least 2 fields, got %q", line)
	}

	ref := parts[0]
	ltIdx := strings.Index(ref, "<")
	if ltIdx < 0 {
		return Edge{}, fmt.Errorf("invalid_edge_syntax: edge line missing '<' separator in %q", ref)
	}

	targetIDStr := ref[1:ltIdx]  // strip leading @
	sourceIDStr := ref[ltIdx+2:] // strip <@

	targetID, err := strconv.Atoi(targetIDStr)
	if err != nil {
		return Edge{}, fmt.Errorf("gcf: invalid target id %q: %w", targetIDStr, err)
	}
	sourceID, err := strconv.Atoi(sourceIDStr)
	if err != nil {
		return Edge{}, fmt.Errorf("gcf: invalid source id %q: %w", sourceIDStr, err)
	}

	targetSym := symByID[targetID]
	sourceSym := symByID[sourceID]
	if targetSym == nil || sourceSym == nil {
		return Edge{}, fmt.Errorf("unknown_edge_reference: edge references unknown symbol id(s): target=%d source=%d", targetID, sourceID)
	}

	edgeType := parts[1]
	status := ""
	if len(parts) >= 3 {
		status = parts[2]
	}

	return Edge{
		Source:   sourceSym.QualifiedName,
		Target:   targetSym.QualifiedName,
		EdgeType: edgeType,
		Status:   status,
	}, nil
}
