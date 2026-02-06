package policy

// DecisionForOutput returns nil for allowed decisions so policy fields are
// omitted from JSON/YAML output, and returns the decision for deny or error.
func DecisionForOutput(dec Decision) *Decision {
	if dec.Allowed {
		return nil
	}
	return &dec
}
