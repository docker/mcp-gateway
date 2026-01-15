package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/policy"
)

// fakeClient provides policy decisions for tests.
type fakeClient struct {
	decision policy.Decision
	err      error
}

// Evaluate returns the configured decision or error.
func (c fakeClient) Evaluate(_ context.Context, _ policy.Request) (policy.Decision, error) {
	if c.err != nil {
		return policy.Decision{}, c.err
	}
	return c.decision, nil
}

// EvaluateBatch returns decisions for all requests.
func (c fakeClient) EvaluateBatch(ctx context.Context, reqs []policy.Request) ([]policy.Decision, error) {
	decisions := make([]policy.Decision, len(reqs))
	for i, req := range reqs {
		decisions[i], _ = c.Evaluate(ctx, req)
	}
	return decisions, nil
}

// TestDecisionForRequestNilClient verifies nil clients return nil decisions.
func TestDecisionForRequestNilClient(t *testing.T) {
	t.Helper()

	decision := DecisionForRequest(t.Context(), nil, policy.Request{})
	require.Nil(t, decision)
}

// TestDecisionForRequestAllowed verifies allowed decisions return nil.
func TestDecisionForRequestAllowed(t *testing.T) {
	t.Helper()

	client := fakeClient{decision: policy.Decision{Allowed: true}}
	decision := DecisionForRequest(t.Context(), client, policy.Request{})
	require.Nil(t, decision)
}

// TestDecisionForRequestDenied verifies denied decisions are returned.
func TestDecisionForRequestDenied(t *testing.T) {
	t.Helper()

	client := fakeClient{decision: policy.Decision{Allowed: false, Reason: "blocked"}}
	decision := DecisionForRequest(t.Context(), client, policy.Request{})
	require.NotNil(t, decision)
	require.False(t, decision.Allowed)
	require.Equal(t, "blocked", decision.Reason)
}

// TestDecisionForRequestError verifies errors are returned as denied decisions.
func TestDecisionForRequestError(t *testing.T) {
	t.Helper()

	client := fakeClient{err: context.Canceled}
	decision := DecisionForRequest(t.Context(), client, policy.Request{})
	require.NotNil(t, decision)
	require.False(t, decision.Allowed)
	require.Equal(t, context.Canceled.Error(), decision.Error)
}

// TestStatusLabelAllowed verifies allowed status labels.
func TestStatusLabelAllowed(t *testing.T) {
	t.Helper()

	require.Equal(t, "Allowed", StatusLabel(nil))
	require.Equal(t, "Allowed", StatusLabel(&policy.Decision{Allowed: true}))
}

// TestStatusLabelBlocked verifies blocked status labels.
func TestStatusLabelBlocked(t *testing.T) {
	t.Helper()

	require.Equal(t, "Blocked", StatusLabel(&policy.Decision{Allowed: false}))
}

// TestStatusLabelError verifies error status labels.
func TestStatusLabelError(t *testing.T) {
	t.Helper()

	require.Equal(t, "Error", StatusLabel(&policy.Decision{Allowed: false, Error: "boom"}))
}

// TestStatusMessageAllowed verifies allowed status messages.
func TestStatusMessageAllowed(t *testing.T) {
	t.Helper()

	require.Equal(t, "Allowed", StatusMessage(nil))
	require.Equal(t, "Allowed", StatusMessage(&policy.Decision{Allowed: true}))
}

// TestStatusMessageBlocked verifies blocked status messages.
func TestStatusMessageBlocked(t *testing.T) {
	t.Helper()

	require.Equal(t, "Blocked", StatusMessage(&policy.Decision{Allowed: false}))
	require.Equal(t, "Blocked (nope)", StatusMessage(&policy.Decision{Allowed: false, Reason: "nope"}))
}

// TestStatusMessageError verifies error status messages.
func TestStatusMessageError(t *testing.T) {
	t.Helper()

	require.Equal(t, "Error (boom)", StatusMessage(&policy.Decision{Allowed: false, Error: "boom"}))
}
