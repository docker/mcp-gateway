package gcf

// GenericDeltaSession is a producer-side helper that manages the re-anchor
// cadence for a stream of generic-profile updates (SPEC Section 10a.8, which is
// non-normative producer policy). It is thin sugar over the primitives: each
// Next emits either a compact delta or, on its chosen cadence, a full re-anchor
// (the spec's "full" outcome), updating its held base. It introduces NO new wire
// syntax — every payload it emits is exactly what EncodeGenericFull or
// EncodeGenericDelta produce, and the decoder accepts them cadence-agnostically.
// N and the size guard are the helper's knobs; they are never wire fields.

// ReanchorMode selects the session's cadence policy.
type ReanchorMode int

const (
	// ReanchorFixedN re-anchors every N turns.
	ReanchorFixedN ReanchorMode = iota
	// ReanchorSizeGuard re-anchors once the cumulative delta since the last
	// anchor reaches the current full payload's size (size-adaptive).
	ReanchorSizeGuard
)

// DefaultReanchorN is the working default cadence for FixedN (SPEC Section 10a.8).
const DefaultReanchorN = 15

// ReanchorPolicy selects when a GenericDeltaSession re-anchors. Construct it with
// FixedN or SizeGuard.
type ReanchorPolicy struct {
	Mode ReanchorMode
	N    int // turns between anchors; FixedN only
}

// FixedN re-anchors every n turns. n <= 0 falls back to DefaultReanchorN.
func FixedN(n int) ReanchorPolicy {
	if n <= 0 {
		n = DefaultReanchorN
	}
	return ReanchorPolicy{Mode: ReanchorFixedN, N: n}
}

// SizeGuard re-anchors once the cumulative delta bytes since the last anchor
// reach the current full payload's byte size: it re-anchors more under heavy
// churn, rarely under light churn, and bounds the delta spent between anchors to
// about one full payload. Production-recommended.
func SizeGuard() ReanchorPolicy {
	return ReanchorPolicy{Mode: ReanchorSizeGuard}
}

// GenericDeltaSession holds the current base and re-anchor state for a producer
// loop. Not safe for concurrent use.
type GenericDeltaSession struct {
	base   GenericSet
	tool   string
	policy ReanchorPolicy
	turn   int
	cum    int // cumulative delta bytes since the last anchor
}

// NewGenericDeltaSession starts a session anchored on base. Call CurrentFull to
// get the initial full payload to transmit, then Next for each subsequent state.
func NewGenericDeltaSession(base GenericSet, tool string, policy ReanchorPolicy) *GenericDeltaSession {
	if policy.Mode == ReanchorFixedN && policy.N <= 0 {
		policy.N = DefaultReanchorN
	}
	return &GenericDeltaSession{base: base, tool: tool, policy: policy}
}

// CurrentFull returns the full payload for the current base (EncodeGenericFull).
// Send this first to establish the base; it is also a valid manual re-anchor.
func (s *GenericDeltaSession) CurrentFull() string {
	return EncodeGenericFull(s.base, s.tool)
}

// Turn returns the number of Next calls so far (the initial full is turn 0).
func (s *GenericDeltaSession) Turn() int { return s.turn }

// Next advances the session by one turn to next, returning the wire to transmit
// and whether it is a full re-anchor (true) or a delta (false). A schema change
// forces a full (Section 10a.7). The held base becomes next either way. The wire
// is byte-identical to calling EncodeGenericFull / EncodeGenericDelta directly.
func (s *GenericDeltaSession) Next(next GenericSet) (string, bool, error) {
	s.turn++

	// Schema change (or a fresh key) cannot be expressed as a delta -> full.
	if next.Key != s.base.Key || !sameStrings(s.base.Fields, next.Fields) {
		return s.reanchor(next), true, nil
	}

	d, err := DiffGenericSets(s.base, next)
	if err != nil {
		return "", false, err
	}
	deltaWire := EncodeGenericDelta(d)

	var reanchor bool
	switch s.policy.Mode {
	case ReanchorSizeGuard:
		reanchor = s.cum+len(deltaWire) >= len(EncodeGenericFull(next, s.tool))
	default: // ReanchorFixedN
		reanchor = s.turn%s.policy.N == 0
	}

	if reanchor {
		return s.reanchor(next), true, nil
	}
	s.base = next
	s.cum += len(deltaWire)
	return deltaWire, false, nil
}

// reanchor emits a full payload for next, advances the base, and resets the
// cumulative-delta counter.
func (s *GenericDeltaSession) reanchor(next GenericSet) string {
	wire := EncodeGenericFull(next, s.tool)
	s.base = next
	s.cum = 0
	return wire
}
