package gcf

import (
	"fmt"
	"sync"
)

// Session tracks symbols that have been transmitted to a client, enabling
// subsequent responses to reference them by ID without full retransmission.
// This makes multi-call workflows progressively cheaper.
//
// Thread-safe: multiple tool handlers may encode concurrently within a session.
type Session struct {
	mu      sync.Mutex
	symbols map[string]int // qualified_name -> global session ID
	nextID  int
}

// NewSession creates a new empty session.
func NewSession() *Session {
	return &Session{
		symbols: make(map[string]int),
	}
}

// Transmitted returns true if the symbol has been sent in a previous response.
func (s *Session) Transmitted(qname string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.symbols[qname]
	return ok
}

// GetID returns the session-global ID for a previously transmitted symbol.
// Returns -1 if not found.
func (s *Session) GetID(qname string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.symbols[qname]
	if !ok {
		return -1
	}
	return id
}

// Record marks symbols as transmitted and assigns session-global IDs.
// Call this after a successful encode to register newly-sent symbols.
func (s *Session) Record(symbols []Symbol) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sym := range symbols {
		if _, exists := s.symbols[sym.QualifiedName]; !exists {
			s.symbols[sym.QualifiedName] = s.nextID
			s.nextID++
		}
	}
}

// nextSessionID returns the next available session ID without consuming it.
func (s *Session) nextSessionID() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nextID
}

// Size returns the number of symbols tracked in this session.
func (s *Session) Size() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.symbols)
}

// Reset clears the session state.
func (s *Session) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.symbols = make(map[string]int)
	s.nextID = 0
}

// EncodeWithSession encodes a payload using GCF with session deduplication.
// Symbols that were already transmitted in prior responses are emitted as
// bare references (`@N  # previously transmitted`) instead of full declarations.
// After encoding, newly-sent symbols are recorded in the session.
func EncodeWithSession(p *Payload, sess *Session) string {
	if sess == nil {
		return Encode(p)
	}

	// Partition symbols into new (need full declaration) and known (bare ref).
	type symbolEntry struct {
		symbol Symbol
		isNew  bool
	}

	entries := make([]symbolEntry, len(p.Symbols))
	for i, s := range p.Symbols {
		entries[i] = symbolEntry{
			symbol: s,
			isNew:  !sess.Transmitted(s.QualifiedName),
		}
	}

	var b stringBuilder
	// Header with session=true marker.
	b.sprintf("GCF profile=graph tool=%s", p.Tool)
	if p.TokenBudget > 0 {
		b.sprintf(" budget=%d", p.TokenBudget)
	}
	if p.TokensUsed > 0 {
		b.sprintf(" tokens=%d", p.TokensUsed)
	}
	b.sprintf(" symbols=%d", len(p.Symbols))
	if len(p.Edges) > 0 {
		b.sprintf(" edges=%d", len(p.Edges))
	}
	b.sprintf(" session=true")
	if p.PackRoot != "" {
		b.sprintf(" pack_root=%s", p.PackRoot)
	}
	b.writeByte('\n')

	// Build ID mapping using session-stable IDs.
	// Known symbols keep their existing session ID.
	// New symbols get the next available session ID.
	localIndex := make(map[string]int, len(p.Symbols))
	for _, e := range entries {
		if !e.isNew {
			localIndex[e.symbol.QualifiedName] = sess.GetID(e.symbol.QualifiedName)
		}
	}
	nextNew := sess.nextSessionID()
	for _, e := range entries {
		if e.isNew {
			localIndex[e.symbol.QualifiedName] = nextNew
			nextNew++
		}
	}

	// Group by distance.
	groups := groupByDistance(p.Symbols)
	groupNames := []string{"targets", "related", "extended"}

	for _, g := range groups {
		if len(g.symbols) == 0 {
			continue
		}
		name := "targets"
		if g.distance < len(groupNames) {
			name = groupNames[g.distance]
		} else {
			b.sprintf("## distance_%d", g.distance)
			b.writeByte('\n')
			name = ""
		}
		if name != "" {
			b.writeString("## ")
			b.writeString(name)
			b.writeByte('\n')
		}

		for _, s := range g.symbols {
			idx := localIndex[s.QualifiedName]
			if sess.Transmitted(s.QualifiedName) {
				// Bare reference: symbol was sent in a prior response.
				b.sprintf("@%d  # previously transmitted", idx)
			} else {
				// Full declaration.
				kind := KindAbbrev[s.Kind]
				if kind == "" {
					kind = s.Kind
				}
				b.sprintf("@%d %s %s %.2f %s", idx, kind, s.QualifiedName, s.Score, s.Provenance)
			}
			b.writeByte('\n')
		}
	}

	// Edges section.
	if len(p.Edges) > 0 {
		b.sprintf("## edges [%d]\n", len(p.Edges))
		for _, e := range p.Edges {
			srcIdx, srcOk := localIndex[e.Source]
			tgtIdx, tgtOk := localIndex[e.Target]
			if !srcOk || !tgtOk {
				continue
			}
			b.sprintf("@%d<@%d %s", tgtIdx, srcIdx, e.EdgeType)
			if e.Status != "" && e.Status != "unchanged" {
				b.writeString(" ")
				b.writeString(e.Status)
			}
			b.writeByte('\n')
		}
	}

	// Record all new symbols in the session.
	var newSymbols []Symbol
	for _, e := range entries {
		if e.isNew {
			newSymbols = append(newSymbols, e.symbol)
		}
	}
	sess.Record(newSymbols)

	return b.String()
}

// stringBuilder is a thin wrapper to keep the encoder readable.
type stringBuilder struct {
	buf []byte
}

func (b *stringBuilder) sprintf(format string, args ...any) {
	b.buf = append(b.buf, fmt.Sprintf(format, args...)...)
}

func (b *stringBuilder) writeString(s string) {
	b.buf = append(b.buf, s...)
}

func (b *stringBuilder) writeByte(c byte) {
	b.buf = append(b.buf, c)
}

func (b *stringBuilder) String() string {
	return string(b.buf)
}
