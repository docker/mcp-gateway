package gcf

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

// StreamEncoder writes GCF output incrementally as symbols and edges arrive.
// Zero buffering: each symbol/edge is written immediately. A trailer summary
// is emitted on Close() with the final counts.
//
// Usage:
//
//	w := os.Stdout // or any io.Writer
//	enc := gcf.NewStreamEncoder(w, "context_for_task", gcf.StreamOptions{TokenBudget: 5000})
//	enc.WriteSymbol(sym1) // emitted immediately
//	enc.WriteSymbol(sym2) // emitted immediately
//	enc.WriteEdge(edge1)  // emitted immediately
//	enc.Close()           // emits ##! summary trailer
type StreamEncoder struct {
	w       io.Writer
	tool    string
	opts    StreamOptions
	mu      sync.Mutex
	started bool

	// Symbol tracking.
	symIndex     map[string]int // qualifiedName -> local ID
	nextID       int
	currentGroup string
	groupCounts  map[string]int // group name -> count
	groupOrder   []string       // group names in header-emission order (deterministic trailer, SPEC §8.4)

	// Edge tracking.
	edgeCount    int
	edgesStarted bool
}

// StreamOptions configures the streaming encoder.
type StreamOptions struct {
	TokenBudget int
	TokensUsed  int
	PackRoot    string
	Session     bool
	// LabeledTrailerCounts opts into the labeled trailer counts form (SPEC §8.4.1):
	// the graph "##! summary" counts field is emitted as label:count
	// (counts=targets:2,related:1,edges:3) instead of the positional default. A
	// producer-side comprehension aid for weaker consumers; decoder-ignored either way.
	LabeledTrailerCounts bool
}

// NewStreamEncoder creates a streaming encoder that writes to w.
// The header is emitted immediately (without symbols= or edges= counts).
func NewStreamEncoder(w io.Writer, tool string, opts StreamOptions) *StreamEncoder {
	enc := &StreamEncoder{
		w:           w,
		tool:        tool,
		opts:        opts,
		symIndex:    make(map[string]int),
		groupCounts: make(map[string]int),
	}
	enc.writeHeader()
	return enc
}

func (enc *StreamEncoder) writeHeader() {
	parts := []string{fmt.Sprintf("GCF profile=graph tool=%s", enc.tool)}
	if enc.opts.TokenBudget > 0 {
		parts = append(parts, fmt.Sprintf("budget=%d", enc.opts.TokenBudget))
	}
	if enc.opts.TokensUsed > 0 {
		parts = append(parts, fmt.Sprintf("tokens=%d", enc.opts.TokensUsed))
	}
	if enc.opts.PackRoot != "" {
		parts = append(parts, fmt.Sprintf("pack_root=%s", enc.opts.PackRoot))
	}
	if enc.opts.Session {
		parts = append(parts, "session=true")
	}
	fmt.Fprintf(enc.w, "%s\n", strings.Join(parts, " "))
	enc.started = true
}

// WriteSymbol emits a symbol line immediately. Symbols are grouped by distance;
// group headers are emitted automatically when the distance changes.
func (enc *StreamEncoder) WriteSymbol(s Symbol) {
	enc.mu.Lock()
	defer enc.mu.Unlock()

	// Determine group name from distance.
	groupNames := []string{"targets", "related", "extended"}
	var groupName string
	if s.Distance < len(groupNames) {
		groupName = groupNames[s.Distance]
	} else {
		groupName = fmt.Sprintf("distance_%d", s.Distance)
	}

	// Emit group header if entering a new group.
	if groupName != enc.currentGroup {
		fmt.Fprintf(enc.w, "## %s\n", groupName)
		enc.currentGroup = groupName
	}
	// Record the group's first appearance so the trailer counts follow the actual
	// group-header order deterministically (SPEC §8.4), not Go map iteration order.
	if enc.groupCounts[groupName] == 0 {
		enc.groupOrder = append(enc.groupOrder, groupName)
	}

	// Assign local ID.
	id := enc.nextID
	enc.symIndex[s.QualifiedName] = id
	enc.nextID++

	// Emit symbol line.
	kind := KindAbbrev[s.Kind]
	if kind == "" {
		kind = s.Kind
	}
	fmt.Fprintf(enc.w, "@%d %s %s %.2f %s\n", id, kind, s.QualifiedName, s.Score, s.Provenance)

	// Track count.
	enc.groupCounts[groupName]++
}

