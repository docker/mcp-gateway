package cli

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/policy"
)

// ClientForCLI returns a policy client for CLI usage.
func ClientForCLI(ctx context.Context) policy.Client {
	client := policy.NewDefaultClient(ctx)
	if _, ok := client.(policy.NoopClient); ok {
		return nil
	}
	return client
}

// DecisionForRequest evaluates policy and returns a decision when denied.
func DecisionForRequest(
	ctx context.Context,
	client policy.Client,
	req policy.Request,
) *policy.Decision {
	if client == nil {
		return nil
	}

	decision, err := client.Evaluate(ctx, req)
	if err != nil {
		return &policy.Decision{
			Allowed: false,
			Error:   err.Error(),
		}
	}
	if decision.Allowed {
		return nil
	}
	return &decision
}

// NormalizeBatchDecisions ensures there is a decision for each request.
func NormalizeBatchDecisions(
	reqs []policy.Request,
	decisions []policy.Decision,
	evalErr error,
) ([]policy.Decision, error) {
	if evalErr == nil && len(decisions) == len(reqs) {
		return decisions, nil
	}

	if evalErr == nil {
		evalErr = fmt.Errorf(
			"batch policy check returned %d decisions for %d requests",
			len(decisions),
			len(reqs),
		)
	}

	normalized := make([]policy.Decision, len(reqs))
	errMsg := evalErr.Error()
	for i := range normalized {
		normalized[i] = policy.Decision{Allowed: false, Error: errMsg}
	}
	return normalized, evalErr
}

// StatusLabel returns a policy status label for human output.
func StatusLabel(decision *policy.Decision) string {
	if decision == nil || decision.Allowed {
		return "Allowed"
	}
	if decision.Error != "" {
		return "Error"
	}
	return "Blocked"
}

// StatusMessage returns a policy status message for human output.
func StatusMessage(decision *policy.Decision) string {
	if decision == nil || decision.Allowed {
		return "Allowed"
	}
	if decision.Error != "" {
		return fmt.Sprintf("Error (%s)", decision.Error)
	}
	if decision.Reason != "" {
		return fmt.Sprintf("Blocked (%s)", decision.Reason)
	}
	return "Blocked"
}