// WriteEdge emits an edge line immediately. The edges section header is
// emitted automatically on the first edge (with [?] deferred count).
// Source and target must reference previously-written symbols.
func (enc *StreamEncoder) WriteEdge(e Edge) {
	enc.mu.Lock()
	defer enc.mu.Unlock()

	srcIdx, srcOk := enc.symIndex[e.Source]
	tgtIdx, tgtOk := enc.symIndex[e.Target]
	if !srcOk || !tgtOk {
		return // skip edges referencing unknown symbols
	}

	// Emit edges section header on first edge.
	if !enc.edgesStarted {
		fmt.Fprintf(enc.w, "## edges [?]\n")
		enc.edgesStarted = true
	}

	// Emit edge line.
	line := fmt.Sprintf("@%d<@%d %s", tgtIdx, srcIdx, e.EdgeType)
	if e.Status != "" && e.Status != "unchanged" {
		line += " " + e.Status
	}
	fmt.Fprintf(enc.w, "%s\n", line)
	enc.edgeCount++
}

// WriteBareRef emits a bare reference for a previously-transmitted symbol
// (session mode). The symbol is registered in the local index but not
// fully declared.
func (enc *StreamEncoder) WriteBareRef(qname string, distance int) {
	enc.mu.Lock()
	defer enc.mu.Unlock()

	groupNames := []string{"targets", "related", "extended"}
	var groupName string
	if distance < len(groupNames) {
		groupName = groupNames[distance]
	} else {
		groupName = fmt.Sprintf("distance_%d", distance)
	}

	if groupName != enc.currentGroup {
		fmt.Fprintf(enc.w, "## %s\n", groupName)
		enc.currentGroup = groupName
	}

	id := enc.nextID
	enc.symIndex[qname] = id
	enc.nextID++

	fmt.Fprintf(enc.w, "@%d  # previously transmitted\n", id)
	enc.groupCounts[groupName]++
}

// Close emits the ##! summary trailer with final counts and flushes.
// Must be called after all symbols and edges have been written.
func (enc *StreamEncoder) Close() error {
	enc.mu.Lock()
	defer enc.mu.Unlock()

	// Build sections list: one entry per non-empty distance group in group-header
	// emission order (SPEC §8.4). Iterating the recorded emission order (not the
	// groupCounts map) makes the trailer deterministic and match the section order,
	// including distance_N groups.
	var sections []string
	for _, g := range enc.groupOrder {
		if c := enc.groupCounts[g]; c > 0 {
			sections = append(sections, fmt.Sprintf("%s:%d", g, c))
		}
	}
	// The edge count is always the last counts entry, even when 0 (SPEC §8.4,
	// §8.4.1): it is what keeps the positional form unambiguous and anchors the
	// labeled form (minimal counts=edges:0). Distance groups with 0 count are
	// omitted, but edges is not.
	sections = append(sections, fmt.Sprintf("edges:%d", enc.edgeCount))

	symbolCount := enc.nextID
	var countsStr string
	if enc.opts.LabeledTrailerCounts {
		// Labeled form (SPEC §8.4.1): label:count per entry.
		countsStr = strings.Join(sections, ",")
	} else {
		// Positional form (default): values only.
		counts := make([]string, 0, len(sections))
		for _, s := range sections {
			parts := strings.SplitN(s, ":", 2)
			if len(parts) == 2 {
				counts = append(counts, parts[1])
			}
		}
		countsStr = strings.Join(counts, ",")
	}
	fmt.Fprintf(enc.w, "##! summary symbols=%d edges=%d counts=%s\n",
		symbolCount, enc.edgeCount, countsStr)

	return nil
}

// SymbolCount returns the number of symbols written so far.
func (enc *StreamEncoder) SymbolCount() int {
	enc.mu.Lock()
	defer enc.mu.Unlock()
	return enc.nextID
}

// EdgeCount returns the number of edges written so far.
func (enc *StreamEncoder) EdgeCount() int {
	enc.mu.Lock()
	defer enc.mu.Unlock()
	return enc.edgeCount
}
